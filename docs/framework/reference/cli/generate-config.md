# gofra generate config

> Generates typed Go configuration code from a protobuf schema.

## Status

Alpha — generated output and annotation format may change before v1.

## Command

```
gofra generate config [flags] <proto-file>
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `proto-file` | Yes | Path to the `.proto` file defining your config schema |

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--out` | `"config"` | Output directory for generated Go files |
| `--package` | `"config"` | Go package name for generated code |
| `--runtime` | `""` | Import path for the framework's `runtime/config` package |

The `--runtime` flag is required when the generated code needs to import
`runtimeconfig`. In generated apps, this is set to the framework module's
`runtime/config` path.

## Proto Schema Requirements

The proto file must:

1. Define exactly one top-level message named `Config`.
2. Use only messages defined in the same proto file (no imported message
   types).
3. Use only supported scalar types: `string`, `int32`, `int64`, `uint32`,
   `uint64`, `bool`, `float`, `double`, and `repeated string`.
4. Not use `enum`, `bytes`, `map`, or other unsupported types.
5. Not mark any field as `secret` under the `public` subtree.

## Proto Annotations

The generator uses custom field annotations from
`gofra/config/v1/annotations.proto`. Import it in your config proto:

```protobuf
import "gofra/config/v1/annotations.proto";
```

### Default Values

Annotate fields with default values that apply when no YAML, env, or flag
override is present:

```protobuf
string name = 1 [(gofra.config.v1.field).default_value.string_value = "myapp"];
int32 port = 2  [(gofra.config.v1.field).default_value.int32_value = 3000];
bool debug = 3  [(gofra.config.v1.field).default_value.bool_value = false];
double rate = 4 [(gofra.config.v1.field).default_value.double_value = 1.5];
```

For `repeated string` fields:

```protobuf
repeated string scopes = 1 [(gofra.config.v1.field).default_value.string_list = {
  values: ["openid", "profile", "email"]
}];
```

Supported default value types:

| Oneof variant | Proto field type |
|---------------|-----------------|
| `string_value` | `string` |
| `int32_value` | `int32` |
| `int64_value` | `int64` |
| `bool_value` | `bool` |
| `double_value` | `float`, `double` |
| `string_list` | `repeated string` |

### Secret Fields

Mark sensitive fields to exclude them from CLI flag registration:

```protobuf
string database_dsn = 1 [(gofra.config.v1.field).secret = true];
```

Secret fields:

- Are excluded from the generated `NewFlagSet()` (not settable via CLI).
- Must not appear under the `public` subtree (enforced at generation time).
- Are still loadable via YAML and environment variables.

## Generated Output

The generator produces three files in the output directory:

### `config_gen.go`

Contains Go struct types for each proto message and a `DefaultConfig()`
function.

**Struct types:**

- One Go struct per proto message, with fields in proto field-number order.
- Field name conversion: `snake_case` → `CamelCase` (with Go naming fixes:
  `Id` → `ID`, `Url` → `URL`, `Dsn` → `DSN`, `Api` → `API`).
- Struct tags:
  - `koanf:"<snake_case>"` — always present.
  - `yaml:"<snake_case>"` — always present.
  - `json:"<camelCase>"` — present only for fields in the `public` subtree
    (uses proto's `jsonName` convention).
- Proto leading comments are preserved as Go comments.

**`DefaultConfig()`:**

```go
func DefaultConfig() *Config
```

Returns a `*Config` with all proto-annotated defaults applied recursively.
Fields without default annotations use Go zero values.

### `load_gen.go`

Contains `NewFlagSet()` and `Load()` functions.

**`NewFlagSet()`:**

```go
func NewFlagSet() *flag.FlagSet
```

Returns a `pflag.FlagSet` with CLI flags for all non-secret, non-repeated
scalar fields. Flag names use dotted key paths matching the YAML/koanf
structure:

```
--app.port
--app.name
--public.app_name
--public.auth.issuer
```

Descriptions are taken from proto comments (trailing period removed).

**`Load()`:**

```go
func Load(args []string, opts ...runtimeconfig.LoadOption) (*Config, error)
```

Convenience wrapper that calls `runtimeconfig.Load(*DefaultConfig(), args, ...)`
with the generated `NewFlagSet()` pre-configured. Additional `LoadOption`
values can be passed to customize behavior.

### `public_gen.go`

Generated only when the root `Config` message has a field named `public` that
is a message type. Contains `BindPublicConfig()` and `PublicConfigHandler()`.

**`BindPublicConfig()`:**

```go
func BindPublicConfig(cfg *Config) (*PublicConfig, error)
```

Returns a copy of the public config subtree. A fresh copy is returned on each
call so mutators cannot modify the shared application config. Returns an error
if `cfg` is nil.

**`PublicConfigHandler()`:**

```go
func PublicConfigHandler(
    cfg *Config,
    opts ...runtimeconfig.Option[PublicConfig],
) http.Handler
```

Returns an HTTP handler that serves the public config as JavaScript at
`/_gofra/config.js`. Wraps `BindPublicConfig` with a `NewResolver` and
delegates to `runtimeconfig.Handler`. Optional mutators can be passed for
per-request config modification.

## Example Proto Schema

```protobuf
syntax = "proto3";

package myapp.config.v1;

import "gofra/config/v1/annotations.proto";

message Config {
  AppConfig app = 1;
  PublicConfig public = 2;
}

message AppConfig {
  string name = 1 [(gofra.config.v1.field).default_value.string_value = "myapp"];
  int32 port = 2 [(gofra.config.v1.field).default_value.int32_value = 3000];
  string database_dsn = 3 [(gofra.config.v1.field).secret = true];
}

message PublicConfig {
  string app_name = 1 [(gofra.config.v1.field).default_value.string_value = "myapp"];
  string api_base_url = 2 [(gofra.config.v1.field).default_value.string_value = "http://localhost:3000"];
}
```

Running:

```bash
gofra generate config \
  --out config \
  --runtime github.com/Gabrielbdd/gofra/runtime/config \
  proto/myapp/config/v1/config.proto
```

Produces `config/config_gen.go`, `config/load_gen.go`, and
`config/public_gen.go`.

## Errors

| Error | Cause |
|-------|-------|
| `does not define a message named Config` | Proto file has no `Config` message |
| `uses imported message type` | Config field uses a message from another proto file |
| `unsupported proto type` | Field uses `enum`, `bytes`, `map`, etc. |
| `marked as secret but is under the public subtree` | Secret field in the `public` message tree |

## Related Pages

- [gofra CLI](gofra.md) — Parent command reference.
- [runtime/config](../runtime/config.md) — The runtime library that the
  generated `Load()` function delegates to.
- [Generated App Layout](../starter/generated-app-layout.md) — Shows the
  generated config in context.
