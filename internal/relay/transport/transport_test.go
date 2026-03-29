package transport

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dunamismax/bore/internal/relay/room"
	"nhooyr.io/websocket"
)

// testServer creates a relay server backed by an httptest.Server.
func testServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	reg := room.NewRegistry(room.DefaultRegistryConfig())
	ctx, cancel := context.WithCancel(context.Background())
	reg.RunReaper(ctx)
	t.Cleanup(cancel)

	cfg := ServerConfig{
		Registry: reg,
	}
	srv := NewServer(cfg)
	ts := httptest.NewUnstartedServer(srv.Handler())
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp4 127.0.0.1:0: %v", err)
	}
	ts.Listener = listener
	ts.Start()
	t.Cleanup(ts.Close)
	return srv, ts
}

// wsURL converts an httptest.Server URL to a WebSocket URL with optional query params.
func wsURL(ts *httptest.Server, path string) string {
	return "ws" + strings.TrimPrefix(ts.URL, "http") + path
}

func httpGetJSON[T any](t *testing.T, url string) T {
	t.Helper()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status = %d, want %d", url, resp.StatusCode, http.StatusOK)
	}

	var decoded T
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode %s: %v", url, err)
	}
	return decoded
}

func TestRelay_StatusEndpoints(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	health := httpGetJSON[healthResponse](t, ts.URL+"/healthz")
	if health.Service != "bore-relay" || health.Status != "ok" {
		t.Fatalf("health = %+v, want service bore-relay / status ok", health)
	}

	status := httpGetJSON[statusResponse](t, ts.URL+"/status")
	if status.Service != "bore-relay" || status.Status != "ok" {
		t.Fatalf("status = %+v, want service bore-relay / status ok", status)
	}
	if status.Rooms.Total != 0 || status.Rooms.Waiting != 0 || status.Rooms.Active != 0 {
		t.Fatalf("empty relay status = %+v, want zero rooms", status.Rooms)
	}
	if status.Limits.MaxMessageSizeBytes != maxMessageSize {
		t.Fatalf("max message size = %d, want %d", status.Limits.MaxMessageSizeBytes, maxMessageSize)
	}
	// Transport stats should exist and be zero at startup.
	if status.Transport.SignalExchanges != 0 || status.Transport.RoomsRelayed != 0 ||
		status.Transport.BytesRelayed != 0 || status.Transport.FramesRelayed != 0 {
		t.Fatalf("empty relay transport = %+v, want all zeros", status.Transport)
	}

	sender, _ := dialSender(t, ctx, ts)
	defer sender.CloseNow()

	status = httpGetJSON[statusResponse](t, ts.URL+"/status")
	if status.Rooms.Total != 1 || status.Rooms.Waiting != 1 || status.Rooms.Active != 0 {
		t.Fatalf("waiting relay status = %+v, want total=1 waiting=1 active=0", status.Rooms)
	}
}

func TestRelay_WebSurface(t *testing.T) {
	_, ts := testServer(t)

	rootResp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer rootResp.Body.Close()

	if rootResp.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", rootResp.StatusCode, http.StatusOK)
	}

	rootBody, err := io.ReadAll(rootResp.Body)
	if err != nil {
		t.Fatalf("read / body: %v", err)
	}
	rootHTML := string(rootBody)
	if !strings.Contains(rootHTML, `bore relay`) {
		t.Fatalf("GET / body missing relay identification")
	}
	if !strings.Contains(rootHTML, `/status`) {
		t.Fatalf("GET / body missing /status link")
	}
}

func TestRelay_HTTPHeaders(t *testing.T) {
	_, ts := testServer(t)

	tests := []struct {
		path        string
		contentType string
		cspContains []string
	}{
		{
			path:        "/",
			contentType: "text/html",
			cspContains: []string{"default-src 'none'", "style-src 'unsafe-inline'", "frame-ancestors 'none'"},
		},
		{
			path:        "/healthz",
			contentType: "application/json",
			cspContains: []string{"default-src 'none'"},
		},
		{
			path:        "/status",
			contentType: "application/json",
			cspContains: []string{"default-src 'none'"},
		},
		{
			path:        "/metrics",
			contentType: "application/json",
			cspContains: []string{"default-src 'none'"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			resp, err := http.Get(ts.URL + tt.path)
			if err != nil {
				t.Fatalf("GET %s: %v", tt.path, err)
			}
			defer resp.Body.Close()

			if got := resp.Header.Get("Cache-Control"); got != "no-store" {
				t.Fatalf("%s Cache-Control = %q, want %q", tt.path, got, "no-store")
			}
			if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
				t.Fatalf("%s X-Content-Type-Options = %q, want %q", tt.path, got, "nosniff")
			}
			if got := resp.Header.Get("X-Frame-Options"); got != "DENY" {
				t.Fatalf("%s X-Frame-Options = %q, want %q", tt.path, got, "DENY")
			}
			if got := resp.Header.Get("Referrer-Policy"); got != "no-referrer" {
				t.Fatalf("%s Referrer-Policy = %q, want %q", tt.path, got, "no-referrer")
			}
			if got := resp.Header.Get("Content-Type"); !strings.Contains(got, tt.contentType) {
				t.Fatalf("%s Content-Type = %q, want substring %q", tt.path, got, tt.contentType)
			}

			csp := resp.Header.Get("Content-Security-Policy")
			for _, want := range tt.cspContains {
				if !strings.Contains(csp, want) {
					t.Fatalf("%s Content-Security-Policy = %q, want substring %q", tt.path, csp, want)
				}
			}
		})
	}
}

// dialSender connects as a sender and returns the WebSocket conn and the room ID.
func dialSender(t *testing.T, ctx context.Context, ts *httptest.Server) (*websocket.Conn, string) {
	t.Helper()
	conn, _, err := websocket.Dial(ctx, wsURL(ts, "/ws"), nil)
	if err != nil {
		t.Fatalf("sender dial: %v", err)
	}

	// Read the room ID (first text message from server).
	typ, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("sender read room ID: %v", err)
	}
	if typ != websocket.MessageText {
		t.Fatalf("expected text message for room ID, got %v", typ)
	}
	roomID := string(data)
	if len(roomID) == 0 {
		t.Fatal("got empty room ID")
	}
	return conn, roomID
}

// dialReceiver connects as a receiver with the given room ID.
func dialReceiver(t *testing.T, ctx context.Context, ts *httptest.Server, roomID string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.Dial(ctx, wsURL(ts, "/ws?room="+roomID), nil)
	if err != nil {
		t.Fatalf("receiver dial: %v", err)
	}
	return conn
}

func TestRelay_BasicBinaryRoundTrip(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()
	receiver := dialReceiver(t, ctx, ts, roomID)
	defer receiver.CloseNow()

	// Sender writes binary data, receiver reads it.
	want := []byte("hello from sender")
	if err := sender.Write(ctx, websocket.MessageBinary, want); err != nil {
		t.Fatalf("sender write: %v", err)
	}

	typ, got, err := receiver.Read(ctx)
	if err != nil {
		t.Fatalf("receiver read: %v", err)
	}
	if typ != websocket.MessageBinary {
		t.Fatalf("expected binary message, got %v", typ)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("data mismatch: got %q, want %q", got, want)
	}

	// Receiver writes back, sender reads it.
	reply := []byte("hello from receiver")
	if err := receiver.Write(ctx, websocket.MessageBinary, reply); err != nil {
		t.Fatalf("receiver write: %v", err)
	}

	typ, got, err = sender.Read(ctx)
	if err != nil {
		t.Fatalf("sender read: %v", err)
	}
	if typ != websocket.MessageBinary {
		t.Fatalf("expected binary message, got %v", typ)
	}
	if !bytes.Equal(got, reply) {
		t.Fatalf("data mismatch: got %q, want %q", got, reply)
	}

	// Clean close.
	sender.Close(websocket.StatusNormalClosure, "done")
}

func TestRelay_MultipleFrames(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()
	receiver := dialReceiver(t, ctx, ts, roomID)
	defer receiver.CloseNow()

	const numFrames = 50

	// Sender sends multiple frames.
	go func() {
		for i := range numFrames {
			data := []byte{byte(i)}
			if err := sender.Write(ctx, websocket.MessageBinary, data); err != nil {
				return
			}
		}
	}()

	// Receiver reads them all.
	for i := range numFrames {
		_, got, err := receiver.Read(ctx)
		if err != nil {
			t.Fatalf("frame %d read: %v", i, err)
		}
		if len(got) != 1 || got[0] != byte(i) {
			t.Fatalf("frame %d: got %v, want [%d]", i, got, i)
		}
	}

	sender.Close(websocket.StatusNormalClosure, "done")
}

func TestRelay_LargeTransfer(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()
	receiver := dialReceiver(t, ctx, ts, roomID)
	defer receiver.CloseNow()

	// Set read limits high enough for the test.
	sender.SetReadLimit(maxMessageSize)
	receiver.SetReadLimit(maxMessageSize)

	// Send 4 MB of random data in one frame.
	const size = 4 << 20
	want := make([]byte, size)
	if _, err := rand.Read(want); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- sender.Write(ctx, websocket.MessageBinary, want)
	}()

	typ, got, err := receiver.Read(ctx)
	if err != nil {
		t.Fatalf("receiver read: %v", err)
	}
	if typ != websocket.MessageBinary {
		t.Fatalf("expected binary, got %v", typ)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("large transfer: data mismatch")
	}

	if err := <-errCh; err != nil {
		t.Fatalf("sender write: %v", err)
	}

	sender.Close(websocket.StatusNormalClosure, "done")
}

func TestRelay_MultiMBStreaming(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()
	receiver := dialReceiver(t, ctx, ts, roomID)
	defer receiver.CloseNow()

	sender.SetReadLimit(maxMessageSize)
	receiver.SetReadLimit(maxMessageSize)

	// Stream 8 MB in 1 MB chunks.
	const chunkSize = 1 << 20
	const numChunks = 8

	var wg sync.WaitGroup
	wg.Add(1)

	// Generate deterministic data.
	allChunks := make([][]byte, numChunks)
	for i := range numChunks {
		allChunks[i] = make([]byte, chunkSize)
		rand.Read(allChunks[i])
	}

	go func() {
		defer wg.Done()
		for _, chunk := range allChunks {
			if err := sender.Write(ctx, websocket.MessageBinary, chunk); err != nil {
				return
			}
		}
		sender.Close(websocket.StatusNormalClosure, "done")
	}()

	for i := range numChunks {
		_, got, err := receiver.Read(ctx)
		if err != nil {
			t.Fatalf("chunk %d read: %v", i, err)
		}
		if !bytes.Equal(got, allChunks[i]) {
			t.Fatalf("chunk %d: data mismatch", i)
		}
	}

	wg.Wait()
}

func TestRelay_SenderDisconnectMidTransfer(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	receiver := dialReceiver(t, ctx, ts, roomID)
	defer receiver.CloseNow()

	// Sender sends one frame then abruptly disconnects.
	if err := sender.Write(ctx, websocket.MessageBinary, []byte("before-drop")); err != nil {
		t.Fatalf("sender write: %v", err)
	}

	// Read the first frame.
	_, got, err := receiver.Read(ctx)
	if err != nil {
		t.Fatalf("receiver read: %v", err)
	}
	if !bytes.Equal(got, []byte("before-drop")) {
		t.Fatalf("got %q, want %q", got, "before-drop")
	}

	// Abrupt disconnect (not a clean close).
	sender.CloseNow()

	// Receiver should get an error or clean close on next read.
	_, _, err = receiver.Read(ctx)
	if err == nil {
		t.Fatal("expected error on read after sender disconnect")
	}
}

func TestRelay_ReceiverDisconnectMidTransfer(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()
	receiver := dialReceiver(t, ctx, ts, roomID)

	// Exchange one frame successfully.
	if err := sender.Write(ctx, websocket.MessageBinary, []byte("ping")); err != nil {
		t.Fatalf("sender write: %v", err)
	}
	_, _, err := receiver.Read(ctx)
	if err != nil {
		t.Fatalf("receiver read: %v", err)
	}

	// Receiver abruptly disconnects.
	receiver.CloseNow()

	// The sender should observe the relay closing after the receiver drops.
	// Waiting on a read is more reliable than expecting the next write to fail
	// within an arbitrary amount of time because close propagation is
	// asynchronous across the WebSocket/TCP stack.
	closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer closeCancel()

	_, _, err = sender.Read(closeCtx)
	if err == nil {
		t.Fatal("expected sender to observe relay closure after receiver disconnect")
	}
}

func TestRelay_BinaryIntegrity(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()
	receiver := dialReceiver(t, ctx, ts, roomID)
	defer receiver.CloseNow()

	// Test that all byte values survive relay (no UTF-8 mangling, no
	// frame type conversion, etc.).
	all256 := make([]byte, 256)
	for i := range 256 {
		all256[i] = byte(i)
	}

	if err := sender.Write(ctx, websocket.MessageBinary, all256); err != nil {
		t.Fatalf("sender write: %v", err)
	}

	typ, got, err := receiver.Read(ctx)
	if err != nil {
		t.Fatalf("receiver read: %v", err)
	}
	if typ != websocket.MessageBinary {
		t.Fatalf("expected binary, got %v", typ)
	}
	if !bytes.Equal(got, all256) {
		t.Fatal("all-byte-values payload was corrupted during relay")
	}
}

func TestRelay_BidirectionalConcurrent(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()
	receiver := dialReceiver(t, ctx, ts, roomID)
	defer receiver.CloseNow()

	const numMsgs = 30

	var wg sync.WaitGroup
	wg.Add(4)

	// sender→receiver direction
	go func() {
		defer wg.Done()
		for i := range numMsgs {
			sender.Write(ctx, websocket.MessageBinary, []byte{byte(i), 0xAA})
		}
	}()
	go func() {
		defer wg.Done()
		for range numMsgs {
			_, _, err := receiver.Read(ctx)
			if err != nil {
				return
			}
		}
	}()

	// receiver→sender direction
	go func() {
		defer wg.Done()
		for i := range numMsgs {
			receiver.Write(ctx, websocket.MessageBinary, []byte{byte(i), 0xBB})
		}
	}()
	go func() {
		defer wg.Done()
		for range numMsgs {
			_, _, err := sender.Read(ctx)
			if err != nil {
				return
			}
		}
	}()

	wg.Wait()
}

func TestRelay_UnknownRoomID(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to join a room that doesn't exist -- should get an HTTP error before upgrade.
	_, resp, err := websocket.Dial(ctx, wsURL(ts, "/ws?room=nonexistent"), nil)
	if err == nil {
		t.Fatal("expected error for nonexistent room")
	}
	if resp != nil && resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRelay_InvalidRoomID(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, resp, err := websocket.Dial(ctx, wsURL(ts, "/ws?room=bad/id"), nil)
	if err == nil {
		t.Fatal("expected error for invalid room ID")
	}
	if resp != nil && resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRelay_StreamingReaderWriter(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()
	receiver := dialReceiver(t, ctx, ts, roomID)
	defer receiver.CloseNow()

	// Use the streaming Writer/Reader API to verify io.Copy-style
	// relay works correctly.
	go func() {
		w, err := sender.Writer(ctx, websocket.MessageBinary)
		if err != nil {
			return
		}
		// Write in small chunks to a single frame.
		for range 100 {
			w.Write([]byte("chunk"))
		}
		w.Close()
	}()

	typ, reader, err := receiver.Reader(ctx)
	if err != nil {
		t.Fatalf("receiver reader: %v", err)
	}
	if typ != websocket.MessageBinary {
		t.Fatalf("expected binary, got %v", typ)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("receiver readall: %v", err)
	}

	want := bytes.Repeat([]byte("chunk"), 100)
	if !bytes.Equal(data, want) {
		t.Fatalf("streaming relay: got %d bytes, want %d", len(data), len(want))
	}
}

func TestRelay_TransportStatsAfterRelay(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()
	receiver := dialReceiver(t, ctx, ts, roomID)
	defer receiver.CloseNow()

	// Exchange one frame through the relay.
	payload := []byte("transport-stats-test")
	if err := sender.Write(ctx, websocket.MessageBinary, payload); err != nil {
		t.Fatalf("sender write: %v", err)
	}
	_, got, err := receiver.Read(ctx)
	if err != nil {
		t.Fatalf("receiver read: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("data mismatch: got %q, want %q", got, payload)
	}

	// Check transport stats are non-zero.
	status := httpGetJSON[statusResponse](t, ts.URL+"/status")
	if status.Transport.RoomsRelayed < 1 {
		t.Fatalf("expected roomsRelayed >= 1, got %d", status.Transport.RoomsRelayed)
	}
	if status.Transport.BytesRelayed < int64(len(payload)) {
		t.Fatalf("expected bytesRelayed >= %d, got %d", len(payload), status.Transport.BytesRelayed)
	}
	if status.Transport.FramesRelayed < 1 {
		t.Fatalf("expected framesRelayed >= 1, got %d", status.Transport.FramesRelayed)
	}
}

func TestRelay_CleanCloseHandshake(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	receiver := dialReceiver(t, ctx, ts, roomID)

	// Sender initiates clean close.
	sender.Close(websocket.StatusNormalClosure, "done")

	// Receiver should receive a close and not hang forever.
	_, _, err := receiver.Read(ctx)
	if err == nil {
		t.Fatal("expected error after peer closed")
	}

	receiver.CloseNow()
}
