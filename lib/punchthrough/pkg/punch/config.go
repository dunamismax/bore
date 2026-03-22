package punch

import "time"

// Config controls hole-punch attempt behavior.
type Config struct {
	// MaxAttempts is the maximum number of punch packets to send before giving up.
	// Default: 5.
	MaxAttempts int

	// RetryInterval is the time between consecutive punch packet sends.
	// Default: 500ms.
	RetryInterval time.Duration

	// Timeout is the overall deadline for the punch attempt, independent of
	// individual retry intervals. If the context already has a deadline, the
	// shorter of the two is used. Default: 10s.
	Timeout time.Duration

	// HandshakeTimeout is the time allowed for the verification handshake
	// after a punch response is received. Default: 3s.
	HandshakeTimeout time.Duration
}

// maxAttempts returns the configured max attempts or the default.
func (c *Config) maxAttempts() int {
	if c.MaxAttempts > 0 {
		return c.MaxAttempts
	}
	return 5
}

// retryInterval returns the configured retry interval or the default.
func (c *Config) retryInterval() time.Duration {
	if c.RetryInterval > 0 {
		return c.RetryInterval
	}
	return 500 * time.Millisecond
}

// timeout returns the configured overall timeout or the default.
func (c *Config) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return 10 * time.Second
}

// handshakeTimeout returns the configured handshake timeout or the default.
func (c *Config) handshakeTimeout() time.Duration {
	if c.HandshakeTimeout > 0 {
		return c.HandshakeTimeout
	}
	return 3 * time.Second
}
