package transport

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// DefaultSelectorTimeout is the timeout for the direct-transport attempt
// before falling back to relay.
const DefaultSelectorTimeout = 3 * time.Second

// Selector implements [Dialer] by trying a direct connection first and
// falling back to a relay connection if direct fails or is not configured.
//
// Selection logic (current, intentionally simple):
//   - If DirectAddr is non-empty, attempt direct transport with a short timeout.
//   - If direct fails or DirectAddr is empty, use the relay.
//
// After each dial, the [LastSelection] field records the transport method
// used and, when applicable, the reason the direct path was skipped.
type Selector struct {
	// RelayURL is the relay server URL (required).
	RelayURL string

	// DirectAddr is the remote peer's direct address in "host:port" form.
	// If empty, relay is used immediately without attempting direct.
	DirectAddr string

	// DirectTimeout overrides the timeout for the direct attempt.
	// Zero uses [DefaultSelectorTimeout].
	DirectTimeout time.Duration

	// LastSelection records the outcome of the most recent dial.
	// Callers can inspect this after DialSender or DialReceiver to
	// understand which transport was used and why.
	LastSelection SelectionResult
}

// DialSender tries direct transport first (if configured), then falls back
// to relay.
func (s *Selector) DialSender(ctx context.Context) (string, Conn, error) {
	if s.DirectAddr != "" {
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
	} else {
		s.LastSelection = SelectionResult{
			Method:   MethodRelay,
			Fallback: FallbackNoDirectAddr,
		}
	}

	slog.Info("transport: using relay", "url", s.RelayURL)
	relay := &RelayDialer{RelayURL: s.RelayURL}
	sessionID, conn, err := relay.DialSender(ctx)
	if err != nil {
		return "", nil, err
	}
	return sessionID, conn, nil
}

// DialReceiver tries direct transport first (if configured), then falls back
// to relay.
func (s *Selector) DialReceiver(ctx context.Context, sessionID string) (Conn, error) {
	if s.DirectAddr != "" {
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
	} else {
		s.LastSelection = SelectionResult{
			Method:   MethodRelay,
			Fallback: FallbackNoDirectAddr,
		}
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
