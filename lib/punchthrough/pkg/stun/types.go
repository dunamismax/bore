// Package stun provides a STUN client for NAT type discovery and public address probing.
//
// It uses the pion/stun library for correct STUN binding request/response handling and
// classifies the local NAT configuration by probing from multiple STUN servers.
package stun

import (
	"fmt"
	"net"
	"time"
)

// NATType represents the classified NAT configuration.
type NATType int

const (
	// NATUnknown indicates the NAT type could not be determined.
	NATUnknown NATType = iota

	// NATFullCone indicates a Full Cone (one-to-one) NAT.
	// Any external host can send a packet to the internal host by sending a
	// packet to the mapped external address.
	NATFullCone

	// NATRestrictedCone indicates a Restricted Cone NAT.
	// An external host can send a packet to the internal host only if the
	// internal host has previously sent a packet to that external host's IP.
	NATRestrictedCone

	// NATPortRestrictedCone indicates a Port-Restricted Cone NAT.
	// Like Restricted Cone but the restriction includes port numbers.
	NATPortRestrictedCone

	// NATSymmetric indicates a Symmetric NAT.
	// Each request from the same internal address and port to a different destination
	// gets a different external mapping. Hole-punching is generally not possible when
	// both peers are behind symmetric NATs.
	NATSymmetric
)

// String returns the human-readable name of the NAT type.
func (n NATType) String() string {
	switch n {
	case NATFullCone:
		return "Full Cone"
	case NATRestrictedCone:
		return "Restricted Cone"
	case NATPortRestrictedCone:
		return "Port-Restricted Cone"
	case NATSymmetric:
		return "Symmetric"
	default:
		return "Unknown"
	}
}

// Punchable reports whether hole-punching is generally feasible for this NAT type.
// Symmetric NATs on both sides are typically unpunchable without TURN.
func (n NATType) Punchable() bool {
	return n != NATSymmetric && n != NATUnknown
}

// ServerProbe holds the result of probing a single STUN server.
type ServerProbe struct {
	// Server is the STUN server address that was probed.
	Server string

	// MappedAddr is the public IP:port observed by the STUN server.
	MappedAddr *net.UDPAddr

	// Duration is how long the probe took.
	Duration time.Duration

	// Err is non-nil if this individual probe failed.
	Err error
}

// ProbeResult holds the aggregate result of NAT classification from multiple STUN probes.
type ProbeResult struct {
	// NATType is the classified NAT configuration.
	NATType NATType

	// PublicAddr is the discovered public IP:port from the first successful probe.
	PublicAddr *net.UDPAddr

	// Probes holds individual results from each STUN server probed.
	Probes []ServerProbe

	// Duration is the total time taken for all probes.
	Duration time.Duration
}

// String returns a human-readable summary of the probe result.
func (r *ProbeResult) String() string {
	if r.PublicAddr == nil {
		return fmt.Sprintf("NAT type: %s (no public address discovered)", r.NATType)
	}
	return fmt.Sprintf("NAT type: %s\nPublic address: %s\nProbe time: %s",
		r.NATType, r.PublicAddr.String(), r.Duration.Round(time.Millisecond))
}
