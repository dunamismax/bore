package stun

import (
	"net"
	"testing"
)

func TestClassifyNAT_InsufficientProbes(t *testing.T) {
	// With zero probes, classification is Unknown.
	got := classifyNAT(nil)
	if got != NATUnknown {
		t.Errorf("classifyNAT(nil) = %v, want NATUnknown", got)
	}

	// With one probe, classification is Unknown.
	got = classifyNAT([]ServerProbe{
		{
			Server:     "stun.example.com:3478",
			MappedAddr: &net.UDPAddr{IP: net.IPv4(203, 0, 113, 1), Port: 12345},
		},
	})
	if got != NATUnknown {
		t.Errorf("classifyNAT(1 probe) = %v, want NATUnknown", got)
	}
}

func TestClassifyNAT_ConeNAT(t *testing.T) {
	// All probes return the same mapped address → cone NAT.
	addr := &net.UDPAddr{IP: net.IPv4(203, 0, 113, 1), Port: 12345}
	probes := []ServerProbe{
		{Server: "stun1.example.com:3478", MappedAddr: addr},
		{Server: "stun2.example.com:3478", MappedAddr: &net.UDPAddr{IP: net.IPv4(203, 0, 113, 1), Port: 12345}},
		{Server: "stun3.example.com:3478", MappedAddr: &net.UDPAddr{IP: net.IPv4(203, 0, 113, 1), Port: 12345}},
	}
	got := classifyNAT(probes)
	if got != NATPortRestrictedCone {
		t.Errorf("classifyNAT(same addrs) = %v, want NATPortRestrictedCone", got)
	}
}

func TestClassifyNAT_SymmetricNAT_DifferentPorts(t *testing.T) {
	// Different mapped ports → Symmetric NAT.
	probes := []ServerProbe{
		{Server: "stun1.example.com:3478", MappedAddr: &net.UDPAddr{IP: net.IPv4(203, 0, 113, 1), Port: 12345}},
		{Server: "stun2.example.com:3478", MappedAddr: &net.UDPAddr{IP: net.IPv4(203, 0, 113, 1), Port: 54321}},
	}
	got := classifyNAT(probes)
	if got != NATSymmetric {
		t.Errorf("classifyNAT(diff ports) = %v, want NATSymmetric", got)
	}
}

func TestClassifyNAT_SymmetricNAT_DifferentIPs(t *testing.T) {
	// Different mapped IPs → Symmetric NAT.
	probes := []ServerProbe{
		{Server: "stun1.example.com:3478", MappedAddr: &net.UDPAddr{IP: net.IPv4(203, 0, 113, 1), Port: 12345}},
		{Server: "stun2.example.com:3478", MappedAddr: &net.UDPAddr{IP: net.IPv4(203, 0, 113, 2), Port: 12345}},
	}
	got := classifyNAT(probes)
	if got != NATSymmetric {
		t.Errorf("classifyNAT(diff IPs) = %v, want NATSymmetric", got)
	}
}

func TestClassifyNAT_TwoProbes(t *testing.T) {
	// Minimum valid: exactly two probes with same address.
	probes := []ServerProbe{
		{Server: "stun1.example.com:3478", MappedAddr: &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 8888}},
		{Server: "stun2.example.com:3478", MappedAddr: &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 8888}},
	}
	got := classifyNAT(probes)
	if got != NATPortRestrictedCone {
		t.Errorf("classifyNAT(2 same) = %v, want NATPortRestrictedCone", got)
	}
}

func TestConfig_Defaults(t *testing.T) {
	cfg := &Config{}

	servers := cfg.servers()
	if len(servers) == 0 {
		t.Fatal("default servers should not be empty")
	}
	if servers[0] != "stun.l.google.com:19302" {
		t.Errorf("first default server = %q, want stun.l.google.com:19302", servers[0])
	}

	timeout := cfg.timeout()
	if timeout.Seconds() != 5 {
		t.Errorf("default timeout = %v, want 5s", timeout)
	}
}

func TestConfig_Custom(t *testing.T) {
	cfg := &Config{
		Servers: []string{"custom.stun.server:3478"},
		Timeout: 10_000_000_000, // 10s
	}

	servers := cfg.servers()
	if len(servers) != 1 || servers[0] != "custom.stun.server:3478" {
		t.Errorf("custom servers = %v, want [custom.stun.server:3478]", servers)
	}

	timeout := cfg.timeout()
	if timeout.Seconds() != 10 {
		t.Errorf("custom timeout = %v, want 10s", timeout)
	}
}

func TestServerProbe_FieldsPopulated(t *testing.T) {
	probe := ServerProbe{
		Server:     "stun.example.com:3478",
		MappedAddr: &net.UDPAddr{IP: net.IPv4(1, 2, 3, 4), Port: 9999},
	}
	if probe.Server != "stun.example.com:3478" {
		t.Errorf("unexpected server: %s", probe.Server)
	}
	if probe.MappedAddr.Port != 9999 {
		t.Errorf("unexpected port: %d", probe.MappedAddr.Port)
	}
	if probe.Err != nil {
		t.Errorf("unexpected error: %v", probe.Err)
	}
}
