# __GOFRA_APP_NAME__

This project was created by `gofra new`.

## Current Starter Scope

This starter is intentionally minimal. It proves the current contract between:

- the framework library imported from `__GOFRA_FRAMEWORK_MODULE__`
- the application-owned files generated into this project

Today the starter includes:

- a runnable Go HTTP server in `cmd/app`
- typed application config in `config/`
- a starter-owned public runtime-config binder in `config/public_config_gen.go`
- a checked-in placeholder runtime-config type under `gen/__GOFRA_PROTO_PACKAGE__/runtime/v1/`
- a minimal embedded web shell in `web/`

## Local Framework Dependency

The framework module is not published yet. `gofra new` therefore adds a local
`replace` directive in `go.mod` so this generated application builds against
the framework checkout that created it.

Once the framework is published, this temporary local replace can be removed.

## Run

```bash
go run ./cmd/app
```
