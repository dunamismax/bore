package transport

import (
	"encoding/json"
	"testing"

	"github.com/dunamismax/bore/internal/punchthrough/stun"
)

func TestCandidateValidate(t *testing.T) {
	tests := []struct {
		name    string
		c       Candidate
		wantErr bool
	}{
		{
			name:    "valid IPv4",
			c:       Candidate{PublicAddr: "203.0.113.5:12345", NATType: stun.NATPortRestrictedCone},
			wantErr: false,
		},
		{
			name:    "valid IPv6",
			c:       Candidate{PublicAddr: "[::1]:8080", NATType: stun.NATFullCone},
			wantErr: false,
		},
		{
			name:    "empty addr",
			c:       Candidate{PublicAddr: "", NATType: stun.NATFullCone},
			wantErr: true,
		},
		{
			name:    "no port",
			c:       Candidate{PublicAddr: "203.0.113.5", NATType: stun.NATFullCone},
			wantErr: true,
		},
		{
			name:    "hostname not IP",
			c:       Candidate{PublicAddr: "example.com:8080", NATType: stun.NATFullCone},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.c.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestCandidatePunchable(t *testing.T) {
	if c := (Candidate{NATType: stun.NATFullCone}); !c.Punchable() {
		t.Error("FullCone should be punchable")
	}
	if c := (Candidate{NATType: stun.NATSymmetric}); c.Punchable() {
		t.Error("Symmetric should not be punchable")
	}
	if c := (Candidate{NATType: stun.NATUnknown}); c.Punchable() {
		t.Error("Unknown should not be punchable")
	}
}

func TestCandidateJSONRoundTrip(t *testing.T) {
	original := Candidate{
		PublicAddr: "198.51.100.1:4321",
		NATType:    stun.NATPortRestrictedCone,
		DirectPort: 5000,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Candidate
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.PublicAddr != original.PublicAddr {
		t.Errorf("PublicAddr: got %q, want %q", decoded.PublicAddr, original.PublicAddr)
	}
	if decoded.NATType != original.NATType {
		t.Errorf("NATType: got %v, want %v", decoded.NATType, original.NATType)
	}
	if decoded.DirectPort != original.DirectPort {
		t.Errorf("DirectPort: got %d, want %d", decoded.DirectPort, original.DirectPort)
	}
}

func TestCandidateJSONNATTypes(t *testing.T) {
	types := []stun.NATType{
		stun.NATUnknown,
		stun.NATFullCone,
		stun.NATRestrictedCone,
		stun.NATPortRestrictedCone,
		stun.NATSymmetric,
	}
	for _, nat := range types {
		c := Candidate{PublicAddr: "1.2.3.4:5", NATType: nat}
		data, err := json.Marshal(c)
		if err != nil {
			t.Fatalf("Marshal NATType=%v: %v", nat, err)
		}
		var decoded Candidate
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal NATType=%v: %v", nat, err)
		}
		if decoded.NATType != nat {
			t.Errorf("NATType round-trip: got %v, want %v", decoded.NATType, nat)
		}
	}
}

func TestCandidatePairDirectFeasible(t *testing.T) {
	tests := []struct {
		name     string
		local    stun.NATType
		remote   stun.NATType
		feasible bool
	}{
		{"cone+cone", stun.NATPortRestrictedCone, stun.NATPortRestrictedCone, true},
		{"full+symmetric", stun.NATFullCone, stun.NATSymmetric, true},
		{"symmetric+full", stun.NATSymmetric, stun.NATFullCone, true},
		{"symmetric+symmetric", stun.NATSymmetric, stun.NATSymmetric, false},
		{"unknown+cone", stun.NATUnknown, stun.NATPortRestrictedCone, false},
		{"cone+unknown", stun.NATPortRestrictedCone, stun.NATUnknown, false},
		{"unknown+unknown", stun.NATUnknown, stun.NATUnknown, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pair := CandidatePair{
				Local:  Candidate{PublicAddr: "1.2.3.4:5", NATType: tt.local},
				Remote: Candidate{PublicAddr: "5.6.7.8:9", NATType: tt.remote},
			}
			if got := pair.DirectFeasible(); got != tt.feasible {
				t.Errorf("DirectFeasible() = %v, want %v", got, tt.feasible)
			}
		})
	}
}
