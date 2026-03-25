package transport

import (
	"io"
	"sync/atomic"
	"time"
)

// MetricsConn wraps any Conn to track connection quality metrics.
// It is transparent to the caller — reads and writes pass through
// while accumulating byte counters and timing data.
type MetricsConn struct {
	inner     Conn
	startTime time.Time

	bytesSent     atomic.Int64
	bytesReceived atomic.Int64
	writeCount    atomic.Int64
	readCount     atomic.Int64

	// transportType identifies what kind of transport is being measured.
	transportType string
}

// NewMetricsConn wraps an existing Conn with metrics tracking.
func NewMetricsConn(inner Conn, transportType string) *MetricsConn {
	return &MetricsConn{
		inner:         inner,
		startTime:     time.Now(),
		transportType: transportType,
	}
}

// Read implements io.Reader with byte counting.
func (m *MetricsConn) Read(p []byte) (int, error) {
	n, err := m.inner.Read(p)
	if n > 0 {
		m.bytesReceived.Add(int64(n))
		m.readCount.Add(1)
	}
	return n, err
}

// Write implements io.Writer with byte counting.
func (m *MetricsConn) Write(p []byte) (int, error) {
	n, err := m.inner.Write(p)
	if n > 0 {
		m.bytesSent.Add(int64(n))
		m.writeCount.Add(1)
	}
	return n, err
}

// Close implements io.Closer.
func (m *MetricsConn) Close() error {
	return m.inner.Close()
}

// Snapshot returns the current connection quality metrics.
func (m *MetricsConn) Snapshot() ConnectionQuality {
	elapsed := time.Since(m.startTime)

	sent := m.bytesSent.Load()
	received := m.bytesReceived.Load()

	var throughputSend, throughputRecv float64
	if elapsed > 0 {
		throughputSend = float64(sent) / elapsed.Seconds()
		throughputRecv = float64(received) / elapsed.Seconds()
	}

	return ConnectionQuality{
		BytesSent:                 sent,
		BytesReceived:             received,
		TransportType:             m.transportType,
		ThroughputSendBytesPerSec: throughputSend,
		ThroughputRecvBytesPerSec: throughputRecv,
		Duration:                  elapsed,
		WriteCount:                m.writeCount.Load(),
		ReadCount:                 m.readCount.Load(),
	}
}

// Verify MetricsConn satisfies Conn and io.ReadWriteCloser.
var (
	_ Conn      = (*MetricsConn)(nil)
	_ io.Reader = (*MetricsConn)(nil)
	_ io.Writer = (*MetricsConn)(nil)
	_ io.Closer = (*MetricsConn)(nil)
)
