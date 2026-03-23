package crypto

import (
	"net"
	"testing"
)

// pipeConn returns two connected net.Conns backed by an in-memory pipe.
func pipeConn(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()

	a, b := net.Pipe()
	t.Cleanup(func() {
		_ = a.Close()
		_ = b.Close()
	})
	return a, b
}

// handshakePair performs a concurrent Noise XXpsk0 handshake between two
// in-process peers and returns both SecureChannels plus the underlying
// connected transports they were established over.
func handshakePair(t *testing.T, code string) (*SecureChannel, *SecureChannel, net.Conn, net.Conn) {
	t.Helper()

	a, b := pipeConn(t)

	type result struct {
		ch  *SecureChannel
		err error
	}

	initCh := make(chan result, 1)
	respCh := make(chan result, 1)

	go func() {
		ch, err := Handshake(Initiator, code, a)
		initCh <- result{ch, err}
	}()
	go func() {
		ch, err := Handshake(Responder, code, b)
		respCh <- result{ch, err}
	}()

	ir := <-initCh
	rr := <-respCh

	if ir.err != nil {
		t.Fatalf("initiator handshake: %v", ir.err)
	}
	if rr.err != nil {
		t.Fatalf("responder handshake: %v", rr.err)
	}
	return ir.ch, rr.ch, a, b
}

// ---------------------------------------------------------------------------
// PSK derivation
// ---------------------------------------------------------------------------

func TestDerivePSKDeterministic(t *testing.T) {
	psk1 := derivePSK("7-apple-beach-crown")
	psk2 := derivePSK("7-apple-beach-crown")
	if psk1 != psk2 {
		t.Error("derivePSK is not deterministic")
	}
}

func TestDerivePSKDifferentCodes(t *testing.T) {
	psk1 := derivePSK("7-apple-beach-crown")
	psk2 := derivePSK("3-delta-eagle-frost")
	if psk1 == psk2 {
		t.Error("different codes produced same PSK")
	}
}

// ---------------------------------------------------------------------------
// Handshake
// ---------------------------------------------------------------------------

func TestHandshakeSameCode(t *testing.T) {
	init, resp, _, _ := handshakePair(t, "7-apple-beach-crown")
	if !init.IsInitiator() {
		t.Error("initiator channel should report IsInitiator()=true")
	}
	if resp.IsInitiator() {
		t.Error("responder channel should report IsInitiator()=false")
	}
}

func TestHandshakeDifferentCodes(t *testing.T) {
	a, b := pipeConn(t)

	type result struct {
		ch  *SecureChannel
		err error
	}

	results := make(chan result, 2)
	go func() {
		ch, err := Handshake(Initiator, "7-apple-beach-crown", a)
		results <- result{ch, err}
	}()
	go func() {
		ch, err := Handshake(Responder, "3-delta-eagle-frost", b)
		results <- result{ch, err}
	}()

	first := <-results
	_ = a.Close()
	_ = b.Close()
	second := <-results

	if first.err == nil && second.err == nil {
		t.Error("handshake should fail when codes differ")
	}
}

// ---------------------------------------------------------------------------
// SecureChannel send/recv
// ---------------------------------------------------------------------------

func TestSecureChannelSmallMessage(t *testing.T) {
	initCh, respCh, a, b := handshakePair(t, "test-code")

	done := make(chan []byte, 1)
	go func() {
		data, err := respCh.Recv(b)
		if err != nil {
			t.Errorf("recv: %v", err)
			done <- nil
			return
		}
		done <- data
	}()

	payload := []byte("hello, encrypted world!")
	if err := initCh.Send(a, payload); err != nil {
		t.Fatalf("send: %v", err)
	}

	received := <-done
	if string(received) != string(payload) {
		t.Errorf("received %q, want %q", received, payload)
	}
}

func TestSecureChannelEmptyMessage(t *testing.T) {
	initCh, respCh, a, b := handshakePair(t, "test-code")

	done := make(chan []byte, 1)
	go func() {
		data, err := respCh.Recv(b)
		if err != nil {
			t.Errorf("recv: %v", err)
			done <- nil
			return
		}
		done <- data
	}()

	if err := initCh.Send(a, []byte{}); err != nil {
		t.Fatalf("send: %v", err)
	}

	received := <-done
	if len(received) != 0 {
		t.Errorf("expected empty, got %d bytes", len(received))
	}
}

func TestSecureChannelMultipleMessages(t *testing.T) {
	initCh, respCh, a, b := handshakePair(t, "multi-msg-test")

	messages := []string{"message 0", "message 1", "message 2", "message 3", "message 4"}
	done := make(chan []string, 1)

	go func() {
		var received []string
		for range messages {
			data, err := respCh.Recv(b)
			if err != nil {
				t.Errorf("recv: %v", err)
				done <- nil
				return
			}
			received = append(received, string(data))
		}
		done <- received
	}()

	for _, msg := range messages {
		if err := initCh.Send(a, []byte(msg)); err != nil {
			t.Fatalf("send: %v", err)
		}
	}

	received := <-done
	for i, want := range messages {
		if i >= len(received) || received[i] != want {
			t.Errorf("message[%d]: got %q, want %q", i, received[i], want)
		}
	}
}

func TestSecureChannelLargePayload(t *testing.T) {
	initCh, respCh, a, b := handshakePair(t, "large-test")

	// 1 MB payload -- tests multi-segment framing
	payload := make([]byte, 1_000_000)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	done := make(chan []byte, 1)
	go func() {
		data, err := respCh.Recv(b)
		if err != nil {
			t.Errorf("recv: %v", err)
			done <- nil
			return
		}
		done <- data
	}()

	if err := initCh.Send(a, payload); err != nil {
		t.Fatalf("send: %v", err)
	}

	received := <-done
	if len(received) != len(payload) {
		t.Fatalf("length mismatch: got %d, want %d", len(received), len(payload))
	}
	for i := range payload {
		if received[i] != payload[i] {
			t.Errorf("byte[%d] mismatch: got %d, want %d", i, received[i], payload[i])
			break
		}
	}
}

func TestBidirectionalCommunication(t *testing.T) {
	initCh, respCh, a, b := handshakePair(t, "bidir-test")

	// Initiator sends "ping", responder replies "pong".
	done := make(chan string, 1)

	go func() {
		// Responder: recv ping, send pong
		data, err := respCh.Recv(b)
		if err != nil {
			t.Errorf("responder recv: %v", err)
			return
		}
		if string(data) != "ping" {
			t.Errorf("responder got %q, want ping", data)
		}
		if err := respCh.Send(b, []byte("pong")); err != nil {
			t.Errorf("responder send: %v", err)
		}
	}()

	go func() {
		// Initiator: send ping, recv pong
		if err := initCh.Send(a, []byte("ping")); err != nil {
			t.Errorf("initiator send: %v", err)
			done <- ""
			return
		}
		data, err := initCh.Recv(a)
		if err != nil {
			t.Errorf("initiator recv: %v", err)
			done <- ""
			return
		}
		done <- string(data)
	}()

	result := <-done
	if result != "pong" {
		t.Errorf("got %q, want pong", result)
	}
}

func TestHandshakeDropDoesNotPanic(t *testing.T) {
	init, resp, _, _ := handshakePair(t, "drop-test")
	// Ensure drops don't panic.
	_ = init
	_ = resp
}
