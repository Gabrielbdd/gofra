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
- a `compose.yaml` file for local PostgreSQL with a named volume and healthcheck
- `mise run infra` tasks that work with either Docker Compose or Podman Compose
- a minimal embedded web shell in `web/`
- health check endpoints at `/startupz`, `/livez`, `/readyz` (Kubernetes convention)

Config fields, defaults, and descriptions are defined once in the proto file.
Run `mise run generate` after editing the proto to regenerate the Go code.

## Local Framework Dependency

The framework module is not published yet. `gofra new` therefore adds a local
`replace` directive in `go.mod` so this generated application builds against
the framework checkout that created it.

Once the framework is published, this temporary local replace can be removed.

## Run

```bash
mise trust
mise run infra
mise run migrate
mise run dev
```

`mise run dev` depends on `mise run generate`, so config code is always
up-to-date before the server starts.

`mise run infra` starts PostgreSQL through either `docker compose` or
`podman compose`, then waits until the database accepts connections.

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
