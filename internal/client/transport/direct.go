package transport

import (
	"context"
	"fmt"
	"net"
	"time"
)

// DefaultDirectTimeout is the default timeout for establishing a direct
// UDP connection before falling back to relay.
const DefaultDirectTimeout = 3 * time.Second

// DirectDialer implements [Dialer] for direct peer-to-peer UDP connections.
//
// This is a stub implementation. It establishes a connected UDP socket to
// RemoteAddr but does NOT yet perform NAT hole-punching or relay-coordinated
// signaling. Those pieces will be integrated in later phases.
type DirectDialer struct {
	// RemoteAddr is the target peer's address in "host:port" form.
	RemoteAddr string

	// Timeout is the deadline for establishing the UDP connection.
	// Zero uses [DefaultDirectTimeout].
	Timeout time.Duration
}

// DialSender establishes a direct UDP connection as the sender.
//
// TODO: integrate relay-coordinated signaling to exchange peer addresses
// TODO: integrate NAT hole-punching from internal/punchthrough before dialing
// TODO: implement a reliability layer or framing protocol over UDP
func (d *DirectDialer) DialSender(ctx context.Context) (string, Conn, error) {
	conn, err := d.dialUDP(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("direct dial sender: %w", err)
	}
	// Direct transport has no relay-assigned session ID.
	return "", conn, nil
}

// DialReceiver establishes a direct UDP connection as the receiver.
//
// TODO: integrate relay-coordinated signaling to exchange peer addresses
// TODO: integrate NAT hole-punching from internal/punchthrough before dialing
// TODO: implement a reliability layer or framing protocol over UDP
func (d *DirectDialer) DialReceiver(ctx context.Context, _ string) (Conn, error) {
	conn, err := d.dialUDP(ctx)
	if err != nil {
		return nil, fmt.Errorf("direct dial receiver: %w", err)
	}
	return conn, nil
}

// dialUDP resolves the remote address and creates a connected UDP socket.
func (d *DirectDialer) dialUDP(ctx context.Context) (Conn, error) {
	if d.RemoteAddr == "" {
		return nil, fmt.Errorf("no remote address configured for direct transport")
	}

	timeout := d.Timeout
	if timeout == 0 {
		timeout = DefaultDirectTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// TODO: before dialing, run STUN probe and NAT hole-punching
	// from internal/punchthrough to establish mutual reachability.

	dialer := &net.Dialer{}
	netConn, err := dialer.DialContext(ctx, "udp", d.RemoteAddr)
	if err != nil {
		return nil, fmt.Errorf("udp dial %s: %w", d.RemoteAddr, err)
	}

	return &udpConn{conn: netConn}, nil
}

// udpConn wraps a net.Conn (connected UDP socket) to satisfy [Conn].
//
// TODO: add a reliability/framing layer — raw UDP is not sufficient for
// the Noise handshake or chunked file transfer without retransmission.
type udpConn struct {
	conn net.Conn
}

func (c *udpConn) Read(p []byte) (int, error)  { return c.conn.Read(p) }
func (c *udpConn) Write(p []byte) (int, error) { return c.conn.Write(p) }
func (c *udpConn) Close() error                { return c.conn.Close() }
