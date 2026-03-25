package transport

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"time"

	"github.com/dunamismax/bore/internal/punchthrough/stun"
)

// CandidateType classifies how a candidate address was discovered.
type CandidateType int

const (
	// CandidateHost is a local interface address (LAN, loopback).
	CandidateHost CandidateType = iota

	// CandidateServerReflexive is a STUN-mapped public address.
	CandidateServerReflexive

	// CandidateRelay is a relay-allocated address (TURN-style, future).
	CandidateRelay
)

// String returns a human-readable name for the candidate type.
func (ct CandidateType) String() string {
	switch ct {
	case CandidateHost:
		return "host"
	case CandidateServerReflexive:
		return "srflx"
	case CandidateRelay:
		return "relay"
	default:
		return "unknown"
	}
}

// GatheredCandidate is a single candidate address with its type and priority.
type GatheredCandidate struct {
	// Addr is the candidate address in "ip:port" form.
	Addr string `json:"addr"`

	// Type is how this candidate was discovered.
	Type CandidateType `json:"type"`

	// Priority determines candidate ordering. Higher is better.
	// Host candidates are preferred over server-reflexive.
	Priority int `json:"priority"`

	// NATType is the discovered NAT type (only meaningful for srflx candidates).
	NATType stun.NATType `json:"natType,omitempty"`
}

// GatherResult holds the complete set of gathered candidates.
type GatherResult struct {
	// Candidates is the ordered list of gathered candidates (highest priority first).
	Candidates []GatheredCandidate

	// UDPConn is the bound UDP socket used for STUN probing.
	// Reuse this for hole-punching to preserve NAT bindings.
	UDPConn *net.UDPConn

	// NATType is the classified NAT type from STUN probes.
	NATType stun.NATType

	// Duration is the total time taken for gathering.
	Duration time.Duration
}

// BestCandidate returns the highest-priority candidate, or nil if none.
func (r *GatherResult) BestCandidate() *GatheredCandidate {
	if len(r.Candidates) == 0 {
		return nil
	}
	return &r.Candidates[0]
}

// ToLegacyCandidate converts the gather result to the legacy Candidate type
// for backward compatibility with the signaling protocol.
func (r *GatherResult) ToLegacyCandidate() *Candidate {
	best := r.BestCandidate()
	if best == nil {
		return nil
	}

	var directPort int
	if r.UDPConn != nil {
		directPort = r.UDPConn.LocalAddr().(*net.UDPAddr).Port
	}

	return &Candidate{
		PublicAddr: best.Addr,
		NATType:    r.NATType,
		DirectPort: directPort,
	}
}

// GatherConfig controls multi-candidate gathering behavior.
type GatherConfig struct {
	// STUNConfig is the STUN probing configuration.
	STUNConfig *stun.Config

	// IncludeHost controls whether local interface addresses are gathered.
	// Default: true.
	IncludeHost bool

	// IncludeSTUN controls whether STUN server-reflexive candidates are gathered.
	// Default: true.
	IncludeSTUN bool
}

// GatherCandidates performs ICE-like multi-candidate gathering.
//
// It discovers candidates from multiple sources:
//  1. Host candidates: non-loopback local interface addresses
//  2. Server-reflexive candidates: STUN-mapped public addresses
//
// Candidates are returned sorted by priority (host > srflx > relay).
func GatherCandidates(ctx context.Context, cfg *GatherConfig) (*GatherResult, error) {
	if cfg == nil {
		cfg = &GatherConfig{
			IncludeHost: true,
			IncludeSTUN: true,
		}
	}

	start := time.Now()
	result := &GatherResult{
		NATType: stun.NATUnknown,
	}

	// Gather host candidates.
	if cfg.IncludeHost {
		hostCandidates, err := gatherHostCandidates()
		if err != nil {
			slog.Debug("gather: host candidate discovery failed", "error", err)
		} else {
			result.Candidates = append(result.Candidates, hostCandidates...)
		}
	}

	// Gather STUN server-reflexive candidates.
	if cfg.IncludeSTUN {
		stunCandidates, udpConn, natType, err := gatherSTUNCandidates(ctx, cfg.STUNConfig)
		if err != nil {
			slog.Debug("gather: STUN candidate discovery failed", "error", err)
		} else {
			result.Candidates = append(result.Candidates, stunCandidates...)
			result.UDPConn = udpConn
			result.NATType = natType
		}
	}

	// Sort by priority (highest first).
	sort.Slice(result.Candidates, func(i, j int) bool {
		return result.Candidates[i].Priority > result.Candidates[j].Priority
	})

	result.Duration = time.Since(start)

	slog.Info("gather: candidate gathering complete",
		"candidates", len(result.Candidates),
		"nat_type", result.NATType.String(),
		"duration", result.Duration,
	)

	if len(result.Candidates) == 0 {
		return result, fmt.Errorf("no candidates gathered")
	}

	return result, nil
}

// gatherHostCandidates discovers local non-loopback IPv4 addresses.
func gatherHostCandidates() ([]GatheredCandidate, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list interfaces: %w", err)
	}

	var candidates []GatheredCandidate

	for _, iface := range ifaces {
		// Skip down, loopback, and point-to-point interfaces.
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			ip := ipNet.IP.To4()
			if ip == nil {
				continue // skip IPv6 for now
			}

			// Skip link-local addresses (169.254.x.x).
			if ip[0] == 169 && ip[1] == 254 {
				continue
			}

			candidate := GatheredCandidate{
				Addr:     fmt.Sprintf("%s:0", ip.String()),
				Type:     CandidateHost,
				Priority: priorityForType(CandidateHost, isPrivateIP(ip)),
			}
			candidates = append(candidates, candidate)

			slog.Debug("gather: found host candidate",
				"addr", ip.String(),
				"iface", iface.Name,
			)
		}
	}

	return candidates, nil
}

// gatherSTUNCandidates runs STUN probes and returns server-reflexive candidates.
func gatherSTUNCandidates(ctx context.Context, cfg *stun.Config) ([]GatheredCandidate, *net.UDPConn, stun.NATType, error) {
	if cfg == nil {
		cfg = &stun.Config{}
	}

	probeResult, err := stun.Probe(ctx, cfg)
	if err != nil {
		return nil, nil, stun.NATUnknown, fmt.Errorf("STUN probe: %w", err)
	}

	if probeResult.PublicAddr == nil {
		return nil, nil, stun.NATUnknown, fmt.Errorf("STUN probe returned no public address")
	}

	// Bind a UDP socket for hole-punching, reusing local port if possible.
	var localAddr *net.UDPAddr
	if cfg.LocalAddr != nil {
		la, err := net.ResolveUDPAddr("udp4", *cfg.LocalAddr)
		if err == nil {
			localAddr = la
		}
	}

	conn, err := net.ListenUDP("udp4", localAddr)
	if err != nil {
		return nil, nil, stun.NATUnknown, fmt.Errorf("bind UDP: %w", err)
	}

	// Create a candidate from the STUN result.
	candidates := []GatheredCandidate{
		{
			Addr:     probeResult.PublicAddr.String(),
			Type:     CandidateServerReflexive,
			Priority: priorityForType(CandidateServerReflexive, false),
			NATType:  probeResult.NATType,
		},
	}

	// Also add candidates from additional successful STUN probes with different mapped addrs.
	seen := map[string]bool{probeResult.PublicAddr.String(): true}
	for _, probe := range probeResult.Probes {
		if probe.Err != nil || probe.MappedAddr == nil {
			continue
		}
		addr := probe.MappedAddr.String()
		if seen[addr] {
			continue
		}
		seen[addr] = true
		candidates = append(candidates, GatheredCandidate{
			Addr:     addr,
			Type:     CandidateServerReflexive,
			Priority: priorityForType(CandidateServerReflexive, false) - len(candidates),
			NATType:  probeResult.NATType,
		})
	}

	slog.Info("gather: STUN candidates discovered",
		"count", len(candidates),
		"nat_type", probeResult.NATType.String(),
		"primary_addr", probeResult.PublicAddr.String(),
	)

	return candidates, conn, probeResult.NATType, nil
}

// priorityForType returns a priority value based on candidate type.
// Higher values indicate higher priority.
//
// Priority scheme (ICE-inspired):
//   - Host (private network): 1000 (highest -- same LAN is fastest)
//   - Host (public IP):        900
//   - Server-reflexive:        500
//   - Relay:                   100 (lowest -- highest latency)
func priorityForType(ct CandidateType, isPrivate bool) int {
	switch ct {
	case CandidateHost:
		if isPrivate {
			return 1000
		}
		return 900
	case CandidateServerReflexive:
		return 500
	case CandidateRelay:
		return 100
	default:
		return 0
	}
}

// isPrivateIP checks if an IPv4 address is in a private network range.
func isPrivateIP(ip net.IP) bool {
	privateRanges := []struct {
		start net.IP
		end   net.IP
	}{
		{net.IPv4(10, 0, 0, 0), net.IPv4(10, 255, 255, 255)},
		{net.IPv4(172, 16, 0, 0), net.IPv4(172, 31, 255, 255)},
		{net.IPv4(192, 168, 0, 0), net.IPv4(192, 168, 255, 255)},
	}

	for _, r := range privateRanges {
		if bytesInRange(ip.To4(), r.start.To4(), r.end.To4()) {
			return true
		}
	}
	return false
}

// bytesInRange checks if ip is between start and end (inclusive).
func bytesInRange(ip, start, end net.IP) bool {
	for i := 0; i < len(ip) && i < len(start) && i < len(end); i++ {
		if ip[i] < start[i] {
			return false
		}
		if ip[i] > end[i] {
			return false
		}
	}
	return true
}
