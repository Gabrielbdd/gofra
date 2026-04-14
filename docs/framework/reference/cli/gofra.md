# gofra CLI

> Project bootstrapping and code generation for Gofra applications.

## Status

Alpha — commands and flags may change before v1.

## Installation

```bash
go install databit.com.br/gofra/cmd/gofra@latest
```

Or run directly from a framework checkout:

```bash
go run ./cmd/gofra
```

## Commands

### `gofra new`

Creates a new Gofra application from the canonical starter template.

```
gofra new [flags] <directory>
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `directory` | Yes | Target directory for the new application |

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--module` | derived from directory name | Go module path for the generated app |
| `--framework-dir` | auto-detected | Path to local gofra framework checkout |

**Behavior:**

1. Copies the canonical starter template to `<directory>`.
2. Replaces template variables with the app name, module path, and framework
   references.
3. Prints next steps: `cd <directory>`, `mise trust`, `mise run dev`.

**Example:**

```bash
gofra new ../myapp
gofra new --module github.com/myorg/myapp ../myapp
```

### `gofra generate config`

Generates configuration loading code from a `.proto` file.

```
gofra generate config [flags] <proto-file>
```

This command reads a protobuf schema defining your application's configuration
and generates Go code for loading and validating that config.

### `gofra help`

Displays usage information.

```
gofra help
gofra --help
gofra -h
```

## Generated App Structure

See [Generated App Layout](../starter/generated-app-layout.md) for the full
structure of a `gofra new` application.

## Related Pages

- [Generated App Layout](../starter/generated-app-layout.md) — What `gofra
  new` produces.
- [runtime/config](../runtime/config.md) — Configuration loading used by
  generated apps.
