# 14 — Tooling: mise & gofra CLI

> Parent: [Index](00-index.md) | Prev: [Frontend](13-frontend.md) | Next: [Docker Compose](15-docker-compose.md)

---

## mise

**Decision #26.** mise replaces Makefile for both tool version management and
task running.

### Current Framework Repo Workflow

The framework repo now has a real `mise.toml`, but it is intentionally smaller
than the eventual generated-app task set. Today it focuses on the starter and
the current Go-only implementation slices:

```toml
# mise.toml
[tools]
go = "1.23"

[tasks.test]
run = "env GOCACHE=${GOCACHE:-/tmp/gofra-gocache} go test ./..."

[tasks.gofra]
run = "env GOCACHE=${GOCACHE:-/tmp/gofra-gocache} go run ./cmd/gofra --help"

[tasks."gen:runtimeconfig"]
run = "env GOCACHE=${GOCACHE:-/tmp/gofra-gocache} go run ./cmd/gofra generate runtime-config -h"

[tasks.new]
run = "env GOCACHE=${GOCACHE:-/tmp/gofra-gocache} go run ./cmd/gofra new {{arg(i=0)}}"

[tasks."new:module"]
run = "env GOCACHE=${GOCACHE:-/tmp/gofra-gocache} go run ./cmd/gofra new --module {{arg(i=1)}} {{arg(i=0)}}"

[tasks."smoke:new"]
run = """
tmpdir=...
go run ./cmd/gofra new \"$tmpdir\"
(cd \"$tmpdir\" && go test ./...)
"""
```

This is the current way to exercise project generation:

```bash
mise trust
mise install
mise run test
mise run new -- ../myapp
mise run smoke:new
```

`mise run smoke:new` is the current regression check for the starter contract:
it generates a temporary app and verifies that the generated project passes
`go test ./...`.

`gofra generate runtime-config` is now part of the public CLI surface, with
normal app developers usually relying on `mise run gen` or `mise run dev`
rather than invoking the generator directly.

`mise trust` is required once per checkout before mise will execute the repo's
task file.

### Tool Versions

```toml
# generated app mise.toml (target shape)
[tools]
go = "1.23"
node = "22"
"go:github.com/bufbuild/buf/cmd/buf" = "latest"
"go:google.golang.org/protobuf/cmd/protoc-gen-go" = "latest"
"go:connectrpc.com/connect/cmd/protoc-gen-connect-go" = "latest"
"go:github.com/restatedev/sdk-go/protoc-gen-go-restate" = "latest"
```

This is still the target `mise.toml` shape for generated applications after the
broader framework contract is implemented. It is not the full task set that the
framework repo ships today.

### Generated App Task Definitions

```toml
[tasks.gen]
description = "Generate all code from proto + SQL"
depends = ["gen:go", "gen:ts", "gen:runtimeconfig", "gen:sql"]

[tasks."gen:go"]
run = "buf generate"
sources = ["proto/**/*.proto"]
outputs = ["gen/**/*.go"]

[tasks."gen:ts"]
run = "cd web && npx buf generate"
sources = ["proto/**/*.proto"]
outputs = ["web/src/gen/**/*.ts"]

[tasks."gen:runtimeconfig"]
run = "gofra generate runtime-config"
sources = ["proto/**/*runtime_config.proto"]
outputs = ["config/public_config_types_gen.go", "config/public_config_gen.go", "web/src/gen/runtime/runtime-config.ts", "web/src/gen/runtime/runtime-config.global.d.ts"]

[tasks."gen:sql"]
run = "sqlc generate"
sources = ["db/queries/*.sql", "db/migrations/*.sql", "sqlc.yaml"]
outputs = ["db/sqlc/*.go"]

[tasks.dev]
depends = ["dev:api", "dev:web"]

[tasks."dev:api"]
run = "restate deployments register http://localhost:9080 --force 2>/dev/null || true && air"
depends = ["gen:go"]

[tasks."dev:web"]
run = "cd web && npm run dev"
depends = ["gen:ts"]

[tasks.infra]
run = "docker compose up -d"

[tasks.build]
depends = ["gen", "build:web"]
run = "go build -o gofra-app ./cmd/app"

[tasks."build:web"]
run = "cd web && npm run build"

[tasks.test]
run = "go test ./..."

[tasks.lint]
run = "buf lint && golangci-lint run"

[tasks.migrate]
run = "goose -dir db/migrations postgres $DATABASE_URL up"

[tasks."migrate:create"]
run = "goose -dir db/migrations create {{arg(i=0)}} sql"
```

These are still the target generated-app tasks. Tasks are incremental —
`sources` and `outputs` track what changed. `mise run gen` only regenerates
when proto or SQL files change.

The runtime-config generator runs after protobuf codegen and emits:

- a generated Go `PublicConfig` subtree in `config/public_config_types_gen.go`
- typed Go binding code in `config/public_config_gen.go`
- a generated TS loader in `web/src/gen/runtime/`

This keeps the public browser-config contract synchronized across Go and
TypeScript. The end-user flow is: add a proto field, set `public.*`, regenerate,
and use the typed field on the frontend.

### Target Generated App Workflow

```bash
mise install              # Install Go, Node, buf, protoc plugins
docker compose up -d      # Start Postgres, Restate, Zitadel
cd web && npm install     # Frontend deps
mise run migrate          # Run migrations
mise run dev              # Start Go (air) + Vite
```

This remains the intended generated-app workflow. `mise run dev` starts both
processes, but the browser entrypoint is the Go server on
`http://localhost:3000`. Go serves API routes and `/_gofra/config.js`
directly, and proxies frontend pages/assets to Vite for HMR.

## gofra CLI

**Decision #27.** The `gofra` binary handles project bootstrap and code
generation — tasks that need to understand Go project structure, imports, and
interface implementations.

At the repo level, tooling should be organized around three surfaces:

- `cmd/gofra/` as the only public CLI entrypoint
- public runtime packages as the only import surface for generated apps
- the canonical starter as the scaffold contract behind `gofra new`

That means Gofra should trend toward one public CLI with many subcommands, not
multiple public binaries for each generator slice.

```bash
gofra new myapp                            # → ./myapp from the canonical starter
gofra generate service ProcessPodcast     # → app/services/process_podcast.go
gofra generate object ShoppingCart        # → app/objects/shopping_cart.go
gofra generate workflow OrderCheckout     # → app/workflows/order_checkout.go
gofra generate proto posts               # → proto/myapp/posts/v1/posts.proto
gofra generate migration create_posts    # → db/migrations/..._create_posts.sql
gofra generate runtime-config            # → sync generated public config + frontend loader
```

Tasks (build, test, lint, dev) stay in mise. Starter bootstrap and generators
stay in gofra.

The intended implementation split behind that CLI is:

- `internal/scaffold/` for `gofra new`
- `internal/generate/` for `gofra generate ...`

The current repo now uses `internal/scaffold/` and
`internal/generate/runtimeconfig/` for the implemented runtime-config slice.

## Current Scaffold Strategy

Gofra now ships one canonical starter so `gofra new` works immediately, but the
starter is intentionally minimal while the broader framework contract is still
settling.

The current implementation strategy is:

1. Build a reusable framework slice at the repo root.
2. Wire that slice into `internal/scaffold/starter/full/`.
3. Test `gofra new` by generating a real app into a temp directory and running
   `go test ./...` there.
4. Extract narrower post-create generators only after the base starter contract
   is coherent.

The runtime-config feature follows this pattern today:

- reusable framework code in public runtime packages
- project bootstrap in `internal/scaffold/`
- generator internals in `internal/generate/runtimeconfig/`
- the public CLI surface in `cmd/gofra/`
- starter-owned app wiring in `internal/scaffold/starter/full/`

In current code, those responsibilities are implemented primarily by:

- `runtimeconfig/`
- `internal/scaffold/`
- `internal/generate/runtimeconfig/`
- `internal/scaffold/starter/full/`

`gofra new` currently performs one job only:

- copy the canonical starter into a destination directory
- rewrite reserved placeholders such as module path, app name, proto package,
  and the temporary local framework `replace`

**Reason**: one starter is enough to make project creation real now without
committing to conditional scaffold composition too early.

## Decisions in This Section

| # | Decision | Rationale |
|---|----------|-----------|
| 26 | mise for tools + tasks | Pins versions. Replaces Makefile. Incremental builds. Parallel execution. |
| 27 | gofra CLI for project bootstrap and generators | `gofra new` and generators need Go code for imports and interfaces. Tasks are declarative TOML. |
