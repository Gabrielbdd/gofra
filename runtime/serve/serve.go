// Package runtimeserve manages the HTTP server lifecycle including startup,
// signal handling, and graceful multi-phase shutdown. It is stdlib-only and
// does not depend on any specific router, database, or service mesh.
package runtimeserve

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

// Default timeout values for the shutdown sequence. These fit within a
// 30-second Kubernetes terminationGracePeriodSeconds budget.
const (
	DefaultReadinessDrainDelay     = 2 * time.Second
	DefaultShutdownTimeout         = 15 * time.Second
	DefaultResourceShutdownTimeout = 3 * time.Second
)

// Health is the interface that runtimeserve uses to coordinate lifecycle
// state with a health checker. Both methods are called exactly once.
type Health interface {
	// MarkStarted is called after the TCP listener binds successfully.
	MarkStarted()
	// SetNotReady is called at the beginning of the shutdown sequence.
	SetNotReady()
}

// Config holds the parameters for Serve. Only Handler and Addr are required.
type Config struct {
	// Handler is the root HTTP handler to serve.
	Handler http.Handler

	// Addr is the TCP address to listen on, e.g. ":3000".
	Addr string

	// Health receives lifecycle callbacks. Nil is safe.
	Health Health

	// Logger is used for structured startup and shutdown messages.
	// Defaults to slog.Default() if nil.
	Logger *slog.Logger

	// ReadinessDrainDelay is the pause between marking not-ready and
	// beginning HTTP shutdown. This gives load balancers time to stop
	// sending new traffic. Default: 2s.
	ReadinessDrainDelay time.Duration

	// ShutdownTimeout is the maximum time to wait for in-flight HTTP
	// requests to complete. Default: 15s.
	ShutdownTimeout time.Duration

	// ResourceShutdownTimeout is the maximum time given to OnShutdown
	// for resource cleanup (OTEL flush, DB close, etc.). Default: 3s.
	ResourceShutdownTimeout time.Duration

	// OnShutdown is called after HTTP shutdown completes. Use it to flush
	// telemetry, close database pools, or release other resources. Nil is
	// safe.
	OnShutdown func(context.Context) error
}

// Serve starts an HTTP server and blocks until shutdown completes.
//
// Shutdown is triggered by context cancellation or by SIGINT/SIGTERM. The
// shutdown sequence is:
//
//  1. Mark health as not-ready and wait ReadinessDrainDelay.
//  2. Gracefully shut down the HTTP server (stop accepting, drain in-flight).
//  3. Call OnShutdown for resource cleanup.
//
// Serve returns nil on clean shutdown. It returns an error if the listener
// cannot bind or if shutdown encounters a fatal problem.
func Serve(ctx context.Context, cfg Config) error {
	cfg = applyDefaults(cfg)
	log := cfg.Logger

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}

	if cfg.Health != nil {
		cfg.Health.MarkStarted()
	}

	log.Info("server started", "addr", ln.Addr().String())

	srv := &http.Server{
		Handler:           cfg.Handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Wrap context with signal handling so either context cancellation or
	// SIGINT/SIGTERM triggers shutdown.
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start serving in the background.
	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	// Wait for shutdown signal or serve error.
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case <-ctx.Done():
	}

	// Restore default signal handlers so a second Ctrl+C force-kills.
	stop()

	log.Info("shutdown starting")

	// Phase 1: Readiness drain.
	if cfg.Health != nil {
		cfg.Health.SetNotReady()
	}
	time.Sleep(cfg.ReadinessDrainDelay)

	// Phase 2: Graceful HTTP shutdown.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("http shutdown error", "error", err)
	}

	// Phase 3: Resource cleanup.
	if cfg.OnShutdown != nil {
		resourceCtx, resourceCancel := context.WithTimeout(context.Background(), cfg.ResourceShutdownTimeout)
		defer resourceCancel()

		if err := cfg.OnShutdown(resourceCtx); err != nil {
			log.Error("resource shutdown error", "error", err)
		}
	}

	log.Info("shutdown complete")
	return nil
}

func applyDefaults(cfg Config) Config {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.ReadinessDrainDelay == 0 {
		cfg.ReadinessDrainDelay = DefaultReadinessDrainDelay
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = DefaultShutdownTimeout
	}
	if cfg.ResourceShutdownTimeout == 0 {
		cfg.ResourceShutdownTimeout = DefaultResourceShutdownTimeout
	}
	return cfg
}
