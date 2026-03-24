package stun

import "time"

// DefaultSTUNServers is the default list of public STUN servers used for NAT probing.
// These are well-known, highly available servers from Google and Cloudflare.
var DefaultSTUNServers = []string{
	"stun.l.google.com:19302",
	"stun1.l.google.com:19302",
	"stun.cloudflare.com:3478",
}

// Config controls STUN probing behavior.
type Config struct {
	// Servers is the list of STUN servers to probe. If empty, DefaultSTUNServers is used.
	Servers []string

	// Timeout is the per-probe timeout. Default: 5s.
	Timeout time.Duration

	// LocalAddr is an optional local address to bind UDP sockets to.
	// If nil, the OS chooses.
	LocalAddr *string
}

// servers returns the configured STUN servers or the defaults.
func (c *Config) servers() []string {
	if len(c.Servers) > 0 {
		return c.Servers
	}
	return DefaultSTUNServers
}

// timeout returns the configured per-probe timeout or the default.
func (c *Config) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return 5 * time.Second
}
