# 15 — Development Environment: Docker Compose

> Parent: [Index](00-index.md) | Prev: [Tooling](14-tooling.md) | Next: [Testing](16-testing.md)


## Addendum to Architecture Design Document
## Last Updated: 2026-04-12

---

## What This Solves

`mise run dev` starts the application code (Go server, Vite frontend). But the
application depends on infrastructure services that the developer shouldn't
build from source: PostgreSQL, Restate Server, Zitadel, and optionally Jaeger
for trace viewing.

Docker Compose manages these infrastructure services. The application code
runs natively (not in Docker) for fast iteration — hot reload with `air` for
Go and HMR with Vite for React. Infrastructure runs in Docker for isolation
and reproducibility.

---

## Design Principles

**Application code runs on the host, not in Docker.** The Go server and Vite
frontend run natively for fast compilation, hot reload, and debugger access.
Putting them in Docker adds build latency, volume mount complexity, and makes
debugging harder.

**Infrastructure runs in Docker.** Postgres, Restate, Zitadel, and Jaeger
are black-box dependencies. The developer doesn't need to compile them or
understand their internals. Docker provides consistent versions across all
developer machines.

**Single `docker compose up` starts everything.** No ordering scripts. Docker
Compose's `depends_on` with health checks handles startup order.

**Data persists across restarts.** Named volumes for Postgres and Restate data.
`docker compose down` stops containers but keeps data. `docker compose down -v`
destroys data for a clean start.

---

## docker-compose.yml

```yaml
# docker-compose.yml
# Infrastructure services for local Gofra development.
# The Go server and Vite frontend run on the host via `mise run dev`.

services:
  # ─── PostgreSQL ─────────────────────────────────────────
  postgres:
    image: postgres:17-alpine
    environment:
      POSTGRES_USER: gofra
      POSTGRES_PASSWORD: gofra
      POSTGRES_DB: forge_dev
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      # Create Zitadel's database on first start
      - ./docker/init-dbs.sql:/docker-entrypoint-initdb.d/init-dbs.sql:ro
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U gofra"]
      interval: 5s
      timeout: 3s
      retries: 5

  # ─── Restate Server ────────────────────────────────────
  restate:
    image: docker.restate.dev/restatedev/restate:latest
    environment:
      RESTATE_LOG_FILTER: "restate=info"
    ports:
      - "8080:8080"   # Ingress (where the app sends durable invocations)
      - "9070:9070"   # Admin UI (http://localhost:9070)
      - "5122:5122"   # Node communication
    volumes:
      - restate_data:/restate-data
    extra_hosts:
      - "host.docker.internal:host-gateway"  # Access host-running Go server
    command: ["--node-name=restate-dev"]
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9070/health"]
      interval: 5s
      timeout: 3s
      retries: 10

  # ─── Zitadel ───────────────────────────────────────────
  zitadel:
    image: ghcr.io/zitadel/zitadel:latest
    command: start-from-init --masterkeyFromEnv --tlsMode disabled
    environment:
      ZITADEL_MASTERKEY: "MustBe32CharactersLongMasterKey!"
      ZITADEL_DATABASE_POSTGRES_HOST: postgres
      ZITADEL_DATABASE_POSTGRES_PORT: 5432
      ZITADEL_DATABASE_POSTGRES_DATABASE: zitadel
      ZITADEL_DATABASE_POSTGRES_USER_USERNAME: zitadel
      ZITADEL_DATABASE_POSTGRES_USER_PASSWORD: zitadel
      ZITADEL_DATABASE_POSTGRES_USER_SSL_MODE: disable
      ZITADEL_DATABASE_POSTGRES_ADMIN_USERNAME: gofra
      ZITADEL_DATABASE_POSTGRES_ADMIN_PASSWORD: gofra
      ZITADEL_DATABASE_POSTGRES_ADMIN_SSL_MODE: disable
      ZITADEL_EXTERNALSECURE: "false"
      ZITADEL_EXTERNALPORT: 8081
      ZITADEL_EXTERNALDOMAIN: localhost
      ZITADEL_FIRSTINSTANCE_ORG_HUMAN_USERNAME: admin@gofra.local
      ZITADEL_FIRSTINSTANCE_ORG_HUMAN_PASSWORD: "Admin1234!"
    ports:
      - "8081:8080"   # Zitadel UI + API (http://localhost:8081)
    depends_on:
      postgres:
        condition: service_healthy

  # ─── Jaeger (optional — tracing) ───────────────────────
  jaeger:
    image: jaegertracing/jaeger:2
    profiles: ["tracing"]  # Only starts with: docker compose --profile tracing up
    ports:
      - "4317:4317"   # OTLP gRPC (receives traces from app + Restate)
      - "4318:4318"   # OTLP HTTP
      - "16686:16686" # Jaeger UI (http://localhost:16686)

volumes:
  postgres_data:
  restate_data:
```

### Init Script for Multiple Databases

Postgres hosts two databases: `forge_dev` (the application) and `zitadel`
(Zitadel's internal data). The init script creates the Zitadel database and
user on first start:

```sql
-- docker/init-dbs.sql
-- Creates additional databases needed by infrastructure services.
-- The main `forge_dev` database is created by POSTGRES_DB env var.

CREATE USER zitadel WITH PASSWORD 'zitadel';
CREATE DATABASE zitadel OWNER zitadel;
```

**Reason for one Postgres instance**: Running two Postgres containers wastes
resources. Zitadel supports shared Postgres instances — it creates its own
schemas within the `zitadel` database and doesn't interfere with the
application's `forge_dev` database. This mirrors production, where a managed
Postgres instance often hosts multiple databases.

---

## Port Map

| Port | Service | Purpose |
|------|---------|---------|
| 3000 | Go server (host) | Browser entrypoint, Connect RPC API, `/_gofra/config.js` |
| 5173 | Vite (host) | Frontend dev server with HMR (behind Go proxy) |
| 5432 | Postgres (Docker) | Database |
| 8080 | Restate (Docker) | Ingress — app sends durable invocations here |
| 8081 | Zitadel (Docker) | Login UI + Management API |
| 9070 | Restate (Docker) | Admin UI — inspect invocations |
| 9080 | Go server (host) | Restate service endpoint — Restate pushes invocations here |
| 16686 | Jaeger (Docker) | Trace viewer (only with `--profile tracing`) |

**Reason for Zitadel on `:8081` instead of `:8080`**: Restate uses `:8080`
for ingress. Both are infrastructure services the developer interacts with
rarely. The Go server on `:3000` is the daily browser entrypoint, and Vite on
`:5173` stays behind it for frontend development.

---

## Developer Workflow

### First Time Setup

```bash
# 1. Start infrastructure
docker compose up -d

# 2. Wait for services to be healthy
docker compose ps   # all should show "healthy"

# 3. Install tools and dependencies
mise install
cd web && npm install && cd ..

# 4. Configure Zitadel (one-time)
#    Open http://localhost:8081, login as admin@gofra.local / Admin1234!
#    Create a project, application, and note the client_id
#    Copy client_id to .env file

# 5. Run migrations
mise run migrate

# 6. Register Restate service endpoint
restate deployments register http://host.docker.internal:9080 --force

# 7. Start application
mise run dev
```

### Daily Development

```bash
docker compose up -d    # Start infra (if not already running)
mise run dev            # Start Go server + Vite + register Restate
```

### Tracing (When Needed)

```bash
docker compose --profile tracing up -d   # Start infra + Jaeger
# View traces at http://localhost:16686
```

### Clean Reset

```bash
docker compose down -v   # Stop everything and delete all data
docker compose up -d     # Fresh start
mise run migrate         # Re-apply migrations
mise run seed            # Re-seed data
```

---

## Mise Integration

The `mise run dev` task starts the application and registers with Restate.
It assumes infrastructure is already running via Docker Compose:

```toml
# mise.toml

[tasks."infra"]
description = "Start infrastructure services"
run = "docker compose up -d"

[tasks."infra:stop"]
description = "Stop infrastructure services"
run = "docker compose down"

[tasks."infra:reset"]
description = "Stop and destroy all infrastructure data"
run = "docker compose down -v"

[tasks."infra:tracing"]
description = "Start infrastructure with Jaeger tracing"
run = "docker compose --profile tracing up -d"

[tasks."infra:logs"]
description = "Tail infrastructure logs"
run = "docker compose logs -f"

[tasks.dev]
description = "Start all development services"
depends = ["dev:api", "dev:web"]

[tasks."dev:api"]
description = "Start Go server with hot reload"
run = """
# Register service endpoint with Restate on startup
restate deployments register http://localhost:9080 --force 2>/dev/null || true
air
"""
depends = ["gen:go"]

[tasks."dev:web"]
description = "Start Vite dev server"
run = "cd web && npm run dev"
depends = ["gen:ts"]
```

**Reason `infra` and `dev` are separate tasks**: Infrastructure starts once
and stays running. Application code restarts frequently (hot reload,
debugging, switching branches). Separating them avoids restarting Postgres
and Zitadel every time the developer restarts the Go server.

**Reason for `restate deployments register` in `dev:api`**: The Restate
Server needs to know where the Go service endpoint is. This registration
tells Restate "my handlers are at `http://localhost:9080`." It's idempotent
(--force) and runs on every `dev:api` start because the registration is
lost if the Restate container restarts.

---

## .env File

```bash
# .env — local development (not committed to git)
# Created once during setup, values come from Zitadel console

# Database
DATABASE_URL=postgres://gofra:gofra@localhost:5432/forge_dev?sslmode=disable

# Restate
RESTATE_INGRESS_URL=http://localhost:8080

# Zitadel
ZITADEL_ISSUER=http://localhost:8081
ZITADEL_CLIENT_ID=<from-zitadel-console>
ZITADEL_PROJECT_ID=<from-zitadel-console>
ZITADEL_SERVICE_ACCOUNT_KEY=<path-to-json-key>

# OTEL (only when running with --profile tracing)
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
```

**Reason for `.env` separate from `gofra.yaml`**: `gofra.yaml` contains
project defaults (checked into git). `.env` contains machine-specific values
(not checked in): Zitadel client IDs that differ per developer, database
passwords, service account keys. The koanf config loader reads both — env
vars from `.env` override values in `gofra.yaml`.

---

## Network Topology

```
┌─────────────────── Docker Network ──────────────────────┐
│                                                         │
│  postgres:5432 ◄── zitadel (connects as zitadel user)   │
│       ▲              ▲                                  │
│       │              │                                  │
│  restate:8080    zitadel:8080                           │
│       ▲          (exposed as :8081)                     │
│       │                                                 │
│  jaeger:4317 ◄── restate (sends OTLP traces)           │
│                                                         │
└──────────┬──────────────┬─────────────────────────────┘
           │              │
    host.docker.internal  │
           │              │
┌──────────▼──────────────▼─────────────── Host ─────────┐
│                                                         │
│  Go server :3000 ──► postgres:5432 (via localhost)       │
│       │          ──► restate:8080 (ingress, localhost)   │
│       │          ──► zitadel:8081 (OIDC, localhost)      │
│       │          ──► jaeger:4317 (OTLP, localhost)       │
│       │                                                  │
│  Go Restate endpoint :9080                               │
│       ▲                                                  │
│       │ restate pushes invocations via host.docker.internal
│       │                                                  │
│  Browser :3000 ──► Go :3000 ──► Vite :5173 (SPA/HMR proxy) │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**Reason for `host.docker.internal`**: Restate Server runs in Docker. The Go
service endpoint runs on the host. Restate needs to reach the host's `:9080`.
`host.docker.internal` resolves to the host's IP from within Docker
containers. The `extra_hosts` directive in docker-compose ensures this works
on Linux (where it's not available by default).

**Reason the Go server connects to services via `localhost`**: All Docker
services expose ports on the host (`5432`, `8080`, `8081`). The Go server
connects to them as if they were local — same connection strings work for
local development without Docker.

---

## Healthcheck Strategy

Docker Compose uses healthchecks to order service startup:

1. **Postgres** starts first and becomes healthy when `pg_isready` succeeds.
2. **Zitadel** starts after Postgres is healthy (needs the database).
3. **Restate** starts independently (no database dependency).
4. **Jaeger** starts independently (stateless collector).
5. **The Go server** (on the host) starts after `docker compose up -d`
   finishes. It connects to all services via localhost.

`depends_on: condition: service_healthy` ensures Zitadel doesn't start
before Postgres is accepting connections. Without this, Zitadel crashes
on startup because its database isn't ready.

---

## Zitadel Bootstrap

On first `docker compose up`, Zitadel initializes itself (`start-from-init`):

1. Creates its database schemas in the `zitadel` Postgres database
2. Creates a default instance and organization
3. Creates an admin user (`admin@gofra.local` / `Admin1234!`)

The developer then manually:

1. Opens `http://localhost:8081` and logs in as admin
2. Creates a Project (e.g., "myapp")
3. Creates an Application (type: PKCE/SPA, redirect URI: `http://localhost:3000/auth/callback`)
4. Enables "Assert roles on authentication" in project settings
5. Copies the Client ID to `.env`

**Reason for manual Zitadel setup**: Automating Zitadel project/application
creation requires calling the Zitadel API with a service account — which
itself requires a project and application to exist (chicken-and-egg). The
manual setup takes 2 minutes and happens once. A future `gofra setup` CLI
command could automate this using Zitadel's machine-to-machine bootstrap flow.

---

## What's NOT in Docker

| Service | Why it runs on the host |
|---------|------------------------|
| Go server | Hot reload with `air`. Debugger attachment. Fast compilation. |
| Vite | HMR requires direct filesystem access. Sub-second rebuilds. |
| buf / sqlc | Code generators run once, not continuously. |
| goose CLI | Runs as a one-shot command, not a long-running service. |

---

## Decision Log (Docker Compose)

| # | Decision | Rationale |
|---|----------|-----------|
| 106 | App code on host, infra in Docker | Hot reload and debugger access for app code. Reproducible versions for infra. |
| 107 | Single Postgres for both app and Zitadel | Saves resources. Separate databases within one instance. Mirrors production managed Postgres. |
| 108 | Restate single-node in dev | Cluster features (replication, snapshots) aren't needed for development. |
| 109 | Jaeger behind `--profile tracing` | Not every dev session needs tracing. Saves Docker resources. Available when needed. |
| 110 | Named volumes for data persistence | `docker compose down` keeps data. `docker compose down -v` for clean reset. |
| 111 | `host.docker.internal` for Restate → host | Restate in Docker must reach the Go service endpoint on the host's :9080. |
| 112 | `infra` and `dev` as separate mise tasks | Infra starts once and stays. App restarts frequently. Don't restart Postgres on every code change. |
| 113 | Manual Zitadel bootstrap | Automating requires solving a chicken-and-egg problem. 2-minute manual setup, done once. |
| 114 | `.env` for machine-specific values | Zitadel client IDs and service account keys differ per developer. Not in git. |
