# 14 — Tooling: mise & forge CLI

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
depends = ["gen:go", "gen:ts", "gen:sql"]

[tasks."gen:go"]
run = "buf generate"
sources = ["proto/**/*.proto"]
outputs = ["gen/**/*.go"]

[tasks."gen:ts"]
run = "cd web && npx buf generate"
sources = ["proto/**/*.proto"]
outputs = ["web/src/gen/**/*.ts"]

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
run = "go build -o forge-app ./cmd/app"

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

### Developer Workflow

```bash
mise install              # Install Go, Node, buf, protoc plugins
docker compose up -d      # Start Postgres, Restate, Zitadel
cd web && npm install     # Frontend deps
mise run migrate          # Run migrations
mise run dev              # Start Go (air) + Vite
```

## forge CLI

**Decision #27.** The `forge` binary handles code generation only — tasks that
need to understand Go project structure, imports, and interface implementations.

```bash
forge generate service ProcessPodcast     # → app/services/process_podcast.go
forge generate object ShoppingCart        # → app/objects/shopping_cart.go
forge generate workflow OrderCheckout     # → app/workflows/order_checkout.go
forge generate proto posts               # → proto/myapp/posts/v1/posts.proto
forge generate migration create_posts    # → db/migrations/..._create_posts.sql
```

Tasks (build, test, lint, dev) stay in mise. Generators stay in forge.

## Decisions in This Section

| # | Decision | Rationale |
|---|----------|-----------|
| 26 | mise for tools + tasks | Pins versions. Replaces Makefile. Incremental builds. Parallel execution. |
| 27 | forge CLI for generators only | Generators need Go code for imports and interfaces. Tasks are declarative TOML. |
