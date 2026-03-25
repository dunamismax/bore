package punch

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/dunamismax/bore/internal/punchthrough/stun"
)

// Punch protocol message types. Each message is a fixed 24-byte frame:
//   - bytes 0-3:   magic number (0x50554E43 -- "PUNC")
//   - byte 4:      message type
//   - bytes 5-7:   reserved (zero)
//   - bytes 8-15:  nonce (random, echoed in ACK)
//   - bytes 16-23: timestamp (Unix nanoseconds, for RTT measurement)
const (
	msgPing byte = 0x01 // Initial punch probe
	msgPong byte = 0x02 // Response to a punch probe
	msgAck  byte = 0x03 // Verification acknowledgment

	punchMsgSize = 24
)

var punchMagic = [4]byte{0x50, 0x55, 0x4E, 0x43} // "PUNC"

// SelectStrategy determines the optimal hole-punching strategy based on the
// NAT types of both peers. Returns StrategyNone with ErrUnpunchable if the
// NAT combination cannot be punched.
func SelectStrategy(localNAT, peerNAT stun.NATType) (Strategy, error) {
	// Symmetric + Symmetric is unpunchable without TURN.
	if localNAT == stun.NATSymmetric && peerNAT == stun.NATSymmetric {
		return StrategyNone, ErrUnpunchable
	}

	// Unknown NAT on either side means we can't select a strategy.
	if localNAT == stun.NATUnknown || peerNAT == stun.NATUnknown {
		return StrategyNone, fmt.Errorf("%w: NAT type unknown", ErrUnpunchable)
	}

	// If either side has a Full Cone NAT, the other side can send directly
	// because Full Cone allows unsolicited inbound packets.
	if localNAT == stun.NATFullCone || peerNAT == stun.NATFullCone {
		return StrategyDirectOpen, nil
	}

	// All other cone combinations use simultaneous open.
	// This includes:
	//   - Restricted + Restricted
	//   - Port-Restricted + Port-Restricted
	//   - Restricted + Port-Restricted
	//   - Symmetric + Cone (any cone subtype)
	//     The cone side gets a consistent mapping, so the symmetric side
	//     can send to the cone's known mapped address.
	return StrategySimultaneousOpen, nil
}

// Attempt performs a UDP hole-punch to the specified peer address.
//
// It sends punch probe packets to the peer's public address at regular intervals
// while simultaneously listening for the peer's probe packets. When a probe is
// received, a response is sent to verify bidirectional communication.
//
// The localNAT and peerNAT parameters are used for strategy selection. If the
// NAT combination is unpunchable (both symmetric), Attempt returns immediately
// with ErrUnpunchable.
//
// The conn parameter must be a bound UDP connection (the same one used for STUN
// probing to preserve the NAT binding).
func Attempt(ctx context.Context, conn *net.UDPConn, peerAddr *net.UDPAddr, localNAT, peerNAT stun.NATType, cfg *Config) (*PunchResult, error) {
	if peerAddr == nil {
		return nil, ErrInvalidPeer
	}
	if cfg == nil {
		cfg = &Config{}
	}

	result := &PunchResult{
		PeerAddr:  peerAddr,
		LocalAddr: conn.LocalAddr().(*net.UDPAddr),
	}

	// Select strategy based on NAT types.
	strategy, err := SelectStrategy(localNAT, peerNAT)
	if err != nil {
		result.Strategy = strategy
		return result, err
	}
	result.Strategy = strategy

	// Apply overall timeout.
	timeout := cfg.timeout()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	slog.Debug("punch attempt starting",
		"peer", peerAddr.String(),
		"local_nat", localNAT.String(),
		"peer_nat", peerNAT.String(),
		"strategy", strategy.String(),
		"max_attempts", cfg.maxAttempts(),
		"retry_interval", cfg.retryInterval(),
	)

	// Generate a random nonce for this punch session.
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return result, fmt.Errorf("generate nonce: %w", err)
	}

	// Run the punch loop.
	punchResult, err := runPunchLoop(ctx, conn, peerAddr, nonce, cfg, start)
	if punchResult != nil {
		result.Success = punchResult.Success
		result.Attempts = punchResult.Attempts
		result.RTT = punchResult.RTT
	}
	result.Duration = time.Since(start)

	if result.Success {
		slog.Debug("punch succeeded",
			"peer", peerAddr.String(),
			"attempts", result.Attempts,
			"rtt", result.RTT,
			"duration", result.Duration,
		)
	} else {
		slog.Debug("punch failed",
			"peer", peerAddr.String(),
			"attempts", result.Attempts,
			"duration", result.Duration,
			"error", err,
		)
	}

	return result, err
}

// punchLoopResult carries results from the punch loop goroutines back to Attempt.
type punchLoopResult struct {
	Success  bool
	Attempts int
	RTT      time.Duration
}

// runPunchLoop coordinates the send and receive goroutines for the punch attempt.
func runPunchLoop(ctx context.Context, conn *net.UDPConn, peerAddr *net.UDPAddr, nonce [8]byte, cfg *Config, start time.Time) (*punchLoopResult, error) {
	maxAttempts := cfg.maxAttempts()
	interval := cfg.retryInterval()
	handshakeTimeout := cfg.handshakeTimeout()

	// Channel to signal when a pong (response to our ping) is received.
	pongReceived := make(chan time.Duration, 1) // carries RTT
	// Channel to signal when a ping from the peer is received.
	peerPingReceived := make(chan *net.UDPAddr, 1)
	// Channel for receive errors.
	recvErr := make(chan error, 1)

	// Start receiver goroutine.
	go func() {
		buf := make([]byte, punchMsgSize)
		for {
			// Set a short read deadline so we can check context cancellation.
			if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
				recvErr <- fmt.Errorf("set read deadline: %w", err)
				return
			}

			n, raddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// Check if context is done.
					select {
					case <-ctx.Done():
						return
					default:
						continue
					}
				}
				recvErr <- err
				return
			}

			if n < punchMsgSize {
				continue
			}

			// Validate magic.
			if buf[0] != punchMagic[0] || buf[1] != punchMagic[1] ||
				buf[2] != punchMagic[2] || buf[3] != punchMagic[3] {
				continue
			}

			msgType := buf[4]

			switch msgType {
			case msgPing:
				// Peer sent us a ping -- respond with a pong echoing their nonce and timestamp.
				slog.Debug("punch received ping",
					"from", raddr.String(),
				)
				pongMsg := buildMessage(msgPong, [8]byte(buf[8:16]), binary.BigEndian.Uint64(buf[16:24]))
				if _, err := conn.WriteToUDP(pongMsg, raddr); err != nil {
					slog.Debug("punch pong send failed", "error", err)
				}

				select {
				case peerPingReceived <- raddr:
				default:
				}

			case msgPong:
				// Peer responded to our ping -- measure RTT from the echoed timestamp.
				sentNanos := binary.BigEndian.Uint64(buf[16:24])
				rtt := time.Since(time.Unix(0, int64(sentNanos)))

				slog.Debug("punch received pong",
					"from", raddr.String(),
					"rtt", rtt,
				)

				select {
				case pongReceived <- rtt:
				default:
				}

			case msgAck:
				// Verification ACK received -- punch is fully confirmed.
				slog.Debug("punch received ack",
					"from", raddr.String(),
				)
				sentNanos := binary.BigEndian.Uint64(buf[16:24])
				rtt := time.Since(time.Unix(0, int64(sentNanos)))
				select {
				case pongReceived <- rtt:
				default:
				}
			}
		}
	}()

	// Send punch pings at the retry interval.
	attempts := 0
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Send the first ping immediately.
	attempts++
	pingMsg := buildMessage(msgPing, nonce, uint64(time.Now().UnixNano()))
	if _, err := conn.WriteToUDP(pingMsg, peerAddr); err != nil {
		return &punchLoopResult{Attempts: attempts}, fmt.Errorf("send ping: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return &punchLoopResult{Attempts: attempts}, ErrTimeout

		case err := <-recvErr:
			return &punchLoopResult{Attempts: attempts}, fmt.Errorf("receive: %w", err)

		case rtt := <-pongReceived:
			// Got a pong -- now send an ACK to confirm bidirectional communication.
			ackMsg := buildMessage(msgAck, nonce, uint64(time.Now().UnixNano()))
			if _, err := conn.WriteToUDP(ackMsg, peerAddr); err != nil {
				slog.Debug("punch ack send failed", "error", err)
			}
			return &punchLoopResult{
				Success:  true,
				Attempts: attempts,
				RTT:      rtt,
			}, nil

		case peerAddr := <-peerPingReceived:
			// Got a ping from peer (we already sent pong in the receiver).
			// Wait briefly for our own pong to come back to confirm both directions.
			ackCtx, ackCancel := context.WithTimeout(ctx, handshakeTimeout)

			select {
			case rtt := <-pongReceived:
				ackCancel()
				ackMsg := buildMessage(msgAck, nonce, uint64(time.Now().UnixNano()))
				if _, err := conn.WriteToUDP(ackMsg, peerAddr); err != nil {
					slog.Debug("punch ack send failed", "error", err)
				}
				return &punchLoopResult{
					Success:  true,
					Attempts: attempts,
					RTT:      rtt,
				}, nil
			case <-ackCtx.Done():
				ackCancel()
				// Peer pinged us but we never got our pong back -- continue trying.
				slog.Debug("punch handshake incomplete, continuing",
					"peer", peerAddr.String(),
				)
			}

		case <-ticker.C:
			if attempts >= maxAttempts {
				return &punchLoopResult{Attempts: attempts}, ErrMaxAttempts
			}
			attempts++
			pingMsg := buildMessage(msgPing, nonce, uint64(time.Now().UnixNano()))
			if _, err := conn.WriteToUDP(pingMsg, peerAddr); err != nil {
				slog.Debug("punch ping send failed",
					"attempt", attempts,
					"error", err,
				)
			} else {
				slog.Debug("punch ping sent",
					"attempt", attempts,
					"peer", peerAddr.String(),
				)
			}
		}
	}
}

// buildMessage constructs a punch protocol message.
func buildMessage(msgType byte, nonce [8]byte, timestamp uint64) []byte {
	msg := make([]byte, punchMsgSize)
	copy(msg[0:4], punchMagic[:])
	msg[4] = msgType
	// bytes 5-7 reserved (already zero)
	copy(msg[8:16], nonce[:])
	binary.BigEndian.PutUint64(msg[16:24], timestamp)
	return msg
}
