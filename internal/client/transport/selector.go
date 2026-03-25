package transport

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"time"

	"github.com/dunamismax/bore/internal/punchthrough/stun"
)

// DefaultSelectorTimeout is the timeout for the direct-transport attempt
// before falling back to relay. Set to 8 seconds to give hole-punching
// a reasonable window across diverse NAT types while keeping user-perceived
// latency acceptable when direct fails.
const DefaultSelectorTimeout = 8 * time.Second

// Selector implements [Dialer] by trying a direct connection first and
// falling back to a relay connection if direct fails or is not configured.
//
// When EnableDirect is true, the selector runs STUN discovery, exchanges
// candidates through the relay signaling channel, and attempts hole-punching
// before falling back.
//
// After each dial, the [LastSelection] field records the transport method
// used and, when applicable, the reason the direct path was skipped.
type Selector struct {
	// RelayURL is the relay server URL (required).
	RelayURL string

	// DirectAddr is the remote peer's direct address in "host:port" form.
	// If empty and EnableDirect is false, relay is used immediately.
	// Deprecated: prefer EnableDirect for the full discovery+signaling path.
	DirectAddr string

	// EnableDirect enables STUN discovery and relay-coordinated signaling
	// for direct transport. When true, the selector:
	//  1. Runs STUN probes to discover the local candidate
	//  2. Exchanges candidates through the relay's /signal endpoint
	//  3. Evaluates the NAT combination for hole-punch feasibility
	//  4. Attempts hole-punching
	//  5. Falls back to relay if any step fails
	EnableDirect bool

	// SessionID is the room ID for the current transfer session.
	// Required for signaling when EnableDirect is true.
	SessionID string

	// Role is "sender" or "receiver" -- used for signaling.
	Role string

	// STUNConfig overrides the STUN probing configuration.
	// Nil uses defaults.
	STUNConfig *stun.Config

	// DirectTimeout overrides the timeout for the direct attempt.
	// Zero uses [DefaultSelectorTimeout].
	DirectTimeout time.Duration

	// LastSelection records the outcome of the most recent dial.
	// Callers can inspect this after DialSender or DialReceiver to
	// understand which transport was used and why.
	LastSelection SelectionResult

	// LastMetricsConn holds the MetricsConn wrapper for the most recent
	// connection, if metrics tracking is enabled. Callers can use this
	// to read connection quality metrics after transfer completes.
	LastMetricsConn *MetricsConn

	// discoveredConn is the UDP socket from STUN probing, reused for
	// hole-punching to preserve NAT bindings.
	discoveredConn *net.UDPConn
}

// DialSender tries direct transport first (if configured), then falls back
// to relay.
func (s *Selector) DialSender(ctx context.Context) (string, Conn, error) {
	if s.EnableDirect {
		return s.dialSenderWithDiscovery(ctx)
	}

	if s.DirectAddr != "" {
		return s.dialSenderLegacy(ctx)
	}

	s.LastSelection = SelectionResult{
		Method:   MethodRelay,
		Fallback: FallbackNoDirectAddr,
	}

	slog.Info("transport: using relay", "url", s.RelayURL)
	relay := &RelayDialer{RelayURL: s.RelayURL}
	return relay.DialSender(ctx)
}

// DialReceiver tries direct transport first (if configured), then falls back
// to relay.
func (s *Selector) DialReceiver(ctx context.Context, sessionID string) (Conn, error) {
	if s.EnableDirect && sessionID != "" {
		s.SessionID = sessionID
		return s.dialReceiverWithDiscovery(ctx, sessionID)
	}

	if s.DirectAddr != "" {
		return s.dialReceiverLegacy(ctx, sessionID)
	}

	s.LastSelection = SelectionResult{
		Method:   MethodRelay,
		Fallback: FallbackNoDirectAddr,
	}

	slog.Info("transport: using relay", "url", s.RelayURL)
	relay := &RelayDialer{RelayURL: s.RelayURL}
	return relay.DialReceiver(ctx, sessionID)
}

// dialSenderWithDiscovery runs the full discovery → signaling → punch → fallback path.
func (s *Selector) dialSenderWithDiscovery(ctx context.Context) (string, Conn, error) {
	// First, create the room via relay (we need the session ID for signaling).
	relay := &RelayDialer{RelayURL: s.RelayURL}
	sessionID, relayConn, err := relay.DialSender(ctx)
	if err != nil {
		return "", nil, err
	}

	// Try direct path in the background. On failure, use relayConn.
	directConn, directErr := s.attemptDirect(ctx, sessionID, "sender")
	if directErr == nil && directConn != nil {
		// Direct succeeded -- close the relay conn, use direct.
		relayConn.Close()
		s.LastSelection = SelectionResult{
			Method:   MethodDirect,
			Fallback: FallbackNone,
		}
		mc := NewMetricsConn(directConn, "quic")
		s.LastMetricsConn = mc
		return sessionID, mc, nil
	}

	// Direct failed -- use relay.
	slog.Info("transport: direct attempt failed, using relay",
		"err", directErr,
		"session", sessionID,
	)

	// Already have the relay conn open.
	mc := NewMetricsConn(relayConn, "relay")
	s.LastMetricsConn = mc
	return sessionID, mc, nil
}

// dialReceiverWithDiscovery runs the full discovery → signaling → punch → fallback path.
func (s *Selector) dialReceiverWithDiscovery(ctx context.Context, sessionID string) (Conn, error) {
	// Try direct path. On failure, fall back to relay.
	directConn, directErr := s.attemptDirect(ctx, sessionID, "receiver")
	if directErr == nil && directConn != nil {
		s.LastSelection = SelectionResult{
			Method:   MethodDirect,
			Fallback: FallbackNone,
		}
		mc := NewMetricsConn(directConn, "quic")
		s.LastMetricsConn = mc
		return mc, nil
	}

	slog.Info("transport: direct attempt failed, using relay",
		"err", directErr,
		"session", sessionID,
	)

	relay := &RelayDialer{RelayURL: s.RelayURL}
	relayConn, err := relay.DialReceiver(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	mc := NewMetricsConn(relayConn, "relay")
	s.LastMetricsConn = mc
	return mc, nil
}

// attemptDirect runs STUN → signaling → NAT check → punch.
func (s *Selector) attemptDirect(ctx context.Context, sessionID, role string) (Conn, error) {
	// Step 1: STUN discovery.
	localCandidate, udpConn, err := DiscoverCandidate(ctx, s.STUNConfig)
	if err != nil {
		s.LastSelection = SelectionResult{
			Method:    MethodRelay,
			Fallback:  FallbackSTUNFailed,
			DirectErr: err,
		}
		if udpConn != nil {
			udpConn.Close()
		}
		return nil, err
	}
	s.discoveredConn = udpConn

	// Step 2: Exchange candidates via relay signaling.
	remoteCandidate, err := ExchangeCandidates(ctx, s.RelayURL, sessionID, role, localCandidate)
	if err != nil {
		s.LastSelection = SelectionResult{
			Method:    MethodRelay,
			Fallback:  FallbackSignalingFailed,
			DirectErr: err,
		}
		udpConn.Close()
		return nil, err
	}

	if remoteCandidate == nil {
		s.LastSelection = SelectionResult{
			Method:   MethodRelay,
			Fallback: FallbackNoDirectAddr,
		}
		udpConn.Close()
		return nil, errors.New("remote peer has no candidate")
	}

	// Step 3: Check NAT feasibility.
	pair := &CandidatePair{
		Local:  *localCandidate,
		Remote: *remoteCandidate,
	}

	if !pair.DirectFeasible() {
		s.LastSelection = SelectionResult{
			Method:    MethodRelay,
			Fallback:  FallbackNATUnfavorable,
			DirectErr: errors.New("NAT combination not feasible for direct"),
		}
		udpConn.Close()
		return nil, errors.New("NAT combination not favorable")
	}

	// Step 4: Attempt direct connection with hole-punching.
	d := &DirectDialer{
		CandidatePair: pair,
		PunchConn:     udpConn,
		Timeout:       s.directTimeout(),
		Mode:          TransportQUIC,
		Role:          role,
	}

	_, conn, err := d.DialSender(ctx)
	if err != nil {
		reason := classifyDirectError(err)
		s.LastSelection = SelectionResult{
			Method:    MethodRelay,
			Fallback:  reason,
			DirectErr: err,
		}
		udpConn.Close()
		return nil, err
	}

	return conn, nil
}

// dialSenderLegacy is the legacy path using a pre-configured DirectAddr.
func (s *Selector) dialSenderLegacy(ctx context.Context) (string, Conn, error) {
	slog.Info("transport: attempting direct connection", "addr", s.DirectAddr)
	d := &DirectDialer{
		RemoteAddr: s.DirectAddr,
		Timeout:    s.directTimeout(),
	}
	sessionID, conn, err := d.DialSender(ctx)
	if err == nil {
		slog.Info("transport: direct connection established")
		s.LastSelection = SelectionResult{
			Method:   MethodDirect,
			Fallback: FallbackNone,
		}
		return sessionID, conn, nil
	}
	reason := classifyDirectError(err)
	slog.Info("transport: direct failed, falling back to relay",
		"err", err,
		"fallback_reason", reason.String(),
	)
	s.LastSelection = SelectionResult{
		Method:    MethodRelay,
		Fallback:  reason,
		DirectErr: err,
	}

	slog.Info("transport: using relay", "url", s.RelayURL)
	relay := &RelayDialer{RelayURL: s.RelayURL}
	return relay.DialSender(ctx)
}

// dialReceiverLegacy is the legacy path using a pre-configured DirectAddr.
func (s *Selector) dialReceiverLegacy(ctx context.Context, sessionID string) (Conn, error) {
	slog.Info("transport: attempting direct connection", "addr", s.DirectAddr)
	d := &DirectDialer{
		RemoteAddr: s.DirectAddr,
		Timeout:    s.directTimeout(),
	}
	conn, err := d.DialReceiver(ctx, sessionID)
	if err == nil {
		slog.Info("transport: direct connection established")
		s.LastSelection = SelectionResult{
			Method:   MethodDirect,
			Fallback: FallbackNone,
		}
		return conn, nil
	}
	reason := classifyDirectError(err)
	slog.Info("transport: direct failed, falling back to relay",
		"err", err,
		"fallback_reason", reason.String(),
	)
	s.LastSelection = SelectionResult{
		Method:    MethodRelay,
		Fallback:  reason,
		DirectErr: err,
	}

	slog.Info("transport: using relay", "url", s.RelayURL)
	relay := &RelayDialer{RelayURL: s.RelayURL}
	return relay.DialReceiver(ctx, sessionID)
}

func (s *Selector) directTimeout() time.Duration {
	if s.DirectTimeout > 0 {
		return s.DirectTimeout
	}
	return DefaultSelectorTimeout
}

// classifyDirectError maps a direct-dial error to a FallbackReason.
func classifyDirectError(err error) FallbackReason {
	if err == nil {
		return FallbackNone
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return FallbackTimeout
	}
	return FallbackDialFailed
}
