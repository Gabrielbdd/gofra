# __GOFRA_APP_NAME__

This project was created by `gofra new`.

## Current Starter Scope

This starter is intentionally minimal. It proves the current contract between:

- the framework library imported from `__GOFRA_FRAMEWORK_MODULE__`
- the application-owned files generated into this project

Today the starter includes:

- a runnable Go HTTP server in `cmd/app` using chi, with health check probes
  and graceful shutdown via the framework's `runtime/health` and `runtime/serve`
- a proto-driven config schema in `proto/__GOFRA_PROTO_PACKAGE__/config/v1/config.proto`
- config code generation via `mise run generate` (produces `config/*_gen.go`)
- optional YAML overrides in `gofra.yaml`
- a `compose.yaml` file for local PostgreSQL + ZITADEL, with named volumes and healthchecks
- `mise run infra` tasks that work with either Docker Compose or Podman Compose
- a minimal embedded web shell in `web/`
- health check endpoints at `/startupz`, `/livez`, `/readyz` (Kubernetes convention)

ZITADEL is provisioned alongside Postgres so identity is a ready dependency on
day one. The Go binary still treats auth as **opt-in**: the JWT middleware
only activates when both `auth.issuer` and `auth.audience` are set in
`gofra.yaml`. A fresh generated app runs without any ZITADEL configuration.

Config fields, defaults, and descriptions are defined once in the proto file.
Run `mise run generate` after editing the proto to regenerate the Go code.

## Run

```bash
mise trust
mise run infra
mise run migrate
mise run dev
```

`mise run dev` depends on `mise run generate`, so config code is always
up-to-date before the server starts.

`mise run infra` starts PostgreSQL and ZITADEL through either `docker compose`
or `podman compose`, then waits until Postgres accepts connections and ZITADEL
returns 200 on `/debug/healthz`. ZITADEL is exposed at
`http://localhost:${GOFRA_ZITADEL_PORT:-8081}`.

The default database settings already line up across `compose.yaml`,
`gofra.yaml`, and the migration tasks, so no `.env` file is required for the
out-of-the-box setup. If you need to change the image, port, or credentials,
start from `.env.example`.

For a full clean rebuild of local database state:

```bash
mise run infra:reset
mise run infra
mise run migrate
```

## Tasks

The starter ships with these `mise` tasks:

| Task | Purpose |
| --- | --- |
| `mise run generate` | Regenerate config code from the proto schema. |
| `mise run test` | Run `go test ./...` after regenerating config code. |
| `mise run build` | Build the application binary to `bin/__GOFRA_APP_NAME__`. |
| `mise run dev` | Start the backend locally (depends on `generate`). |
| `mise run infra` | Start local infrastructure (Postgres + ZITADEL) via Compose. |
| `mise run infra:stop` / `infra:reset` / `infra:logs` | Manage local infrastructure. |
| `mise run migrate` / `migrate:create` / `migrate:down` / `migrate:status` | Manage database migrations via `goose`. |
| `mise run seed` | Seed the database with development data. |

## Build a container image

The starter ships a multi-stage `Dockerfile` that produces a static,
distroless binary:

```bash
docker build -t __GOFRA_APP_NAME__:dev .
```

The resulting image runs as the non-root `nonroot` user and exposes port
`3000`. Override the exposed port if you change `app.port` in `gofra.yaml`.

## CI

The starter also includes `.github/workflows/ci.yml`, which on every pull
request and push to `main`:

1. installs the pinned Go toolchain via `mise`
2. runs `mise run test`
3. runs `mise run build`
4. builds the Docker image locally (without pushing)

That workflow is intentionally quiet on publishing — pushing to a registry
is an opt-in concern, added per project when deployment actually needs it.
