// Package transport implements the WebSocket relay transport layer.
//
// The server exposes a /ws endpoint for peer connections plus lightweight
// /healthz, /status, and /metrics endpoints for operator visibility.
// Senders connect without a room query parameter to create a new room;
// the server replies with the room ID as the first text message. Receivers
// connect with ?room=ROOM_ID to join an existing room. Once both peers are
// connected, the server relays WebSocket frames bidirectionally without
// inspection.
package transport

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/dunamismax/bore/internal/relay/metrics"
	"github.com/dunamismax/bore/internal/relay/ratelimit"
	"github.com/dunamismax/bore/internal/relay/room"
	"github.com/dunamismax/bore/internal/relay/webui"
	"github.com/dunamismax/bore/internal/roomid"
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
	counters *metrics.Counters

	// Rate limiters
	wsLimiter     *ratelimit.Limiter
	signalLimiter *ratelimit.Limiter

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
	senderMsg   chan []byte
	receiverMsg chan []byte
}

// ServerConfig configures the relay server.
type ServerConfig struct {
	Addr     string
	Registry *room.Registry
	Logger   *slog.Logger
	Counters *metrics.Counters

	// WSRateLimit controls the per-IP rate limit for /ws connections.
	// Zero values disable rate limiting on /ws.
	WSRateLimit ratelimit.Config

	// SignalRateLimit controls the per-IP rate limit for /signal connections.
	// Zero values disable rate limiting on /signal.
	SignalRateLimit ratelimit.Config

	// ReadTimeout is the maximum duration for reading the entire request.
	// Zero defaults to 30 seconds.
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out writes of the response.
	// Zero defaults to 30 seconds.
	WriteTimeout time.Duration

	// IdleTimeout is the maximum amount of time to wait for the next request.
	// Zero defaults to 120 seconds.
	IdleTimeout time.Duration

	// ReadHeaderTimeout is the amount of time allowed to read request headers.
	// Zero defaults to 10 seconds.
	ReadHeaderTimeout time.Duration

	// MaxHeaderBytes controls the maximum number of bytes the server will
	// read parsing the request header. Zero defaults to 1 MB.
	MaxHeaderBytes int
}

// DefaultServerConfig returns a ServerConfig with production-ready defaults.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Addr: ":8080",
		WSRateLimit: ratelimit.Config{
			Rate:   30,
			Window: time.Minute,
		},
		SignalRateLimit: ratelimit.Config{
			Rate:   30,
			Window: time.Minute,
		},
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
	}
}

// NewServer creates a new relay server.
func NewServer(cfg ServerConfig) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Counters == nil {
		cfg.Counters = metrics.NewCounters()
	}

	// Apply timeout defaults.
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 30 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 30 * time.Second
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = 120 * time.Second
	}
	if cfg.ReadHeaderTimeout == 0 {
		cfg.ReadHeaderTimeout = 10 * time.Second
	}
	if cfg.MaxHeaderBytes == 0 {
		cfg.MaxHeaderBytes = 1 << 20
	}

	s := &Server{
		registry: cfg.Registry,
		logger:   cfg.Logger,
		started:  time.Now(),
		counters: cfg.Counters,
		pending:  make(map[string]*peerState),
		signals:  make(map[string]*signalState),
	}

	// Initialize rate limiters only if configured.
	if cfg.WSRateLimit.Rate > 0 && cfg.WSRateLimit.Window > 0 {
		s.wsLimiter = ratelimit.NewLimiter(cfg.WSRateLimit)
	}
	if cfg.SignalRateLimit.Rate > 0 && cfg.SignalRateLimit.Window > 0 {
		s.signalLimiter = ratelimit.NewLimiter(cfg.SignalRateLimit)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/ws", s.handleWS)
	mux.HandleFunc("/signal", s.handleSignal)
	mux.Handle("/", webui.NewHandler())

	s.httpSrv = &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
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

// Shutdown gracefully shuts down the server and releases rate limiter resources.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.wsLimiter != nil {
		s.wsLimiter.Stop()
	}
	if s.signalLimiter != nil {
		s.signalLimiter.Stop()
	}
	return s.httpSrv.Shutdown(ctx)
}

type healthResponse struct {
	Service string `json:"service"`
	Status  string `json:"status"`
}

type statusResponse struct {
	Service       string          `json:"service"`
	Status        string          `json:"status"`
	UptimeSeconds int64           `json:"uptimeSeconds"`
	Rooms         statusRooms     `json:"rooms"`
	Limits        statusLimits    `json:"limits"`
	Transport     statusTransport `json:"transport"`
}

type statusRooms struct {
	Total   int `json:"total"`
	Waiting int `json:"waiting"`
	Active  int `json:"active"`
}

type statusTransport struct {
	SignalExchanges  int64 `json:"signalExchanges"`
	SignalingStarted int64 `json:"signalingStarted"`
	RoomsRelayed     int64 `json:"roomsRelayed"`
	BytesRelayed     int64 `json:"bytesRelayed"`
	FramesRelayed    int64 `json:"framesRelayed"`
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
	msnap := s.counters.Snapshot()

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
		Transport: statusTransport{
			SignalExchanges:  msnap.SignalExchanges,
			SignalingStarted: msnap.SignalingStarted,
			RoomsRelayed:     msnap.RoomsRelayed,
			BytesRelayed:     msnap.BytesRelayed,
			FramesRelayed:    msnap.FramesRelayed,
		},
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	snap := s.counters.Snapshot()
	writeJSON(w, http.StatusOK, snap)
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
	// Rate limiting.
	if s.wsLimiter != nil {
		ip := ratelimit.ExtractIP(r)
		if !s.wsLimiter.Allow(ip) {
			s.counters.RateLimitHit()
			s.logger.Warn("ws: rate limited", "ip", ip)
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
	}

	roomID := r.URL.Query().Get("room")
	if roomID == "" {
		s.handleSender(w, r)
		return
	}
	if err := roomid.Validate(roomID); err != nil {
		http.Error(w, "invalid room ID", http.StatusBadRequest)
		return
	}
	s.handleReceiver(w, r, roomID)
}

// handleSender creates a new room, sends the room ID to the sender as a
// text message, waits for a receiver to join, then relays frames between
// the two peers.
func (s *Server) handleSender(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		s.logger.Error("sender: websocket accept failed", "error", err)
		s.counters.WSError()
		return
	}
	defer conn.CloseNow()

	s.counters.WSConnect()
	defer s.counters.WSDisconnect()

	conn.SetReadLimit(maxMessageSize)

	// Create a room in the registry. We pass nil for the net.Conn
	// because connection management lives in the transport layer.
	rm, err := s.registry.Create(nil)
	if err != nil {
		s.logger.Error("sender: room creation failed", "error", err)
		conn.Close(websocket.StatusInternalError, "room creation failed")
		return
	}
	s.counters.RoomCreated()

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
	s.counters.RoomRelayed()

	// Relay frames bidirectionally until one side disconnects.
	if err := Relay(r.Context(), conn, recvConn, s.counters); err != nil {
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
		s.counters.WSError()
		return
	}
	defer conn.CloseNow()

	s.counters.WSConnect()
	defer s.counters.WSDisconnect()

	conn.SetReadLimit(maxMessageSize)

	// Join the room in the registry.
	if _, err := s.registry.Join(roomID, nil); err != nil {
		s.logger.Error("receiver: join failed", "room", roomID, "error", err)
		conn.Close(websocket.StatusPolicyViolation, "room not joinable")
		return
	}
	s.counters.RoomJoined()

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
// envelope -- it is a passthrough for signaling data.
func (s *Server) handleSignal(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room")
	role := r.URL.Query().Get("role")

	if roomID == "" || (role != "sender" && role != "receiver") {
		http.Error(w, "missing room or invalid role", http.StatusBadRequest)
		return
	}
	if err := roomid.Validate(roomID); err != nil {
		http.Error(w, "invalid room ID", http.StatusBadRequest)
		return
	}

	// Rate limiting.
	if s.signalLimiter != nil {
		ip := ratelimit.ExtractIP(r)
		if !s.signalLimiter.Allow(ip) {
			s.counters.RateLimitHit()
			s.logger.Warn("signal: rate limited", "ip", ip, "room", roomID, "role", role)
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
	}

	s.mu.Lock()
	_, roomExists := s.pending[roomID]
	s.mu.Unlock()
	if !roomExists {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		s.logger.Error("signal: websocket accept failed", "room", roomID, "role", role, "error", err)
		s.counters.WSError()
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	conn.SetReadLimit(64 * 1024)

	s.counters.SignalingStarted()

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
		s.counters.WSError()
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
		// Channel full -- another connection for the same role/room.
		s.logger.Warn("signal: duplicate role connection", "room", roomID, "role", role)
	}

	// Wait for the peer's candidate.
	select {
	case peerData := <-peerCh:
		// Forward the peer's candidate to this peer.
		if err := conn.Write(r.Context(), websocket.MessageText, peerData); err != nil {
			s.logger.Error("signal: write failed", "room", roomID, "role", role, "error", err)
			s.counters.WSError()
		}
		s.counters.SignalExchange()
	case <-r.Context().Done():
		s.logger.Info("signal: peer did not arrive", "room", roomID, "role", role)
	case <-time.After(30 * time.Second):
		s.logger.Info("signal: timeout waiting for peer", "room", roomID, "role", role)
	}
}
