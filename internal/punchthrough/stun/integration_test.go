//go:build integration

package stun

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestProbeServer_RealSTUN(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		t.Fatalf("bind UDP: %v", err)
	}
	defer conn.Close()

	probe, err := ProbeServer(ctx, conn, "stun.l.google.com:19302", 5*time.Second)
	if err != nil {
		t.Fatalf("ProbeServer: %v", err)
	}
	if probe.MappedAddr == nil {
		t.Fatal("expected mapped address, got nil")
	}
	if probe.MappedAddr.IP == nil {
		t.Fatal("expected mapped IP, got nil")
	}
	if probe.MappedAddr.Port == 0 {
		t.Fatal("expected non-zero mapped port")
	}
	t.Logf("mapped address: %s (via %s in %s)", probe.MappedAddr, probe.Server, probe.Duration)
}

func TestProbe_RealSTUN(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := Probe(ctx, &Config{
		Servers: []string{
			"stun.l.google.com:19302",
			"stun1.l.google.com:19302",
		},
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.PublicAddr == nil {
		t.Fatal("expected public address, got nil")
	}
	if result.NATType == NATUnknown {
		t.Log("NAT type unknown -- might have only one successful probe")
	}

	t.Logf("NAT type: %s", result.NATType)
	t.Logf("Public address: %s", result.PublicAddr)
	t.Logf("Total probe time: %s", result.Duration)
	for _, p := range result.Probes {
		if p.Err != nil {
			t.Logf("  ✗ %s: %v (%s)", p.Server, p.Err, p.Duration)
		} else {
			t.Logf("  ✓ %s: %s (%s)", p.Server, p.MappedAddr, p.Duration)
		}
	}
}

func TestProbe_RealSTUN_DefaultServers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := Probe(ctx, nil)
	if err != nil {
		t.Fatalf("Probe with defaults: %v", err)
	}
	if result.PublicAddr == nil {
		t.Fatal("expected public address, got nil")
	}
	t.Logf("NAT type: %s, public: %s, duration: %s", result.NATType, result.PublicAddr, result.Duration)
}
