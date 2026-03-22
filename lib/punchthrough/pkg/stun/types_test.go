package stun

import (
	"net"
	"testing"
	"time"
)

func TestNATType_String(t *testing.T) {
	tests := []struct {
		natType NATType
		want    string
	}{
		{NATUnknown, "Unknown"},
		{NATFullCone, "Full Cone"},
		{NATRestrictedCone, "Restricted Cone"},
		{NATPortRestrictedCone, "Port-Restricted Cone"},
		{NATSymmetric, "Symmetric"},
		{NATType(99), "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.natType.String(); got != tt.want {
			t.Errorf("NATType(%d).String() = %q, want %q", tt.natType, got, tt.want)
		}
	}
}

func TestNATType_Punchable(t *testing.T) {
	tests := []struct {
		natType NATType
		want    bool
	}{
		{NATUnknown, false},
		{NATFullCone, true},
		{NATRestrictedCone, true},
		{NATPortRestrictedCone, true},
		{NATSymmetric, false},
	}
	for _, tt := range tests {
		if got := tt.natType.Punchable(); got != tt.want {
			t.Errorf("NATType(%d).Punchable() = %v, want %v", tt.natType, got, tt.want)
		}
	}
}

func TestProbeResult_String(t *testing.T) {
	t.Run("with public address", func(t *testing.T) {
		r := &ProbeResult{
			NATType:    NATPortRestrictedCone,
			PublicAddr: &net.UDPAddr{IP: net.IPv4(203, 0, 113, 1), Port: 12345},
			Duration:   142 * time.Millisecond,
		}
		s := r.String()
		if s == "" {
			t.Fatal("String() returned empty")
		}
		if want := "Port-Restricted Cone"; !containsStr(s, want) {
			t.Errorf("String() missing %q in: %s", want, s)
		}
		if want := "203.0.113.1:12345"; !containsStr(s, want) {
			t.Errorf("String() missing %q in: %s", want, s)
		}
	})

	t.Run("without public address", func(t *testing.T) {
		r := &ProbeResult{
			NATType: NATUnknown,
		}
		s := r.String()
		if want := "Unknown"; !containsStr(s, want) {
			t.Errorf("String() missing %q in: %s", want, s)
		}
		if want := "no public address"; !containsStr(s, want) {
			t.Errorf("String() missing %q in: %s", want, s)
		}
	})
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
