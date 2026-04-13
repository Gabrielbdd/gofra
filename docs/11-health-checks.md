# 11 — Health Checks

> Parent: [Index](00-index.md) | Prev: [CORS](10-cors.md) | Next: [Graceful Shutdown](12-graceful-shutdown.md)


## Addendum to Architecture Design Document
## Last Updated: 2026-04-12

---

## Cloud-Native Health Check Model

Kubernetes defines three types of probes, each answering a different question.
Gofra implements all three as plain HTTP endpoints on the chi mux — not
Connect RPC methods — because probes are infrastructure concerns consumed by
orchestrators, not API clients.

| Probe | Question | Failure Action | Endpoint |
|-------|----------|----------------|----------|
| **Startup** | Has the app finished initializing? | Block liveness/readiness checks. Keep retrying. | `GET /healthz/startup` |
| **Liveness** | Is the process alive and not deadlocked? | Kill and restart the container. | `GET /healthz/live` |
| **Readiness** | Can the app handle traffic right now? | Remove from load balancer. Stop sending requests. Don't restart. | `GET /healthz/ready` |

**Reason for three separate endpoints**: A single `/healthz` conflates "is
the process alive?" with "can it serve traffic?" If the database is
temporarily unreachable, the process is alive (liveness = pass) but shouldn't
receive traffic (readiness = fail). Using one endpoint for both would cause
Kubernetes to restart the container when it should only stop routing traffic —
and the restart would not fix the database.

**Reason for HTTP endpoints, not Connect RPC methods**: Kubernetes probes
use plain HTTP GET. They don't speak Connect protocol. Load balancers (ALB,
Nginx, Envoy) check plain HTTP endpoints. These must be standard HTTP routes
on the chi mux, outside the Connect handler tree.

---

## What Each Probe Checks

### Startup Probe: `GET /healthz/startup`

**Purpose**: Tell Kubernetes the app has finished its initialization sequence
and is ready for liveness/readiness probes to begin.

**Checks**:
1. Database connection pool is established
2. Restate service endpoint is listening
3. Migrations have been applied (if `auto_migrate` is enabled)
4. OTEL trace provider is initialized

**Behavior**: Returns `200 OK` once all startup conditions are met. Before
that, returns `503 Service Unavailable`.

**Why this matters**: Gofra runs migrations on startup (when `auto_migrate`
is true) and registers with the Restate Server. These take time. Without a
startup probe, Kubernetes would run the liveness probe during initialization,
see failures, and restart the container in a loop. The startup probe gives
the app time to finish initializing.

```go
// gofra/health.go

type HealthChecker struct {
    db             *sql.DB
    restateReady   *atomic.Bool  // set to true after Restate endpoint starts
    startupDone    *atomic.Bool  // set to true after all initialization completes
}

func (h *HealthChecker) StartupHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if !h.startupDone.Load() {
            w.WriteHeader(http.StatusServiceUnavailable)
            json.NewEncoder(w).Encode(map[string]string{"status": "starting"})
            return
        }
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"status": "started"})
    }
}
```

### Liveness Probe: `GET /healthz/live`

**Purpose**: Tell Kubernetes the process is alive and not deadlocked. If this
fails, Kubernetes should kill and restart the container.

**Checks**:
1. The HTTP server goroutine is responding (implied by handling the request)
2. That's it.

**What it does NOT check**: Database connectivity, Restate reachability,
external API availability. These are transient failures that don't require
a container restart. The process is alive — it just can't reach a dependency.
Restarting won't fix a down database.

**Behavior**: Always returns `200 OK` as long as the HTTP server is running.
If the process is deadlocked, it won't respond at all, and Kubernetes times
out the probe.

```go
func (h *HealthChecker) LivenessHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
    }
}
```

**Reason liveness is trivially simple**: The most common health check mistake
is making the liveness probe check the database. When the database is down,
every instance fails the liveness probe, Kubernetes restarts all of them
simultaneously, they all try to reconnect at once (thundering herd), and
the database gets worse. A liveness probe should only detect conditions
where the process itself is broken — deadlocks, memory corruption, stuck
goroutines — not dependency failures.

### Readiness Probe: `GET /healthz/ready`

**Purpose**: Tell Kubernetes the app can handle traffic right now. If this
fails, Kubernetes removes the pod from the service's endpoint list — no
traffic is routed to it — but does NOT restart the container. When the
dependency recovers, the readiness probe passes again, and traffic resumes.

**Checks**:
1. Database is reachable (`SELECT 1` with a short timeout)
2. Restate Server ingress is reachable (HTTP HEAD to ingress URL)

```go
func (h *HealthChecker) ReadinessHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
        defer cancel()

        checks := map[string]error{
            "database": h.checkDatabase(ctx),
            "restate":  h.checkRestate(ctx),
        }

        allOK := true
        result := make(map[string]string)
        for name, err := range checks {
            if err != nil {
                result[name] = err.Error()
                allOK = false
            } else {
                result[name] = "ok"
            }
        }

        if allOK {
            result["status"] = "ready"
            w.WriteHeader(http.StatusOK)
        } else {
            result["status"] = "not_ready"
            w.WriteHeader(http.StatusServiceUnavailable)
        }
        json.NewEncoder(w).Encode(result)
    }
}

func (h *HealthChecker) checkDatabase(ctx context.Context) error {
    return h.db.PingContext(ctx)
}

func (h *HealthChecker) checkRestate(ctx context.Context) error {
    req, _ := http.NewRequestWithContext(ctx, "HEAD", h.restateIngressURL+"/restate/health", nil)
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return fmt.Errorf("restate unreachable: %w", err)
    }
    resp.Body.Close()
    if resp.StatusCode >= 400 {
        return fmt.Errorf("restate unhealthy: %d", resp.StatusCode)
    }
    return nil
}
```

**Reason for 2-second timeout**: The readiness probe runs every few seconds.
If a dependency check takes longer than 2 seconds, it's effectively down
from the application's perspective. Short timeout prevents probe goroutines
from piling up.

**Reason for checking Restate in readiness**: If the Restate Server is
unreachable, Connect handlers that dispatch durable work will fail. The
application can't fulfill its contract. Better to stop receiving traffic
than to accept requests and fail them all.

---

## Mux Registration

```go
// cmd/app/main.go
func main() {
    // ... setup ...

    health := &gofra.HealthChecker{
        db:               db,
        restateIngressURL: cfg.Restate.IngressURL,
        startupDone:      &atomic.Bool{},
        restateReady:     &atomic.Bool{},
    }

    mux := chi.NewRouter()

    // Health endpoints — outside CORS, outside auth, outside Connect
    mux.Get("/healthz/startup", health.StartupHandler())
    mux.Get("/healthz/live", health.LivenessHandler())
    mux.Get("/healthz/ready", health.ReadinessHandler())

    // CORS, auth, Connect handlers...
    mux.Use(gofra.CORSMiddleware(cfg.CORS))
    // ...

    // After all initialization is complete
    health.startupDone.Store(true)

    gofra.Serve(ctx, mux, ":3000", restateEndpoint, ":9080")
}
```

**Reason health endpoints are registered before middleware**: Health probes
must not go through CORS, auth, or any other middleware. Kubernetes doesn't
send auth tokens. Load balancers don't send CORS headers. These are bare
HTTP GET requests that must return immediately.

---

## Kubernetes Deployment Manifest

```yaml
# k8s/deployment.yaml (relevant section)
spec:
  containers:
    - name: gofra-app
      image: myapp:latest
      ports:
        - containerPort: 3000
          name: http

      startupProbe:
        httpGet:
          path: /healthz/startup
          port: http
        periodSeconds: 5
        failureThreshold: 24      # 24 × 5s = 120s max startup time
        timeoutSeconds: 3

      livenessProbe:
        httpGet:
          path: /healthz/live
          port: http
        periodSeconds: 15
        failureThreshold: 3       # 3 consecutive failures = restart
        timeoutSeconds: 2

      readinessProbe:
        httpGet:
          path: /healthz/ready
          port: http
        periodSeconds: 10
        failureThreshold: 2       # 2 failures = remove from LB
        successThreshold: 2       # 2 successes = add back to LB
        timeoutSeconds: 3
```

**Reason for `failureThreshold: 24` on startup**: Gofra may run migrations
on startup, which can take significant time for large schemas. 24 × 5s = 120
seconds gives enough headroom. If the app hasn't started after 2 minutes,
something is genuinely wrong.

**Reason for `successThreshold: 2` on readiness**: Prevents flapping. If the
database recovers after a brief hiccup, requiring two consecutive successes
before routing traffic again avoids sending requests to an instance that
passes once and immediately fails again.

**Reason liveness checks less frequently than readiness** (`15s` vs `10s`):
Liveness failures cause restarts — destructive and slow. Readiness failures
only remove from load balancer — non-destructive and recoverable. Check
readiness more often to react faster to transient failures.

---

## Non-Kubernetes Environments

The same endpoints work for any infrastructure:

| Platform | Liveness | Readiness |
|----------|----------|-----------|
| **Kubernetes** | `livenessProbe` | `readinessProbe` |
| **Docker Compose** | `HEALTHCHECK` (use `/healthz/ready`) | — |
| **AWS ECS** | Container health check | Target group health check |
| **AWS ALB** | — | Target group `health_check_path: /healthz/ready` |
| **Fly.io** | `[[services.tcp_checks]]` | `[[services.http_checks]]` path = `/healthz/ready` |
| **Google Cloud Run** | Startup probe | Liveness check via `/healthz/ready` |
| **Nginx upstream** | — | `proxy_pass` health check |

For platforms that only support a single health check (Docker Compose,
simple load balancers), use `/healthz/ready`. It's the most useful single
endpoint because it checks both process health and dependency availability.

```yaml
# docker-compose.yml
services:
  app:
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:3000/healthz/ready"]
      interval: 10s
      timeout: 3s
      retries: 3
      start_period: 30s
```

---

## What the Endpoints Return

All endpoints return JSON for debuggability, even though Kubernetes only
looks at the status code.

**Startup** (during initialization):
```json
HTTP/1.1 503 Service Unavailable
{"status": "starting"}
```

**Startup** (after initialization):
```json
HTTP/1.1 200 OK
{"status": "started"}
```

**Liveness** (always, as long as process is alive):
```json
HTTP/1.1 200 OK
{"status": "alive"}
```

**Readiness** (healthy):
```json
HTTP/1.1 200 OK
{"status": "ready", "database": "ok", "restate": "ok"}
```

**Readiness** (database down):
```json
HTTP/1.1 503 Service Unavailable
{"status": "not_ready", "database": "connect: connection refused", "restate": "ok"}
```

**Reason for JSON bodies**: Status codes are sufficient for probes. The JSON
bodies are for humans — when a developer curls the endpoint to debug why an
instance isn't receiving traffic, the body tells them which dependency is
down. No sensitive information is included (no connection strings, no
credentials, no internal IPs).

---

## Decision Log (Health Checks)

| # | Decision | Rationale |
|---|----------|-----------|
| 88 | Three separate endpoints (startup, live, ready) | Each answers a different question with different failure consequences. Conflating them causes incorrect restarts or missed traffic removal. |
| 89 | Plain HTTP routes, not Connect RPC | Kubernetes probes use plain HTTP GET. Load balancers don't speak Connect. |
| 90 | Liveness checks nothing except "can the process respond" | Checking dependencies in liveness causes thundering herd restarts when a dependency is down. Liveness = process health, not dependency health. |
| 91 | Readiness checks database + Restate | If either is unreachable, the app can't fulfill its contract. Stop routing traffic until recovery. |
| 92 | Health endpoints before middleware in mux | Probes must not go through CORS, auth, or rate limiting. They are infrastructure, not API. |
| 93 | 2-second timeout on dependency checks | Long checks pile up goroutines. If a dependency takes >2s to respond, treat it as down. |
| 94 | JSON response bodies | For human debugging. Status codes are what orchestrators use. Bodies explain what's wrong. |
| 95 | `successThreshold: 2` on readiness | Prevents flapping. Requires stability before routing traffic back. |
