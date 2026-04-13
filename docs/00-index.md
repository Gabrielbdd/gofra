# Gofra Framework — Architecture Documentation

## What Is Gofra

Gofra is an opinionated, batteries-included Go framework for general web
applications. It borrows the productivity goal of Phoenix, Laravel, and Rails,
but positions itself as an API-first system: Connect RPC for typed APIs,
Restate for durable execution, PostgreSQL for domain data, Zitadel for
identity, and a React SPA as the default frontend — all shipped as a single
binary plus required infrastructure.

## How to Read This Documentation

This documentation is the complete engineering design for Gofra. It records
every architectural decision with its rationale. It is structured for both
human engineers reading sequentially and AI agents searching for specific topics.
The final document, [V1 Readiness Checklist](18-readiness-checklist.md),
defines which promises are in scope for a credible v1 release and which claims
must be softened or deferred until the framework actually solves them.

### Document Map

| # | Document | What It Covers |
|---|----------|---------------|
| 01 | [Design Principles & Technology Choices](01-foundations.md) | Why Go, Connect RPC, Restate, Postgres, React. Design principles. Non-goals. |
| 02 | [System Architecture](02-system-architecture.md) | Runtime diagram, framework-repo layout, generated-app structure, two listeners, code generation flow. |
| 03 | [API Layer — Connect RPC & Protobuf](03-api-layer.md) | Proto file conventions, AIP guidelines, buf config, code generation, handler patterns. |
| 04 | [Durable Execution — Restate](04-restate.md) | What Restate replaces, Services/Objects/Workflows, Connect→Restate bridge, event dispatch. |
| 05 | [Database — sqlc & goose](05-database.md) | sqlc for queries, goose for migrations, pgx driver, auto-migrate, transactions, seed data. |
| 06 | [Configuration — koanf](06-configuration.md) | YAML + env + flags, four-layer precedence, typed struct, secrets handling. |
| 07 | [Observability — slog & OpenTelemetry](07-observability.md) | slog handler, OTEL traces/metrics, otelconnect, Restate trace correlation, Jaeger. |
| 08 | [Authentication & Authorization — Zitadel](08-auth.md) | Zitadel coupling, OIDC flow, JWT validation, RBAC, permission mapping, user management. |
| 09 | [Error Handling](09-errors.md) | Connect error codes, helper functions, Restate terminal vs retryable, frontend parsing, panic recovery. |
| 10 | [CORS](10-cors.md) | connectrpc.com/cors + rs/cors, credential handling, Connect GET requests. |
| 11 | [Health Checks](11-health-checks.md) | Three-probe model (startup/liveness/readiness), Kubernetes alignment, non-K8s platforms. |
| 12 | [Graceful Shutdown](12-graceful-shutdown.md) | Four-phase shutdown, readiness drain, gofra.Serve(), Kubernetes budget alignment. |
| 13 | [Frontend — React SPA](13-frontend.md) | Vite, TanStack Router/Query, Connect-Query, shadcn, dev proxy, production embed. |
| 14 | [Tooling — mise & gofra CLI](14-tooling.md) | mise.toml, task definitions, gofra generators, developer workflow. |
| 15 | [Development Environment — Docker Compose](15-docker-compose.md) | Postgres, Restate, Zitadel, Jaeger setup, port map, network topology. |
| 16 | [Testing](16-testing.md) | Connect handler tests, Restate integration tests, RestateRecorder, factories. |
| 17 | [Decision Log](17-decision-log.md) | All 134 numbered decisions with rationale, organized by subsystem. |
| 18 | [V1 Readiness Checklist](18-readiness-checklist.md) | The promises worth keeping for v1, release blockers, ship gates, and deferred scope. |

### Technology Stack Summary

```
API:          Protocol Buffers → buf generate → Connect RPC (Go) + Connect-Query (TS)
Server:       Go + chi router + Connect handlers + Restate SDK
Database:     PostgreSQL + sqlc (queries) + goose (migrations) + pgx (driver)
Auth:         Zitadel (OIDC, users, orgs) + JWT validation + RBAC permissions in Go
Frontend:     React + Vite + TanStack Router + TanStack Query + shadcn + Tailwind 4
Observability: slog + OpenTelemetry + otelconnect + Jaeger
Config:       koanf (YAML + env + flags)
Tooling:      mise (tool versions + tasks) + gofra CLI (generators)
Deployment:   Single Go binary (embedded SPA) + Restate Server + PostgreSQL
```

### For AI Agents

Each document is self-contained. To find information about a specific topic:

- **How does auth work?** → `08-auth.md`
- **What database library is used?** → `05-database.md`
- **How are errors returned to the frontend?** → `09-errors.md`
- **What's the shutdown sequence?** → `12-graceful-shutdown.md`
- **Why was X chosen over Y?** → `17-decision-log.md` (search by topic)
- **What is actually in scope for v1?** → `18-readiness-checklist.md`
- **What's the framework repo layout or generated app structure?** → `02-system-architecture.md`
- **What proto conventions are followed?** → `03-api-layer.md`

Every design decision is numbered (Decision #1 through #134) and includes
a rationale. The decision log in `17-decision-log.md` is the complete
searchable index.
