package transport

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
)

// benchTLSCert generates a TLS cert pair for benchmarks.
func benchTLSCert(b *testing.B) tls.Certificate {
	b.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		b.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		b.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{certDER}, PrivateKey: key}
}

// setupQUICPair creates a connected QUIC client/server pair for benchmarking.
func setupQUICPair(b *testing.B) (*QUICConn, *QUICConn) {
	b.Helper()

	serverUDP, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		b.Fatal(err)
	}

	clientUDP, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		b.Fatal(err)
	}

	cert := benchTLSCert(b)
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

	ctx := context.Background()

	serverTr := &quic.Transport{Conn: serverUDP}
	b.Cleanup(func() { serverTr.Close() })

	ln, err := serverTr.Listen(serverTLS, quicConf)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { ln.Close() })

	type sr struct {
		conn   *quic.Conn
		stream *quic.Stream
		err    error
	}
	serverCh := make(chan sr, 1)
	go func() {
		conn, err := ln.Accept(ctx)
		if err != nil {
			serverCh <- sr{err: err}
			return
		}
		stream, err := conn.AcceptStream(ctx)
		serverCh <- sr{conn: conn, stream: stream, err: err}
	}()

	clientTr := &quic.Transport{Conn: clientUDP}
	b.Cleanup(func() { clientTr.Close() })

	cConn, err := clientTr.Dial(ctx, serverUDP.LocalAddr(), clientTLS, quicConf)
	if err != nil {
		b.Fatal(err)
	}

	cStream, err := cConn.OpenStreamSync(ctx)
	if err != nil {
		b.Fatal(err)
	}

	// Write to trigger stream delivery.
	if _, err := cStream.Write([]byte("init")); err != nil {
		b.Fatal(err)
	}

	res := <-serverCh
	if res.err != nil {
		b.Fatal(res.err)
	}

	// Drain the init byte on server side.
	initBuf := make([]byte, 4)
	if _, err := res.stream.Read(initBuf); err != nil {
		b.Fatal(err)
	}

	client := &QUICConn{conn: cConn, stream: cStream}
	server := &QUICConn{conn: res.conn, stream: res.stream}

	b.Cleanup(func() {
		client.Close()
		server.Close()
	})

	return client, server
}

// BenchmarkQUICThroughput measures QUIC transport throughput.
func BenchmarkQUICThroughput(b *testing.B) {
	client, server := setupQUICPair(b)

	data := make([]byte, 64*1024) // 64 KB chunks
	for i := range data {
		data[i] = byte(i % 256)
	}
	readBuf := make([]byte, 128*1024)

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := client.Write(data)
		if err != nil {
			b.Fatal(err)
		}

		total := 0
		for total < len(data) {
			n, err := server.Read(readBuf)
			if err != nil {
				b.Fatal(err)
			}
			total += n
		}
	}
}

// BenchmarkReliableConnThroughput measures ReliableConn throughput for comparison.
// NOTE: ReliableConn uses stop-and-wait, so this benchmark will be significantly
// slower than QUIC. It may also be flaky due to the simple reliability protocol.
// Skipped by default because it is slow.
func BenchmarkReliableConnThroughput(b *testing.B) {
	b.Skip("ReliableConn benchmark skipped: stop-and-wait is known slow for throughput comparison")
}
