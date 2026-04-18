# 3. Understanding what was generated

You have a running Gofra app from [Tutorial 1](01-your-first-gofra-app.md)
and you know how to verify it from [Tutorial 2](02-verify-your-app.md).

Now the important part: **what did `gofra new` actually put in your
repository, and what is each piece for?**

Keep the generated `hello/` directory open in your editor as you read.

## The shape of the app

```
hello/
├── cmd/app/main.go          # Your process entrypoint
├── proto/hello/config/v1/
│   └── config.proto          # Source of truth for app configuration
├── config/                   # Generated from config.proto (git-tracked)
├── db/
│   ├── migrations/           # goose SQL migrations
│   ├── queries/              # sqlc query files
│   ├── seeds/                # goose --no-versioning seed data
│   ├── sqlc/                 # Generated from queries (git-tracked)
│   └── embed.go              # Embeds migrations into the binary
├── web/
│   ├── index.html            # Starter web shell
│   └── embed.go              # Embeds web assets into the binary
├── scripts/                  # Compose, env loading, Postgres wait helper
├── gofra.yaml                # YAML overrides of proto defaults
├── compose.yaml              # Local Postgres (Docker or Podman)
├── Dockerfile                # Multi-stage build → distroless image
├── mise.toml                 # Task runner definitions
├── .github/workflows/ci.yml  # Test + build + docker image on PR and main
└── go.mod                    # Normal Go module
```

Read that tree once and hold onto the structure. It matches how the app
runs.

## `cmd/app/main.go` — the process

This is the only Go `main` package in the app. If you want to know what the
process does, this file is the whole story. Open it — it is under 150 lines.

From top to bottom it does five things:

1. **Load config.** `config.Load(os.Args[1:])` returns a typed `Config` built
   from defaults → `gofra.yaml` → env vars → CLI flags. You will exercise
   this precedence in [Tutorial 4](04-changing-configuration.md).
2. **Open the database pool.** `runtimedatabase.Open` creates a
   `pgxpool.Pool` using the config. If `database.auto_migrate` is `true`,
   `runtimedatabase.Migrate` applies embedded migrations before serving any
   traffic.
3. **Wire auth (optional).** When `auth.issuer` and `auth.audience` are both
   set, a JWT middleware is installed. When either is empty, the server logs
   `auth disabled` and skips the middleware entirely. This is why the
   starter runs without ZITADEL.
4. **Build the HTTP surface.** A root `http.ServeMux` serves the three
   health probes. A `chi.Router` serves the config endpoint and the web
   shell. The chi router is mounted at `/` on the root mux — everything
   non-probe flows through it.
5. **Start the server.** `runtimeserve.Serve` blocks until SIGINT/SIGTERM,
   then runs the graceful-shutdown sequence and closes the database pool in
   the `OnShutdown` callback.

Notice what is **not** in `main.go`: there is no Gofra wrapper, no hidden
application lifecycle, no struct embedding chain. The framework is imported
as normal Go packages. If you wanted to replace any layer you could.

## `proto/.../config.proto` — the schema

The file at `proto/<your-package>/config/v1/config.proto` defines every
configuration field your app accepts. It is the single source of truth for:

- **Go types** — `Config`, `AppConfig`, `PublicConfig`, `DatabaseConfig`,
  `AuthConfig`.
- **Defaults** — through `(gofra.config.v1.field).default_value` annotations.
- **Descriptions** — proto comments become Go doc comments.
- **Secrets** — `(gofra.config.v1.field).secret = true` excludes a field from
  CLI flags and from the public config endpoint. The `database.dsn` field is
  the only one marked secret in the starter.
- **Public surface** — fields under the `public` message are the ones served
  to the browser at `/_gofra/config.js`. Nothing else is reachable from
  JavaScript.

Running `mise run generate` reads this file and regenerates `config/`.

## `config/` — generated, but checked in

The `config/` directory does not exist the moment `gofra new` finishes. It
is produced by `mise run generate` (which is a dependency of `mise run dev`,
so it runs on first `dev` automatically).

Three files live there:

- `config_gen.go` — the Go structs and `DefaultConfig()` function that match
  the proto schema.
- `load_gen.go` — the `NewFlagSet` and `Load(args []string)` functions that
  apply the four-layer precedence.
- `public_gen.go` — `PublicConfigHandler`, which serves only the `public`
  subtree as JavaScript.

These files are committed to git. Regenerating is only needed when the proto
schema changes.

## `db/` — the database boundary

The starter ships a minimal but real database story:

- **`db/migrations/00001_create_posts.sql`** — a goose migration with
  `-- +goose Up` and `-- +goose Down` sections. File names are
  zero-padded so sqlc and goose see them in the same order.
- **`db/queries/posts.sql`** — sqlc query file. Each named query
  (`-- name: ... :one/:many/:exec`) compiles into a type-safe Go function.
- **`db/seeds/seed.sql`** — applied via `goose --no-versioning`. Seed scripts
  should be idempotent (use `ON CONFLICT DO NOTHING`).
- **`db/sqlc/`** — generated by `mise run gen:sql`. Do not edit by hand.
- **`db/embed.go`** — `//go:embed migrations/*.sql` bundles migration files
  into the binary. `runtimedatabase.Migrate(ctx, pool, db.Migrations)`
  consumes that embedded FS.

Two conventions to internalize:

- **sqlc reads goose migrations as the schema.** There is no separate
  `schema.sql`. What goose applies is what sqlc compiles against.
- **Migrations ship with the binary.** Decision
  [#51](https://github.com/Gabrielbdd/gofra/blob/main/docs/17-decision-log.md)
  covers this — a single deployable artifact is worth more than a flexible
  external migration tool.

## `web/` — the browser surface

The `web/` directory is deliberately tiny for the v0 starter: one HTML file
and one `embed.go` file.

- **`web/index.html`** loads `/_gofra/config.js` before any inline script
  runs. It reads `window.__GOFRA_CONFIG__` and renders the app name, the
  API base URL, and the full config as pretty-printed JSON.
- **`web/embed.go`** embeds the static assets and exposes `Handler()`, a
  standard `http.FileServerFS` handler. `main.go` mounts it at `/*` on the
  chi router.

When you replace this with a real SPA (React, Vue, Svelte, vanilla — it
doesn't matter), the contract stays: drop your build artifacts into `web/`,
keep `embed.go`, keep loading `/_gofra/config.js` before your app code.

## `gofra.yaml` — YAML overrides

```yaml
app:
  name: hello
  port: 3000

database:
  dsn: "postgres://postgres:postgres@localhost:5432/hello?sslmode=disable"
  auto_migrate: true
```

This file is the YAML layer in the four-layer precedence. Anything in here
overrides proto defaults but is in turn overridden by env vars and CLI
flags. The file is deliberately short — defaults live in the proto, not
here, so this file only captures project-specific overrides.

Secrets do not belong here. `database.dsn` is included only because the
starter's development-only defaults are not sensitive.

## `compose.yaml` — local infra

One service: Postgres 18, pinned to `postgres:18.3-alpine3.23`. A named
volume keeps data across restarts. `pg_isready` provides a healthcheck so
`mise run infra` can wait before returning.

Environment variables from `.env.example` override image, port, user,
password, and database name. If you never create a `.env`, the defaults
match what `gofra.yaml` expects, and the app runs out of the box.

## `mise.toml` — your day-to-day surface

`mise` is both the task runner and the toolchain manager. It pins Go 1.25
for this project, and it is the only thing your teammates need to install
to build and run the app.

Key tasks:

| Task | What it does |
|------|--------------|
| `mise run generate` | Regenerates `config/` from the proto file. |
| `mise run gen:sql` | Regenerates `db/sqlc/` from `db/queries/`. |
| `mise run test` | Regenerates config, then runs `go test ./...`. |
| `mise run build` | Regenerates config, then builds `bin/<app>`. |
| `mise run dev` | Regenerates config, then runs `go run ./cmd/app`. |
| `mise run infra` / `infra:stop` / `infra:reset` / `infra:logs` | Control the local Postgres container. |
| `mise run migrate` / `migrate:create` / `migrate:down` / `migrate:status` | Goose migration commands. |
| `mise run seed` | Apply `db/seeds/` via goose `--no-versioning`. |

Notice that `test`, `build`, and `dev` all depend on `generate`. The proto
schema is always applied before any code runs. Forgetting `mise run generate`
after a proto change is not something you can do.

## `Dockerfile` and `.github/workflows/ci.yml` — day-1 deployability

- **`Dockerfile`** — multi-stage build. Builder is `golang:1.25-alpine`;
  runtime is `gcr.io/distroless/static-debian12:nonroot`. The image runs
  as a non-root user and exposes port 3000.
- **`.github/workflows/ci.yml`** — on every PR and push to `main` it
  installs `mise`, runs `mise run test`, runs `mise run build`, and
  performs a local `docker build` via `docker/build-push-action@v6` (no
  push). Pushing to a registry is explicitly left as a per-project
  concern.

These are committed to the starter because decision
[#144](https://github.com/Gabrielbdd/gofra/blob/main/docs/17-decision-log.md)
treats day-1 deployability — runnable, buildable, testable — as a framework
promise.

## What you learned

- **Your app is a thin shell around framework runtime packages.** Less than
  150 lines of `main.go` wire up config, database, auth, health, and serve.
- **Proto is the source of truth for configuration.** Go, YAML, env vars,
  and the browser all derive from it.
- **Generated code is checked in.** You never ship code that has not been
  regenerated, because every test/build/dev task depends on `generate`.
- **sqlc reads goose migrations as the schema.** One source of truth, two
  tools consuming it.
- **The starter is minimal on purpose.** What is here is real; anything
  missing is not yet stable enough to scaffold.

## Next

- [Tutorial 4: Changing configuration](04-changing-configuration.md) — use
  the three override layers (YAML, env, flags) to change the app's port and
  see the precedence in action.
