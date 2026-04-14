# runtime/health

> Kubernetes-aligned health check probes: startup, liveness, and readiness.

## Status

Alpha — API may change before v1.

## Import

```go
import "databit.com.br/gofra/runtime/health"
```

The package is named `runtimehealth` in code.

## API

### Types

```go
type CheckFunc func(context.Context) error
```

Reports whether a dependency is healthy. Return `nil` for healthy, a non-nil
error for failure.

```go
type Check struct {
    Name string
    Fn   CheckFunc
}
```

Pairs a human-readable name with a health check function.

```go
type Checker struct { /* unexported fields */ }
```

Manages startup, liveness, and readiness state and exposes HTTP handlers for
each probe.

### Functions

```go
func New(checks ...Check) *Checker
```

Creates a `Checker` with the given readiness checks. Checks are evaluated in
order on every readiness probe request.

```go
func (c *Checker) MarkStarted()
```

Signals that application initialization is complete. Until called, the startup
and readiness probes return 503.

```go
func (c *Checker) SetNotReady()
```

Forces the readiness probe to return 503 regardless of check results. Used
during graceful shutdown to drain traffic before stopping.

```go
func (c *Checker) StartupHandler() http.Handler
func (c *Checker) LivenessHandler() http.Handler
func (c *Checker) ReadinessHandler() http.Handler
```

Return HTTP handlers for each probe type.

### Constants

| Constant | Value | Description |
|----------|-------|-------------|
| `DefaultStartupPath` | `"/startupz"` | Kubernetes-convention startup probe path |
| `DefaultLivenessPath` | `"/livez"` | Kubernetes-convention liveness probe path |
| `DefaultReadinessPath` | `"/readyz"` | Kubernetes-convention readiness probe path |

## Defaults

- Startup path: `/startupz`
- Liveness path: `/livez`
- Readiness path: `/readyz`
- All probes return `503 Service Unavailable` until `MarkStarted()` is called.

## Behavior

### Startup Probe

Returns `200 OK` after `MarkStarted()` has been called. Returns
`503 Service Unavailable` before that. No dependency checks are executed.

### Liveness Probe

Always returns `200 OK` once started. Does not check external dependencies.
This tells the orchestrator the process is alive and not deadlocked.

### Readiness Probe

Returns `200 OK` if all of the following are true:

1. `MarkStarted()` has been called.
2. `SetNotReady()` has not been called.
3. All registered `Check` functions return `nil`.

Returns `503 Service Unavailable` otherwise. Checks run in order on every
request; the first failure short-circuits.

### Response Format

All probes return plain-text bodies: `"ok\n"` for 200, `"not ready\n"` or
`"not started\n"` for 503.

## Errors and Edge Cases

- If a `CheckFunc` panics, the panic propagates to the HTTP server's recovery
  middleware.
- Checks with the same name are allowed but may make debugging harder.
- `SetNotReady()` is permanent — there is no `SetReady()` to reverse it.

## Examples

```go
checker := runtimehealth.New(
    runtimehealth.Check{
        Name: "postgres",
        Fn: func(ctx context.Context) error {
            return db.PingContext(ctx)
        },
    },
)

mux := http.NewServeMux()
mux.Handle(runtimehealth.DefaultStartupPath, checker.StartupHandler())
mux.Handle(runtimehealth.DefaultLivenessPath, checker.LivenessHandler())
mux.Handle(runtimehealth.DefaultReadinessPath, checker.ReadinessHandler())

// After initialization is complete:
checker.MarkStarted()
```

## Related Pages

- [runtime/serve](serve.md) — Integrates with the health checker for
  graceful shutdown.
- [Generated App Layout](../starter/generated-app-layout.md) — Shows health
  check wiring in the starter app.
