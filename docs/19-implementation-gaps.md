# 19 — Implementation & Documentation Gaps

> Parent: [Index](00-index.md) | Prev: [V1 Readiness Checklist](18-readiness-checklist.md)
>
> This document tracks what is documented in the design docs but not yet
> implemented in the framework runtime, the generated project (`gofra new`),
> the CLI generators, or end-user documentation. It complements
> [18-readiness-checklist.md](18-readiness-checklist.md) with concrete
> file-level and feature-level tracking.

---

## How To Use This Document

Each section uses a checklist. Mark items as they land. When a section is fully
complete, add a **Completed** note with the date and relevant commit/PR.

Items are grouped by subsystem, not by priority. Use the
[V1 Readiness Checklist](18-readiness-checklist.md) and the suggested order of
work there to decide what to build next.

---

## 1. Runtime Packages (`runtime/`)

Reusable framework code that generated apps import. Today only
`runtime/config` exists.

### 1.1 Server Lifecycle

Source docs: [02-system-architecture.md](02-system-architecture.md),
[12-graceful-shutdown.md](12-graceful-shutdown.md)

- [ ] `runtime/serve` — `gofra.Serve(ctx, ServeConfig{...})` main entrypoint
  - [ ] Signal handling (SIGTERM/SIGINT via `signal.NotifyContext`)
  - [ ] Four-phase shutdown: readiness drain (2s) → HTTP (15s) → Restate (5s) → resources (3s)
  - [ ] Second-signal force kill escalation
  - [ ] `OnShutdown` callback for OTEL flush + DB close

### 1.2 Health Checks

Source doc: [11-health-checks.md](11-health-checks.md)

- [ ] `runtime/health` — `HealthChecker` struct
  - [ ] `GET /healthz/startup` — initialization complete (503 → 200)
  - [ ] `GET /healthz/live` — process alive (always 200)
  - [ ] `GET /healthz/ready` — traffic ready (DB ping + Restate HEAD, 2s timeout)
  - [ ] `SetNotReady()` for shutdown drain
  - [ ] JSON response bodies for debugging

### 1.3 Error Helpers

Source doc: [09-errors.md](09-errors.md)

- [ ] `runtime/errors` — Connect error code helpers
  - [ ] `gofra.NotFound(resource, identifier)` — CodeNotFound + ResourceInfo
  - [ ] `gofra.AlreadyExists(resource, identifier)` — CodeAlreadyExists
  - [ ] `gofra.InvalidArgument(violations map[string]string)` — CodeInvalidArgument + BadRequest FieldViolations
  - [ ] `gofra.PermissionDenied(msg)` — CodePermissionDenied
  - [ ] `gofra.Aborted(msg)` — CodeAborted
  - [ ] `gofra.Internal(err)` — CodeInternal (log original, send generic)
  - [ ] Panic recovery middleware (returns Connect JSON, not plain text)

### 1.4 Database

Source doc: [05-database.md](05-database.md)

- [ ] `runtime/database` — pgx pool management
  - [ ] `gofra.OpenDB(cfg DatabaseConfig) (*pgxpool.Pool, error)`
  - [ ] Auto-migrate on startup (opt-in via `database.auto_migrate`)
  - [ ] Embedded migrations support (`//go:embed db/migrations/*.sql` + `goose.SetBaseFS`)

### 1.5 Auth & Authorization

Source doc: [08-auth.md](08-auth.md)

- [ ] `runtime/auth` — JWT validation and context
  - [ ] `auth.NewAccessTokenVerifier(issuerURL, audience)` — JWKS-backed verifier
  - [ ] `auth.WithUser(ctx, user)` / `auth.UserFromContext(ctx)` — context accessors
  - [ ] `User` struct: ID, Email, Roles, OrgID
  - [ ] Auth interceptor (extract Bearer, validate, set context)
  - [ ] Public RPC allowlist (private-by-default)
- [ ] `runtime/authz` — permission enforcement
  - [ ] `authz.HasPermission(roles []string, perm Permission) bool`
  - [ ] Static `RolePermissions` map pattern

### 1.6 CORS

Source doc: [10-cors.md](10-cors.md)

- [ ] `runtime/cors` — CORS middleware builder
  - [ ] Integration with `connectrpc.com/cors` + `rs/cors`
  - [ ] Explicit origins, `AllowCredentials: false`, `MaxAge: 7200`
  - [ ] First in middleware chain

### 1.7 Observability

Source doc: [07-observability.md](07-observability.md)

- [ ] `runtime/observability` — OTEL + slog setup
  - [ ] `gofra.SetupOTEL(ctx, cfg)` — trace/metric providers, OTLP export
  - [ ] W3C TraceContext propagator
  - [ ] `otelconnect` interceptor wiring
  - [ ] Custom slog handler injecting `trace_id` / `span_id`
  - [ ] Dev mode (text handler, AlwaysSample) vs prod (JSON, ratio-based)

### 1.8 Restate Client

Source doc: [04-restate.md](04-restate.md)

- [ ] `runtime/restate` — thin ingress client wrapper
  - [ ] `gofra.NewRestateClient(ingressURL)`
  - [ ] `.Service(name, handler)`, `.Object(name, key, handler)`, `.Workflow(name, key, handler)`
  - [ ] `.Send()` (fire-and-forget) and `.Request()` (wait) on each sender

### 1.9 Testing Utilities

Source doc: [16-testing.md](16-testing.md)

- [ ] `runtime/testing` — test helpers
  - [ ] `gofra.TestDB(t)` — isolated test database with migrations applied
  - [ ] `gofra.NewRestateRecorder()` — capture dispatches without running Restate
  - [ ] `factory.Create[T](db, opts)` / `factory.CreateMany[T](db, n)` — test factories

---

## 2. Generated Project (`gofra new` Starter)

Files and structure that `internal/scaffold/starter/full/` should produce.
Today the starter produces: `cmd/app/main.go`, `go.mod`, `gofra.yaml`,
`mise.toml`, `proto/.../config.proto`, `web/embed.go`, `web/index.html`,
`.gitignore`, `README.md`.

### 2.1 Server Bootstrap

Source docs: [02-system-architecture.md](02-system-architecture.md),
[12-graceful-shutdown.md](12-graceful-shutdown.md)

- [ ] `cmd/app/main.go` uses chi router instead of raw `http.ServeMux`
- [ ] Middleware chain: CORS → panic recovery → auth interceptor → otelconnect
- [ ] Two listeners: `:3000` (HTTP/Connect) and `:9080` (Restate endpoint)
- [ ] Graceful shutdown via `gofra.Serve()` instead of `http.ListenAndServe`
- [ ] DB pool initialization on startup
- [ ] OTEL setup on startup
- [ ] Health check endpoint registration
- [ ] Restate handler binding

### 2.2 Infrastructure Files

Source doc: [15-docker-compose.md](15-docker-compose.md)

- [ ] `docker-compose.yml`
  - [ ] `postgres:17-alpine` — app DB + Zitadel DB, healthcheck, named volume
  - [ ] `restate:latest` — ingress (:8080), admin UI (:9070), `host.docker.internal`
  - [ ] `zitadel:latest` — `:8081`, depends on postgres, initial admin user
  - [ ] `jaeger:2` — profile `tracing`, ports 4317/4318/16686
  - [ ] Postgres init script for Zitadel database creation
- [ ] `.env.example` — template with all required env vars
  - [ ] `DATABASE_URL`
  - [ ] `RESTATE_INGRESS_URL`
  - [ ] `ZITADEL_ISSUER`, `ZITADEL_CLIENT_ID`, `ZITADEL_PROJECT_ID`
  - [ ] `OTEL_EXPORTER_OTLP_ENDPOINT`

### 2.3 Database

Source doc: [05-database.md](05-database.md)

- [ ] `sqlc.yaml` configuration file
- [ ] `db/migrations/` directory (with initial example migration)
- [ ] `db/queries/` directory (with example query file)
- [ ] `db/seeds/` directory (with example seed file)

### 2.4 Protobuf & Code Generation

Source doc: [03-api-layer.md](03-api-layer.md)

- [ ] `buf.yaml` — module configuration
- [ ] `buf.gen.yaml` — codegen config (Go + TypeScript targets)
- [ ] Example service proto (e.g., `proto/<app>/posts/v1/posts.proto`)
- [ ] `gen/` directory in `.gitignore`

### 2.5 App Directories

Source docs: [02-system-architecture.md](02-system-architecture.md),
[03-api-layer.md](03-api-layer.md), [04-restate.md](04-restate.md),
[08-auth.md](08-auth.md)

- [ ] `app/rpc/` — with example Connect handler stub
- [ ] `app/services/` — with example Restate Service stub
- [ ] `app/authz/permissions.go` — role-permission map skeleton
- [ ] `app/middleware/` — auth, CORS, recovery wiring

### 2.6 Frontend (React SPA)

Source doc: [13-frontend.md](13-frontend.md)

- [ ] `web/package.json` — React, Vite, TanStack Router/Query, Connect-Query, shadcn, Tailwind 4
- [ ] `web/vite.config.ts`
- [ ] `web/tsconfig.json`
- [ ] `web/index.html` — loads `/_gofra/config.js`, mounts React root
- [ ] `web/src/main.tsx` — React entry point
- [ ] `web/src/routes/__root.tsx` — TanStack Router layout
- [ ] `web/src/routes/index.tsx` — home route
- [ ] `web/src/lib/transport.ts` — Connect transport with `runtimeConfig.apiBaseUrl`
- [ ] `web/src/lib/auth.ts` — `react-oidc-context` + OIDC config from runtime config
- [ ] `web/src/lib/errors.ts` — error parsing helpers
- [ ] `web/src/lib/runtime-config.ts` — re-export of generated loader
- [ ] `web/src/components/ui/` — initial shadcn components
- [ ] Dev proxy from Go (:3000) to Vite (:5173)
- [ ] Production `//go:embed all:dist` with SPA fallback handler

### 2.7 Mise Tasks

Source doc: [14-tooling.md](14-tooling.md)

Today only `generate` and `dev` exist. Missing:

- [ ] `gen` — umbrella task (gen:go + gen:ts + gen:sql + gen:config)
- [ ] `gen:go` — `buf generate` for Go Connect stubs
- [ ] `gen:ts` — `buf generate` for TypeScript
- [ ] `gen:sql` — `sqlc generate`
- [ ] `dev:api` — Go server with air hot reload + Restate registration
- [ ] `dev:web` — Vite dev server
- [ ] `infra` — `docker compose up -d`
- [ ] `infra:stop` — `docker compose down`
- [ ] `infra:reset` — `docker compose down -v`
- [ ] `infra:tracing` — `docker compose --profile tracing up -d`
- [ ] `infra:logs` — `docker compose logs -f`
- [ ] `build` — Go binary + frontend assets
- [ ] `build:web` — frontend assets only
- [ ] `test` — `go test ./...`
- [ ] `lint` — `buf lint` + `golangci-lint run`
- [ ] `migrate` — `goose up`
- [ ] `migrate:create` — new migration file
- [ ] `migrate:down` — rollback last migration
- [ ] `migrate:status` — show migration status
- [ ] `seed` — `goose --no-versioning` seed data

### 2.8 Config Additions

Source doc: [06-configuration.md](06-configuration.md)

The config proto today has `app` and `public` sections. Missing:

- [ ] `database` section (host, port, user, password/secret, dbname, sslmode, auto_migrate)
- [ ] `restate` section (ingress_url, endpoint_port)
- [ ] `auth` section (issuer, audience, service_account_key/secret)
- [ ] `observability` section (log_level, otel_endpoint, sampling_ratio)

---

## 3. CLI Generators

Source doc: [14-tooling.md](14-tooling.md)

Today only `gofra new` and `gofra generate config` exist.

- [ ] `gofra generate service <Name>` → `app/services/<name>.go` (Restate Service scaffold)
- [ ] `gofra generate object <Name>` → `app/objects/<name>.go` (Restate Virtual Object scaffold)
- [ ] `gofra generate workflow <Name>` → `app/workflows/<name>.go` (Restate Workflow scaffold)
- [ ] `gofra generate proto <Name>` → `proto/<app>/<name>/v1/<name>.proto` (proto + buf registration)
- [ ] `gofra generate migration <Name>` → `db/migrations/<timestamp>_<name>.sql`

---

## 4. End-User Documentation

Today there is **no** user-facing documentation. The 18 docs under `docs/` are
internal design/architecture documents written for framework developers.

### 4.1 Getting Started

- [ ] Installation guide (go install, binary download, prerequisites)
- [ ] Quickstart tutorial: `gofra new` → running app → first API endpoint → first Restate handler
- [ ] Prerequisites reference (Go version, mise, Docker, Node.js)

### 4.2 Guides (How-To)

- [ ] Adding a Connect RPC service (proto → buf generate → handler → registration)
- [ ] Adding a database migration and queries (goose + sqlc workflow)
- [ ] Adding a Restate Service / Virtual Object / Workflow
- [ ] Adding frontend routes and consuming generated API types
- [ ] Setting up authentication (Zitadel project creation, OIDC config, env vars)
- [ ] Adding runtime configuration fields (proto → gofra generate config → use in app/SPA)
- [ ] Writing tests (handler tests with httptest, integration tests with real Restate)
- [ ] Deploying to production (single binary build, required infra, env vars, health checks)
- [ ] Error handling patterns (which Connect codes when, frontend error parsing)
- [ ] Background work patterns (when to use Service vs Object vs Workflow)

### 4.3 Reference

- [ ] CLI reference — all commands, flags, examples
- [ ] Mise task reference — all tasks with descriptions and dependencies
- [ ] Configuration reference — all `gofra.yaml` keys, env var mapping, CLI flags, defaults
- [ ] Project structure reference — what each directory and file is for
- [ ] Docker Compose reference — services, ports, profiles, volumes
- [ ] Runtime config API — `/_gofra/config.js` contract, adding fields, mutators
- [ ] Error codes reference — Connect codes, helper functions, error detail types
- [ ] Health check reference — endpoints, probe behavior, Kubernetes manifest

### 4.4 Concepts

- [ ] Architecture overview (two listeners, Connect + Restate, single binary, chi mux)
- [ ] The mutation boundary (safe in handlers vs needs Restate, retry semantics)
- [ ] Auth model (Zitadel + OIDC, token lifecycle, role-based authz, private-by-default RPCs)
- [ ] Durable execution model (Services vs Objects vs Workflows, journaling, replay)
- [ ] Code generation flow (proto → buf → Go/TS, SQL → sqlc, config proto → gofra generate)
- [ ] Configuration precedence (defaults → YAML → env → flags)

---

## 5. Cross-Cutting Concerns

Items that span multiple subsystems.

### 5.1 Readiness Checklist Open Items

These are tracked in [18-readiness-checklist.md](18-readiness-checklist.md) but
listed here for completeness:

- [ ] Mutation boundary patterns and tests (checklist item 3)
- [ ] Tenancy decision: single-tenant or multi-tenant (checklist item 4)
- [ ] Request-to-Restate boundary: context, traces, error semantics (checklist item 5)
- [ ] Startup/shutdown lifecycle contracts (checklist item 6)
- [ ] Production security baseline: webhook verification, rate limiting decision, TLS/secret guidance (checklist item 7)
- [ ] Reproducible tooling: pinned versions, Restate registration, sync across tools (checklist item 8)
- [ ] Operational baseline: logs/traces/metrics minimum, runbooks (checklist item 9)
- [ ] Test matrix: auth, mutation boundary, workflows, tenancy (checklist item 10)

### 5.2 Tool Version Pinning

Source docs: [14-tooling.md](14-tooling.md), [15-docker-compose.md](15-docker-compose.md)

- [ ] Pin Docker image tags (no `:latest` for Restate, Zitadel)
- [ ] Pin buf, sqlc, goose, golangci-lint, air versions in mise.toml
- [ ] Pin Node.js version in mise.toml

### 5.3 Smoke Test Coverage

- [ ] `mise run smoke:new` verifies full generated app (currently only tests config generation)
- [ ] Smoke test covers `docker compose up`, `mise run migrate`, `mise run dev` cycle
- [ ] Smoke test covers buf codegen (requires buf + proto deps)
