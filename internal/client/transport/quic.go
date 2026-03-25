// Package transport -- QUIC-based direct transport.
//
// This file implements a QUIC transport layer over a hole-punched UDP socket.
// QUIC replaces the custom ReliableConn stop-and-wait protocol with
// production-quality reliability, congestion control, and flow management.
//
// The design:
//   - After hole-punching succeeds, one side acts as QUIC server and the
//     other as QUIC client over the same UDP socket.
//   - The sender always acts as the QUIC client; the receiver acts as
//     the QUIC server. This is deterministic based on the bore role.
//   - A self-signed TLS certificate is generated per-session. TLS
//     verification is skipped because bore's Noise XXpsk0 handshake
//     provides the actual authentication and encryption layer above QUIC.
//   - A single QUIC stream carries the Noise handshake and file transfer.
package transport

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"math/big"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

const (
	// quicHandshakeTimeout is the deadline for QUIC connection establishment.
	quicHandshakeTimeout = 5 * time.Second

	// quicIdleTimeout is the maximum time a QUIC connection can be idle
	// before being closed. Set generously for large file transfers.
	quicIdleTimeout = 60 * time.Second

	// quicMaxStreamData is the per-stream flow control window.
	// 16 MB allows large chunks to flow without stalling.
	quicMaxStreamData = 16 * 1024 * 1024

	// quicMaxData is the connection-level flow control window.
	quicMaxData = 32 * 1024 * 1024

	// quicALPN is the ALPN protocol identifier for bore's QUIC transport.
	quicALPN = "bore/1"
)

// QUICConn wraps a QUIC connection and stream to implement the Conn interface
// (io.ReadWriteCloser). It provides the stream semantics required by the
// Noise handshake and transfer engine.
type QUICConn struct {
	conn      *quic.Conn
	stream    *quic.Stream
	transport *quic.Transport // owned transport, closed on Close
	listener  *quic.Listener  // owned listener (server side), closed on Close
	mu        sync.Mutex
	closed    bool
}

// Read implements io.Reader over the QUIC stream.
func (c *QUICConn) Read(p []byte) (int, error) {
	return c.stream.Read(p)
}

// Write implements io.Writer over the QUIC stream.
func (c *QUICConn) Write(p []byte) (int, error) {
	return c.stream.Write(p)
}

// Close tears down the QUIC stream and connection.
func (c *QUICConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true

	// Close the stream first.
	var streamErr error
	if c.stream != nil {
		streamErr = c.stream.Close()
	}

	// Then close the QUIC connection.
	var connErr error
	if c.conn != nil {
		connErr = c.conn.CloseWithError(0, "done")
	}

	// Close listener and transport if owned.
	if c.listener != nil {
		c.listener.Close()
	}
	if c.transport != nil {
		c.transport.Close()
	}

	if streamErr != nil {
		return streamErr
	}
	return connErr
}

// Metrics returns connection quality metrics from the QUIC connection.
func (c *QUICConn) Metrics() QUICMetrics {
	return QUICMetrics{
		// QUIC connection is alive -- basic quality signal.
		Connected: !c.closed,
	}
}

// Verify QUICConn satisfies Conn.
var _ Conn = (*QUICConn)(nil)

// QUICMetrics holds observable connection quality data from a QUIC transport.
type QUICMetrics struct {
	// Connected indicates whether the QUIC connection is still alive.
	Connected bool
}

// DialQUICClient establishes a QUIC connection as the client (sender) over
// a pre-existing UDP socket connected to the peer. The peerAddr is the
// remote peer's address discovered via hole-punching.
func DialQUICClient(ctx context.Context, udpConn net.PacketConn, peerAddr net.Addr) (*QUICConn, error) {
	tlsConf := generateClientTLS()

	quicConf := &quic.Config{
		HandshakeIdleTimeout:           quicHandshakeTimeout,
		MaxIdleTimeout:                 quicIdleTimeout,
		InitialStreamReceiveWindow:     quicMaxStreamData,
		MaxStreamReceiveWindow:         quicMaxStreamData,
		InitialConnectionReceiveWindow: quicMaxData,
		MaxConnectionReceiveWindow:     quicMaxData,
	}

	tr := &quic.Transport{
		Conn: udpConn,
	}

	conn, err := tr.Dial(ctx, peerAddr, tlsConf, quicConf)
	if err != nil {
		tr.Close()
		return nil, fmt.Errorf("quic dial: %w", err)
	}

	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		conn.CloseWithError(1, "open stream failed")
		tr.Close()
		return nil, fmt.Errorf("quic open stream: %w", err)
	}

	return &QUICConn{conn: conn, stream: stream, transport: tr}, nil
}

// ListenQUICServer accepts a QUIC connection as the server (receiver) over
// a pre-existing UDP socket. It waits for the client to connect and open
// a stream.
func ListenQUICServer(ctx context.Context, udpConn net.PacketConn) (*QUICConn, error) {
	tlsConf := generateServerTLS()

	quicConf := &quic.Config{
		HandshakeIdleTimeout:           quicHandshakeTimeout,
		MaxIdleTimeout:                 quicIdleTimeout,
		InitialStreamReceiveWindow:     quicMaxStreamData,
		MaxStreamReceiveWindow:         quicMaxStreamData,
		InitialConnectionReceiveWindow: quicMaxData,
		MaxConnectionReceiveWindow:     quicMaxData,
	}

	tr := &quic.Transport{
		Conn: udpConn,
	}

	ln, err := tr.Listen(tlsConf, quicConf)
	if err != nil {
		return nil, fmt.Errorf("quic listen: %w", err)
	}

	conn, err := ln.Accept(ctx)
	if err != nil {
		ln.Close()
		return nil, fmt.Errorf("quic accept: %w", err)
	}

	stream, err := conn.AcceptStream(ctx)
	if err != nil {
		conn.CloseWithError(1, "accept stream failed")
		ln.Close()
		tr.Close()
		return nil, fmt.Errorf("quic accept stream: %w", err)
	}

	return &QUICConn{conn: conn, stream: stream, transport: tr, listener: ln}, nil
}

// generateServerTLS creates a self-signed TLS config for the QUIC server.
// TLS verification is not security-relevant here because bore's Noise XXpsk0
// handshake provides the actual authentication above the QUIC transport.
func generateServerTLS() *tls.Config {
	cert, err := generateSelfSignedCert()
	if err != nil {
		// This should never fail in practice.
		panic(fmt.Sprintf("generate TLS cert: %v", err))
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{quicALPN},
		MinVersion:   tls.VersionTLS13,
	}
}

// generateClientTLS creates a TLS config for the QUIC client.
// InsecureSkipVerify is true because the TLS layer is not the security
// boundary -- bore's Noise handshake is.
func generateClientTLS() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true, //nolint: gosec // Noise handshake provides auth
		NextProtos:         []string{quicALPN},
		MinVersion:         tls.VersionTLS13,
	}
}

// generateSelfSignedCert creates an ephemeral self-signed certificate for QUIC.
func generateSelfSignedCert() (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create cert: %w", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}, nil
}

// quicConnFromPunchedSocket is a helper that wraps a punched UDP socket in
// QUIC transport. The role parameter ("sender" or "receiver") determines
// whether this side acts as the QUIC client or server.
//
// Convention: sender = QUIC client, receiver = QUIC server.
// This is deterministic so both sides agree without additional signaling.
func quicConnFromPunchedSocket(ctx context.Context, udpConn *net.UDPConn, peerAddr *net.UDPAddr, role string) (*QUICConn, error) {
	switch role {
	case "sender":
		return DialQUICClient(ctx, udpConn, peerAddr)
	case "receiver":
		return ListenQUICServer(ctx, udpConn)
	default:
		return nil, fmt.Errorf("unknown role for QUIC setup: %q", role)
	}
}

// ConnWithMetrics is an optional interface that Conn implementations can
// satisfy to expose transport quality metrics.
type ConnWithMetrics interface {
	Conn
	io.Reader
	io.Writer

	// ConnectionMetrics returns current quality metrics for the connection.
	ConnectionMetrics() ConnectionQuality
}

// ConnectionQuality holds transport quality measurements.
type ConnectionQuality struct {
	// RTT is the measured round-trip time. Zero if not measured.
	RTT time.Duration

	// LossRate is the observed packet loss rate (0.0 to 1.0).
	// Only meaningful for UDP-based transports.
	LossRate float64

	// BytesSent is the total bytes written to the transport.
	BytesSent int64

	// BytesReceived is the total bytes read from the transport.
	BytesReceived int64

	// TransportType identifies the transport ("quic", "reliable-udp", "relay").
	TransportType string

	// ThroughputSendBytesPerSec is the average send throughput in bytes/second.
	ThroughputSendBytesPerSec float64

	// ThroughputRecvBytesPerSec is the average receive throughput in bytes/second.
	ThroughputRecvBytesPerSec float64

	// Duration is the elapsed time since the connection was established.
	Duration time.Duration

	// WriteCount is the number of Write calls made.
	WriteCount int64

	// ReadCount is the number of Read calls made.
	ReadCount int64
}
