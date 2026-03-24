package transport

import (
	"errors"
	"testing"
)

func TestMethodString(t *testing.T) {
	tests := []struct {
		m    Method
		want string
	}{
		{MethodUnknown, "unknown"},
		{MethodDirect, "direct"},
		{MethodRelay, "relay"},
	}
	for _, tt := range tests {
		if got := tt.m.String(); got != tt.want {
			t.Errorf("Method(%d).String() = %q, want %q", tt.m, got, tt.want)
		}
	}
}

func TestFallbackReasonString(t *testing.T) {
	tests := []struct {
		r    FallbackReason
		want string
	}{
		{FallbackNone, "none"},
		{FallbackNoDirectAddr, "no direct address available"},
		{FallbackDialFailed, "direct dial failed"},
		{FallbackTimeout, "direct dial timed out"},
	}
	for _, tt := range tests {
		if got := tt.r.String(); got != tt.want {
			t.Errorf("FallbackReason(%d).String() = %q, want %q", tt.r, got, tt.want)
		}
	}
}

func TestSelectionResultString(t *testing.T) {
	// Direct success.
	r := SelectionResult{Method: MethodDirect, Fallback: FallbackNone}
	s := r.String()
	if s != "transport=direct" {
		t.Errorf("unexpected: %q", s)
	}

	// Relay fallback with error.
	r = SelectionResult{
		Method:    MethodRelay,
		Fallback:  FallbackDialFailed,
		DirectErr: errors.New("connection refused"),
	}
	s = r.String()
	if s == "" {
		t.Error("expected non-empty string")
	}

	// Relay fallback without error (no direct addr).
	r = SelectionResult{
		Method:   MethodRelay,
		Fallback: FallbackNoDirectAddr,
	}
	s = r.String()
	if s == "" {
		t.Error("expected non-empty string")
	}
}
