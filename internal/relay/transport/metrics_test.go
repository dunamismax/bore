package transport

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dunamismax/bore/internal/relay/metrics"
	"github.com/dunamismax/bore/internal/relay/ratelimit"
	"github.com/dunamismax/bore/internal/relay/room"
	"nhooyr.io/websocket"
)

func TestMetrics_Endpoint(t *testing.T) {
	_, ts := testServer(t)

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /metrics: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var snap metrics.Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		t.Fatalf("decode /metrics: %v", err)
	}

	// Fresh server should have zero counters.
	if snap.RoomsCreated != 0 {
		t.Errorf("RoomsCreated = %d, want 0", snap.RoomsCreated)
	}
	if snap.ActiveWSConnections != 0 {
		t.Errorf("ActiveWSConnections = %d, want 0", snap.ActiveWSConnections)
	}
}

func TestMetrics_IncrementOnTransfer(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()
	receiver := dialReceiver(t, ctx, ts, roomID)
	defer receiver.CloseNow()

	// Send a frame.
	if err := sender.Write(ctx, websocket.MessageBinary, []byte("payload")); err != nil {
		t.Fatalf("sender write: %v", err)
	}
	_, _, err := receiver.Read(ctx)
	if err != nil {
		t.Fatalf("receiver read: %v", err)
	}

	// Check metrics.
	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	var snap metrics.Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		t.Fatalf("decode /metrics: %v", err)
	}

	if snap.RoomsCreated < 1 {
		t.Errorf("RoomsCreated = %d, want >= 1", snap.RoomsCreated)
	}
	if snap.RoomsJoined < 1 {
		t.Errorf("RoomsJoined = %d, want >= 1", snap.RoomsJoined)
	}
	if snap.TotalWSConnections < 2 {
		t.Errorf("TotalWSConnections = %d, want >= 2", snap.TotalWSConnections)
	}
	if snap.FramesRelayed < 1 {
		t.Errorf("FramesRelayed = %d, want >= 1", snap.FramesRelayed)
	}
	if snap.BytesRelayed < 7 {
		t.Errorf("BytesRelayed = %d, want >= 7", snap.BytesRelayed)
	}

	sender.Close(websocket.StatusNormalClosure, "done")
}

// testRateLimitedServer creates a server with tight rate limits for testing.
func testRateLimitedServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	reg := room.NewRegistry(room.DefaultRegistryConfig())
	ctx, cancel := context.WithCancel(context.Background())
	reg.RunReaper(ctx)
	t.Cleanup(cancel)

	cfg := ServerConfig{
		Registry: reg,
		WSRateLimit: ratelimit.Config{
			Rate:   2,
			Window: time.Minute,
		},
		SignalRateLimit: ratelimit.Config{
			Rate:   2,
			Window: time.Minute,
		},
	}
	srv := NewServer(cfg)
	ts := httptest.NewUnstartedServer(srv.Handler())
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ts.Listener = listener
	ts.Start()
	t.Cleanup(ts.Close)
	return srv, ts
}

func TestRateLimit_WSEndpoint(t *testing.T) {
	_, ts := testRateLimitedServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First two connections should succeed (rate = 2).
	conn1, roomID1 := dialSender(t, ctx, ts)
	defer conn1.CloseNow()
	_ = roomID1

	conn2, roomID2 := dialSender(t, ctx, ts)
	defer conn2.CloseNow()
	_ = roomID2

	// Third connection should be rate limited.
	_, _, err := websocket.Dial(ctx, wsURL(ts, "/ws"), nil)
	if err == nil {
		t.Fatal("expected rate limit error on 3rd /ws connection")
	}
}

func TestRateLimit_SignalEndpoint(t *testing.T) {
	_, ts := testRateLimitedServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a room so we have a valid room ID.
	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()

	// First two signal connections should work (rate = 2).
	sigConn1, _, err := websocket.Dial(ctx, wsURL(ts, "/signal?room="+roomID+"&role=sender"), nil)
	if err != nil {
		t.Fatalf("first signal dial: %v", err)
	}
	defer sigConn1.CloseNow()

	sigConn2, _, err := websocket.Dial(ctx, wsURL(ts, "/signal?room="+roomID+"&role=receiver"), nil)
	if err != nil {
		t.Fatalf("second signal dial: %v", err)
	}
	defer sigConn2.CloseNow()

	// Third signal connection should be rate limited.
	resp, err := http.Get(ts.URL + "/signal?room=" + roomID + "&role=sender")
	if err != nil {
		t.Fatalf("GET /signal: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", resp.StatusCode)
	}
}

func TestMetrics_RateLimitHitsTracked(t *testing.T) {
	_, ts := testRateLimitedServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Exhaust the WS rate limit.
	for range 2 {
		c, _ := dialSender(t, ctx, ts)
		defer c.CloseNow()
	}

	// This one gets rate limited.
	websocket.Dial(ctx, wsURL(ts, "/ws"), nil)

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	var snap metrics.Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		t.Fatalf("decode /metrics: %v", err)
	}

	if snap.RateLimitHits < 1 {
		t.Errorf("RateLimitHits = %d, want >= 1", snap.RateLimitHits)
	}
}
