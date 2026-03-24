// Package transport implements the WebSocket relay transport layer.
//
// The server exposes a /ws endpoint for peer connections plus lightweight
// /healthz and /status endpoints for operator visibility. Senders connect
// without a room query parameter to create a new room; the server replies with
// the room ID as the first text message. Receivers connect with
// ?room=ROOM_ID to join an existing room. Once both peers are connected, the
// server relays WebSocket frames bidirectionally without inspection.
package transport

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/dunamismax/bore/internal/relay/room"
	"github.com/dunamismax/bore/internal/relay/webui"
	"nhooyr.io/websocket"
)

// maxMessageSize is the per-message read limit for relayed WebSocket
// connections. 64 MB is generous for any reasonable bore chunk size
// and protects the server from pathological single-message payloads.
const maxMessageSize = 64 << 20 // 64 MB

// Server handles WebSocket connections and relays data between paired peers.
type Server struct {
	registry *room.Registry
	httpSrv  *http.Server
	logger   *slog.Logger
	started  time.Time

	mu      sync.Mutex
	pending map[string]*peerState

	sigMu   sync.Mutex
	signals map[string]*signalState
}

// peerState tracks a room where the sender is waiting for a receiver.
type peerState struct {
	room   *room.Room
	recvCh chan *websocket.Conn // receiver delivers its conn here
	doneCh chan struct{}        // closed when relay finishes
}

// signalState tracks the signaling exchange for a room.
// Each peer connects to /signal?room=ID&role=sender|receiver to exchange
// candidate data for direct transport evaluation.
type signalState struct {
	senderMsg  chan []byte
	receiverMsg chan []byte
}

// ServerConfig configures the relay server.
type ServerConfig struct {
	Addr     string
	Registry *room.Registry
	Logger   *slog.Logger
}

// NewServer creates a new relay server.
func NewServer(cfg ServerConfig) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	s := &Server{
		registry: cfg.Registry,
		logger:   cfg.Logger,
		started:  time.Now(),
		pending:  make(map[string]*peerState),
		signals:  make(map[string]*signalState),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/ws", s.handleWS)
	mux.HandleFunc("/signal", s.handleSignal)
	mux.Handle("/", webui.NewHandler())

	s.httpSrv = &http.Server{
		Addr:    cfg.Addr,
		Handler: mux,
	}

	return s
}

// Handler returns the HTTP handler for use with httptest.
func (s *Server) Handler() http.Handler {
	return s.httpSrv.Handler
}

// ListenAndServe starts the HTTP server on the configured address.
func (s *Server) ListenAndServe() error {
	return s.httpSrv.ListenAndServe()
}

// Serve accepts connections on the given listener.
func (s *Server) Serve(ln net.Listener) error {
	return s.httpSrv.Serve(ln)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}

type healthResponse struct {
	Service string `json:"service"`
	Status  string `json:"status"`
}

type statusResponse struct {
	Service       string       `json:"service"`
	Status        string       `json:"status"`
	UptimeSeconds int64        `json:"uptimeSeconds"`
	Rooms         statusRooms  `json:"rooms"`
	Limits        statusLimits `json:"limits"`
}

type statusRooms struct {
	Total   int `json:"total"`
	Waiting int `json:"waiting"`
	Active  int `json:"active"`
}

type statusLimits struct {
	MaxRooms            int   `json:"maxRooms"`
	RoomTTLSeconds      int64 `json:"roomTTLSeconds"`
	ReapIntervalSeconds int64 `json:"reapIntervalSeconds"`
	MaxMessageSizeBytes int64 `json:"maxMessageSizeBytes"`
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Service: "bore-relay",
		Status:  "ok",
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	snapshot := s.registry.Snapshot()

	writeJSON(w, http.StatusOK, statusResponse{
		Service:       "bore-relay",
		Status:        "ok",
		UptimeSeconds: int64(time.Since(s.started).Seconds()),
		Rooms: statusRooms{
			Total:   snapshot.TotalRooms,
			Waiting: snapshot.WaitingRooms,
			Active:  snapshot.ActiveRooms,
		},
		Limits: statusLimits{
			MaxRooms:            snapshot.MaxRooms,
			RoomTTLSeconds:      int64(snapshot.RoomTTL.Seconds()),
			ReapIntervalSeconds: int64(snapshot.ReapInterval.Seconds()),
			MaxMessageSizeBytes: maxMessageSize,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// handleWS routes WebSocket connections to sender or receiver handlers
// based on the presence of a "room" query parameter.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room")
	if roomID == "" {
		s.handleSender(w, r)
	} else {
		s.handleReceiver(w, r, roomID)
	}
}

// handleSender creates a new room, sends the room ID to the sender as a
// text message, waits for a receiver to join, then relays frames between
// the two peers.
func (s *Server) handleSender(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		s.logger.Error("sender: websocket accept failed", "error", err)
		return
	}
	defer conn.CloseNow()

	conn.SetReadLimit(maxMessageSize)

	// Create a room in the registry. We pass nil for the net.Conn
	// because connection management lives in the transport layer.
	rm, err := s.registry.Create(nil)
	if err != nil {
		s.logger.Error("sender: room creation failed", "error", err)
		conn.Close(websocket.StatusInternalError, "room creation failed")
		return
	}

	// Register as pending so the receiver handler can find us.
	ps := &peerState{
		room:   rm,
		recvCh: make(chan *websocket.Conn, 1),
		doneCh: make(chan struct{}),
	}
	s.mu.Lock()
	s.pending[rm.ID] = ps
	s.mu.Unlock()

	// Cleanup on exit: remove from pending map and registry.
	defer func() {
		s.mu.Lock()
		delete(s.pending, rm.ID)
		s.mu.Unlock()
		s.registry.Remove(rm.ID)
	}()

	// Send room ID to sender as the first text message.
	if err := conn.Write(r.Context(), websocket.MessageText, []byte(rm.ID)); err != nil {
		s.logger.Error("sender: failed to send room ID", "room", rm.ID, "error", err)
		return
	}

	s.logger.Info("sender: waiting for receiver", "room", rm.ID)

	// Wait for receiver to join.
	var recvConn *websocket.Conn
	select {
	case recvConn = <-ps.recvCh:
	case <-r.Context().Done():
		s.logger.Info("sender: disconnected while waiting", "room", rm.ID)
		return
	case <-rm.Done():
		return
	}

	s.logger.Info("relay: starting", "room", rm.ID)

	// Relay frames bidirectionally until one side disconnects.
	if err := Relay(r.Context(), conn, recvConn); err != nil {
		s.logger.Info("relay: ended", "room", rm.ID, "reason", err.Error())
	} else {
		s.logger.Info("relay: completed cleanly", "room", rm.ID)
	}

	// Signal receiver handler that the relay is done.
	close(ps.doneCh)
}

// handleReceiver joins an existing room by ID, then blocks while the
// sender handler relays frames between the two peers.
func (s *Server) handleReceiver(w http.ResponseWriter, r *http.Request, roomID string) {
	// Look up the pending room before accepting the WebSocket upgrade.
	s.mu.Lock()
	ps, ok := s.pending[roomID]
	s.mu.Unlock()
	if !ok {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		s.logger.Error("receiver: websocket accept failed", "room", roomID, "error", err)
		return
	}
	defer conn.CloseNow()

	conn.SetReadLimit(maxMessageSize)

	// Join the room in the registry.
	if _, err := s.registry.Join(roomID, nil); err != nil {
		s.logger.Error("receiver: join failed", "room", roomID, "error", err)
		conn.Close(websocket.StatusPolicyViolation, "room not joinable")
		return
	}

	s.logger.Info("receiver: joined", "room", roomID)

	// Deliver our connection to the sender handler.
	select {
	case ps.recvCh <- conn:
	case <-r.Context().Done():
		return
	}

	// Block until the relay completes. The sender handler owns the relay
	// loop; we just need to keep this handler alive so the HTTP server
	// doesn't close our WebSocket connection.
	select {
	case <-ps.doneCh:
	case <-r.Context().Done():
	}
}

// handleSignal implements the relay-coordinated candidate exchange.
//
// Each peer connects via WebSocket to /signal?room=ROOM_ID&role=sender|receiver
// and sends a single JSON signaling message containing their direct-path
// candidate. The relay buffers both messages and forwards the sender's
// candidate to the receiver and vice versa.
//
// The relay does not inspect the candidate content beyond parsing the JSON
// envelope — it is a passthrough for signaling data.
func (s *Server) handleSignal(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room")
	role := r.URL.Query().Get("role")

	if roomID == "" || (role != "sender" && role != "receiver") {
		http.Error(w, "missing room or invalid role", http.StatusBadRequest)
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		s.logger.Error("signal: websocket accept failed", "room", roomID, "role", role, "error", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	conn.SetReadLimit(64 * 1024)

	// Get or create the signal state for this room.
	s.sigMu.Lock()
	ss, ok := s.signals[roomID]
	if !ok {
		ss = &signalState{
			senderMsg:   make(chan []byte, 1),
			receiverMsg: make(chan []byte, 1),
		}
		s.signals[roomID] = ss
	}
	s.sigMu.Unlock()

	// Cleanup signal state after a timeout.
	defer func() {
		s.sigMu.Lock()
		// Only delete if this is still the same signal state.
		if current, exists := s.signals[roomID]; exists && current == ss {
			delete(s.signals, roomID)
		}
		s.sigMu.Unlock()
	}()

	// Read this peer's candidate message.
	_, data, err := conn.Read(r.Context())
	if err != nil {
		s.logger.Error("signal: read failed", "room", roomID, "role", role, "error", err)
		return
	}

	s.logger.Info("signal: received candidate", "room", roomID, "role", role)

	// Determine which channels to use based on role.
	var myCh, peerCh chan []byte
	if role == "sender" {
		myCh = ss.senderMsg
		peerCh = ss.receiverMsg
	} else {
		myCh = ss.receiverMsg
		peerCh = ss.senderMsg
	}

	// Publish our candidate.
	select {
	case myCh <- data:
	default:
		// Channel full — another connection for the same role/room.
		s.logger.Warn("signal: duplicate role connection", "room", roomID, "role", role)
	}

	// Wait for the peer's candidate.
	select {
	case peerData := <-peerCh:
		// Forward the peer's candidate to this peer.
		if err := conn.Write(r.Context(), websocket.MessageText, peerData); err != nil {
			s.logger.Error("signal: write failed", "room", roomID, "role", role, "error", err)
		}
	case <-r.Context().Done():
		s.logger.Info("signal: peer did not arrive", "room", roomID, "role", role)
	case <-time.After(30 * time.Second):
		s.logger.Info("signal: timeout waiting for peer", "room", roomID, "role", role)
	}
}
