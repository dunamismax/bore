package stun

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	pionstun "github.com/pion/stun/v3"
)

// ProbeServer sends a STUN binding request to a single server and returns the mapped address.
// The probe uses the provided UDP connection so that multiple probes from the same local port
// can be compared for NAT classification.
func ProbeServer(ctx context.Context, conn *net.UDPConn, server string, timeout time.Duration) (*ServerProbe, error) {
	start := time.Now()
	probe := &ServerProbe{Server: server}

	raddr, err := net.ResolveUDPAddr("udp4", server)
	if err != nil {
		probe.Err = fmt.Errorf("resolve %s: %w", server, err)
		probe.Duration = time.Since(start)
		return probe, probe.Err
	}

	// Build STUN binding request.
	msg, err := pionstun.Build(pionstun.TransactionID, pionstun.BindingRequest)
	if err != nil {
		probe.Err = fmt.Errorf("build STUN message: %w", err)
		probe.Duration = time.Since(start)
		return probe, probe.Err
	}

	// Set deadline for this probe.
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(timeout)
	}
	if dl := time.Now().Add(timeout); dl.Before(deadline) {
		deadline = dl
	}
	if err := conn.SetWriteDeadline(deadline); err != nil {
		probe.Err = fmt.Errorf("set write deadline: %w", err)
		probe.Duration = time.Since(start)
		return probe, probe.Err
	}

	// Send request.
	if _, err := conn.WriteToUDP(msg.Raw, raddr); err != nil {
		probe.Err = fmt.Errorf("send to %s: %w", server, ErrTimeout)
		probe.Duration = time.Since(start)
		return probe, probe.Err
	}

	// Read response.
	if err := conn.SetReadDeadline(deadline); err != nil {
		probe.Err = fmt.Errorf("set read deadline: %w", err)
		probe.Duration = time.Since(start)
		return probe, probe.Err
	}

	buf := make([]byte, 1500)
	n, _, readErr := conn.ReadFromUDP(buf)
	probe.Duration = time.Since(start)

	if readErr != nil {
		if netErr, ok := readErr.(net.Error); ok && netErr.Timeout() {
			probe.Err = fmt.Errorf("probe %s: %w", server, ErrTimeout)
		} else {
			probe.Err = fmt.Errorf("read from %s: %w", server, readErr)
		}
		return probe, probe.Err
	}

	// Parse response.
	resp := new(pionstun.Message)
	resp.Raw = buf[:n]
	if err := resp.Decode(); err != nil {
		probe.Err = fmt.Errorf("decode response from %s: %w", server, ErrMalformedResponse)
		return probe, probe.Err
	}

	// Extract mapped address.
	var xorAddr pionstun.XORMappedAddress
	if err := xorAddr.GetFrom(resp); err != nil {
		// Try non-XOR mapped address as fallback.
		var mappedAddr pionstun.MappedAddress
		if err2 := mappedAddr.GetFrom(resp); err2 != nil {
			probe.Err = fmt.Errorf("no address in response from %s: %w", server, ErrNoMappedAddress)
			return probe, probe.Err
		}
		probe.MappedAddr = &net.UDPAddr{IP: mappedAddr.IP, Port: mappedAddr.Port}
	} else {
		probe.MappedAddr = &net.UDPAddr{IP: xorAddr.IP, Port: xorAddr.Port}
	}

	return probe, nil
}

// Probe runs STUN probes against all configured servers and classifies the NAT type.
//
// It sends binding requests from the same local UDP port to each server. By comparing
// the mapped addresses returned by different servers, it determines whether the NAT
// assigns the same external mapping (cone NAT) or different mappings (symmetric NAT).
//
// At least two successful probes are needed for NAT classification. A single successful
// probe can still discover the public address but the NAT type will be NATUnknown.
func Probe(ctx context.Context, cfg *Config) (*ProbeResult, error) {
	if cfg == nil {
		cfg = &Config{}
	}

	start := time.Now()
	servers := cfg.servers()
	timeout := cfg.timeout()

	result := &ProbeResult{
		NATType: NATUnknown,
		Probes:  make([]ServerProbe, 0, len(servers)),
	}

	// Bind a single local UDP socket so all probes share the same source port.
	var laddr *net.UDPAddr
	if cfg.LocalAddr != nil {
		var err error
		laddr, err = net.ResolveUDPAddr("udp4", *cfg.LocalAddr)
		if err != nil {
			return nil, fmt.Errorf("resolve local address: %w", err)
		}
	}

	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		return nil, fmt.Errorf("bind UDP socket: %w", err)
	}
	defer conn.Close()

	slog.Debug("stun probe starting",
		"servers", servers,
		"local_addr", conn.LocalAddr().String(),
		"timeout", timeout,
	)

	// Probe each server sequentially. Sequential probing is simpler and avoids
	// interleaving responses from multiple servers on the same socket.
	var successful []ServerProbe
	for _, server := range servers {
		select {
		case <-ctx.Done():
			result.Duration = time.Since(start)
			return result, ctx.Err()
		default:
		}

		probe, probeErr := ProbeServer(ctx, conn, server, timeout)
		result.Probes = append(result.Probes, *probe)

		if probeErr != nil {
			slog.Debug("stun probe failed",
				"server", server,
				"error", probeErr,
				"duration", probe.Duration,
			)
			continue
		}

		slog.Debug("stun probe succeeded",
			"server", server,
			"mapped_addr", probe.MappedAddr.String(),
			"duration", probe.Duration,
		)
		successful = append(successful, *probe)
	}

	result.Duration = time.Since(start)

	if len(successful) == 0 {
		return result, ErrAllProbesFailed
	}

	// Set public address from the first successful probe.
	result.PublicAddr = successful[0].MappedAddr

	// Classify NAT type.
	result.NATType = classifyNAT(successful)

	return result, nil
}

// classifyNAT determines NAT type from successful probe results.
//
// Classification logic:
//   - If all probes return the same mapped address (IP and port), the NAT is cone-type.
//     We classify as Port-Restricted Cone since we cannot distinguish between Full Cone,
//     Restricted Cone, and Port-Restricted Cone without a STUN server that supports
//     CHANGE-REQUEST (RFC 5780). Most public STUN servers don't support this.
//   - If probes return different mapped ports for different servers, the NAT is Symmetric.
//   - With only one successful probe, classification is not possible (returns NATUnknown).
func classifyNAT(probes []ServerProbe) NATType {
	if len(probes) < 2 {
		return NATUnknown
	}

	// Compare mapped addresses across all probes.
	refAddr := probes[0].MappedAddr
	for i := 1; i < len(probes); i++ {
		other := probes[i].MappedAddr
		if !refAddr.IP.Equal(other.IP) || refAddr.Port != other.Port {
			// Different mapping for different destinations = Symmetric NAT.
			return NATSymmetric
		}
	}

	// All probes returned the same mapped address.
	// Without CHANGE-REQUEST support we cannot distinguish between cone subtypes.
	// Port-Restricted Cone is the most conservative (and common) assumption.
	return NATPortRestrictedCone
}
