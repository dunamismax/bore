package punch

import (
	"net"
	"testing"
	"time"
)

func TestStrategy_String(t *testing.T) {
	tests := []struct {
		strategy Strategy
		want     string
	}{
		{StrategyNone, "None"},
		{StrategySimultaneousOpen, "Simultaneous Open"},
		{StrategyDirectOpen, "Direct Open"},
		{Strategy(99), "None"},
	}
	for _, tt := range tests {
		if got := tt.strategy.String(); got != tt.want {
			t.Errorf("Strategy(%d).String() = %q, want %q", tt.strategy, got, tt.want)
		}
	}
}

func TestPunchResult_String_Success(t *testing.T) {
	r := &PunchResult{
		Success:  true,
		Strategy: StrategySimultaneousOpen,
		PeerAddr: &net.UDPAddr{IP: net.IPv4(203, 0, 113, 1), Port: 12345},
		Attempts: 3,
		RTT:      34 * time.Millisecond,
		Duration: 892 * time.Millisecond,
	}
	s := r.String()
	if s == "" {
		t.Fatal("String() returned empty")
	}
	assertContains(t, s, "succeeded")
	assertContains(t, s, "Simultaneous Open")
	assertContains(t, s, "203.0.113.1:12345")
	assertContains(t, s, "Attempts: 3")
}

func TestPunchResult_String_Failure(t *testing.T) {
	r := &PunchResult{
		Success:  false,
		Strategy: StrategySimultaneousOpen,
		PeerAddr: &net.UDPAddr{IP: net.IPv4(198, 51, 100, 23), Port: 39102},
		Attempts: 5,
		Duration: 10 * time.Second,
	}
	s := r.String()
	assertContains(t, s, "failed")
	assertContains(t, s, "198.51.100.23:39102")
	assertContains(t, s, "Attempts: 5")
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return
		}
	}
	t.Errorf("string %q does not contain %q", s, substr)
}
