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
| `Addr` | Yes | — | TCP address to listen on (e.g., `":8080"`) |
| `Health` | No | `nil` | Lifecycle callbacks for health state |
| `Logger` | No | `slog.Default()` | Structured logger for server events |
| `ReadinessDrainDelay` | No | `2s` | Pause between marking not-ready and starting HTTP shutdown |
| `ShutdownTimeout` | No | `15s` | Max time for in-flight requests to complete |
| `ResourceShutdownTimeout` | No | `3s` | Max time for the `OnShutdown` callback |
| `OnShutdown` | No | `nil` | Called after HTTP shutdown for resource cleanup (e.g., closing DB pools) |

### Health Interface

```go
type Health interface {
    MarkStarted()
    SetNotReady()
}
```

If provided, `Serve` calls `MarkStarted()` after the listener is ready and
`SetNotReady()` at the start of shutdown. The `runtime/health.Checker` type
satisfies this interface.

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

## Behavior

### Startup

1. Bind to `Addr` and start accepting connections.
2. Call `Health.MarkStarted()` if a health provider is configured.
3. Log the listening address.

### Shutdown Sequence

When the context is cancelled or a signal is received:

1. **Drain phase:** Call `Health.SetNotReady()` and wait
   `ReadinessDrainDelay`. This gives load balancers time to stop sending
   traffic.
2. **HTTP shutdown:** Call `http.Server.Shutdown()` with `ShutdownTimeout`.
   In-flight requests are allowed to complete; new connections are refused.
3. **Resource cleanup:** Call `OnShutdown` (if set) with
   `ResourceShutdownTimeout`. Use this for closing database pools, flushing
   buffers, etc.

Each phase has an independent timeout. If a phase exceeds its timeout, the
server proceeds to the next phase.

### Signal Handling

`Serve` listens for `SIGINT` and `SIGTERM`. On the first signal, graceful
shutdown begins. The context passed to `Serve` can also trigger shutdown via
cancellation.

## Errors and Edge Cases

- If `Addr` is already in use, `Serve` returns an error immediately.
- If `Handler` is `nil`, the behavior is the same as `http.DefaultServeMux`.
- If `OnShutdown` returns an error, it is logged but does not change the
  return value of `Serve` (which returns `nil` for clean shutdown).
- If shutdown times out, `Serve` still returns `nil` — the timeouts are
  enforced but the server process continues to exit.

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
