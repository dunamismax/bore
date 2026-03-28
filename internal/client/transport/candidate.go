package transport

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/dunamismax/bore/internal/punchthrough/stun"
)

// Candidate describes a peer's network reachability for direct transport.
//
// During rendezvous, each peer runs a STUN probe to discover its public
// address and NAT type, then publishes a Candidate through the relay's
// signaling channel. The other peer consumes the candidate and uses it
// to decide whether and how to attempt a direct connection.
//
// The candidate exchange is the first step of the relay-coordinated
// direct-path negotiation flow.
type Candidate struct {
	// PublicAddr is the peer's STUN-discovered public UDP address.
	// Format: "ip:port".
	PublicAddr string `json:"publicAddr"`

	// NATType is the peer's classified NAT configuration from STUN probing.
	NATType stun.NATType `json:"natType"`

	// DirectPort is the local UDP port the peer is listening on for
	// direct punch attempts. This may differ from the STUN-mapped port
	// if the NAT remaps it.
	DirectPort int `json:"directPort,omitempty"`
}

// Validate checks that the candidate has the minimum fields needed for
// a direct transport attempt.
func (c Candidate) Validate() error {
	if c.PublicAddr == "" {
		return fmt.Errorf("candidate has no public address")
	}
	host, _, err := net.SplitHostPort(c.PublicAddr)
	if err != nil {
		return fmt.Errorf("invalid public address %q: %w", c.PublicAddr, err)
	}
	if net.ParseIP(host) == nil {
		return fmt.Errorf("public address host is not a valid IP: %q", host)
	}
	return nil
}

// Punchable reports whether a direct connection attempt is worth trying
// given the peer's NAT type. Both peers' NAT types should be checked;
// this method only inspects one side.
func (c Candidate) Punchable() bool {
	return c.NATType.Punchable()
}

// MarshalJSON implements json.Marshaler for Candidate.
func (c Candidate) MarshalJSON() ([]byte, error) {
	type alias struct {
		PublicAddr string `json:"publicAddr"`
		NATType    string `json:"natType"`
		DirectPort int    `json:"directPort,omitempty"`
	}
	return json.Marshal(alias{
		PublicAddr: c.PublicAddr,
		NATType:    c.NATType.String(),
		DirectPort: c.DirectPort,
	})
}

// UnmarshalJSON implements json.Unmarshaler for Candidate.
func (c *Candidate) UnmarshalJSON(data []byte) error {
	type alias struct {
		PublicAddr string `json:"publicAddr"`
		NATType    string `json:"natType"`
		DirectPort int    `json:"directPort,omitempty"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	c.PublicAddr = a.PublicAddr
	c.DirectPort = a.DirectPort
	c.NATType = parseNATType(a.NATType)
	return nil
}

// parseNATType converts a human-readable NAT type string back to a NATType value.
func parseNATType(s string) stun.NATType {
	switch s {
	case "Full Cone":
		return stun.NATFullCone
	case "Restricted Cone":
		return stun.NATRestrictedCone
	case "Port-Restricted Cone":
		return stun.NATPortRestrictedCone
	case "Symmetric":
		return stun.NATSymmetric
	default:
		return stun.NATUnknown
	}
}

// CandidatePair holds both peers' candidates for evaluating whether a
// direct connection is feasible. The relay-coordinated signaling flow
// produces a CandidatePair before the selector attempts direct transport.
type CandidatePair struct {
	Local  Candidate `json:"local"`
	Remote Candidate `json:"remote"`
}

// DirectFeasible reports whether the NAT combination allows hole-punching.
// Returns false if either peer has an unknown NAT type or both are symmetric.
func (p CandidatePair) DirectFeasible() bool {
	if p.Local.NATType == stun.NATUnknown || p.Remote.NATType == stun.NATUnknown {
		return false
	}
	if p.Local.NATType == stun.NATSymmetric && p.Remote.NATType == stun.NATSymmetric {
		return false
	}
	return true
}
