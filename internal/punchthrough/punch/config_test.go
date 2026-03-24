package punch

import (
	"testing"
	"time"
)

func TestConfig_Defaults(t *testing.T) {
	cfg := &Config{}

	if got := cfg.maxAttempts(); got != 5 {
		t.Errorf("default maxAttempts = %d, want 5", got)
	}
	if got := cfg.retryInterval(); got != 500*time.Millisecond {
		t.Errorf("default retryInterval = %v, want 500ms", got)
	}
	if got := cfg.timeout(); got != 10*time.Second {
		t.Errorf("default timeout = %v, want 10s", got)
	}
	if got := cfg.handshakeTimeout(); got != 3*time.Second {
		t.Errorf("default handshakeTimeout = %v, want 3s", got)
	}
}

func TestConfig_Custom(t *testing.T) {
	cfg := &Config{
		MaxAttempts:      10,
		RetryInterval:    200 * time.Millisecond,
		Timeout:          30 * time.Second,
		HandshakeTimeout: 5 * time.Second,
	}

	if got := cfg.maxAttempts(); got != 10 {
		t.Errorf("custom maxAttempts = %d, want 10", got)
	}
	if got := cfg.retryInterval(); got != 200*time.Millisecond {
		t.Errorf("custom retryInterval = %v, want 200ms", got)
	}
	if got := cfg.timeout(); got != 30*time.Second {
		t.Errorf("custom timeout = %v, want 30s", got)
	}
	if got := cfg.handshakeTimeout(); got != 5*time.Second {
		t.Errorf("custom handshakeTimeout = %v, want 5s", got)
	}
}

func TestConfig_ZeroValuesFallToDefaults(t *testing.T) {
	cfg := &Config{
		MaxAttempts:      0,
		RetryInterval:    0,
		Timeout:          0,
		HandshakeTimeout: 0,
	}

	if got := cfg.maxAttempts(); got != 5 {
		t.Errorf("zero maxAttempts = %d, want default 5", got)
	}
	if got := cfg.retryInterval(); got != 500*time.Millisecond {
		t.Errorf("zero retryInterval = %v, want default 500ms", got)
	}
	if got := cfg.timeout(); got != 10*time.Second {
		t.Errorf("zero timeout = %v, want default 10s", got)
	}
	if got := cfg.handshakeTimeout(); got != 3*time.Second {
		t.Errorf("zero handshakeTimeout = %v, want default 3s", got)
	}
}
