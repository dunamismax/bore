// Package ratelimit provides IP-based rate limiting for the relay server.
//
// It implements a token bucket algorithm per IP address with automatic
// cleanup of stale entries. The limiter is safe for concurrent use.
package ratelimit

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// Config configures a rate limiter.
type Config struct {
	// Rate is the number of allowed events per Window.
	Rate int

	// Window is the time window for the rate limit.
	Window time.Duration

	// CleanupInterval is how often stale entries are removed.
	// Zero defaults to 1 minute.
	CleanupInterval time.Duration
}

// bucket tracks token consumption for a single IP.
type bucket struct {
	tokens    int
	lastReset time.Time
}

// Limiter enforces per-IP rate limits using a token bucket algorithm.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	config  Config
	stopCh  chan struct{}
}

// NewLimiter creates a new rate limiter with the given configuration.
// Call Stop when the limiter is no longer needed to release resources.
func NewLimiter(cfg Config) *Limiter {
	if cfg.CleanupInterval == 0 {
		cfg.CleanupInterval = time.Minute
	}

	l := &Limiter{
		buckets: make(map[string]*bucket),
		config:  cfg,
		stopCh:  make(chan struct{}),
	}

	go l.cleaner()
	return l
}

// Allow checks whether a request from the given IP is allowed.
// Returns true if the request is within the rate limit, false otherwise.
func (l *Limiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, ok := l.buckets[ip]
	if !ok {
		l.buckets[ip] = &bucket{
			tokens:    l.config.Rate - 1,
			lastReset: now,
		}
		return true
	}

	// Reset tokens if the window has elapsed.
	if now.Sub(b.lastReset) >= l.config.Window {
		b.tokens = l.config.Rate - 1
		b.lastReset = now
		return true
	}

	if b.tokens > 0 {
		b.tokens--
		return true
	}

	return false
}

// Stop releases the background cleanup goroutine.
func (l *Limiter) Stop() {
	close(l.stopCh)
}

// cleaner periodically removes entries that have not been touched
// within 2× the configured window.
func (l *Limiter) cleaner() {
	ticker := time.NewTicker(l.config.CleanupInterval)
	defer ticker.Stop()

	staleThreshold := 2 * l.config.Window

	for {
		select {
		case <-l.stopCh:
			return
		case now := <-ticker.C:
			l.mu.Lock()
			for ip, b := range l.buckets {
				if now.Sub(b.lastReset) > staleThreshold {
					delete(l.buckets, ip)
				}
			}
			l.mu.Unlock()
		}
	}
}

// Len returns the number of tracked IPs. Useful for testing and metrics.
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}

// ExtractIP extracts the IP address from an HTTP request, stripping
// the port. It does not trust X-Forwarded-For by default.
func ExtractIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
