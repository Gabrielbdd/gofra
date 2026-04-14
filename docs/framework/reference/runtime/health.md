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
error for failure. The context carries a 2-second timeout per check.

```go
type Check struct {
    Name string
    Fn   CheckFunc
}
```

Pairs a human-readable name with a health check function. If `Fn` is `nil`,
the check is treated as healthy (`"ok"`).

```go
type Checker struct { /* unexported fields */ }
```

Manages startup, liveness, and readiness state and exposes HTTP handlers for
each probe. Thread-safe — all state transitions use atomic operations.

### Functions

```go
func New(checks ...Check) *Checker
```

Creates a `Checker` with the given readiness checks. Checks are evaluated in
order on every readiness probe request. Passing no checks creates a checker
with no dependency checks (readiness depends only on startup/shutdown state).

```go
func (c *Checker) MarkStarted()
```

Signals that application initialization is complete. Until called, the startup
and readiness probes return 503.

```go
func (c *Checker) SetNotReady()
```

Forces the readiness probe to return 503 regardless of check results. This is
permanent — there is no `SetReady()` to reverse it. Intended for use during
graceful shutdown.

```go
func (c *Checker) StartupHandler() http.Handler
func (c *Checker) LivenessHandler() http.Handler
func (c *Checker) ReadinessHandler() http.Handler
```

Return HTTP handlers for each probe type. All handlers accept `GET` and `HEAD`
methods only; other methods receive `405 Method Not Allowed` with an
`Allow: GET, HEAD` header.

### Constants

| Constant | Value | Description |
|----------|-------|-------------|
| `DefaultStartupPath` | `"/startupz"` | Kubernetes-convention startup probe path |
| `DefaultLivenessPath` | `"/livez"` | Kubernetes-convention liveness probe path |
| `DefaultReadinessPath` | `"/readyz"` | Kubernetes-convention readiness probe path |

## Defaults

| Setting | Value |
|---------|-------|
| Startup path | `/startupz` |
| Liveness path | `/livez` |
| Readiness path | `/readyz` |
| Per-check timeout | 2 seconds |
| Content-Type | `application/json` |

## Behavior

### Response Format

All probes return JSON with `Content-Type: application/json`. For `HEAD`
requests, the status code is set but the body is omitted.

### Startup Probe

| State | Status | Body |
|-------|--------|------|
| Before `MarkStarted()` | `503` | `{"status":"starting"}` |
| After `MarkStarted()` | `200` | `{"status":"started"}` |

### Liveness Probe

| State | Status | Body |
|-------|--------|------|
| Always | `200` | `{"status":"alive"}` |

The liveness probe always returns 200 regardless of startup state. It does not
check external dependencies. The only signal is whether the process can
respond to HTTP at all.

### Readiness Probe

| State | Status | Body |
|-------|--------|------|
| Before `MarkStarted()` | `503` | `{"status":"starting"}` |
| After `SetNotReady()` | `503` | `{"status":"shutting_down"}` |
| All checks pass | `200` | `{"status":"ready","checks":{"name":"ok",...}}` |
| Any check fails | `503` | `{"status":"not_ready","checks":{"name":"error message",...}}` |
| No checks registered | `200` | `{"status":"ready"}` |

When checks are registered, the `checks` field is a map of check name to
result string (`"ok"` or the error message).

**All checks run on every request** — a failing check does not short-circuit
evaluation of subsequent checks. This means the response always reports the
status of every registered check, making it useful for diagnostics.

Each check runs with a 2-second context timeout. If a check does not return
within 2 seconds, the context is cancelled.

When no checks are registered, the `checks` field is omitted from the
response entirely.

## Errors and Edge Cases

- If a `CheckFunc` panics, the panic propagates to the HTTP server's recovery
  middleware (if any).
- If a check's `Fn` is `nil`, it is treated as healthy and reports `"ok"`.
- Checks with the same name are allowed but the last one's result wins in
  the response map, making earlier results invisible.
- `SetNotReady()` is permanent — there is no way to reverse it.
- `MarkStarted()` is idempotent — calling it multiple times is safe.

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

Example readiness response with checks:

```json
{
  "status": "ready",
  "checks": {
    "postgres": "ok"
  }
}
```

Example readiness response with a failure:

```json
{
  "status": "not_ready",
  "checks": {
    "postgres": "connection refused",
    "redis": "ok"
  }
}
```

## Related Pages

- [runtime/serve](serve.md) — Integrates with the health checker for
  graceful shutdown via the `Health` interface.
- [Generated App Layout](../starter/generated-app-layout.md) — Shows health
  check wiring in the starter app.
