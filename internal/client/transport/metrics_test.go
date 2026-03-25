package transport

import (
	"bytes"
	"testing"
	"time"
)

func TestMetricsConnSatisfiesConn(t *testing.T) {
	var _ Conn = (*MetricsConn)(nil)
}

func TestMetricsConnTracksWriteBytes(t *testing.T) {
	inner := newMockConn(nil)
	mc := NewMetricsConn(inner, "test")

	data := []byte("hello metrics")
	n, err := mc.Write(data)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(data) {
		t.Fatalf("Write: wrote %d, want %d", n, len(data))
	}

	snap := mc.Snapshot()
	if snap.BytesSent != int64(len(data)) {
		t.Errorf("BytesSent = %d, want %d", snap.BytesSent, len(data))
	}
	if snap.WriteCount != 1 {
		t.Errorf("WriteCount = %d, want 1", snap.WriteCount)
	}
	if snap.TransportType != "test" {
		t.Errorf("TransportType = %q, want %q", snap.TransportType, "test")
	}
}

func TestMetricsConnTracksReadBytes(t *testing.T) {
	data := []byte("metrics read test")
	inner := newMockConn(data)
	mc := NewMetricsConn(inner, "test")

	buf := make([]byte, 64)
	n, err := mc.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if n != len(data) {
		t.Fatalf("Read: read %d, want %d", n, len(data))
	}

	snap := mc.Snapshot()
	if snap.BytesReceived != int64(len(data)) {
		t.Errorf("BytesReceived = %d, want %d", snap.BytesReceived, len(data))
	}
	if snap.ReadCount != 1 {
		t.Errorf("ReadCount = %d, want 1", snap.ReadCount)
	}
}

func TestMetricsConnThroughput(t *testing.T) {
	inner := newMockConn(nil)
	mc := NewMetricsConn(inner, "throughput-test")

	// Write some data.
	data := bytes.Repeat([]byte("x"), 10000)
	mc.Write(data) //nolint: errcheck

	// Wait a bit to let throughput be measurable.
	time.Sleep(10 * time.Millisecond)

	snap := mc.Snapshot()
	if snap.Duration <= 0 {
		t.Error("Duration should be > 0")
	}
	if snap.ThroughputSendBytesPerSec <= 0 {
		t.Error("send throughput should be > 0")
	}
}

func TestMetricsConnMultipleOps(t *testing.T) {
	inner := newMockConn(nil)
	mc := NewMetricsConn(inner, "multi")

	// Multiple writes.
	mc.Write([]byte("aaa")) //nolint: errcheck
	mc.Write([]byte("bbb")) //nolint: errcheck
	mc.Write([]byte("ccc")) //nolint: errcheck

	snap := mc.Snapshot()
	if snap.BytesSent != 9 {
		t.Errorf("BytesSent = %d, want 9", snap.BytesSent)
	}
	if snap.WriteCount != 3 {
		t.Errorf("WriteCount = %d, want 3", snap.WriteCount)
	}
}

func TestMetricsConnClose(t *testing.T) {
	mc := newMockConn(nil)
	metrics := NewMetricsConn(mc, "close-test")

	if err := metrics.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !mc.closed {
		t.Error("inner conn should be closed")
	}
}
