# Gofra

Gofra is an opinionated Go framework aimed at the same general problem space as
Rails, Laravel, and Phoenix, but with an API-first architecture built around
Go, Connect RPC, Restate, PostgreSQL, and a default SPA frontend.

The repo is still early. The three surfaces that matter are:

- the `gofra` CLI for project bootstrap and generators
- reusable framework code under [`runtime/`](runtime), such as [`runtime/config/`](runtime/config)
- the canonical starter that `gofra new` copies into new applications

## Current Status

The current generator contract is deliberately small:

- `gofra new` copies a full runnable starter from
  `internal/scaffold/starter/full/`
- the generated app imports framework packages from this repo as a library
- app-owned files such as `cmd/app`, `config/`, `proto/`, `gen/`, and `web/`
  are created in the generated project
- the generated app reserves `public.*` for browser-safe runtime config values
  derived from `proto/<app>/runtime/v1/runtime_config.proto`
- the generated app currently uses a local `replace` directive back to the
  framework checkout that created it because the framework module is not
  published yet

This is the current ownership split:

- CLI-owned: developer tooling and app generators
- framework-owned: reusable behavior that generated apps import
- starter-owned: the canonical generated app layout and placeholder files that
  `gofra new` copies today
- future generated files: app-local code that later generators will rewrite or
  replace once more of the framework is implemented

The intended runtime-config DX is:

- add a field to `runtime_config.proto`
- set the value under `public.*` in YAML, env, or flags
- regenerate and consume the typed field on the frontend

## Quick Start

Install the pinned Go toolchain with `mise`:

```bash
mise trust
mise install
```

Run the framework tests:

```bash
mise run test
```

Generate a new app from the canonical starter:

```bash
mise run new -- ../myapp
```

Generate a new app with an explicit module path:

```bash
mise run new:module -- ../myapp example.com/myapp
```

Run the starter smoke test end to end:

```bash
mise run smoke:new
```

That task generates a temporary app and runs `go test ./...` inside it.

If this is your first time using `mise` in this checkout, run `mise trust`
once before invoking `mise run ...`.

## Current Commands

These tasks exist in the repo today:

- `mise run test` runs `go test ./...` for the framework repo
- `mise run gofra` shows the current `gofra` CLI help
- `mise run gen:runtimeconfig` shows the current runtime-config generator help
- `mise run new -- <path>` creates a new app from the canonical starter
- `mise run new:module -- <path> <module>` creates a new app with an explicit
  module path
- `mise run smoke:new` verifies that starter generation produces a buildable
  app

You can also run the underlying commands directly:

```bash
go run ./cmd/gofra --help
go run ./cmd/gofra new ../myapp
go run ./cmd/gofra generate runtime-config -h
go test ./...
```

## Repository Layout

The important directories right now are:

- `cmd/gofra/`: the public CLI entrypoint
- `internal/scaffold/`: starter copy logic and generation tests
- `internal/scaffold/starter/full/`: the canonical generated-app source tree
- `runtime/config/`: reusable config loading and public runtime-config handler
- `runtime/health/`: HTTP health check probes (startup, liveness, readiness)
- `runtime/serve/`: HTTP server lifecycle with graceful shutdown
- `internal/generate/config/`: the config code generator internals
- `docs/`: the architecture and product contract

The intended long-term organization is:

- one public CLI under `cmd/gofra/`
- private scaffold and generator internals under `internal/`
- public app-facing runtime packages under a stable prefix instead of indefinite
  flat root-package growth

## Documentation

Start with these docs:

- [`docs/00-index.md`](docs/00-index.md)
- [`docs/02-system-architecture.md`](docs/02-system-architecture.md)
- [`docs/14-tooling.md`](docs/14-tooling.md)
- [`docs/18-readiness-checklist.md`](docs/18-readiness-checklist.md)
