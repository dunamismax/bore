// Package metrics -- simple log-based alerting for relay health.
//
// Alerts checks relay counters periodically and logs warnings when
// thresholds are exceeded. This is intentionally minimal: no webhook
// integrations, no external dependencies. Operators can tail relay logs
// or pipe them through existing alerting infrastructure.
package metrics

import (
	"context"
	"log/slog"
	"time"
)

// AlertConfig controls the relay health alerting thresholds.
type AlertConfig struct {
	// CheckInterval is how often the alert checker runs.
	// Zero defaults to 60 seconds.
	CheckInterval time.Duration

	// RateLimitThreshold triggers a warning when cumulative rate limit
	// hits exceed this value since the last check. Zero disables.
	RateLimitThreshold int64

	// WSErrorThreshold triggers a warning when cumulative WebSocket
	// errors exceed this value since the last check. Zero disables.
	WSErrorThreshold int64

	// RoomUtilizationPct triggers a warning when room utilization
	// exceeds this percentage of MaxRooms. Zero disables.
	RoomUtilizationPct float64
}

// DefaultAlertConfig returns sensible production defaults.
func DefaultAlertConfig() AlertConfig {
	return AlertConfig{
		CheckInterval:      60 * time.Second,
		RateLimitThreshold: 50,
		WSErrorThreshold:   20,
		RoomUtilizationPct: 80.0,
	}
}

// RoomSnapshot provides room utilization data for alerting.
type RoomSnapshot struct {
	TotalRooms int
	MaxRooms   int
}

// RunAlerts starts a background goroutine that periodically checks relay
// health counters and logs warnings. It stops when ctx is cancelled.
//
// The roomFn callback provides current room utilization without coupling
// the metrics package to the room package.
func (c *Counters) RunAlerts(ctx context.Context, cfg AlertConfig, roomFn func() RoomSnapshot) {
	interval := cfg.CheckInterval
	if interval == 0 {
		interval = 60 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastRateLimitHits int64
	var lastWSErrors int64

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap := c.Snapshot()

			// Check rate limit hits since last check.
			if cfg.RateLimitThreshold > 0 {
				delta := snap.RateLimitHits - lastRateLimitHits
				if delta > cfg.RateLimitThreshold {
					slog.Warn("relay alert: high rate limit activity",
						"hits_since_last_check", delta,
						"threshold", cfg.RateLimitThreshold,
						"total_hits", snap.RateLimitHits,
					)
				}
			}
			lastRateLimitHits = snap.RateLimitHits

			// Check WebSocket errors since last check.
			if cfg.WSErrorThreshold > 0 {
				delta := snap.WSErrors - lastWSErrors
				if delta > cfg.WSErrorThreshold {
					slog.Warn("relay alert: elevated WebSocket errors",
						"errors_since_last_check", delta,
						"threshold", cfg.WSErrorThreshold,
						"total_errors", snap.WSErrors,
					)
				}
			}
			lastWSErrors = snap.WSErrors

			// Check room utilization.
			if cfg.RoomUtilizationPct > 0 && roomFn != nil {
				rooms := roomFn()
				if rooms.MaxRooms > 0 {
					pct := float64(rooms.TotalRooms) / float64(rooms.MaxRooms) * 100
					if pct > cfg.RoomUtilizationPct {
						slog.Warn("relay alert: room utilization high",
							"current_rooms", rooms.TotalRooms,
							"max_rooms", rooms.MaxRooms,
							"utilization_pct", pct,
							"threshold_pct", cfg.RoomUtilizationPct,
						)
					}
				}
			}
		}
	}
}
