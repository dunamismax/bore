package metrics

import (
	"sync"
	"testing"
)

func TestCounters_BasicIncrement(t *testing.T) {
	c := NewCounters()

	c.WSConnect()
	c.WSConnect()
	c.WSDisconnect()
	c.RoomCreated()
	c.RoomCreated()
	c.RoomCreated()
	c.RoomJoined()
	c.RoomJoined()
	c.RoomExpired()
	c.RoomRelayed()
	c.BytesRelayed(1024)
	c.BytesRelayed(2048)
	c.FrameRelayed()
	c.FrameRelayed()
	c.FrameRelayed()
	c.RateLimitHit()
	c.WSError()
	c.SignalExchange()

	s := c.Snapshot()
	if s.ActiveWSConnections != 1 {
		t.Errorf("ActiveWSConnections = %d, want 1", s.ActiveWSConnections)
	}
	if s.TotalWSConnections != 2 {
		t.Errorf("TotalWSConnections = %d, want 2", s.TotalWSConnections)
	}
	if s.RoomsCreated != 3 {
		t.Errorf("RoomsCreated = %d, want 3", s.RoomsCreated)
	}
	if s.RoomsJoined != 2 {
		t.Errorf("RoomsJoined = %d, want 2", s.RoomsJoined)
	}
	if s.RoomsExpired != 1 {
		t.Errorf("RoomsExpired = %d, want 1", s.RoomsExpired)
	}
	if s.RoomsRelayed != 1 {
		t.Errorf("RoomsRelayed = %d, want 1", s.RoomsRelayed)
	}
	if s.BytesRelayed != 3072 {
		t.Errorf("BytesRelayed = %d, want 3072", s.BytesRelayed)
	}
	if s.FramesRelayed != 3 {
		t.Errorf("FramesRelayed = %d, want 3", s.FramesRelayed)
	}
	if s.RateLimitHits != 1 {
		t.Errorf("RateLimitHits = %d, want 1", s.RateLimitHits)
	}
	if s.WSErrors != 1 {
		t.Errorf("WSErrors = %d, want 1", s.WSErrors)
	}
	if s.SignalExchanges != 1 {
		t.Errorf("SignalExchanges = %d, want 1", s.SignalExchanges)
	}
	if s.UptimeSeconds < 0 {
		t.Errorf("UptimeSeconds = %d, should be >= 0", s.UptimeSeconds)
	}
}

func TestCounters_Concurrent(t *testing.T) {
	c := NewCounters()

	var wg sync.WaitGroup
	const goroutines = 50
	const ops = 100

	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range ops {
				c.WSConnect()
				c.WSDisconnect()
				c.RoomCreated()
				c.BytesRelayed(100)
				c.FrameRelayed()
			}
		}()
	}
	wg.Wait()

	s := c.Snapshot()
	if s.ActiveWSConnections != 0 {
		t.Errorf("ActiveWSConnections = %d, want 0 (equal connects and disconnects)", s.ActiveWSConnections)
	}
	total := int64(goroutines * ops)
	if s.TotalWSConnections != total {
		t.Errorf("TotalWSConnections = %d, want %d", s.TotalWSConnections, total)
	}
	if s.RoomsCreated != total {
		t.Errorf("RoomsCreated = %d, want %d", s.RoomsCreated, total)
	}
	if s.BytesRelayed != total*100 {
		t.Errorf("BytesRelayed = %d, want %d", s.BytesRelayed, total*100)
	}
	if s.FramesRelayed != total {
		t.Errorf("FramesRelayed = %d, want %d", s.FramesRelayed, total)
	}
}

func TestCounters_SnapshotIsPointInTime(t *testing.T) {
	c := NewCounters()
	c.RoomCreated()

	s1 := c.Snapshot()
	c.RoomCreated()
	s2 := c.Snapshot()

	if s1.RoomsCreated != 1 {
		t.Errorf("s1.RoomsCreated = %d, want 1", s1.RoomsCreated)
	}
	if s2.RoomsCreated != 2 {
		t.Errorf("s2.RoomsCreated = %d, want 2", s2.RoomsCreated)
	}
}
