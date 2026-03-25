package transport

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"io"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
)

func TestQUICConnSatisfiesConn(t *testing.T) {
	var _ Conn = (*QUICConn)(nil)
}

func TestGenerateSelfSignedCert(t *testing.T) {
	cert, err := generateSelfSignedCert()
	if err != nil {
		t.Fatalf("generateSelfSignedCert: %v", err)
	}
	if len(cert.Certificate) == 0 {
		t.Error("cert has no certificates")
	}
	if cert.PrivateKey == nil {
		t.Error("cert has no private key")
	}
}

func TestGenerateServerTLS(t *testing.T) {
	conf := generateServerTLS()
	if len(conf.Certificates) == 0 {
		t.Error("no certificates in server TLS config")
	}
	if len(conf.NextProtos) == 0 || conf.NextProtos[0] != quicALPN {
		t.Errorf("unexpected ALPN: %v", conf.NextProtos)
	}
	if conf.MinVersion != 0x0304 { // TLS 1.3
		t.Errorf("unexpected min version: %#x", conf.MinVersion)
	}
}

func TestGenerateClientTLS(t *testing.T) {
	conf := generateClientTLS()
	if !conf.InsecureSkipVerify {
		t.Error("client TLS should skip verify (Noise handles auth)")
	}
	if len(conf.NextProtos) == 0 || conf.NextProtos[0] != quicALPN {
		t.Errorf("unexpected ALPN: %v", conf.NextProtos)
	}
}

func TestQUICConnCloseIdempotent(t *testing.T) {
	// A QUICConn with nil internals should handle double-close gracefully.
	qc := &QUICConn{closed: false}
	qc.closed = true // simulate already-closed

	err := qc.Close()
	if err != nil {
		t.Errorf("Close on already-closed conn: %v", err)
	}
}

func TestQUICConnMetrics(t *testing.T) {
	qc := &QUICConn{closed: false}
	m := qc.Metrics()
	if !m.Connected {
		t.Error("metrics should show connected")
	}

	qc.closed = true
	m = qc.Metrics()
	if m.Connected {
		t.Error("metrics should show disconnected after close")
	}
}

// TestQUICClientServerLoopback verifies QUIC transport over a loopback UDP socket pair.
func TestQUICClientServerLoopback(t *testing.T) {
	// Use a single shared UDP socket like the real hole-punch flow does.
	// In production, after hole-punching, both sides share their UDP socket
	// with the QUIC transport. For the test we need two sockets to simulate
	// two peers -- one for the server and one for the client.

	serverConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen server: %v", err)
	}

	clientConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use minimal TLS config matching the working standalone test.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	cert := tls.Certificate{Certificate: [][]byte{certDER}, PrivateKey: key}

	serverTLS := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{quicALPN},
	}
	clientTLS := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{quicALPN},
	}

	quicConf := &quic.Config{}

	// Create server transport and listener.
	serverTr := &quic.Transport{Conn: serverConn}
	t.Cleanup(func() { serverTr.Close() })

	ln, err := serverTr.Listen(serverTLS, quicConf)
	if err != nil {
		t.Fatalf("quic listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	// Server accepts in background.
	type serverResult struct {
		conn   *quic.Conn
		stream *quic.Stream
		err    error
	}
	serverCh := make(chan serverResult, 1)
	go func() {
		conn, err := ln.Accept(ctx)
		if err != nil {
			serverCh <- serverResult{err: err}
			return
		}
		stream, err := conn.AcceptStream(ctx)
		serverCh <- serverResult{conn: conn, stream: stream, err: err}
	}()

	// Client dials.
	clientTr := &quic.Transport{Conn: clientConn}
	t.Cleanup(func() { clientTr.Close() })

	clientConn2, err := clientTr.Dial(ctx, serverConn.LocalAddr(), clientTLS, quicConf)
	if err != nil {
		t.Fatalf("client dial: %v", err)
	}
	defer clientConn2.CloseWithError(0, "done")

	clientStream, err := clientConn2.OpenStreamSync(ctx)
	if err != nil {
		t.Fatalf("client open stream: %v", err)
	}

	// QUIC streams are lazy: OpenStreamSync returns immediately but the
	// stream is not announced to the remote until data flows. Write the
	// test payload to trigger stream delivery.
	testPayload := []byte("hello from bore QUIC transport")
	n, err := clientStream.Write(testPayload)
	if err != nil {
		t.Fatalf("client write: %v", err)
	}
	if n != len(testPayload) {
		t.Fatalf("client write: wrote %d, want %d", n, len(testPayload))
	}

	// Wait for server to accept stream.
	sr := <-serverCh
	if sr.err != nil {
		t.Fatalf("server error: %v", sr.err)
	}
	defer sr.conn.CloseWithError(0, "done")

	// Build QUICConn wrappers.
	clientQUIC := &QUICConn{conn: clientConn2, stream: clientStream}
	serverQUIC := &QUICConn{conn: sr.conn, stream: sr.stream}

	defer clientQUIC.Close()
	defer serverQUIC.Close()

	// Server reads the payload that triggered stream delivery.
	buf := make([]byte, 256)
	n, err = serverQUIC.Read(buf)
	if err != nil {
		t.Fatalf("server read: %v", err)
	}
	if string(buf[:n]) != string(testPayload) {
		t.Errorf("server got %q, want %q", buf[:n], testPayload)
	}

	// Server writes back, client reads.
	responsePayload := []byte("bore QUIC response")
	n, err = serverQUIC.Write(responsePayload)
	if err != nil {
		t.Fatalf("server write: %v", err)
	}
	if n != len(responsePayload) {
		t.Fatalf("server write: wrote %d, want %d", n, len(responsePayload))
	}

	n, err = clientQUIC.Read(buf)
	if err != nil {
		t.Fatalf("client read: %v", err)
	}
	if string(buf[:n]) != string(responsePayload) {
		t.Errorf("client got %q, want %q", buf[:n], responsePayload)
	}
}

// TestQUICLargeTransfer verifies QUIC can handle a larger payload (1 MB).
func TestQUICLargeTransfer(t *testing.T) {
	serverConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen server: %v", err)
	}

	clientConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	cert := tls.Certificate{Certificate: [][]byte{certDER}, PrivateKey: key}

	serverTLS := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{quicALPN},
	}
	clientTLS := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{quicALPN},
	}

	quicConf := &quic.Config{
		MaxIdleTimeout:                 30 * time.Second,
		InitialStreamReceiveWindow:     quicMaxStreamData,
		MaxStreamReceiveWindow:         quicMaxStreamData,
		InitialConnectionReceiveWindow: quicMaxData,
		MaxConnectionReceiveWindow:     quicMaxData,
	}

	serverTr := &quic.Transport{Conn: serverConn}
	t.Cleanup(func() { serverTr.Close() })

	ln, err := serverTr.Listen(serverTLS, quicConf)
	if err != nil {
		t.Fatalf("quic listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	type serverResult struct {
		conn   *quic.Conn
		stream *quic.Stream
		err    error
	}
	serverCh := make(chan serverResult, 1)
	go func() {
		conn, err := ln.Accept(ctx)
		if err != nil {
			serverCh <- serverResult{err: err}
			return
		}
		stream, err := conn.AcceptStream(ctx)
		serverCh <- serverResult{conn: conn, stream: stream, err: err}
	}()

	clientTr := &quic.Transport{Conn: clientConn}
	t.Cleanup(func() { clientTr.Close() })

	clientQConn, err := clientTr.Dial(ctx, serverConn.LocalAddr(), clientTLS, quicConf)
	if err != nil {
		t.Fatalf("client dial: %v", err)
	}
	defer clientQConn.CloseWithError(0, "done")

	clientStream, err := clientQConn.OpenStreamSync(ctx)
	if err != nil {
		t.Fatalf("client open stream: %v", err)
	}

	// QUIC streams are lazy -- write data to trigger server-side stream acceptance.
	const dataSize = 1024 * 1024
	data := make([]byte, dataSize)
	for i := range data {
		data[i] = byte(i % 256)
	}

	// Start writing immediately to trigger stream delivery.
	writeErrCh := make(chan error, 1)
	go func() {
		_, err := clientStream.Write(data)
		writeErrCh <- err
	}()

	sr := <-serverCh
	if sr.err != nil {
		t.Fatalf("server error: %v", sr.err)
	}
	defer sr.conn.CloseWithError(0, "done")

	serverQUIC := &QUICConn{conn: sr.conn, stream: sr.stream}
	defer serverQUIC.Close()

	received := make([]byte, 0, dataSize)
	buf := make([]byte, 32*1024)
	for len(received) < dataSize {
		n, err := serverQUIC.Read(buf)
		if err != nil {
			if err == io.EOF && len(received) >= dataSize {
				break
			}
			t.Fatalf("read at offset %d: %v", len(received), err)
		}
		received = append(received, buf[:n]...)
	}

	if writeErr := <-writeErrCh; writeErr != nil {
		t.Fatalf("write error: %v", writeErr)
	}

	if len(received) != dataSize {
		t.Fatalf("received %d bytes, want %d", len(received), dataSize)
	}

	// Verify data integrity.
	for i := 0; i < dataSize; i++ {
		if received[i] != byte(i%256) {
			t.Errorf("byte %d: got %d, want %d", i, received[i], byte(i%256))
			break
		}
	}
}

func TestQuicConnFromPunchedSocketInvalidRole(t *testing.T) {
	conn, _ := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	defer conn.Close()

	_, err := quicConnFromPunchedSocket(context.Background(), conn, &net.UDPAddr{}, "invalid")
	if err == nil {
		t.Error("expected error for invalid role")
	}
}

func TestQUICConstants(t *testing.T) {
	if quicALPN != "bore/1" {
		t.Errorf("quicALPN = %q, want %q", quicALPN, "bore/1")
	}
	if quicHandshakeTimeout != 5*time.Second {
		t.Errorf("quicHandshakeTimeout = %v, want 5s", quicHandshakeTimeout)
	}
	if quicIdleTimeout != 60*time.Second {
		t.Errorf("quicIdleTimeout = %v, want 60s", quicIdleTimeout)
	}
}
