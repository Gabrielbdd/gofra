# 07 — Observability: slog & OpenTelemetry

> Parent: [Index](00-index.md) | Prev: [Configuration](06-configuration.md) | Next: [Auth](08-auth.md)


## Addendum to Architecture Design Document
## Last Updated: 2026-04-12

---

## The Problem

A Gofra application has three execution contexts, each with different
observability characteristics:

1. **Connect RPC handlers** — synchronous, request-response. The user is
   waiting. Latency matters. Traces should show the full RPC call including
   database queries and outbound HTTP calls.

2. **Restate durable handlers** — async, potentially long-running. The handler
   may crash and resume from a journal replay. Log statements re-execute on
   replay, producing duplicates. Traces span multiple execution attempts.

3. **Restate Server itself** — a separate Rust process. It emits its own OTLP
   traces for invocation lifecycle (dispatch, journal, retry, completion).

The challenge is making these three systems produce correlated, useful
telemetry — where a single user action (e.g., "create a post") produces a
trace that spans the Connect RPC handler, the Restate invocation that indexes
it for search, and any downstream service calls.

---

## Design Decisions

### 1. slog is the logging interface

**Decision**: Use `log/slog` from the Go standard library for all application
logging. No zerolog, no zap.

**Reason**: slog is the standard. It ships with Go since 1.21. It supports
structured key-value pairs, configurable handlers (JSON, text, custom), and
context-aware logging via `slog.InfoContext(ctx, ...)`. Third-party handlers
can wrap it to add behavior (OpenTelemetry correlation, filtering).

Using the standard means: every Go library that logs via slog is automatically
captured by our handler. No adapters between logging libraries. New team
members already know the API.

### 2. OpenTelemetry for traces, metrics, and log correlation

**Decision**: Use OpenTelemetry (OTEL) as the telemetry framework. OTLP as
the export protocol. No vendor-specific SDKs.

**Reason**: OpenTelemetry is the industry standard for observability. It
supports traces, metrics, and logs with a unified context propagation model.
All three of our systems (Connect RPC, Restate, Postgres) have OTEL
integrations. Using OTEL means the developer can export to any backend —
Jaeger, Grafana Tempo, Datadog, Honeycomb — without changing application code.

### 3. otelconnect for Connect RPC instrumentation

**Decision**: Use `connectrpc.com/otelconnect` as a Connect interceptor for
automatic RPC tracing and metrics.

**Reason**: otelconnect is maintained by the Connect team. It produces
traces and metrics following the OpenTelemetry RPC semantic conventions:
`rpc.system`, `rpc.service`, `rpc.method`, `rpc.connect_rpc.error_code`.
It propagates trace context via W3C `traceparent` headers. One interceptor
gives us per-RPC spans, duration histograms, and error rate metrics for free.

### 4. Restate Server exports its own OTLP traces

**Decision**: Configure the Restate Server to export traces to the same OTLP
collector that the application uses.

**Reason**: Restate Server traces show invocation lifecycle — when an
invocation was dispatched, journaled, retried, completed. These traces are
correlated with the application's traces via W3C TraceContext. When the
Connect handler dispatches a Restate invocation, the trace context propagates
through the Restate Server to the Restate handler. The result: one trace
spanning the entire user request, from Connect RPC to Restate handler to
downstream services.

### 5. ctx.Log() in Restate handlers to suppress replay duplicates

**Decision**: In Restate handlers, use `ctx.Log()` (the Restate SDK's
replay-aware logger) instead of `slog.InfoContext()` for logs that should
not duplicate on replay.

**Reason**: When Restate replays a handler from its journal, all code
re-executes — including log statements. A handler that logs "sending email"
on the first execution will log "sending email" again on replay, even though
the email was already sent. The Restate SDK's `ctx.Log()` suppresses log
output during replay, preventing misleading duplicates.

However, `ctx.Log()` returns a `*slog.Logger`, so the API is still slog. The
handler uses `ctx.Log().Info("sending email", ...)` — same syntax, same
structured key-value pairs, but with replay-aware filtering.

### 6. Trace context propagation across the Connect → Restate boundary

**Decision**: When a Connect handler dispatches work to Restate via the
ingress client, the trace context from the incoming RPC request propagates
to the Restate invocation.

**Reason**: Without this, the Connect RPC trace and the Restate invocation
trace are disconnected. The developer sees an RPC that "succeeded" but cannot
trace what happened to the background work it dispatched.

Restate's ingress API supports W3C TraceContext. The Restate ingress Go SDK
propagates context from the caller's `context.Context`. Since otelconnect
injects a span into the request context, and the Connect handler passes that
context to the Restate ingress call, the trace propagates automatically.

Inside Restate handlers, the Go SDK supports `restate.WrapContext(ctx, externalCtx)`
to embed an external `context.Context` (carrying OTEL trace/span) into the
Restate context. This makes the trace context available inside `restate.Run()`
blocks.

---

## Architecture

```
Browser
  │
  │  HTTP request (no trace context)
  ▼
┌─────────────────────────────────────────────────────┐
│ Gofra Application                                   │
│                                                     │
│  Connect RPC Handler                                │
│  ┌───────────────────────────────────────────────┐   │
│  │ otelconnect interceptor                      │   │
│  │  → creates span: PostsService/CreatePost     │   │
│  │  → records: rpc.system, rpc.method, duration │   │
│  │  → propagates trace context to Restate       │   │
│  └───────────────────────────────────────────────┘   │
│         │                                           │
│         │ slog.InfoContext(ctx, "post created")      │
│         │  → enriched with trace_id, span_id        │
│         │                                           │
│         │ restateingress.ServiceSend(ctx, ...)       │
│         │  → trace context flows via W3C headers    │
│         ▼                                           │
│  ┌───────────────────────────────────────────────┐   │
│  │ Restate Handler (e.g., SearchIndexer)        │   │
│  │                                              │   │
│  │ ctx.Log().Info("indexing post")               │   │
│  │  → suppressed during replay                  │   │
│  │  → enriched with trace_id, span_id           │   │
│  │  → enriched with restate.invocation.id       │   │
│  │                                              │   │
│  │ restate.Run(ctx, func() { ... })             │   │
│  │  → Restate Server creates child span         │   │
│  └───────────────────────────────────────────────┘   │
│                                                     │
│  OTLP Exporter ──────────────────────────────────►  │
└─────────────────────────────────────────────────────┘
                                                    │
                                                    ▼
┌────────────────────────┐    ┌────────────────────────┐
│ Restate Server         │    │ OTLP Collector         │
│  → exports own traces  │───►│ (Jaeger, Grafana, etc) │
│  → per-invocation spans│    │                        │
│  → journal step spans  │    │ Correlated traces:     │
└────────────────────────┘    │  Connect RPC span      │
                              │   └─ Restate invoke    │
                              │       └─ Run: index    │
                              │       └─ Run: update   │
                              └────────────────────────┘
```

---

## Implementation

### Bootstrap: Initialize OTEL + slog

```go
// cmd/app/main.go
func main() {
    ctx := context.Background()
    cfg := config.Load()

    // ── OpenTelemetry setup ──────────────────────────────

    shutdown := setupOTEL(ctx, cfg)
    defer shutdown(ctx)

    // ── slog setup ───────────────────────────────────────

    logger := setupLogger(cfg)
    slog.SetDefault(logger)

    // ── Application ──────────────────────────────────────

    db, _ := gofra.OpenDB(cfg.Database)
    // ... rest of setup ...
}
```

### OTEL Provider Initialization

```go
// gofra/otel.go
func setupOTEL(ctx context.Context, cfg *config.Config) func(context.Context) error {
    // Trace exporter — sends spans to OTLP collector
    traceExporter, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(cfg.OTEL.Endpoint),
        otlptracegrpc.WithInsecure(), // TLS in production
    )
    if err != nil {
        slog.Error("failed to create trace exporter", "err", err)
        os.Exit(1)
    }

    // Metric exporter — sends metrics to OTLP collector
    metricExporter, err := otlpmetricgrpc.New(ctx,
        otlpmetricgrpc.WithEndpoint(cfg.OTEL.Endpoint),
        otlpmetricgrpc.WithInsecure(),
    )
    if err != nil {
        slog.Error("failed to create metric exporter", "err", err)
        os.Exit(1)
    }

    // Resource — identifies this service in traces/metrics
    res := resource.NewWithAttributes(
        semconv.SchemaURL,
        semconv.ServiceNameKey.String(cfg.App.Name),
        semconv.ServiceVersionKey.String(cfg.App.Version),
        semconv.DeploymentEnvironmentKey.String(cfg.App.Env),
    )

    // Trace provider
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(traceExporter),
        sdktrace.WithResource(res),
        sdktrace.WithSampler(sampler(cfg)),
    )
    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{},   // W3C traceparent
        propagation.Baggage{},        // W3C baggage
    ))

    // Meter provider
    mp := sdkmetric.NewMeterProvider(
        sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
        sdkmetric.WithResource(res),
    )
    otel.SetMeterProvider(mp)

    // Shutdown function
    return func(ctx context.Context) error {
        var errs []error
        errs = append(errs, tp.Shutdown(ctx))
        errs = append(errs, mp.Shutdown(ctx))
        return errors.Join(errs...)
    }
}

func sampler(cfg *config.Config) sdktrace.Sampler {
    if cfg.App.Env == "development" {
        return sdktrace.AlwaysSample()
    }
    // Production: sample 10% of traces, always sample errors
    return sdktrace.ParentBased(
        sdktrace.TraceIDRatioBased(0.1),
    )
}
```

**Reason for `AlwaysSample` in development**: During development, you want to
see every trace. In production, you sample to control volume and cost. The
`ParentBased` sampler respects the sampling decision of upstream services,
maintaining trace integrity across service boundaries.

**Reason for W3C TraceContext propagator**: This is the standard that Restate
uses for trace correlation. When the Connect handler sends context to the
Restate ingress API, the `traceparent` header is propagated, linking the
Connect span to the Restate invocation span.

### Logger Setup

```go
// gofra/logging.go
func setupLogger(cfg *config.Config) *slog.Logger {
    var handler slog.Handler

    if cfg.App.Env == "development" {
        // Human-readable, colored, with source location
        handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
            Level:     slog.LevelDebug,
            AddSource: true,
        })
    } else {
        // Structured JSON for log aggregation
        handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
            Level: slog.LevelInfo,
        })
    }

    // Wrap with OTEL trace correlation
    // Every log entry automatically gets trace_id and span_id
    // if a span exists in the context
    handler = NewOTELHandler(handler)

    return slog.New(handler)
}
```

**Reason for two handler modes**: In development, developers read logs in a
terminal. Colored, human-readable text is faster to scan. In production, logs
go to a collector (CloudWatch, Loki, Datadog). JSON is machine-parseable and
supports querying by field.

**Reason for wrapping with OTELHandler**: This injects `trace_id` and `span_id`
into every log entry that has a span in its context. When a developer sees an
error in logs, they can search for the `trace_id` in Jaeger/Grafana to see the
full request trace. This correlation is automatic — no manual attribute passing.

### OTEL-Aware slog Handler

```go
// gofra/otel_slog_handler.go
type OTELHandler struct {
    inner slog.Handler
}

func NewOTELHandler(inner slog.Handler) *OTELHandler {
    return &OTELHandler{inner: inner}
}

func (h *OTELHandler) Enabled(ctx context.Context, level slog.Level) bool {
    return h.inner.Enabled(ctx, level)
}

func (h *OTELHandler) Handle(ctx context.Context, r slog.Record) error {
    spanCtx := trace.SpanContextFromContext(ctx)
    if spanCtx.IsValid() {
        r.AddAttrs(
            slog.String("trace_id", spanCtx.TraceID().String()),
            slog.String("span_id", spanCtx.SpanID().String()),
        )
        if spanCtx.IsSampled() {
            r.AddAttrs(slog.Bool("trace_sampled", true))
        }
    }
    return h.inner.Handle(ctx, r)
}

func (h *OTELHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    return &OTELHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *OTELHandler) WithGroup(name string) slog.Handler {
    return &OTELHandler{inner: h.inner.WithGroup(name)}
}
```

**Reason for a custom handler instead of `otelslog.NewLogger`**: The official
`otelslog` bridge sends logs to the OTEL Logs API, which then exports them via
OTLP. This is the "full OTEL logs" path. Our handler does something simpler and
more immediately useful: it enriches existing slog output (stdout JSON) with
trace IDs. Logs still go to stdout (where they're collected by your existing
log pipeline). The trace correlation comes for free.

If the team later wants to send logs through the OTEL Logs pipeline (for
unified logs + traces in one backend), the otelslog bridge can be added as an
additional handler via `slog-multi` — the two approaches compose.

### Connect RPC Instrumentation

```go
// In main.go, when creating interceptors:
otelInterceptor, err := otelconnect.NewInterceptor(
    otelconnect.WithTrustRemote(),              // internal service: trust incoming traces
    otelconnect.WithoutServerPeerAttributes(),   // reduce cardinality
)
if err != nil {
    slog.Error("failed to create otel interceptor", "err", err)
    os.Exit(1)
}

interceptors := connect.WithInterceptors(
    otelInterceptor,       // OTEL tracing + metrics (first, so it wraps everything)
    validationInterceptor, // protovalidate
    authInterceptor,       // auth
)
```

**Reason otelInterceptor is first in the chain**: It must wrap all other
interceptors so the span captures the full request duration, including
validation and auth overhead.

**Reason for `WithTrustRemote`**: In an internal deployment where the Connect
server is behind a gateway, incoming trace context should be trusted so traces
are connected rather than starting new roots.

**Reason for `WithoutServerPeerAttributes`**: By default, otelconnect tags
every span with the client's IP and port. This creates high cardinality in
metrics (every unique client IP becomes a label). Dropping these reduces
metric storage cost significantly.

### What otelconnect produces automatically

**Traces** — one span per RPC call:
```
Name:       myapp.posts.v1.PostsService/CreatePost
Attributes:
  rpc.system:               connect_rpc
  rpc.service:              myapp.posts.v1.PostsService
  rpc.method:               CreatePost
  rpc.connect_rpc.error_code: (empty or error code)
Duration:   12ms
Status:     OK
```

**Metrics** — per RPC:
```
rpc.server.duration          histogram   ms per call
rpc.server.request.size      histogram   bytes per request
rpc.server.response.size     histogram   bytes per response
rpc.server.requests_per_rpc  histogram   messages per stream
```

No custom code needed. The interceptor produces all of this.

### Restate Handler Logging

```go
// app/services/search_indexer.go
func (s SearchIndexer) Index(ctx restate.Context, req IndexPostRequest) error {
    // ctx.Log() is a *slog.Logger that suppresses during replay
    ctx.Log().Info("indexing post",
        "post_id", req.PostID,
    )

    post, err := restate.Run(ctx, func(ctx restate.RunContext) (models.Post, error) {
        return query.Find[models.Post](s.DB, req.PostID)
    }, restate.WithName("load-post"))
    if err != nil {
        ctx.Log().Error("failed to load post", "post_id", req.PostID, "err", err)
        return err
    }

    _, err = restate.Run(ctx, func(ctx restate.RunContext) (restate.Void, error) {
        return restate.Void{}, search.Index("posts", post.ID, post.ToSearchableMap())
    }, restate.WithName("index"))
    if err != nil {
        ctx.Log().Error("failed to index post", "post_id", req.PostID, "err", err)
        return err
    }

    ctx.Log().Info("post indexed successfully", "post_id", req.PostID)
    return nil
}
```

**Reason for `ctx.Log()` instead of `slog.InfoContext(ctx, ...)`**: If this
handler crashes after the "load-post" step and Restate replays it, the first
`Run` block is replayed from the journal (no re-execution). But the `ctx.Log()`
calls before and between `Run` blocks re-execute during replay. The Restate
SDK's logger suppresses these during replay, preventing duplicate "indexing
post" messages in the logs.

If the developer uses `slog.InfoContext(ctx, ...)` directly (bypassing the
Restate logger), the log will emit on every replay. This isn't catastrophic
but produces confusing duplicate entries. The framework should document this
in the `gofra generate service` template comments.

### Restate Server Trace Configuration

```toml
# restate-server configuration (restate.toml or env vars)
tracing-endpoint = "http://otel-collector:4317"  # OTLP gRPC

# Or via environment variable:
# RESTATE_TRACING_ENDPOINT=http://otel-collector:4317
```

The Restate Server exports traces per invocation:
- `invoke` span covering the handler execution
- child spans for each journal step (`Run`, `Sleep`, `ServiceSend`)
- attributes: `restate.invocation.id`, `restate.service.name`,
  `restate.handler.name`

These traces are correlated with the application's traces because the
Restate Server propagates W3C TraceContext from the ingress call through
to the handler invocation.

### Application-Level Metrics

Beyond what otelconnect and Restate provide automatically, the application
can emit custom metrics:

```go
// gofra/metrics.go
var (
    meter = otel.Meter("gofra")

    PostsCreated, _ = meter.Int64Counter("gofra.posts.created",
        metric.WithDescription("Total posts created"),
    )

    DBQueryDuration, _ = meter.Float64Histogram("gofra.db.query_duration",
        metric.WithDescription("Database query duration in milliseconds"),
        metric.WithUnit("ms"),
    )
)

// In a handler:
PostsCreated.Add(ctx, 1, metric.WithAttributes(
    attribute.String("status", post.Status),
))
```

**Reason for the `gofra.` prefix**: OTEL metrics should be namespaced. The
prefix makes it clear these are application metrics, not infrastructure
metrics from otelconnect or the Restate Server.

---

## Gofra Configuration

```yaml
# gofra.yaml
observability:
  # OTLP collector endpoint
  endpoint: "${OTEL_EXPORTER_OTLP_ENDPOINT:-localhost:4317}"

  # Log level: debug, info, warn, error
  log_level: "${LOG_LEVEL:-info}"

  # Trace sampling rate (0.0 to 1.0). Ignored in development (always 1.0).
  trace_sample_rate: 0.1

  # Service name for OTEL resource
  service_name: "${OTEL_SERVICE_NAME:-myapp}"
```

```toml
# mise.toml — dev task starts collector alongside app
[tasks."dev:collector"]
description = "Start Jaeger for local trace viewing"
run = """
docker run -d --name jaeger \
  -p 4317:4317 -p 4318:4318 -p 16686:16686 \
  jaegertracing/jaeger:2.4.0
"""
```

In development, `mise run dev:collector` starts a local Jaeger instance. The
Restate Server and the Gofra app both export to `localhost:4317`. Traces are
viewable at `http://localhost:16686`.

---

## The Full Trace for "Create Post"

A user submits a new post. Here's what the observability stack captures:

```
Trace ID: abc123...

[Connect RPC]  PostsService/CreatePost          12ms
  │  rpc.system: connect_rpc
  │  rpc.method: CreatePost
  │
  ├─ [DB]  INSERT INTO posts (...)              3ms
  │     db.system: postgresql
  │     db.statement: INSERT INTO posts...
  │
  └─ [Restate Ingress]  ServiceSend             2ms
       │  restate.service: SearchIndexer
       │  restate.handler: Index
       │
       └─ [Restate Invoke]  SearchIndexer/Index  45ms
            │  restate.invocation.id: inv_abc123
            │
            ├─ [Run: load-post]                  5ms
            │     db.statement: SELECT * FROM posts...
            │
            └─ [Run: index]                      38ms
                  search.engine: postgres_fts
                  search.index: posts
```

**Log entries** for this request:

```json
{"time":"...","level":"INFO","msg":"post created","post_id":42,"trace_id":"abc123","span_id":"def456"}
{"time":"...","level":"INFO","msg":"indexing post","post_id":42,"trace_id":"abc123","span_id":"789xyz"}
{"time":"...","level":"INFO","msg":"post indexed successfully","post_id":42,"trace_id":"abc123","span_id":"789xyz"}
```

Every log entry has `trace_id`. Search for it in Jaeger to see the full trace.
Click on a span to see its attributes. The entire flow — from the user's HTTP
request to the background search indexing — is one connected trace.

---

## Decision Log (Observability)

| # | Decision | Rationale |
|---|----------|-----------|
| 35 | slog for logging | Standard library since Go 1.21. Structured. Context-aware. No third-party dependency. |
| 36 | Custom OTELHandler over otelslog bridge | Enriches stdout logs with trace_id/span_id. Works with existing log pipelines. Bridge can be added later for full OTEL Logs. |
| 37 | JSON logs in prod, text in dev | JSON for machine parsing in collectors. Text for human reading in terminals. |
| 38 | otelconnect interceptor | Maintained by Connect team. Per-RPC spans + metrics. Standard OTEL semantic conventions. Zero custom code. |
| 39 | otelconnect first in interceptor chain | Must wrap auth + validation to capture full request duration. |
| 40 | `WithTrustRemote()` | Internal service trusts upstream trace context. Produces connected traces instead of disconnected roots. |
| 41 | `WithoutServerPeerAttributes()` | Reduces metric cardinality. Client IP/port labels create explosive label space. |
| 42 | ctx.Log() in Restate handlers | Suppresses duplicate log entries during journal replay. Returns standard *slog.Logger. |
| 43 | Restate Server exports OTLP | Same collector as the app. Invocation traces correlate with Connect RPC traces via W3C TraceContext. |
| 44 | AlwaysSample in dev, ratio-based in prod | See everything locally. Control volume and cost in production. |
| 45 | W3C TraceContext propagator | Standard. Restate uses it. Connect uses it. No vendor-specific propagation format. |
| 46 | `gofra.` metric prefix | Namespace application metrics. Distinguish from otelconnect (`rpc.server.*`) and Restate (`restate_*`) metrics. |
| 47 | Jaeger in dev via mise task | One `mise run dev:collector` for local trace viewing. No mandatory infra — just a Docker container. |
