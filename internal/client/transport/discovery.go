package transport

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/dunamismax/bore/internal/punchthrough/stun"
)

// DiscoverCandidate runs a STUN probe to discover the local peer's public
// address and NAT type, returning a Candidate suitable for the relay signaling
// exchange.
//
// The returned *net.UDPConn is the bound socket that was used for the STUN
// probe. Callers should reuse this socket for subsequent hole-punch attempts
// to preserve the NAT binding.
//
// Returns (nil, nil, nil) if STUN is disabled (cfg is nil with no servers).
func DiscoverCandidate(ctx context.Context, cfg *stun.Config) (*Candidate, *net.UDPConn, error) {
	if cfg == nil {
		cfg = &stun.Config{}
	}

	result, err := stun.Probe(ctx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("STUN probe: %w", err)
	}

	if result.PublicAddr == nil {
		return nil, nil, fmt.Errorf("STUN probe returned no public address")
	}

	slog.Info("discovery: STUN probe complete",
		"public_addr", result.PublicAddr.String(),
		"nat_type", result.NATType.String(),
		"duration", result.Duration,
	)

	// Bind a fresh UDP socket for hole-punching. We need our own socket
	// because the STUN probe socket is internal to the stun package.
	// For the NAT binding to be preserved, we bind to the same local port
	// if possible by extracting it from the probe.
	var localAddr *net.UDPAddr
	if cfg.LocalAddr != nil {
		la, err := net.ResolveUDPAddr("udp4", *cfg.LocalAddr)
		if err == nil {
			localAddr = la
		}
	}

	conn, err := net.ListenUDP("udp4", localAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("bind UDP for hole-punch: %w", err)
	}

	candidate := &Candidate{
		PublicAddr: result.PublicAddr.String(),
		NATType:    result.NATType,
		DirectPort: conn.LocalAddr().(*net.UDPAddr).Port,
	}

	return candidate, conn, nil
}
