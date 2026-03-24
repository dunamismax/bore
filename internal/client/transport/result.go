package transport

import "fmt"

// Method describes which transport was ultimately used for a connection.
type Method int

const (
	// MethodUnknown means no transport has been attempted yet.
	MethodUnknown Method = iota

	// MethodDirect means the connection was established directly between peers.
	MethodDirect

	// MethodRelay means the connection goes through a relay server.
	MethodRelay
)

// String returns a human-readable name for the transport method.
func (m Method) String() string {
	switch m {
	case MethodDirect:
		return "direct"
	case MethodRelay:
		return "relay"
	default:
		return "unknown"
	}
}

// FallbackReason describes why the selector did not use the direct path.
type FallbackReason int

const (
	// FallbackNone means no fallback occurred — direct transport succeeded,
	// or no direct attempt was configured.
	FallbackNone FallbackReason = iota

	// FallbackNoDirectAddr means no direct peer address was available,
	// so direct transport was never attempted.
	FallbackNoDirectAddr

	// FallbackDialFailed means the direct dial attempt returned an error.
	FallbackDialFailed

	// FallbackTimeout means the direct dial attempt exceeded its timeout.
	FallbackTimeout
)

// String returns a human-readable description of the fallback reason.
func (r FallbackReason) String() string {
	switch r {
	case FallbackNone:
		return "none"
	case FallbackNoDirectAddr:
		return "no direct address available"
	case FallbackDialFailed:
		return "direct dial failed"
	case FallbackTimeout:
		return "direct dial timed out"
	default:
		return "unknown"
	}
}

// SelectionResult captures the outcome of the transport selection process.
// It records which method was used and, if the direct path was skipped or
// failed, why the selector fell back to relay.
type SelectionResult struct {
	// Method is the transport that was ultimately used.
	Method Method

	// Fallback describes why direct transport was not used.
	// FallbackNone if direct succeeded or was not configured.
	Fallback FallbackReason

	// DirectErr is the error from the direct dial attempt, if any.
	// nil when direct was not attempted or succeeded.
	DirectErr error
}

// String returns a one-line summary suitable for logging.
func (s SelectionResult) String() string {
	if s.Fallback == FallbackNone {
		return fmt.Sprintf("transport=%s", s.Method)
	}
	if s.DirectErr != nil {
		return fmt.Sprintf("transport=%s fallback=%s direct_err=%v", s.Method, s.Fallback, s.DirectErr)
	}
	return fmt.Sprintf("transport=%s fallback=%s", s.Method, s.Fallback)
}
