package transport

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/dunamismax/bore/internal/punchthrough/punch"
)

// TransportMode controls which reliability layer is used for direct transport.
type TransportMode int

const (
	// TransportQUIC uses QUIC over the punched UDP socket (default).
	// Provides production-quality congestion control and flow management.
	TransportQUIC TransportMode = iota

	// TransportReliableUDP uses the custom ReliableConn stop-and-wait protocol.
	// Simpler but limited throughput. Retained as fallback.
	TransportReliableUDP
)

// DefaultDirectTimeout is the default timeout for establishing a direct
// UDP connection before falling back to relay.
const DefaultDirectTimeout = 3 * time.Second

// DirectDialer implements [Dialer] for direct peer-to-peer UDP connections.
//
// When CandidatePair is set and the NAT combination is favorable, it runs
// UDP hole-punching via the punchthrough engine. On success, the resulting
// socket is wrapped in a QUIC transport (default) or [ReliableConn] (legacy)
// to provide the stream semantics required by the Noise handshake and
// transfer engine.
//
// When CandidatePair is nil, it falls back to a simple connected UDP socket
// to RemoteAddr (which will typically fail unless both peers are on the same
// LAN or have public IPs).
type DirectDialer struct {
	// RemoteAddr is the target peer's address in "host:port" form.
	// Used as a fallback when CandidatePair is nil.
	RemoteAddr string

	// CandidatePair holds both peers' discovered candidates from
	// the relay-coordinated signaling exchange.
	CandidatePair *CandidatePair

	// PunchConn is a pre-bound UDP socket to use for hole-punching.
	// Should be the same socket used during STUN probing to preserve
	// NAT bindings. If nil, a new socket is created.
	PunchConn *net.UDPConn

	// Timeout is the deadline for establishing the UDP connection.
	// Zero uses [DefaultDirectTimeout].
	Timeout time.Duration

	// Mode selects the reliability layer: TransportQUIC (default) or
	// TransportReliableUDP (legacy stop-and-wait).
	Mode TransportMode

	// Role is "sender" or "receiver" -- used to determine QUIC client/server
	// assignment. Sender acts as QUIC client; receiver acts as QUIC server.
	Role string
}

// DialSender establishes a direct UDP connection as the sender.
func (d *DirectDialer) DialSender(ctx context.Context) (string, Conn, error) {
	conn, err := d.dialDirect(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("direct dial sender: %w", err)
	}
	// Direct transport has no relay-assigned session ID.
	return "", conn, nil
}

// DialReceiver establishes a direct UDP connection as the receiver.
func (d *DirectDialer) DialReceiver(ctx context.Context, _ string) (Conn, error) {
	conn, err := d.dialDirect(ctx)
	if err != nil {
		return nil, fmt.Errorf("direct dial receiver: %w", err)
	}
	return conn, nil
}

// dialDirect attempts to establish a direct connection, first trying
// hole-punching if a CandidatePair is available, otherwise falling back
// to a simple UDP dial.
func (d *DirectDialer) dialDirect(ctx context.Context) (Conn, error) {
	timeout := d.Timeout
	if timeout == 0 {
		timeout = DefaultDirectTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// If we have a candidate pair, attempt hole-punching.
	if d.CandidatePair != nil {
		return d.dialWithPunch(ctx)
	}

	// Fallback: simple connected UDP socket.
	return d.dialUDP(ctx)
}

// dialWithPunch uses the punchthrough engine to establish a hole-punched
// UDP connection, then wraps it in a ReliableConn.
func (d *DirectDialer) dialWithPunch(ctx context.Context) (Conn, error) {
	pair := d.CandidatePair

	// Validate the remote candidate.
	if err := pair.Remote.Validate(); err != nil {
		return nil, fmt.Errorf("invalid remote candidate: %w", err)
	}

	// Parse the remote's public address.
	peerAddr, err := net.ResolveUDPAddr("udp4", pair.Remote.PublicAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve peer addr: %w", err)
	}

	// Use the pre-bound socket or create one.
	conn := d.PunchConn
	if conn == nil {
		conn, err = net.ListenUDP("udp4", nil)
		if err != nil {
			return nil, fmt.Errorf("bind UDP: %w", err)
		}
	}

	slog.Info("direct: attempting hole-punch",
		"peer", peerAddr.String(),
		"local_nat", pair.Local.NATType.String(),
		"remote_nat", pair.Remote.NATType.String(),
	)

	result, err := punch.Attempt(ctx, conn, peerAddr,
		pair.Local.NATType, pair.Remote.NATType, &punch.Config{
			Timeout:       d.effectiveTimeout(),
			MaxAttempts:   10,
			RetryInterval: 200 * time.Millisecond,
		})
	if err != nil {
		return nil, fmt.Errorf("hole-punch: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("hole-punch did not establish bidirectional communication")
	}

	slog.Info("direct: hole-punch succeeded",
		"peer", peerAddr.String(),
		"rtt", result.RTT,
		"attempts", result.Attempts,
	)

	// Select reliability layer based on transport mode.
	if d.Mode == TransportQUIC {
		slog.Info("direct: establishing QUIC over punched socket",
			"role", d.Role,
			"peer", peerAddr.String(),
		)

		qconn, err := quicConnFromPunchedSocket(ctx, conn, peerAddr, d.Role)
		if err != nil {
			slog.Info("direct: QUIC setup failed, falling back to ReliableConn",
				"error", err,
			)
			// Fall through to ReliableConn as a graceful degradation.
			return d.wrapReliable(conn, peerAddr)
		}
		return qconn, nil
	}

	// Legacy path: ReliableConn stop-and-wait.
	return d.wrapReliable(conn, peerAddr)
}

// wrapReliable creates a connected UDP socket and wraps it in ReliableConn.
func (d *DirectDialer) wrapReliable(conn *net.UDPConn, peerAddr *net.UDPAddr) (Conn, error) {
	connectedConn, err := net.DialUDP("udp4", conn.LocalAddr().(*net.UDPAddr), peerAddr)
	if err != nil {
		return nil, fmt.Errorf("connect UDP after punch: %w", err)
	}
	return NewReliableConn(connectedConn), nil
}

// dialUDP resolves the remote address and creates a connected UDP socket.
// This is the fallback path when no CandidatePair is available.
func (d *DirectDialer) dialUDP(ctx context.Context) (Conn, error) {
	if d.RemoteAddr == "" {
		return nil, fmt.Errorf("no remote address configured for direct transport")
	}

	// For non-punch paths, always use ReliableConn since we don't have the
	// unconnected UDP socket needed for QUIC transport setup.
	dialer := &net.Dialer{}
	netConn, err := dialer.DialContext(ctx, "udp", d.RemoteAddr)
	if err != nil {
		return nil, fmt.Errorf("udp dial %s: %w", d.RemoteAddr, err)
	}

	return NewReliableConn(netConn), nil
}

func (d *DirectDialer) effectiveTimeout() time.Duration {
	if d.Timeout > 0 {
		return d.Timeout
	}
	return DefaultDirectTimeout
}
