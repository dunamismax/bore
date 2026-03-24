package punch

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dunamismax/bore/internal/punchthrough/stun"
)

func TestSelectStrategy(t *testing.T) {
	tests := []struct {
		name     string
		localNAT stun.NATType
		peerNAT  stun.NATType
		want     Strategy
		wantErr  error
	}{
		{
			name:     "symmetric+symmetric is unpunchable",
			localNAT: stun.NATSymmetric,
			peerNAT:  stun.NATSymmetric,
			want:     StrategyNone,
			wantErr:  ErrUnpunchable,
		},
		{
			name:     "unknown local NAT is unpunchable",
			localNAT: stun.NATUnknown,
			peerNAT:  stun.NATPortRestrictedCone,
			want:     StrategyNone,
			wantErr:  ErrUnpunchable,
		},
		{
			name:     "unknown peer NAT is unpunchable",
			localNAT: stun.NATPortRestrictedCone,
			peerNAT:  stun.NATUnknown,
			want:     StrategyNone,
			wantErr:  ErrUnpunchable,
		},
		{
			name:     "full cone local allows direct open",
			localNAT: stun.NATFullCone,
			peerNAT:  stun.NATPortRestrictedCone,
			want:     StrategyDirectOpen,
		},
		{
			name:     "full cone peer allows direct open",
			localNAT: stun.NATPortRestrictedCone,
			peerNAT:  stun.NATFullCone,
			want:     StrategyDirectOpen,
		},
		{
			name:     "both full cone allows direct open",
			localNAT: stun.NATFullCone,
			peerNAT:  stun.NATFullCone,
			want:     StrategyDirectOpen,
		},
		{
			name:     "port-restricted+port-restricted uses simultaneous open",
			localNAT: stun.NATPortRestrictedCone,
			peerNAT:  stun.NATPortRestrictedCone,
			want:     StrategySimultaneousOpen,
		},
		{
			name:     "restricted+port-restricted uses simultaneous open",
			localNAT: stun.NATRestrictedCone,
			peerNAT:  stun.NATPortRestrictedCone,
			want:     StrategySimultaneousOpen,
		},
		{
			name:     "symmetric+cone uses simultaneous open",
			localNAT: stun.NATSymmetric,
			peerNAT:  stun.NATPortRestrictedCone,
			want:     StrategySimultaneousOpen,
		},
		{
			name:     "cone+symmetric uses simultaneous open",
			localNAT: stun.NATRestrictedCone,
			peerNAT:  stun.NATSymmetric,
			want:     StrategySimultaneousOpen,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SelectStrategy(tt.localNAT, tt.peerNAT)
			if got != tt.want {
				t.Errorf("SelectStrategy(%v, %v) = %v, want %v", tt.localNAT, tt.peerNAT, got, tt.want)
			}
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("SelectStrategy(%v, %v) error = %v, want %v", tt.localNAT, tt.peerNAT, err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("SelectStrategy(%v, %v) unexpected error: %v", tt.localNAT, tt.peerNAT, err)
			}
		})
	}
}

func TestAttempt_InvalidPeer(t *testing.T) {
	conn := bindLoopback(t)
	defer conn.Close()

	_, err := Attempt(context.Background(), conn, nil, stun.NATPortRestrictedCone, stun.NATPortRestrictedCone, nil)
	if !errors.Is(err, ErrInvalidPeer) {
		t.Errorf("Attempt(nil peer) error = %v, want ErrInvalidPeer", err)
	}
}

func TestAttempt_Unpunchable(t *testing.T) {
	conn := bindLoopback(t)
	defer conn.Close()

	peer := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9999}
	result, err := Attempt(context.Background(), conn, peer, stun.NATSymmetric, stun.NATSymmetric, nil)
	if !errors.Is(err, ErrUnpunchable) {
		t.Errorf("Attempt(symmetric+symmetric) error = %v, want ErrUnpunchable", err)
	}
	if result == nil {
		t.Fatal("result should not be nil even on unpunchable")
	}
	if result.Strategy != StrategyNone {
		t.Errorf("result.Strategy = %v, want StrategyNone", result.Strategy)
	}
}

func TestAttempt_Timeout(t *testing.T) {
	conn := bindLoopback(t)
	defer conn.Close()

	// Point at a port where nobody is listening.
	peer := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 19999}
	cfg := &Config{
		MaxAttempts:   2,
		RetryInterval: 50 * time.Millisecond,
		Timeout:       500 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	result, err := Attempt(ctx, conn, peer, stun.NATPortRestrictedCone, stun.NATPortRestrictedCone, cfg)
	elapsed := time.Since(start)

	if result.Success {
		t.Fatal("expected punch to fail")
	}
	if result.Attempts < 1 {
		t.Errorf("expected at least 1 attempt, got %d", result.Attempts)
	}
	if err == nil {
		t.Error("expected non-nil error")
	}
	// Should complete reasonably quickly with a short timeout.
	if elapsed > 3*time.Second {
		t.Errorf("took too long: %v", elapsed)
	}
}

func TestAttempt_MaxAttemptsExhausted(t *testing.T) {
	conn := bindLoopback(t)
	defer conn.Close()

	// Point at a port where nobody is listening.
	peer := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 19998}
	cfg := &Config{
		MaxAttempts:   3,
		RetryInterval: 50 * time.Millisecond,
		Timeout:       10 * time.Second, // Long timeout so max attempts hits first.
	}

	result, err := Attempt(context.Background(), conn, peer, stun.NATPortRestrictedCone, stun.NATPortRestrictedCone, cfg)

	if result.Success {
		t.Fatal("expected punch to fail")
	}
	if result.Attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", result.Attempts)
	}
	if !errors.Is(err, ErrMaxAttempts) {
		t.Errorf("expected ErrMaxAttempts, got %v", err)
	}
}

func TestAttempt_LoopbackSuccess(t *testing.T) {
	// Simulate a successful hole-punch using two loopback UDP sockets.
	// One goroutine acts as the "peer" responding to pings with pongs.
	conn1 := bindLoopback(t)
	defer conn1.Close()
	conn2 := bindLoopback(t)
	defer conn2.Close()

	peer1Addr := conn1.LocalAddr().(*net.UDPAddr)
	peer2Addr := conn2.LocalAddr().(*net.UDPAddr)

	cfg := &Config{
		MaxAttempts:      10,
		RetryInterval:    50 * time.Millisecond,
		Timeout:          5 * time.Second,
		HandshakeTimeout: 2 * time.Second,
	}

	// Start "peer 2" — the other side doing the same thing.
	peer2Done := make(chan *PunchResult, 1)
	peer2Err := make(chan error, 1)
	go func() {
		result, err := Attempt(context.Background(), conn2, peer1Addr, stun.NATPortRestrictedCone, stun.NATPortRestrictedCone, cfg)
		peer2Done <- result
		peer2Err <- err
	}()

	// "Peer 1" attempts from this side.
	result1, err1 := Attempt(context.Background(), conn1, peer2Addr, stun.NATPortRestrictedCone, stun.NATPortRestrictedCone, cfg)

	// Check peer 1 result.
	if err1 != nil {
		t.Fatalf("peer1 punch error: %v", err1)
	}
	if !result1.Success {
		t.Fatal("peer1 punch should have succeeded")
	}
	if result1.Attempts < 1 {
		t.Errorf("peer1 attempts = %d, want >= 1", result1.Attempts)
	}
	if result1.RTT <= 0 {
		t.Error("peer1 RTT should be positive on success")
	}
	if result1.Strategy != StrategySimultaneousOpen {
		t.Errorf("peer1 strategy = %v, want StrategySimultaneousOpen", result1.Strategy)
	}

	// Check peer 2 result.
	result2 := <-peer2Done
	err2 := <-peer2Err
	if err2 != nil {
		t.Fatalf("peer2 punch error: %v", err2)
	}
	if !result2.Success {
		t.Fatal("peer2 punch should have succeeded")
	}
	if result2.RTT <= 0 {
		t.Error("peer2 RTT should be positive on success")
	}
}

func TestAttempt_ContextCancellation(t *testing.T) {
	conn := bindLoopback(t)
	defer conn.Close()

	peer := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 19997}
	cfg := &Config{
		MaxAttempts:   100, // Many attempts so context cancellation hits first.
		RetryInterval: 50 * time.Millisecond,
		Timeout:       30 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after a short delay.
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	result, err := Attempt(ctx, conn, peer, stun.NATPortRestrictedCone, stun.NATPortRestrictedCone, cfg)
	if result.Success {
		t.Fatal("expected failure on cancelled context")
	}
	if err == nil {
		t.Error("expected non-nil error on cancelled context")
	}
}

func TestBuildMessage(t *testing.T) {
	nonce := [8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	ts := uint64(1711111111000000000)

	msg := buildMessage(msgPing, nonce, ts)

	if len(msg) != punchMsgSize {
		t.Fatalf("message size = %d, want %d", len(msg), punchMsgSize)
	}

	// Check magic.
	if msg[0] != 0x50 || msg[1] != 0x55 || msg[2] != 0x4E || msg[3] != 0x43 {
		t.Errorf("magic = %x, want 50554E43", msg[0:4])
	}

	// Check message type.
	if msg[4] != msgPing {
		t.Errorf("msgType = %d, want %d", msg[4], msgPing)
	}

	// Check reserved bytes are zero.
	if msg[5] != 0 || msg[6] != 0 || msg[7] != 0 {
		t.Errorf("reserved bytes not zero: %x", msg[5:8])
	}

	// Check nonce.
	for i := 0; i < 8; i++ {
		if msg[8+i] != nonce[i] {
			t.Errorf("nonce[%d] = %x, want %x", i, msg[8+i], nonce[i])
		}
	}

	// Check timestamp.
	gotTS := binary.BigEndian.Uint64(msg[16:24])
	if gotTS != ts {
		t.Errorf("timestamp = %d, want %d", gotTS, ts)
	}
}

func TestBuildMessage_AllTypes(t *testing.T) {
	nonce := [8]byte{}
	for _, msgType := range []byte{msgPing, msgPong, msgAck} {
		msg := buildMessage(msgType, nonce, 0)
		if msg[4] != msgType {
			t.Errorf("buildMessage(%d): got type %d", msgType, msg[4])
		}
	}
}

func TestAttempt_NilConfig(t *testing.T) {
	// Verify that Attempt works with a nil config (uses defaults).
	conn := bindLoopback(t)
	defer conn.Close()

	// Will fail (no peer) but should not panic.
	peer := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 19996}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	result, _ := Attempt(ctx, conn, peer, stun.NATPortRestrictedCone, stun.NATPortRestrictedCone, nil)
	if result.Success {
		t.Fatal("expected failure with no peer")
	}
}

// bindLoopback creates a UDP connection bound to localhost on an OS-assigned port.
func bindLoopback(t *testing.T) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("failed to bind loopback UDP: %v", err)
	}
	return conn
}
