# 12 — Graceful Shutdown

> Parent: [Index](00-index.md) | Prev: [Health Checks](11-health-checks.md) | Next: [Frontend](13-frontend.md)


## Addendum to Architecture Design Document
## Last Updated: 2026-04-12

---

## The Problem

A Gofra application runs two network listeners in one process:

1. **HTTP server** (`:3000`) — serves Connect RPC handlers and the SPA.
   Clients have in-flight requests that must complete.
2. **Restate service endpoint** (`:9080`) — receives invocations from the
   Restate Server. In-flight invocations are journaled by Restate and will
   be retried on another instance if this one dies — but a clean shutdown
   avoids unnecessary retries.

Plus shared resources: a database connection pool and the OTEL trace provider.

On SIGTERM, the process must:
- Stop receiving new requests
- Let in-flight requests finish
- Let the load balancer stop sending traffic (readiness drain)
- Close connections in the right order
- Exit within a deadline (Kubernetes default: 30 seconds)

Getting this wrong means: dropped requests (502s for users), leaked database
connections, lost OTEL spans, and unnecessary Restate invocation retries.

---

## Shutdown Sequence

```
SIGTERM received
    │
    │  Phase 1: Signal readiness drain (2 seconds)
    │  ─────────────────────────────────────────────
    │  • Set readiness probe to 503 (stop receiving new traffic)
    │  • Wait 2 seconds for load balancer to deregister
    │  • New requests during this window still get served
    │
    │  Phase 2: Stop HTTP listener (up to 15 seconds)
    │  ─────────────────────────────────────────────
    │  • http.Server.Shutdown() — stops accepting new connections
    │  • Waits for in-flight HTTP requests to complete
    │  • Deadline: 15 seconds (then force-close)
    │
    │  Phase 3: Stop Restate endpoint (up to 5 seconds)
    │  ─────────────────────────────────────────────
    │  • Cancel the Restate server's context
    │  • In-flight Restate invocations complete or are interrupted
    │  • Restate Server detects the endpoint is gone and retries
    │    interrupted invocations on another instance
    │
    │  Phase 4: Close resources
    │  ─────────────────────────────────────────────
    │  • Flush OTEL trace provider (export pending spans)
    │  • Close database connection pool
    │  • Exit
    │
    ▼
Process exits (total budget: ~25 seconds of 30-second K8s grace period)
```

**Reason for this order**: HTTP clients are humans waiting for responses —
they must be served first. Restate invocations are durable — if interrupted,
Restate retries them automatically on another instance. Database connections
close last because both HTTP handlers and Restate handlers may use them
during their drain periods. OTEL flushes before the process dies so the
final spans aren't lost.

**Reason for 2-second readiness drain**: Between SIGTERM and the load
balancer deregistering the pod, there's a race. Kubernetes removes the pod
from the Service endpoints asynchronously. During this window, the LB may
still send requests. Marking readiness as 503 immediately and waiting 2
seconds gives the LB time to deregister before we stop accepting connections.
Without this delay, new requests arrive to a closed listener and get
connection refused errors.

---

## Implementation

```go
// gofra/serve.go
package gofra

import (
    "context"
    "errors"
    "log/slog"
    "net"
    "net/http"
    "os/signal"
    "sync/atomic"
    "syscall"
    "time"

    "github.com/restatedev/sdk-go/server"
)

type ServeConfig struct {
    HTTPHandler  http.Handler
    HTTPAddr     string          // ":3000"
    RestateSetup func() (*server.Restate, error)
    RestateAddr  string          // ":9080"
    Health       *HealthChecker
    OnShutdown   func(ctx context.Context) error // OTEL flush, DB close
}

const (
    readinessDrainDelay = 2 * time.Second
    httpShutdownTimeout = 15 * time.Second
    restateStopTimeout  = 5 * time.Second
    hardShutdownTimeout = 3 * time.Second
)

func Serve(ctx context.Context, cfg ServeConfig) error {
    // ── Signal handling ──────────────────────────────────
    ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    // ── HTTP Server ──────────────────────────────────────
    httpServer := &http.Server{
        Addr:              cfg.HTTPAddr,
        Handler:           cfg.HTTPHandler,
        ReadHeaderTimeout: 10 * time.Second,
        IdleTimeout:       60 * time.Second,
    }

    // ── Restate Endpoint ─────────────────────────────────
    restateCtx, restateCancel := context.WithCancel(context.Background())
    defer restateCancel()

    // ── Start servers ────────────────────────────────────
    errCh := make(chan error, 2)

    go func() {
        slog.Info("http server starting", "addr", cfg.HTTPAddr)
        if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
            errCh <- err
        }
    }()

    go func() {
        slog.Info("restate endpoint starting", "addr", cfg.RestateAddr)
        rs, err := cfg.RestateSetup()
        if err != nil {
            errCh <- err
            return
        }
        if err := rs.Start(restateCtx, cfg.RestateAddr); err != nil && restateCtx.Err() == nil {
            errCh <- err
        }
    }()

    // Signal that startup is complete
    if cfg.Health != nil {
        cfg.Health.startupDone.Store(true)
    }

    slog.Info("gofra application started",
        "http", cfg.HTTPAddr,
        "restate", cfg.RestateAddr,
    )

    // ── Wait for signal or fatal error ───────────────────
    select {
    case err := <-errCh:
        // A server failed to start — fatal
        slog.Error("server error", "err", err)
        return err
    case <-ctx.Done():
        slog.Info("shutdown signal received")
    }

    // Restore default signal behavior so a second Ctrl+C force-kills
    stop()

    // ── Phase 1: Readiness drain ─────────────────────────
    if cfg.Health != nil {
        cfg.Health.SetNotReady()
    }
    slog.Info("readiness set to not-ready, draining",
        "delay", readinessDrainDelay,
    )
    time.Sleep(readinessDrainDelay)

    // ── Phase 2: Stop HTTP server ────────────────────────
    httpCtx, httpCancel := context.WithTimeout(context.Background(), httpShutdownTimeout)
    defer httpCancel()

    slog.Info("shutting down http server",
        "timeout", httpShutdownTimeout,
    )
    if err := httpServer.Shutdown(httpCtx); err != nil {
        slog.Error("http shutdown error", "err", err)
        // Force close if graceful shutdown timed out
        httpServer.Close()
    }
    slog.Info("http server stopped")

    // ── Phase 3: Stop Restate endpoint ───────────────────
    slog.Info("shutting down restate endpoint",
        "timeout", restateStopTimeout,
    )
    restateCancel()
    // Give Restate SDK a moment to finish in-flight work
    time.Sleep(restateStopTimeout)
    slog.Info("restate endpoint stopped")

    // ── Phase 4: Close resources ─────────────────────────
    if cfg.OnShutdown != nil {
        resourceCtx, resourceCancel := context.WithTimeout(
            context.Background(), hardShutdownTimeout,
        )
        defer resourceCancel()

        slog.Info("closing resources")
        if err := cfg.OnShutdown(resourceCtx); err != nil {
            slog.Error("resource cleanup error", "err", err)
        }
    }

    slog.Info("shutdown complete")
    return nil
}
```

### HealthChecker Integration

The `HealthChecker` from the health checks addendum gains a `SetNotReady()`
method:

```go
func (h *HealthChecker) SetNotReady() {
    h.shuttingDown.Store(true)
}

func (h *HealthChecker) ReadinessHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // During shutdown, always return 503
        if h.shuttingDown.Load() {
            w.WriteHeader(http.StatusServiceUnavailable)
            json.NewEncoder(w).Encode(map[string]string{"status": "shutting_down"})
            return
        }
        // ... normal readiness checks
    }
}
```

---

## Usage in main.go

```go
// cmd/app/main.go
func main() {
    cfg, _ := config.Load()

    // OTEL setup
    otelShutdown := setupOTEL(cfg)

    // Database
    db, _ := gofra.OpenDB(cfg.Database)
    queries := sqlc.New(db)

    // Health
    health := gofra.NewHealthChecker(db, cfg.Restate.IngressURL)

    // HTTP mux
    mux := chi.NewRouter()
    mux.Get("/healthz/startup", health.StartupHandler())
    mux.Get("/healthz/live", health.LivenessHandler())
    mux.Get("/healthz/ready", health.ReadinessHandler())
    mux.Use(gofra.CORSMiddleware(cfg.CORS))
    mux.Use(gofra.RecoveryMiddleware)
    // ... mount Connect handlers, SPA fallback ...

    // Restate setup (deferred — starts inside Serve)
    restateSetup := func() (*server.Restate, error) {
        rs := server.NewRestate()
        rs.Bind(restate.Reflect(services.MailService{Queries: queries}))
        rs.Bind(restate.Reflect(services.SearchIndexer{Queries: queries}))
        rs.Bind(restate.Reflect(workflows.OrderCheckout{Queries: queries}))
        return rs, nil
    }

    // Run
    err := gofra.Serve(context.Background(), gofra.ServeConfig{
        HTTPHandler:  mux,
        HTTPAddr:     fmt.Sprintf(":%d", cfg.App.Port),
        RestateSetup: restateSetup,
        RestateAddr:  fmt.Sprintf(":%d", cfg.Restate.ServicePort),
        Health:       health,
        OnShutdown: func(ctx context.Context) error {
            var errs []error
            errs = append(errs, otelShutdown(ctx))
            db.Close()
            return errors.Join(errs...)
        },
    })
    if err != nil {
        slog.Error("application error", "err", err)
        os.Exit(1)
    }
}
```

---

## What Happens to Restate Invocations During Shutdown

When the Restate service endpoint shuts down:

1. **Completed steps are safe.** Restate journaled them. They won't re-execute.
2. **In-flight `Run` blocks** are interrupted. The `Run` block's work may
   or may not have completed. If it didn't complete, Restate retries the
   invocation on another instance, replaying completed steps from the journal
   and re-executing the interrupted `Run` block.
3. **Suspended invocations** (waiting on Awakeables, Durable Promises, or
   sleep) are unaffected. Their state lives in the Restate Server, not in
   the endpoint process. They'll resume on any available instance.

**Reason this is safe**: Restate's execution model guarantees that `Run` blocks
are either fully committed to the journal or retried. The developer doesn't
need to handle shutdown in Restate handlers — the SDK and server handle it.
This is the fundamental benefit of durable execution: process shutdown is a
normal event, not an error condition.

---

## Kubernetes Alignment

The shutdown budget must fit within Kubernetes' `terminationGracePeriodSeconds`
(default: 30 seconds):

```
Phase 1: Readiness drain         2 seconds
Phase 2: HTTP shutdown          15 seconds
Phase 3: Restate stop            5 seconds
Phase 4: Resource cleanup        3 seconds
────────────────────────────────────────────
Total                           25 seconds  (within 30-second budget)
```

**Reason for 5-second safety margin** (30 - 25 = 5): If any phase runs
slightly over its timeout, the total must still fit within 30 seconds.
After 30 seconds, Kubernetes sends SIGKILL — the process dies immediately
with no cleanup. The margin prevents this.

```yaml
# k8s/deployment.yaml
spec:
  terminationGracePeriodSeconds: 30  # default, matches our budget
```

If the application needs longer drains (e.g., very slow HTTP requests), increase
`terminationGracePeriodSeconds` and adjust the constants in `gofra/serve.go`.

---

## Second Signal Escalation

After SIGTERM, calling `stop()` restores default signal behavior. If the
developer presses Ctrl+C again (or the system sends a second SIGTERM), the
Go runtime handles it with the default behavior: immediate termination.

This is useful during development: "I sent SIGTERM and it's taking too long,
let me force-kill it." In production, Kubernetes sends SIGKILL after the
grace period anyway, so escalation is rarely needed.

---

## What the Logs Look Like

```
INFO  gofra application started            http=:3000 restate=:9080
...
INFO  shutdown signal received
INFO  readiness set to not-ready, draining  delay=2s
INFO  shutting down http server             timeout=15s
INFO  http server stopped
INFO  shutting down restate endpoint        timeout=5s
INFO  restate endpoint stopped
INFO  closing resources
INFO  shutdown complete
```

Every phase is logged with its timeout. If something hangs, the operator can
see where it stopped.

---

## Decision Log (Graceful Shutdown)

| # | Decision | Rationale |
|---|----------|-----------|
| 115 | `signal.NotifyContext` for SIGTERM/SIGINT | Standard Go pattern. Context-based cancellation integrates with the rest of the codebase. |
| 116 | Four-phase shutdown: drain → HTTP → Restate → resources | HTTP clients are humans waiting. Restate invocations are durable (automatically retried). DB closes last because both use it. |
| 117 | 2-second readiness drain delay | Gives the load balancer time to deregister the pod before the HTTP listener closes. Prevents connection-refused errors during the race window. |
| 118 | `http.Server.Shutdown()` for HTTP | Standard library. Stops accepting new connections, waits for in-flight requests to complete, respects a deadline. |
| 119 | Context cancellation for Restate endpoint | `rs.Start(ctx, addr)` respects context cancellation. Cancelling the context tells the SDK to stop accepting new invocations. |
| 120 | Restate invocations are safe to interrupt | Completed `Run` steps are journaled. Interrupted invocations are retried on another instance. Durable execution makes shutdown a non-event. |
| 121 | 25-second total budget within 30-second K8s grace | 5-second safety margin before SIGKILL. Each phase has its own timeout so no single phase can consume the entire budget. |
| 122 | Second signal force-kills | `stop()` after `ctx.Done()` restores default behavior. Ctrl+C twice = immediate exit. Useful during development. |
| 123 | `OnShutdown` callback for resource cleanup | OTEL flush and DB close are application-specific. The framework provides the hook; `main.go` provides the logic. |
