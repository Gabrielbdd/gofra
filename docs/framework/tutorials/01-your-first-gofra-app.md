# 1. Your first Gofra app

In this tutorial you will create a fresh Gofra application from the starter,
start its local PostgreSQL container, run the app, and open it in the
browser.

By the end, you will understand what `gofra new` actually produces and how a
generated Gofra app is run in the normal local development flow.

## What you need

Three tools on your machine:

- **Go** — any recent version. The generated app pins its own toolchain
  through `mise`, so the Go version on your host is only used to install the
  `gofra` CLI.
- **[mise](https://mise.jdx.dev)** — task runner and toolchain manager. The
  generated app depends on it for every repeatable task.
- **Docker Desktop**, **Docker Engine**, or **Podman** — needed to run the
  local PostgreSQL container. Gofra's compose scripts accept either engine.

No database server, no Node.js, and no extra SDKs are required.

## 1. Install the CLI

```bash
go install github.com/Gabrielbdd/gofra/cmd/gofra@latest
```

This installs the `gofra` binary into `$(go env GOPATH)/bin`. Make sure that
directory is on your `PATH`. Verify the install:

```bash
gofra
```

You should see usage output similar to:

```
Usage:
  gofra new [--module module/path] <directory>
  gofra generate config [flags] <proto-file>
```

The CLI has exactly two jobs: scaffold a new application, and regenerate
config code from a proto file. Every other workflow in a generated app
happens through `mise`.

## 2. Create the app

```bash
gofra new hello
cd hello
```

`gofra new hello` writes a complete application tree into a directory named
`hello`. The generated app is a self-contained Go module: there is no
`replace` directive, and no dependency on the filesystem layout that created
it. You could commit it to git and share it immediately.

Trust the pinned task runner:

```bash
mise trust
```

This is a one-time step that tells `mise` it is safe to run tasks from this
directory.

## 3. Start PostgreSQL

```bash
mise run infra
```

This starts the Postgres service defined in `compose.yaml` using whichever
compose engine you have (`docker compose` or `podman compose`), then waits
until the database accepts connections.

While that runs, two things worth noticing:

- The compose file uses a pinned image (`postgres:18.3-alpine3.23`) and a
  named volume. If you stop the container and restart it, your data survives.
- No `.env` file is needed for this first run. The defaults in `gofra.yaml`
  match the defaults in `compose.yaml`.

## 4. Start the app

```bash
mise run dev
```

This regenerates config code from the proto schema (via the `generate`
dependency), then runs `go run ./cmd/app`.

The first invocation takes longer because `go` is building the binary and
populating the module cache. Subsequent runs are close to instant.

When the server is ready you will see structured log lines similar to:

```
level=INFO msg="migration applied" version=1 duration=...
level=INFO msg="starting app" app=hello addr=:3000
```

Two observations:

- `migration applied` means the app ran the embedded `00001_create_posts.sql`
  migration against the local Postgres on startup. This happens because
  `database.auto_migrate: true` is set in `gofra.yaml`.
- `addr=:3000` is the default HTTP listen address. You will change this in
  [Tutorial 4](04-changing-configuration.md).

## 5. Open it

Visit <http://localhost:3000> in your browser.

The page is the starter's embedded web shell. It displays the app name, the
API base URL, and the full runtime config JSON. The page is rendered from a
single `web/index.html` file served from Go. It is not an SPA framework yet —
it is the minimum web surface that proves the browser is receiving config
from the server.

## What you learned

- **The CLI is a scaffold generator.** It produces an app, then steps out of
  the way. The generated app has no framework-wrapper CLI; everything is
  `mise run <task>` or plain `go`.
- **The generated app is a normal Go module.** It depends on
  `github.com/Gabrielbdd/gofra` through the Go proxy like any other library.
  There is no magic linkage back to where it was generated.
- **Infra is external to the app.** The app runs on the host. PostgreSQL
  runs in a container. `mise run infra` owns that lifecycle.
- **Auto-migration is on in dev.** On startup the app applied the starter's
  migrations. Decision [#52](https://github.com/Gabrielbdd/gofra/blob/main/docs/17-decision-log.md)
  explains why this is opt-in rather than always-on.

## Stop it

- `Ctrl+C` stops the Go process.
- `mise run infra:stop` stops Postgres but keeps the named volume. Your data
  survives.
- `mise run infra:reset` stops Postgres and removes the volume. Next
  `mise run infra` starts with a blank database.

## Next

- [Tutorial 2: Verify your app is alive](02-verify-your-app.md) — health
  probes, the public config endpoint, and the web shell's data flow.
