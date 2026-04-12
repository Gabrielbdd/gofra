# 17 — Decision Log

> Parent: [Index](00-index.md) | Prev: [Testing](16-testing.md)
>
> This is the complete list of all architectural decisions. Each decision has
> a number, a short description, and a rationale. Search this document to
> understand why any choice was made.

---

## Foundations (Decisions #1–34)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 1 | Go | Compiled binary. Static types. Goroutines. Standard library. | [01](01-foundations.md) |
| 2 | Connect RPC | Type-safe API from proto. Works in browsers. Standard `net/http`. | [01](01-foundations.md) |
| 3 | buf for codegen | Manages proto deps, lints, detects breaking changes. Replaces protoc. | [01](01-foundations.md) |
| 4 | AIP conventions | Consistent naming, pagination, errors. Enforced by linting. | [03](03-api-layer.md) |
| 5 | `{action}_time` fields | AIP-148. `create_time` not `created_at`. One convention. | [03](03-api-layer.md) |
| 6 | Pagination on all List methods | AIP-158. Adding later is breaking. `page_token` is opaque. | [03](03-api-layer.md) |
| 7 | `request_id` for idempotency | AIP-155. Maps to Restate idempotency key. E2E exactly-once. | [03](03-api-layer.md) |
| 8 | `FieldMask` on Update | AIP-161. Prevents overwrite with proto zero values. | [03](03-api-layer.md) |
| 9 | `validate_only` on mutations | AIP-163. Enables frontend dry-run validation. | [03](03-api-layer.md) |
| 10 | Soft delete with `delete_time` | AIP-164. Undelete via custom method. | [03](03-api-layer.md) |
| 11 | `etag` for concurrency | AIP-154. Optimistic locking without DB locks. | [03](03-api-layer.md) |
| 12 | Connect error codes | AIP-193. Fixed set. Generic frontend handling. | [09](09-errors.md) |
| 13 | Restate for durable ops | Crash recovery at step level. Replaces queue + scheduler + saga. | [04](04-restate.md) |
| 14 | Restate SDK direct (no wrapper) | One API to learn. Type-safe context hierarchy. | [04](04-restate.md) |
| 15 | Virtual Objects for stateful entities | Per-key single-writer. No DB locks. | [04](04-restate.md) |
| 16 | Workflows for auth flows | Suspend on Durable Promise. Resume on user action. | [04](04-restate.md) |
| 17 | Postgres for domain data | Complex queries, joins, FTS. Restate K/V for operational state. | [05](05-database.md) |
| 18 | Explicit DI via struct fields | No service container. No globals. | [01](01-foundations.md) |
| 19 | Explicit `ToProto()`/`FromProto()` conversion | Model ≠ API message. Two types, two concerns. | [02](02-system-architecture.md) |
| 20 | chi router | `net/http` compatible. Connect handlers mount directly. | [02](02-system-architecture.md) |
| 21 | SQL migrations (plain SQL) | SQL is the schema language. No DSL translation layer. | [05](05-database.md) |
| 22 | React + TanStack + shadcn | Largest ecosystem. Connect-Query for E2E type safety. | [13](13-frontend.md) |
| 23 | Vite | Sub-second HMR. Proxy in dev, embed in prod. | [13](13-frontend.md) |
| 24 | SPA (no SSR) | Decouples frontend and backend. Contract is proto. | [13](13-frontend.md) |
| 25 | `embed.FS` for production | Single binary deployment. | [13](13-frontend.md) |
| 26 | mise for tools + tasks | Pins versions. Replaces Makefile. Incremental builds. | [14](14-tooling.md) |
| 27 | forge CLI for generators only | Generators need Go code. Tasks are TOML. | [14](14-tooling.md) |
| 28 | Events as map + loop | ~15 lines. Each listener durable via Restate. | [04](04-restate.md) |
| 29 | No server-side templates | API-first. Frontend is replaceable. | [13](13-frontend.md) |
| 30 | `RestateRecorder` for handler tests | Fast HTTP tests without Docker. | [16](16-testing.md) |
| 31 | Docker-based Restate integration tests | Durable handlers need real journal. | [16](16-testing.md) |
| 32 | Skip AIP-122 (resource name hierarchy) | Web apps use `id`/`slug`, not `publishers/123/books/456`. | [03](03-api-layer.md) |
| 33 | Skip AIP-151 (long-running operations) | Restate Workflows are strictly more capable. | [03](03-api-layer.md) |
| 34 | Skip AIP-127 (HTTP transcoding) | Connect handles HTTP mapping automatically. | [03](03-api-layer.md) |

## Observability (Decisions #35–47)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 35 | slog for logging | Standard library since Go 1.21. Structured. Context-aware. | [07](07-observability.md) |
| 36 | Custom OTELHandler over otelslog bridge | Enriches stdout logs with trace_id/span_id. Works with existing pipelines. | [07](07-observability.md) |
| 37 | JSON logs in prod, text in dev | JSON for machines. Text for humans. | [07](07-observability.md) |
| 38 | otelconnect interceptor | Per-RPC spans + metrics. Standard OTEL semantic conventions. | [07](07-observability.md) |
| 39 | otelconnect first in interceptor chain | Must wrap auth + validation to capture full request duration. | [07](07-observability.md) |
| 40 | `WithTrustRemote()` for internal services | Connected traces instead of disconnected roots. | [07](07-observability.md) |
| 41 | `WithoutServerPeerAttributes()` | Reduces metric cardinality. | [07](07-observability.md) |
| 42 | `ctx.Log()` in Restate handlers | Suppresses duplicate logs during journal replay. | [07](07-observability.md) |
| 43 | Restate Server exports OTLP | Same collector. Correlated traces via W3C TraceContext. | [07](07-observability.md) |
| 44 | AlwaysSample in dev, ratio-based in prod | See everything locally. Control volume in production. | [07](07-observability.md) |
| 45 | W3C TraceContext propagator | Standard. Restate and Connect both use it. | [07](07-observability.md) |
| 46 | `forge.` metric prefix | Distinguish app metrics from otelconnect and Restate metrics. | [07](07-observability.md) |
| 47 | Jaeger in dev via mise task | One command for local trace viewing. | [07](07-observability.md) |

## Database (Decisions #48–57)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 48 | sqlc over custom query builder | SQL validated at generation time. Zero framework code. | [05](05-database.md) |
| 49 | goose over custom migrator | Battle-tested, embeddable, Go migrations, out-of-order. | [05](05-database.md) |
| 50 | sqlc reads goose migrations as schema | One source of truth. No separate schema.sql. | [05](05-database.md) |
| 51 | Migrations embedded via `//go:embed` | Binary carries its own migrations. Single-artifact deployment. | [05](05-database.md) |
| 52 | Auto-migrate opt-in, not default | Multiple replicas racing on CREATE TABLE. Safe for dev, risky for prod. | [05](05-database.md) |
| 53 | pgx/v5 as SQL driver | Best Postgres performance. Native type support. | [05](05-database.md) |
| 54 | Explicit conversion functions (not methods on generated types) | Generated code stays untouched. Conversion in the rpc package. | [05](05-database.md) |
| 55 | No dynamic query builder | sqlc.narg() + COALESCE for optional filters. Raw SQL for truly dynamic. | [05](05-database.md) |
| 56 | Transactions via WithTx + explicit Begin/Commit | No hidden middleware. Every boundary visible. | [05](05-database.md) |
| 57 | Seeds via goose --no-versioning | SQL seeds, applied without version tracking. | [05](05-database.md) |

## Configuration (Decisions #58–66)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 58 | koanf over viper | No forced lowercasing. Modular deps. Correct merge semantics. | [06](06-configuration.md) |
| 59 | YAML over TOML | Supports comments. Familiar from Docker/K8s ecosystem. | [06](06-configuration.md) |
| 60 | Four-layer precedence: defaults → YAML → env → flags | 12-factor app standard. | [06](06-configuration.md) |
| 61 | `FORGE_` prefix for env vars | Prevents collisions with platform vars like PORT. | [06](06-configuration.md) |
| 62 | Single forge.yaml (no per-environment files) | Env vars handle deployment config. Per-env files drift. | [06](06-configuration.md) |
| 63 | Typed struct, not k.String() calls | Compile-time checking. Single place for all options. | [06](06-configuration.md) |
| 64 | No global config singleton | Config passed from main(). Same DI pattern as everything else. | [06](06-configuration.md) |
| 65 | Manual validation over struct tags | Startup-time concern. Simple rules. 10-line function. | [06](06-configuration.md) |
| 66 | Secrets only via env vars | YAML is in version control. Secrets in VCS = security incident. | [06](06-configuration.md) |

## Auth & Authz (Decisions #67–79)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 67 | Couple with Zitadel for identity | Go-native, Connect RPC API, single binary, multi-tenant, OIDC certified. | [08](08-auth.md) |
| 68 | OIDC Authorization Code + PKCE for SPA | Standard for public clients. No client_secret in browser. | [08](08-auth.md) |
| 69 | Stateless JWT validation via JWKS | No per-request call to Zitadel. Keys cached and auto-rotated. | [08](08-auth.md) |
| 70 | coreos/go-oidc for JWT verification | Handles JWKS, key rotation, signature, claims. | [08](08-auth.md) |
| 71 | Roles in Zitadel, permissions in Go | Clean separation. Zitadel manages assignment, app defines meaning. | [08](08-auth.md) |
| 72 | Static role→permission map | Small, testable, version-controlled. Can move to DB later. | [08](08-auth.md) |
| 73 | Resource-level authz in handlers | Requires loading the resource. Can't check in interceptor. | [08](08-auth.md) |
| 74 | JIT user profile creation | No sync. Profile created on first API call. | [08](08-auth.md) |
| 75 | `zitadel_user_id TEXT` as PK | Zitadel IDs are opaque strings. Direct PK avoids surrogate. | [08](08-auth.md) |
| 76 | Admin handlers proxy to Zitadel | SPA doesn't get admin credentials. Forge enforces authz. | [08](08-auth.md) |
| 77 | Access token in memory, not localStorage | XSS protection. Refresh via refresh_token. | [08](08-auth.md) |
| 78 | `urn:zitadel:iam:org:projects:roles` scope | Includes roles in token. No extra API call. | [08](08-auth.md) |
| 79 | Frontend permission checks display-only | Server always re-checks. Never trust the client. | [08](08-auth.md) |

## CORS (Decisions #80–87)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 80 | `connectrpc.com/cors` for header lists | Official package. Tracks protocol changes. | [10](10-cors.md) |
| 81 | `rs/cors` for middleware | Most widely used. Standard `net/http` middleware. | [10](10-cors.md) |
| 82 | `AllowCredentials: true` | SPA sends Bearer JWT. Requires exact origin matching. | [10](10-cors.md) |
| 83 | No wildcard origins | Incompatible with credentials. Security hole in production. | [10](10-cors.md) |
| 84 | Explicit allowed origins in config | Framework adds localhost:5173 in dev automatically. | [10](10-cors.md) |
| 85 | CORS middleware first in chain | Preflight OPTIONS must be handled before routing. | [10](10-cors.md) |
| 86 | `MaxAge: 7200` | Chrome's maximum. Reduces preflight requests. | [10](10-cors.md) |
| 87 | `idempotency_level = NO_SIDE_EFFECTS` for reads | Enables Connect GET. Avoids CORS preflight. | [10](10-cors.md) |

## Health Checks (Decisions #88–95)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 88 | Three separate endpoints (startup/live/ready) | Different questions, different failure consequences. | [11](11-health-checks.md) |
| 89 | Plain HTTP routes, not Connect RPC | Kubernetes and load balancers use plain HTTP GET. | [11](11-health-checks.md) |
| 90 | Liveness checks nothing except process responsiveness | Checking deps causes thundering herd restarts. | [11](11-health-checks.md) |
| 91 | Readiness checks database + Restate | If unreachable, stop routing traffic until recovery. | [11](11-health-checks.md) |
| 92 | Health endpoints before middleware | Must not go through CORS, auth, or rate limiting. | [11](11-health-checks.md) |
| 93 | 2-second timeout on dependency checks | Prevents goroutine pile-up. | [11](11-health-checks.md) |
| 94 | JSON response bodies | For human debugging. Orchestrators use status codes. | [11](11-health-checks.md) |
| 95 | `successThreshold: 2` on readiness | Prevents flapping. | [11](11-health-checks.md) |

## Error Handling (Decisions #96–105)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 96 | Connect error codes only, no custom codes | Standard set. Same as gRPC and AIP-193. | [09](09-errors.md) |
| 97 | `forge.NotFound()`, `forge.Internal()` helpers | Consistent construction with proper error details. | [09](09-errors.md) |
| 98 | `forge.Internal()` logs but hides original error | Security. No stack traces reaching clients. | [09](09-errors.md) |
| 99 | `BadRequest` with `FieldViolation` for validation | Google's standard type. Typed in Go and TypeScript. | [09](09-errors.md) |
| 100 | Restate: terminal vs retryable framework | "Will retrying fix this?" Terminal for logic errors. | [09](09-errors.md) |
| 101 | No custom failed-jobs dashboard | Restate UI + admin API + OTEL traces cover this. | [09](09-errors.md) |
| 102 | Generate error_details.proto for frontend | Enables `findDetails(BadRequestSchema)` in TypeScript. | [09](09-errors.md) |
| 103 | Transport interceptor for global error handling | Auth expiry, server unavailable. Handled once. | [09](09-errors.md) |
| 104 | Custom panic recovery returning Connect JSON | chi's default returns plain text. Connect needs JSON. | [09](09-errors.md) |
| 105 | Warn for validation, Debug for NotFound | Spikes indicate bugs. NotFound is normal navigation. | [09](09-errors.md) |

## Docker Compose (Decisions #106–114)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 106 | App on host, infra in Docker | Hot reload for app. Reproducible versions for infra. | [15](15-docker-compose.md) |
| 107 | Single Postgres for app + Zitadel | Saves resources. Separate databases in one instance. | [15](15-docker-compose.md) |
| 108 | Restate single-node in dev | Cluster features not needed for development. | [15](15-docker-compose.md) |
| 109 | Jaeger behind `--profile tracing` | Not every session needs tracing. | [15](15-docker-compose.md) |
| 110 | Named volumes for persistence | `down` keeps data. `down -v` for clean reset. | [15](15-docker-compose.md) |
| 111 | `host.docker.internal` for Restate → host | Restate in Docker must reach Go on host :9080. | [15](15-docker-compose.md) |
| 112 | Separate `infra` and `dev` mise tasks | Infra starts once. App restarts frequently. | [15](15-docker-compose.md) |
| 113 | Manual Zitadel bootstrap | Chicken-and-egg for API automation. 2-min manual setup. | [15](15-docker-compose.md) |
| 114 | `.env` for machine-specific values | Zitadel client IDs differ per developer. Not in git. | [15](15-docker-compose.md) |

## Graceful Shutdown (Decisions #115–123)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 115 | `signal.NotifyContext` for SIGTERM/SIGINT | Standard Go pattern. Context-based cancellation. | [12](12-graceful-shutdown.md) |
| 116 | Four-phase: drain → HTTP → Restate → resources | HTTP clients wait. Restate invocations are durable. DB closes last. | [12](12-graceful-shutdown.md) |
| 117 | 2-second readiness drain delay | LB deregistration race. Prevents connection-refused errors. | [12](12-graceful-shutdown.md) |
| 118 | `http.Server.Shutdown()` for HTTP | Standard library. Stops accepting, waits for in-flight, respects deadline. | [12](12-graceful-shutdown.md) |
| 119 | Context cancellation for Restate endpoint | `rs.Start(ctx, addr)` respects context cancellation. | [12](12-graceful-shutdown.md) |
| 120 | Restate invocations safe to interrupt | Completed steps journaled. Interrupted invocations retried on another instance. | [12](12-graceful-shutdown.md) |
| 121 | 25-second budget within 30-second K8s grace | 5-second safety margin before SIGKILL. | [12](12-graceful-shutdown.md) |
| 122 | Second signal force-kills | `stop()` restores default. Ctrl+C twice = immediate exit. | [12](12-graceful-shutdown.md) |
| 123 | `OnShutdown` callback for resource cleanup | OTEL flush + DB close. Framework provides hook, main.go provides logic. | [12](12-graceful-shutdown.md) |
