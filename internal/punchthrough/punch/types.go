// Package punch implements a UDP hole-punching engine for NAT traversal.
//
// The engine coordinates simultaneous UDP packet exchange between two peers
// who already know each other's public addresses (from STUN probes). Strategy
// selection is based on the NAT types of both peers. The coordination/signaling
// layer is separate (see pkg/coord).
package punch

import (
	"fmt"
	"net"
	"time"
)

// Strategy represents the hole-punching approach selected based on NAT types.
type Strategy int

const (
	// StrategyNone indicates hole-punching is not viable for the given NAT combination.
	// This occurs when both peers are behind Symmetric NATs.
	StrategyNone Strategy = iota

	// StrategySimultaneousOpen indicates both peers should send UDP packets simultaneously
	// to open NAT bindings in both directions. Used when both peers are behind cone-type NATs
	// or when one side is symmetric and the other is cone-type.
	StrategySimultaneousOpen

	// StrategyDirectOpen indicates one peer has a permissive NAT (Full Cone) that allows
	// unsolicited inbound packets. The other peer can send directly without simultaneous timing.
	StrategyDirectOpen
)

// String returns the human-readable name of the strategy.
func (s Strategy) String() string {
	switch s {
	case StrategySimultaneousOpen:
		return "Simultaneous Open"
	case StrategyDirectOpen:
		return "Direct Open"
	default:
		return "None"
	}
}

// PunchResult holds the outcome of a hole-punch attempt.
type PunchResult struct {
	// Success indicates whether the hole-punch established bidirectional communication.
	Success bool

	// Strategy is the hole-punching approach that was used.
	Strategy Strategy

	// PeerAddr is the peer's public address that was targeted.
	PeerAddr *net.UDPAddr

	// LocalAddr is the local address used for the punch attempt.
	LocalAddr *net.UDPAddr

	// Attempts is the number of punch packets sent before success or timeout.
	Attempts int

	// RTT is the measured round-trip time based on the successful punch-ACK exchange.
	// Zero if the punch did not succeed.
	RTT time.Duration

	// Duration is the total time from start to success or timeout.
	Duration time.Duration
}

// String returns a human-readable summary of the punch result.
func (r *PunchResult) String() string {
	if r.Success {
		return fmt.Sprintf("Punch succeeded (%s)\nPeer: %s\nAttempts: %d\nRTT: %s\nTotal time: %s",
			r.Strategy, r.PeerAddr, r.Attempts,
			r.RTT.Round(time.Microsecond), r.Duration.Round(time.Millisecond))
	}
	return fmt.Sprintf("Punch failed (%s)\nPeer: %s\nAttempts: %d\nTotal time: %s",
		r.Strategy, r.PeerAddr, r.Attempts, r.Duration.Round(time.Millisecond))
}
