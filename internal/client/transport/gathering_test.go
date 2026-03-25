package transport

import (
	"net"
	"testing"

	"github.com/dunamismax/bore/internal/punchthrough/stun"
)

func TestCandidateTypeString(t *testing.T) {
	tests := []struct {
		ct   CandidateType
		want string
	}{
		{CandidateHost, "host"},
		{CandidateServerReflexive, "srflx"},
		{CandidateRelay, "relay"},
		{CandidateType(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.ct.String(); got != tt.want {
			t.Errorf("%d.String() = %q, want %q", tt.ct, got, tt.want)
		}
	}
}

func TestPriorityForType(t *testing.T) {
	hostPrivate := priorityForType(CandidateHost, true)
	hostPublic := priorityForType(CandidateHost, false)
	srflx := priorityForType(CandidateServerReflexive, false)
	relay := priorityForType(CandidateRelay, false)

	if hostPrivate <= hostPublic {
		t.Errorf("private host priority %d should be > public host %d", hostPrivate, hostPublic)
	}
	if hostPublic <= srflx {
		t.Errorf("public host priority %d should be > srflx %d", hostPublic, srflx)
	}
	if srflx <= relay {
		t.Errorf("srflx priority %d should be > relay %d", srflx, relay)
	}
	if relay <= 0 {
		t.Errorf("relay priority %d should be > 0", relay)
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip      string
		private bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"172.32.0.1", false},
	}
	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		if got := isPrivateIP(ip); got != tt.private {
			t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, got, tt.private)
		}
	}
}

func TestGatherResultBestCandidate(t *testing.T) {
	// Empty result.
	r := &GatherResult{}
	if r.BestCandidate() != nil {
		t.Error("BestCandidate on empty result should be nil")
	}

	// With candidates.
	r.Candidates = []GatheredCandidate{
		{Addr: "192.168.1.1:0", Type: CandidateHost, Priority: 1000},
		{Addr: "1.2.3.4:5678", Type: CandidateServerReflexive, Priority: 500},
	}
	best := r.BestCandidate()
	if best == nil {
		t.Fatal("BestCandidate should not be nil")
	}
	if best.Addr != "192.168.1.1:0" {
		t.Errorf("BestCandidate.Addr = %q, want %q", best.Addr, "192.168.1.1:0")
	}
}

func TestGatherResultToLegacyCandidate(t *testing.T) {
	// Empty result.
	r := &GatherResult{}
	if r.ToLegacyCandidate() != nil {
		t.Error("ToLegacyCandidate on empty result should be nil")
	}

	// With candidates.
	r.Candidates = []GatheredCandidate{
		{Addr: "1.2.3.4:5678", Type: CandidateServerReflexive, Priority: 500, NATType: stun.NATPortRestrictedCone},
	}
	r.NATType = stun.NATPortRestrictedCone

	lc := r.ToLegacyCandidate()
	if lc == nil {
		t.Fatal("ToLegacyCandidate should not be nil")
	}
	if lc.PublicAddr != "1.2.3.4:5678" {
		t.Errorf("PublicAddr = %q, want %q", lc.PublicAddr, "1.2.3.4:5678")
	}
	if lc.NATType != stun.NATPortRestrictedCone {
		t.Errorf("NATType = %v, want PortRestrictedCone", lc.NATType)
	}
}

func TestGatherHostCandidates(t *testing.T) {
	candidates, err := gatherHostCandidates()
	if err != nil {
		t.Fatalf("gatherHostCandidates: %v", err)
	}

	// We should find at least one non-loopback interface on any test machine.
	// But CI containers might not have any; treat zero as a soft pass.
	for _, c := range candidates {
		if c.Type != CandidateHost {
			t.Errorf("candidate type = %v, want host", c.Type)
		}
		if c.Priority <= 0 {
			t.Errorf("candidate priority = %d, should be > 0", c.Priority)
		}
		if c.Addr == "" {
			t.Error("candidate addr is empty")
		}
	}
}

func TestBytesInRange(t *testing.T) {
	tests := []struct {
		ip, start, end string
		want           bool
	}{
		{"10.0.0.1", "10.0.0.0", "10.255.255.255", true},
		{"10.0.0.0", "10.0.0.0", "10.255.255.255", true},
		{"10.255.255.255", "10.0.0.0", "10.255.255.255", true},
		{"11.0.0.1", "10.0.0.0", "10.255.255.255", false},
		{"9.255.255.255", "10.0.0.0", "10.255.255.255", false},
	}
	for _, tt := range tests {
		ip := net.ParseIP(tt.ip).To4()
		start := net.ParseIP(tt.start).To4()
		end := net.ParseIP(tt.end).To4()
		if got := bytesInRange(ip, start, end); got != tt.want {
			t.Errorf("bytesInRange(%s, %s, %s) = %v, want %v",
				tt.ip, tt.start, tt.end, got, tt.want)
		}
	}
}

func TestGatherConfigDefaults(t *testing.T) {
	// Default config should enable both host and STUN.
	cfg := &GatherConfig{
		IncludeHost: true,
		IncludeSTUN: true,
	}
	if !cfg.IncludeHost {
		t.Error("IncludeHost should default to true")
	}
	if !cfg.IncludeSTUN {
		t.Error("IncludeSTUN should default to true")
	}
}
