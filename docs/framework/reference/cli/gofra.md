# gofra CLI

> Project bootstrapping and code generation for Gofra applications.

## Status

Alpha — commands and flags may change before v1.

## Installation

```bash
go install github.com/Gabrielbdd/gofra/cmd/gofra@latest
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
| `directory` | Yes | Target directory for the new application (must be empty or non-existent) |

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--module` | Derived from directory name | Go module path for the generated app |

**Behavior:**

1. Validates that the target directory is empty or does not exist.
2. Copies the canonical starter template, replacing template tokens with the
   app name, module path, and the pinned framework module path + version.
3. Strips `.tmpl` extensions from processed files.
4. Prints the created path and next steps.

**Output:**

```
created /absolute/path/to/myapp

next steps:
  cd myapp
  mise trust
  mise run dev
```

**Example:**

```bash
gofra new ../myapp
gofra new --module github.com/myorg/myapp ../myapp
```

### `gofra generate config`

Generates typed Go configuration code from a protobuf schema. See
[Config Generator](generate-config.md) for the full reference.

```
gofra generate config [flags] <proto-file>
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--out` | `"config"` | Output directory for generated Go files |
| `--package` | `"config"` | Go package name for generated code |
| `--runtime` | `""` | Import path for the framework's `runtime/config` package |

### `gofra help`

Displays usage information.

```
gofra help
gofra --help
gofra -h
```

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Command error (failed to generate, invalid input, etc.) |
| `2` | Usage error (missing required arguments, unknown command) |

## Related Pages

- [Config Generator](generate-config.md) — Full reference for
  `gofra generate config`.
- [Generated App Layout](../starter/generated-app-layout.md) — What `gofra
  new` produces.
- [runtime/config](../runtime/config.md) — Configuration loading used by
  generated apps.
