package metrics

import (
	"context"
	"testing"
	"time"
)

func TestRunAlerts_StopsOnCancel(t *testing.T) {
	c := NewCounters()
	cfg := AlertConfig{
		CheckInterval: 10 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		c.RunAlerts(ctx, cfg, nil)
		close(done)
	}()

	// Let it run a few ticks.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunAlerts did not stop after context cancellation")
	}
}

func TestRunAlerts_DetectsRateLimitSpike(t *testing.T) {
	c := NewCounters()
	cfg := AlertConfig{
		CheckInterval:      20 * time.Millisecond,
		RateLimitThreshold: 5,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go c.RunAlerts(ctx, cfg, nil)

	// Generate rate limit hits above threshold.
	for range 10 {
		c.RateLimitHit()
	}

	// Let the checker run at least once.
	time.Sleep(50 * time.Millisecond)

	// Verify counters are tracked (the warning is log-based, so we just
	// confirm the checker ran without panicking).
	snap := c.Snapshot()
	if snap.RateLimitHits != 10 {
		t.Errorf("RateLimitHits = %d, want 10", snap.RateLimitHits)
	}
}

func TestRunAlerts_DetectsRoomUtilization(t *testing.T) {
	c := NewCounters()
	cfg := AlertConfig{
		CheckInterval:      20 * time.Millisecond,
		RoomUtilizationPct: 50.0,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	roomFn := func() RoomSnapshot {
		return RoomSnapshot{TotalRooms: 90, MaxRooms: 100}
	}

	go c.RunAlerts(ctx, cfg, roomFn)

	// Let the checker run.
	time.Sleep(50 * time.Millisecond)

	// Again, the actual alert is a log line. We just verify no panic.
}

func TestDefaultAlertConfig(t *testing.T) {
	cfg := DefaultAlertConfig()
	if cfg.CheckInterval != 60*time.Second {
		t.Errorf("CheckInterval = %v, want 60s", cfg.CheckInterval)
	}
	if cfg.RateLimitThreshold != 50 {
		t.Errorf("RateLimitThreshold = %d, want 50", cfg.RateLimitThreshold)
	}
	if cfg.WSErrorThreshold != 20 {
		t.Errorf("WSErrorThreshold = %d, want 20", cfg.WSErrorThreshold)
	}
	if cfg.RoomUtilizationPct != 80.0 {
		t.Errorf("RoomUtilizationPct = %f, want 80.0", cfg.RoomUtilizationPct)
	}
}
