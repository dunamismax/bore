package transport

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// BuildWSURL tests (existing coverage, preserved)
// ---------------------------------------------------------------------------

func TestBuildWSURLHTTP(t *testing.T) {
	u, err := BuildWSURL("http://localhost:8080", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "ws://localhost:8080/ws"
	if u != want {
		t.Errorf("got %q, want %q", u, want)
	}
}

func TestBuildWSURLHTTPS(t *testing.T) {
	u, err := BuildWSURL("https://relay.example.com", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "wss://relay.example.com/ws"
	if u != want {
		t.Errorf("got %q, want %q", u, want)
	}
}

func TestBuildWSURLWithRoom(t *testing.T) {
	u, err := BuildWSURL("http://localhost:8080", "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "ws://localhost:8080/ws?room=abc123"
	if u != want {
		t.Errorf("got %q, want %q", u, want)
	}
}

func TestBuildWSURLWSScheme(t *testing.T) {
	u, err := BuildWSURL("ws://relay.local:9090", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "ws://relay.local:9090/ws"
	if u != want {
		t.Errorf("got %q, want %q", u, want)
	}
}

func TestBuildWSURLWSSScheme(t *testing.T) {
	u, err := BuildWSURL("wss://relay.example.com", "room1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "wss://relay.example.com/ws?room=room1"
	if u != want {
		t.Errorf("got %q, want %q", u, want)
	}
}

func TestBuildWSURLBadScheme(t *testing.T) {
	if _, err := BuildWSURL("ftp://relay.example.com", ""); err == nil {
		t.Error("expected error for ftp:// scheme")
	}
}

func TestBuildWSURLNoHost(t *testing.T) {
	if _, err := BuildWSURL("http://", ""); err == nil {
		t.Error("expected error for URL with no host")
	}
}

func TestBuildWSURLPortPreserved(t *testing.T) {
	u, err := BuildWSURL("http://localhost:3000", "roomX")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "ws://localhost:3000/ws?room=roomX"
	if u != want {
		t.Errorf("got %q, want %q", u, want)
	}
}

// ---------------------------------------------------------------------------
// Interface conformance tests
// ---------------------------------------------------------------------------

// Verify RelayDialer satisfies Dialer at compile time.
var _ Dialer = (*RelayDialer)(nil)

// Verify DirectDialer satisfies Dialer at compile time.
var _ Dialer = (*DirectDialer)(nil)

// Verify Selector satisfies Dialer at compile time.
var _ Dialer = (*Selector)(nil)

// Verify wsConn satisfies Conn at compile time.
var _ Conn = (*wsConn)(nil)

// Verify udpConn satisfies Conn at compile time.
var _ Conn = (*udpConn)(nil)

// ---------------------------------------------------------------------------
// Conn interface behavioral test with a mock
// ---------------------------------------------------------------------------

// mockConn is a simple in-memory Conn for testing transport-agnostic code.
type mockConn struct {
	*bytes.Buffer
	closed bool
}

func newMockConn(data []byte) *mockConn {
	return &mockConn{Buffer: bytes.NewBuffer(data)}
}

func (c *mockConn) Close() error {
	c.closed = true
	return nil
}

// Verify mockConn satisfies Conn.
var _ Conn = (*mockConn)(nil)

func TestConnInterfaceReadWrite(t *testing.T) {
	// Test that any Conn can be used as io.ReadWriter (which is what
	// the crypto and engine layers require).
	var conn Conn = newMockConn(nil)

	payload := []byte("hello bore transport")
	n, err := conn.Write(payload)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("Write: wrote %d, want %d", n, len(payload))
	}

	buf := make([]byte, 64)
	n, err = conn.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(buf[:n], payload) {
		t.Errorf("Read: got %q, want %q", buf[:n], payload)
	}
}

func TestConnSatisfiesReadWriter(t *testing.T) {
	// The crypto layer takes io.ReadWriter. Verify Conn satisfies it.
	var conn Conn = newMockConn([]byte("test"))
	var rw io.ReadWriter = conn // must compile
	_ = rw
}

func TestConnClose(t *testing.T) {
	mc := newMockConn(nil)
	var conn Conn = mc

	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !mc.closed {
		t.Error("Close did not mark connection as closed")
	}
}

// ---------------------------------------------------------------------------
// DirectDialer error paths
// ---------------------------------------------------------------------------

func TestDirectDialerNoAddr(t *testing.T) {
	d := &DirectDialer{}
	_, _, err := d.DialSender(context.Background())
	if err == nil {
		t.Error("expected error for empty remote address")
	}
}

func TestDirectDialerBadAddr(t *testing.T) {
	d := &DirectDialer{RemoteAddr: "not-a-host:99999999"}
	_, _, err := d.DialSender(context.Background())
	if err == nil {
		t.Error("expected error for bad remote address")
	}
}

func TestDirectDialerReceiverNoAddr(t *testing.T) {
	d := &DirectDialer{}
	_, err := d.DialReceiver(context.Background(), "room1")
	if err == nil {
		t.Error("expected error for empty remote address")
	}
}

// ---------------------------------------------------------------------------
// Selector: relay-only path (no direct addr)
// ---------------------------------------------------------------------------

func TestSelectorFallsBackWhenNoDirectAddr(t *testing.T) {
	// With no DirectAddr set, the selector should go straight to relay.
	// We can't test the actual relay connection without a server, but we
	// can verify the selector doesn't panic and propagates the relay error.
	s := &Selector{
		RelayURL:   "http://127.0.0.1:1", // nothing listening
		DirectAddr: "",
	}

	_, _, err := s.DialSender(context.Background())
	if err == nil {
		t.Error("expected error connecting to non-existent relay")
	}

	_, err = s.DialReceiver(context.Background(), "room1")
	if err == nil {
		t.Error("expected error connecting to non-existent relay")
	}
}

// ---------------------------------------------------------------------------
// Selector: LastSelection tracking
// ---------------------------------------------------------------------------

func TestSelectorRecordsFallbackNoDirectAddr(t *testing.T) {
	s := &Selector{
		RelayURL:   "http://127.0.0.1:1",
		DirectAddr: "",
	}

	// DialSender will fail (no relay), but LastSelection should still be set.
	_, _, _ = s.DialSender(context.Background())

	if s.LastSelection.Method != MethodRelay {
		t.Errorf("Method = %v, want MethodRelay", s.LastSelection.Method)
	}
	if s.LastSelection.Fallback != FallbackNoDirectAddr {
		t.Errorf("Fallback = %v, want FallbackNoDirectAddr", s.LastSelection.Fallback)
	}
	if s.LastSelection.DirectErr != nil {
		t.Errorf("DirectErr should be nil when no direct attempt was made, got %v", s.LastSelection.DirectErr)
	}
}

func TestSelectorRecordsFallbackDialFailed(t *testing.T) {
	// Use an address with no host to trigger a DNS/resolution error in
	// the direct dialer. UDP dial to 127.0.0.1 always "succeeds" because
	// UDP is connectionless, so we need a genuinely invalid address.
	s := &Selector{
		RelayURL:      "http://127.0.0.1:1",
		DirectAddr:    "",                     // empty triggers no-addr path
		DirectTimeout: 100 * time.Millisecond,
	}

	// With empty DirectAddr the selector goes straight to relay with
	// FallbackNoDirectAddr. Test the explicit case separately below.
	_, _, _ = s.DialSender(context.Background())

	if s.LastSelection.Method != MethodRelay {
		t.Errorf("Method = %v, want MethodRelay", s.LastSelection.Method)
	}
	if s.LastSelection.Fallback != FallbackNoDirectAddr {
		t.Errorf("Fallback = %v, want FallbackNoDirectAddr", s.LastSelection.Fallback)
	}
}

func TestSelectorRecordsFallbackDialFailedBadAddr(t *testing.T) {
	// Use a malformed address that will actually fail the UDP dial.
	s := &Selector{
		RelayURL:      "http://127.0.0.1:1",
		DirectAddr:    "not-a-host:99999999", // will fail resolution
		DirectTimeout: 100 * time.Millisecond,
	}

	_, _, _ = s.DialSender(context.Background())

	if s.LastSelection.Method != MethodRelay {
		t.Errorf("Method = %v, want MethodRelay", s.LastSelection.Method)
	}
	if s.LastSelection.Fallback != FallbackDialFailed {
		t.Errorf("Fallback = %v, want FallbackDialFailed", s.LastSelection.Fallback)
	}
	if s.LastSelection.DirectErr == nil {
		t.Error("DirectErr should be non-nil after a failed direct attempt")
	}
}

func TestSelectorRecordsReceiverFallbackNoDirectAddr(t *testing.T) {
	s := &Selector{
		RelayURL:   "http://127.0.0.1:1",
		DirectAddr: "",
	}

	_, _ = s.DialReceiver(context.Background(), "room1")

	if s.LastSelection.Method != MethodRelay {
		t.Errorf("Method = %v, want MethodRelay", s.LastSelection.Method)
	}
	if s.LastSelection.Fallback != FallbackNoDirectAddr {
		t.Errorf("Fallback = %v, want FallbackNoDirectAddr", s.LastSelection.Fallback)
	}
}

func TestSelectorRecordsReceiverFallbackDialFailed(t *testing.T) {
	s := &Selector{
		RelayURL:      "http://127.0.0.1:1",
		DirectAddr:    "not-a-host:99999999",
		DirectTimeout: 100 * time.Millisecond,
	}

	_, _ = s.DialReceiver(context.Background(), "room1")

	if s.LastSelection.Method != MethodRelay {
		t.Errorf("Method = %v, want MethodRelay", s.LastSelection.Method)
	}
	if s.LastSelection.Fallback != FallbackDialFailed {
		t.Errorf("Fallback = %v, want FallbackDialFailed", s.LastSelection.Fallback)
	}
	if s.LastSelection.DirectErr == nil {
		t.Error("DirectErr should be non-nil after a failed direct attempt")
	}
}

func TestSelectionResultStringOutput(t *testing.T) {
	r := SelectionResult{Method: MethodRelay, Fallback: FallbackNoDirectAddr}
	s := r.String()
	if s == "" {
		t.Error("expected non-empty string")
	}
	// Should mention both transport and fallback reason.
	if !contains(s, "relay") || !contains(s, "no direct address") {
		t.Errorf("String() = %q, want mentions of relay and fallback reason", s)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
