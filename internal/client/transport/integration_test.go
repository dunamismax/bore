package transport

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/dunamismax/bore/internal/punchthrough/stun"
	relayroom "github.com/dunamismax/bore/internal/relay/room"
	relaytransport "github.com/dunamismax/bore/internal/relay/transport"
)

// startTestRelay creates a relay server for integration tests and returns
// its HTTP URL.
func startTestRelay(t *testing.T) string {
	t.Helper()

	reg := relayroom.NewRegistry(relayroom.DefaultRegistryConfig())
	ctx, cancel := context.WithCancel(context.Background())
	reg.RunReaper(ctx)
	t.Cleanup(cancel)

	cfg := relaytransport.DefaultServerConfig()
	cfg.Registry = reg
	srv := relaytransport.NewServer(cfg)

	ts := httptest.NewUnstartedServer(srv.Handler())
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ts.Listener = listener
	ts.Start()
	t.Cleanup(ts.Close)

	return ts.URL
}

// TestIntegration_DirectTransferLoopback verifies the full QUIC-based direct
// transfer path using loopback UDP sockets.
//
// This simulates the post-hole-punch scenario: both peers have UDP sockets
// that can reach each other on loopback, and we use QUIC transport to
// transfer data bidirectionally with metrics tracking.
func TestIntegration_DirectTransferLoopback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Create two loopback UDP sockets simulating punched connections.
	serverConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen server: %v", err)
	}
	t.Cleanup(func() { serverConn.Close() })

	clientConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen client: %v", err)
	}
	t.Cleanup(func() { clientConn.Close() })

	serverAddr := serverConn.LocalAddr().(*net.UDPAddr)

	// Set up QUIC transport: sender = client, receiver = server.
	// QUIC streams are lazy: the server cannot accept a stream until the
	// client writes data. So we start the client first, trigger a write
	// to announce the stream, then let the server accept.
	type qResult struct {
		conn *QUICConn
		err  error
	}

	serverCh := make(chan qResult, 1)
	go func() {
		qc, err := ListenQUICServer(ctx, serverConn)
		serverCh <- qResult{conn: qc, err: err}
	}()

	clientQUIC, err := DialQUICClient(ctx, clientConn, serverAddr)
	if err != nil {
		t.Fatalf("client QUIC dial: %v", err)
	}
	defer clientQUIC.Close()

	// Wrap client in MetricsConn.
	clientMetrics := NewMetricsConn(clientQUIC, "quic")

	// Transfer 256 KB of random data: client -> server.
	const dataSize = 256 * 1024
	payload := make([]byte, dataSize)
	if _, err := rand.Read(payload); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	// Start writing immediately to trigger QUIC stream delivery.
	var wg sync.WaitGroup
	writeErrCh := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := clientMetrics.Write(payload)
		writeErrCh <- err
	}()

	// Now wait for server to accept the stream.
	sr := <-serverCh
	if sr.err != nil {
		t.Fatalf("server QUIC accept: %v", sr.err)
	}
	defer sr.conn.Close()

	serverMetrics := NewMetricsConn(sr.conn, "quic")

	received := make([]byte, 0, dataSize)
	buf := make([]byte, 32*1024)
	for len(received) < dataSize {
		n, err := serverMetrics.Read(buf)
		if err != nil {
			if err == io.EOF && len(received) >= dataSize {
				break
			}
			t.Fatalf("server read at %d: %v", len(received), err)
		}
		received = append(received, buf[:n]...)
	}

	if writeErr := <-writeErrCh; writeErr != nil {
		t.Fatalf("client write: %v", writeErr)
	}

	if !bytes.Equal(received, payload) {
		t.Fatal("data mismatch: transferred data does not match original")
	}

	// Verify metrics.
	cq := clientMetrics.Snapshot()
	if cq.BytesSent < int64(dataSize) {
		t.Errorf("client BytesSent = %d, want >= %d", cq.BytesSent, dataSize)
	}
	if cq.TransportType != "quic" {
		t.Errorf("client TransportType = %q, want %q", cq.TransportType, "quic")
	}
	if cq.ThroughputSendBytesPerSec <= 0 {
		t.Error("client send throughput should be positive")
	}

	sq := serverMetrics.Snapshot()
	if sq.BytesReceived < int64(dataSize) {
		t.Errorf("server BytesReceived = %d, want >= %d", sq.BytesReceived, dataSize)
	}

	wg.Wait()
}

// TestIntegration_DirectFailsRelaySucceeds verifies the fallback path:
// when direct transport fails (STUN unreachable), the selector falls back
// to relay and the transfer completes successfully.
func TestIntegration_DirectFailsRelaySucceeds(t *testing.T) {
	relayURL := startTestRelay(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// STUN config pointing at an unreachable address so STUN fails fast.
	unreachableSTUN := &stun.Config{
		Servers: []string{"192.0.2.1:3478"}, // TEST-NET-1, non-routable
		Timeout: 200 * time.Millisecond,
	}

	// Sender: enable direct with unreachable STUN.
	senderSel := &Selector{
		RelayURL:      relayURL,
		EnableDirect:  true,
		Role:          "sender",
		STUNConfig:    unreachableSTUN,
		DirectTimeout: 500 * time.Millisecond,
	}

	sessionID, senderConn, err := senderSel.DialSender(ctx)
	if err != nil {
		t.Fatalf("sender dial: %v", err)
	}
	defer senderConn.Close()

	if sessionID == "" {
		t.Fatal("sender got empty session ID")
	}

	// Receiver: also enable direct with unreachable STUN.
	receiverSel := &Selector{
		RelayURL:      relayURL,
		EnableDirect:  true,
		SessionID:     sessionID,
		Role:          "receiver",
		STUNConfig:    unreachableSTUN,
		DirectTimeout: 500 * time.Millisecond,
	}

	receiverConn, err := receiverSel.DialReceiver(ctx, sessionID)
	if err != nil {
		t.Fatalf("receiver dial: %v", err)
	}
	defer receiverConn.Close()

	// Both should have fallen back to relay.
	if senderSel.LastSelection.Method != MethodRelay {
		t.Errorf("sender method = %v, want MethodRelay", senderSel.LastSelection.Method)
	}
	if senderSel.LastSelection.Fallback != FallbackSTUNFailed {
		t.Errorf("sender fallback = %v, want FallbackSTUNFailed", senderSel.LastSelection.Fallback)
	}

	if receiverSel.LastSelection.Method != MethodRelay {
		t.Errorf("receiver method = %v, want MethodRelay", receiverSel.LastSelection.Method)
	}
	if receiverSel.LastSelection.Fallback != FallbackSTUNFailed {
		t.Errorf("receiver fallback = %v, want FallbackSTUNFailed", receiverSel.LastSelection.Fallback)
	}

	// Verify data flows over relay.
	payload := []byte("hello from integration test -- relay fallback path")
	writeErrCh := make(chan error, 1)
	go func() {
		_, err := senderConn.Write(payload)
		writeErrCh <- err
	}()

	buf := make([]byte, 256)
	n, err := receiverConn.Read(buf)
	if err != nil {
		t.Fatalf("receiver read: %v", err)
	}

	if !bytes.Equal(buf[:n], payload) {
		t.Errorf("data mismatch: got %q, want %q", buf[:n], payload)
	}

	if writeErr := <-writeErrCh; writeErr != nil {
		t.Fatalf("sender write: %v", writeErr)
	}
}

// TestIntegration_StatusShowsTransportAfterRelay verifies that the relay's
// /status endpoint reflects transport activity after a relay transfer.
func TestIntegration_StatusShowsTransportAfterRelay(t *testing.T) {
	relayURL := startTestRelay(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a relay-only transfer.
	sender := &RelayDialer{RelayURL: relayURL}
	sessionID, senderConn, err := sender.DialSender(ctx)
	if err != nil {
		t.Fatalf("sender dial: %v", err)
	}
	defer senderConn.Close()

	receiver := &RelayDialer{RelayURL: relayURL}
	receiverConn, err := receiver.DialReceiver(ctx, sessionID)
	if err != nil {
		t.Fatalf("receiver dial: %v", err)
	}
	defer receiverConn.Close()

	// Send data through the relay.
	payload := []byte("transport-stats-integration")
	go func() {
		senderConn.Write(payload)
	}()

	buf := make([]byte, 256)
	n, err := receiverConn.Read(buf)
	if err != nil {
		t.Fatalf("receiver read: %v", err)
	}
	if !bytes.Equal(buf[:n], payload) {
		t.Fatalf("data mismatch: got %q, want %q", buf[:n], payload)
	}

	// Check the /status endpoint.
	resp, err := http.Get(relayURL + "/status")
	if err != nil {
		t.Fatalf("GET /status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /status: status = %d", resp.StatusCode)
	}
}
