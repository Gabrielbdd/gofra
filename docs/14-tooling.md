# 14 — Tooling: mise & gofra CLI

> Parent: [Index](00-index.md) | Prev: [Frontend](13-frontend.md) | Next: [Docker Compose](15-docker-compose.md)

---

## mise

**Decision #26.** mise replaces Makefile for both tool version management and
task running.

### Tool Versions

```toml
# mise.toml
[tools]
go = "1.23"
node = "22"
"go:github.com/bufbuild/buf/cmd/buf" = "latest"
"go:google.golang.org/protobuf/cmd/protoc-gen-go" = "latest"
"go:connectrpc.com/connect/cmd/protoc-gen-connect-go" = "latest"
"go:github.com/restatedev/sdk-go/protoc-gen-go-restate" = "latest"
```

Every developer gets the same Go, Node, buf, and protoc plugin versions.
`mise install` installs everything.

### Task Definitions

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
run = "go run ./cmd/gofra-gen-runtimeconfig"
sources = ["proto/**/*runtime_config.proto", "config/config.go"]
outputs = ["config/public_config_gen.go", "web/src/gen/runtime/runtime-config.ts"]

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

Tasks are incremental — `sources` and `outputs` track what changed. `mise run gen`
only regenerates when proto or SQL files change.

The runtime-config generator runs after protobuf codegen and emits:

- typed Go binding code in `config/public_config_gen.go`
- a generated TS loader in `web/src/gen/runtime/`

This keeps the public browser-config contract synchronized across Go and
TypeScript without handwritten parallel types.

### Developer Workflow

```bash
mise install              # Install Go, Node, buf, protoc plugins
docker compose up -d      # Start Postgres, Restate, Zitadel
cd web && npm install     # Frontend deps
mise run migrate          # Run migrations
mise run dev              # Start Go (air) + Vite
```

`mise run dev` starts both processes, but the browser entrypoint is the Go
server on `http://localhost:3000`. Go serves API routes and `/_gofra/config.js`
directly, and proxies frontend pages/assets to Vite for HMR.

## gofra CLI

**Decision #27.** The `gofra` binary handles project bootstrap and code
generation — tasks that need to understand Go project structure, imports, and
interface implementations.

```bash
gofra new myapp                            # → ./myapp from the canonical starter
gofra generate service ProcessPodcast     # → app/services/process_podcast.go
gofra generate object ShoppingCart        # → app/objects/shopping_cart.go
gofra generate workflow OrderCheckout     # → app/workflows/order_checkout.go
gofra generate proto posts               # → proto/myapp/posts/v1/posts.proto
gofra generate migration create_posts    # → db/migrations/..._create_posts.sql
```

Tasks (build, test, lint, dev) stay in mise. Starter bootstrap and generators
stay in gofra.

## Current Scaffold Strategy

Gofra now ships one canonical starter so `gofra new` works immediately, but the
starter is intentionally minimal while the broader framework contract is still
settling.

The current implementation strategy is:

1. Build a reusable framework slice at the repo root.
2. Wire that slice into `internal/projectgen/starter/full/`.
3. Test `gofra new` by generating a real app into a temp directory and running
   `go test ./...` there.
4. Extract narrower post-create generators only after the base starter contract
   is coherent.

The runtime-config feature follows this pattern today:

- reusable framework code in `runtimeconfig/`
- project bootstrap in `internal/projectgen/`
- generator internals in `internal/runtimeconfiggen/`
- a public CLI entrypoint in `cmd/gofra/`
- a codegen entrypoint in `cmd/gofra-gen-runtimeconfig/`
- starter-owned app wiring in `internal/projectgen/starter/full/`

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
