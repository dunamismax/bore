package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dunamismax/bore/internal/relay/metrics"
	"github.com/dunamismax/bore/internal/relay/room"
	"github.com/dunamismax/bore/internal/relay/transport"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "relay: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	addr := ":8080"
	if v := os.Getenv("RELAY_ADDR"); v != "" {
		addr = v
	}

	counters := metrics.NewCounters()

	regCfg := room.DefaultRegistryConfig()
	regCfg.OnExpire = func(_ string) {
		counters.RoomExpired()
	}
	reg := room.NewRegistry(regCfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reaperDone := reg.RunReaper(ctx)

	// Start log-based relay health alerting.
	alertCfg := metrics.DefaultAlertConfig()
	go counters.RunAlerts(ctx, alertCfg, func() metrics.RoomSnapshot {
		snap := reg.Snapshot()
		return metrics.RoomSnapshot{
			TotalRooms: snap.TotalRooms,
			MaxRooms:   snap.MaxRooms,
		}
	})

	cfg := transport.DefaultServerConfig()
	cfg.Addr = addr
	cfg.Registry = reg
	cfg.Logger = logger
	cfg.Counters = counters

	srv := transport.NewServer(cfg)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	logger.Info("relay server starting",
		"addr", ln.Addr().String(),
		"readTimeout", cfg.ReadTimeout,
		"writeTimeout", cfg.WriteTimeout,
		"idleTimeout", cfg.IdleTimeout,
		"wsRateLimit", fmt.Sprintf("%d/%s", cfg.WSRateLimit.Rate, cfg.WSRateLimit.Window),
	)

	// Shutdown on SIGTERM/SIGINT.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()

	select {
	case sig := <-sigCh:
		logger.Info("received signal, shutting down", "signal", sig.String())
		cancel()
		<-reaperDone
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		cancel()
		<-reaperDone
		return err
	}
}
