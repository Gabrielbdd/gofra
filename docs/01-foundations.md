# 01 — Design Principles & Technology Choices

> Parent: [Index](00-index.md) | Next: [System Architecture](02-system-architecture.md)

---

## Design Principles

**Explicit over implicit.** Dependencies are passed through struct fields and
function parameters, never accessed via globals. If you read `main.go`, you
can trace every dependency.

**Protocols over frameworks.** The API contract is Protocol Buffers. The RPC
layer is Connect RPC. The durable execution layer is the Restate SDK. Forge
provides project structure and tooling around these protocols — it does not
wrap them in its own abstractions.

**Single source of truth for types.** A `.proto` file defines a resource once.
Code generation produces Go server stubs, Go message types, TypeScript message
types, TypeScript query hooks, and validation rules. No type is manually
written in two places.

**Crash-proof background operations by default.** Any operation that leaves the
HTTP request-response cycle runs through Restate's durable execution. If the
process crashes, the operation resumes from its last completed step.

**Ship as a single artifact.** The production build is one Go binary containing
the API server, the Restate service endpoint, and the compiled frontend assets.
Deploy alongside a Restate Server and a Postgres database. Nothing else.

---

## Positioning

Forge is not trying to recreate server-rendered Rails, Laravel, or Phoenix in
Go. It borrows the batteries-included mindset and convention-driven developer
experience, but its position is different: an API-first framework for general
web applications, with typed Connect RPC contracts, a default React SPA,
required Zitadel-based identity, and Restate for durable workflows.

That makes Forge closer to an opinionated Go application platform than a thin
HTTP toolkit. The tradeoff is deliberate: more upfront structure, fewer
infrastructure decisions left to each project.

---

## Technology Choices

### Go

**Decision #1.** Compiled to a single binary (no runtime to install). Strong
static typing. Goroutines for concurrency. The standard library covers HTTP,
JSON, crypto, and testing. The language is deliberately simple — one obvious
way to do things.

### Connect RPC + Protocol Buffers + buf

**Decision #2, #3.** REST APIs require hand-writing types in both server and
client languages. Connect RPC eliminates this: define the API once in `.proto`,
generate typed server stubs and typed clients.

Connect over raw gRPC because: handlers are standard `net/http.Handler`, works
in browsers without a proxy, supports JSON natively (`curl` any endpoint),
wire-compatible with gRPC.

buf over protoc because: manages proto dependencies, lints proto files, detects
breaking changes, generates code for multiple languages in one command.

### Google AIP Conventions

**Decision #4.** Consistent naming, pagination, errors enforced by linting.
See [API Layer](03-api-layer.md) for the full AIP adoption list.

### Restate

**Decision #13, #14.** Durable execution engine that journals every step.
Crash recovery is automatic. Replaces: queue system, retry library, saga
coordinator, cron daemon, event bus, workflow engine — with one process.
See [Restate](04-restate.md) for details.

### PostgreSQL

**Decision #17.** Source of truth for domain data. Full-text search built in.
JSON columns for semi-structured data. Restate has its own K/V store for
operational state — the two are complementary.

### sqlc + goose + pgx

**Decision #48, #49, #53.** sqlc generates typed Go code from SQL queries.
goose manages SQL migrations. pgx is the most performant Postgres driver
for Go. See [Database](05-database.md).

### koanf

**Decision #58.** Configuration from YAML + env vars + CLI flags with correct
merge semantics. See [Configuration](06-configuration.md).

### Zitadel

**Decision #67.** Required identity system for authentication, users, and
organizations. Go-native, Connect RPC API, self-hostable single binary,
multi-tenant.
See [Auth](08-auth.md).

### React + Vite + TanStack + shadcn + Tailwind 4

**Decision #22, #23, #24.** SPA frontend. Connect-Query bridges TanStack Query
with proto-generated types for end-to-end type safety. Vite for HMR in dev,
`embed.FS` for production. See [Frontend](13-frontend.md).

### mise

**Decision #26.** Tool version management and task runner. Replaces Makefile.
See [Tooling](14-tooling.md).

### slog + OpenTelemetry

**Decision #35, #38.** Standard library logging with OTEL trace correlation.
See [Observability](07-observability.md).

---

## Non-Goals

- No server-side rendering or HTML templating. The Go server serves APIs and static assets.
- Not wrapping Connect RPC or Restate SDK in framework abstractions.
- Not an ORM. sqlc generates from SQL. No model methods.
- Not a service container. Go uses explicit struct fields.
- Not a line-by-line port of Rails, Laravel, or Phoenix. Forge borrows DX ideas, but keeps Go-native patterns and an API-first architecture.

---

## Decisions in This Section

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | Go | Compiled binary. Static types. Goroutines. Standard library. |
| 2 | Connect RPC | Type-safe API from proto. Works in browsers. Standard `net/http`. |
| 3 | buf for codegen | Manages proto deps, lints, detects breaking changes. |
| 13 | Restate for durable ops | Crash recovery at step level. Replaces queue + scheduler + saga + workflow. |
| 17 | Postgres for domain data | Complex queries, joins, FTS. Restate K/V for operational state. |
| 18 | Explicit DI via struct fields | No service container. No globals. |
| 20 | chi router | `net/http` compatible. Connect handlers mount directly. |
| 24 | SPA (no SSR) | Decouples frontend and backend. Contract is the proto file. |
| 29 | No server-side rendering or templates | API-first. Frontend is replaceable. |
