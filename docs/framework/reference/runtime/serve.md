# runtime/serve

> Graceful HTTP server lifecycle with signal handling and shutdown sequencing.

## Status

Alpha — API may change before v1.

## Import

```go
import "databit.com.br/gofra/runtime/serve"
```

The package is named `runtimeserve` in code.

## API

### Serve

```go
func Serve(ctx context.Context, cfg Config) error
```

Starts an HTTP server and blocks until shutdown completes. Shutdown is
triggered by context cancellation or receipt of `SIGINT`/`SIGTERM`. Returns
`nil` on clean shutdown.

### Config

```go
type Config struct {
    Handler                 http.Handler
    Addr                    string
    Health                  Health
    Logger                  *slog.Logger
    ReadinessDrainDelay     time.Duration
    ShutdownTimeout         time.Duration
    ResourceShutdownTimeout time.Duration
    OnShutdown              func(context.Context) error
}
```

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `Handler` | Yes | — | Root HTTP handler to serve |
| `Addr` | Yes | — | TCP address to listen on (e.g., `":3000"`) |
| `Health` | No | `nil` | Lifecycle callbacks for health state |
| `Logger` | No | `slog.Default()` | Structured logger for server events |
| `ReadinessDrainDelay` | No | `2s` | Pause between marking not-ready and starting HTTP shutdown |
| `ShutdownTimeout` | No | `15s` | Max time for in-flight requests to complete |
| `ResourceShutdownTimeout` | No | `3s` | Max time for the `OnShutdown` callback |
| `OnShutdown` | No | `nil` | Called after HTTP shutdown for resource cleanup |

### Health Interface

```go
type Health interface {
    MarkStarted()
    SetNotReady()
}
```

If provided, `Serve` calls `MarkStarted()` after the TCP listener binds
successfully and `SetNotReady()` at the start of shutdown. Both methods are
called exactly once. The `runtime/health.Checker` type satisfies this
interface.

### Constants

| Constant | Value | Description |
|----------|-------|-------------|
| `DefaultReadinessDrainDelay` | `2s` | Default pause before HTTP shutdown |
| `DefaultShutdownTimeout` | `15s` | Default in-flight request deadline |
| `DefaultResourceShutdownTimeout` | `3s` | Default resource cleanup deadline |

## Defaults

| Setting | Default |
|---------|---------|
| Readiness drain delay | 2 seconds |
| Shutdown timeout | 15 seconds |
| Resource shutdown timeout | 3 seconds |
| Logger | `slog.Default()` |
| `ReadHeaderTimeout` | 10 seconds (hardcoded) |
| `IdleTimeout` | 60 seconds (hardcoded) |

The total shutdown budget (2s + 15s + 3s = 20s) fits within the default
Kubernetes `terminationGracePeriodSeconds` of 30 seconds.

## Behavior

### Startup

1. Bind to `Addr` via `net.Listen("tcp", addr)`.
2. Call `Health.MarkStarted()` if a health provider is configured.
3. Log: `"server started"` with field `addr`.
4. Begin serving HTTP requests.

The HTTP server is configured with `ReadHeaderTimeout: 10s` and
`IdleTimeout: 60s` to prevent slowloris attacks and idle connection buildup.

### Signal Handling

`Serve` listens for `SIGINT` and `SIGTERM` using `signal.NotifyContext`. On
the first signal, graceful shutdown begins.

After the first signal, default signal handlers are restored. A second
`SIGINT` or `SIGTERM` will force-kill the process immediately (standard Go
behavior when signal handlers are removed).

### Shutdown Sequence

When the context is cancelled or a signal is received:

1. **Phase 1 — Readiness drain:** Log `"shutdown starting"`. Call
   `Health.SetNotReady()` and sleep for `ReadinessDrainDelay` (default 2s).
   This gives load balancers time to observe the not-ready probe and stop
   sending traffic.

2. **Phase 2 — HTTP shutdown:** Call `http.Server.Shutdown()` with
   `ShutdownTimeout` (default 15s). In-flight requests are allowed to
   complete; new connections are refused. If graceful shutdown fails (timeout
   exceeded), the server is force-closed with `srv.Close()` and the error is
   logged.

3. **Phase 3 — Resource cleanup:** Call `OnShutdown` (if set) with a context
   that has `ResourceShutdownTimeout` (default 3s). If the callback does not
   return before the deadline, `Serve` proceeds without waiting and the
   callback receives `context.DeadlineExceeded` via the context. Errors from
   `OnShutdown` are logged but do not change the return value.

4. Log: `"shutdown complete"`.

### Log Messages

All log messages use the configured `slog.Logger`:

| Message | Level | Fields | When |
|---------|-------|--------|------|
| `"server started"` | Info | `addr` | After listener binds |
| `"shutdown starting"` | Info | — | First shutdown signal |
| `"http graceful shutdown failed, forcing close"` | Error | `error` | HTTP shutdown timeout |
| `"resource shutdown error"` | Error | `error` | `OnShutdown` returns error |
| `"shutdown complete"` | Info | — | All phases done |

## Errors and Edge Cases

- If `Addr` is already in use, `Serve` returns the `net.Listen` error
  immediately (does not start shutdown sequence).
- If the underlying `srv.Serve()` returns an error other than
  `http.ErrServerClosed`, `Serve` returns that error.
- `Serve` always returns `nil` on clean shutdown, even if individual
  phases had logged errors.
- If `OnShutdown` exceeds its timeout, `Serve` does not wait for it to
  return — it logs `context.DeadlineExceeded` and proceeds. The goroutine
  running the callback is abandoned.
- If `Health` is `nil`, `MarkStarted()` and `SetNotReady()` calls are
  skipped.
- Zero-value timeouts in `Config` are replaced with their defaults.

## Examples

```go
checker := runtimehealth.New()

mux := http.NewServeMux()
mux.Handle("/", appHandler)
mux.Handle(runtimehealth.DefaultStartupPath, checker.StartupHandler())
mux.Handle(runtimehealth.DefaultLivenessPath, checker.LivenessHandler())
mux.Handle(runtimehealth.DefaultReadinessPath, checker.ReadinessHandler())

err := runtimeserve.Serve(context.Background(), runtimeserve.Config{
    Handler: mux,
    Addr:    ":8080",
    Health:  checker,
    OnShutdown: func(ctx context.Context) error {
        return db.Close()
    },
})
```

## Related Pages

- [runtime/health](health.md) — Provides the `Health` interface
  implementation.
- [runtime/config](config.md) — Loads the config that supplies the server
  address.
- [Generated App Layout](../starter/generated-app-layout.md) — Shows the
  full wiring in the starter app.
