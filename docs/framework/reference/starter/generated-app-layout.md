# Generated App Layout

> Structure and conventions of an application created by `gofra new`.

## Status

Alpha — the generated layout may change before v1.

## Overview

Running `gofra new <directory>` produces a complete, runnable Go application.
This page documents the directory structure, key files, and conventions of that
generated application.

## Directory Structure

```
<app>/
├── cmd/
│   └── app/
│       └── main.go           # Application entrypoint
├── proto/
│   └── <package>/
│       └── config/
│           └── v1/
│               └── config.proto  # Configuration schema (protobuf)
├── web/
│   ├── embed.go              # Embeds web assets into the binary
│   └── index.html            # SPA starter page
├── go.mod                    # Go module definition
├── gofra.yaml                # Default runtime configuration
├── mise.toml                 # Task runner definitions
├── .gitignore
└── README.md
```

## Key Files

### `cmd/app/main.go`

The application entrypoint. It:

1. Loads configuration via `runtimeconfig.Load()` from CLI args.
2. Creates a health checker with `runtimehealth.New()`.
3. Registers health probe handlers at Kubernetes-convention paths.
4. Sets up a chi router with the config endpoint and web asset handler.
5. Starts the server with `runtimeserve.Serve()`.

### `gofra.yaml`

Default configuration file. The `runtimeconfig.Load` function reads this file
by default. Structure matches the config protobuf schema.

### `proto/<package>/config/v1/config.proto`

Protobuf definition of the application's configuration schema. Used by
`gofra generate config` to produce typed Go config structs.

### `mise.toml`

Task runner definitions for common development commands:

| Task | Description |
|------|-------------|
| `mise run dev` | Run the application locally |
| `mise run test` | Run `go test ./...` |
| `mise run generate` | Run code generation |

### `web/`

Contains the frontend SPA assets. `embed.go` uses Go's `//go:embed` directive
to bundle these assets into the compiled binary for single-binary deployment.

## Dependencies

Generated applications depend on:

| Dependency | Purpose |
|------------|---------|
| `github.com/go-chi/chi/v5` | HTTP router |
| `github.com/spf13/pflag` | CLI flag parsing |
| `databit.com.br/gofra` | Gofra framework runtime packages |

During development, the framework dependency uses a local `replace` directive
in `go.mod`.

## Runtime Behavior

The generated app listens on the port configured in `gofra.yaml` (field
`app.port`). It exposes:

| Path | Handler | Purpose |
|------|---------|---------|
| `/startupz` | Health startup probe | Returns 200 after init |
| `/livez` | Health liveness probe | Always returns 200 |
| `/readyz` | Health readiness probe | Returns 200 if all checks pass |
| `/_gofra/config.js` | Config handler | Serves runtime config as JavaScript |
| `/` | Web handler | Serves the embedded SPA |

Graceful shutdown is handled automatically by `runtimeserve.Serve()`.

## Related Pages

- [gofra CLI](../cli/gofra.md) — The command that generates this layout.
- [runtime/config](../runtime/config.md) — Configuration loading.
- [runtime/health](../runtime/health.md) — Health check probes.
- [runtime/serve](../runtime/serve.md) — Server lifecycle.
