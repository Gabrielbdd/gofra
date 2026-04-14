# runtime/config

> Configuration loading with four-layer precedence and frontend config serving.

## Status

Alpha — API may change before v1.

## Import

```go
import "databit.com.br/gofra/runtime/config"
```

The package is named `runtimeconfig` in code.

## API

### Load

```go
func Load[T any](defaults T, args []string, opts ...LoadOption) (*T, error)
```

Reads configuration from four layers, merged in precedence order (highest
wins):

1. Struct defaults (the `defaults` parameter)
2. YAML file
3. Environment variables
4. CLI flags (parsed from `args`)

Struct fields must carry `koanf:"..."` tags for correct key mapping.

If `*T` implements `interface{ Validate() error }`, validation runs
automatically after unmarshalling. Returns the validated config or the first
error encountered.

### LoadOption

```go
type LoadOption func(*loadSettings)
```

| Option | Effect | Default |
|--------|--------|---------|
| `WithConfigPath(path)` | Sets the default YAML config file path. Can be overridden by the `GOFRA_CONFIG` env var. | `"gofra.yaml"` |
| `WithEnvPrefix(prefix)` | Sets the environment variable prefix. | `"GOFRA_"` |
| `WithFlags(flags)` | Provides a `pflag.FlagSet` for CLI flag overrides. Only explicitly-set flags take effect. | flags layer skipped |
| `WithoutYAML()` | Disables YAML file loading entirely. | YAML enabled |
| `WithoutEnv()` | Disables environment variable loading. | env enabled |
| `WithoutFlags()` | Disables CLI flag loading even when a FlagSet was provided. | flags enabled |

### Resolver and Binding

```go
type Resolver[T any] interface {
    Resolve(context.Context, *http.Request) (*T, error)
}

type Binder[C, T any] func(*C) (*T, error)
type Mutator[T any]   func(context.Context, *http.Request, *T) error
type Option[T any]    func(*settings[T])
```

```go
func NewResolver[C, T any](source *C, bind Binder[C, T], opts ...Option[T]) Resolver[T]
```

Creates a resolver that binds a source config struct to an output type.
The binder is called on every `Resolve()` call.

```go
func WithMutator[T any](mutator Mutator[T]) Option[T]
```

Adds a mutator that runs after binding to modify the resolved config
per-request (e.g., injecting request-specific values).

### Sentinel Errors

```go
var ErrNilSource = errors.New("runtimeconfig: source is nil")
var ErrNilBinder = errors.New("runtimeconfig: binder is nil")
var ErrNilValue  = errors.New("runtimeconfig: binder returned nil value")
```

Returned by `Resolve()` when the resolver is misconfigured.

### HTTP Handler

```go
func Handler[T any](resolver Resolver[T]) http.Handler
func HandlerWithOptions[T any](resolver Resolver[T], opts HandlerOptions) http.Handler
```

Returns an HTTP handler that resolves the config on each request, serializes
it to JSON, and serves it as a JavaScript snippet.

```go
type HandlerOptions struct {
    GlobalName string // defaults to DefaultGlobalName
}
```

### Constants

| Constant | Value | Description |
|----------|-------|-------------|
| `DefaultPath` | `"/_gofra/config.js"` | Default HTTP path for the config endpoint |
| `DefaultGlobalName` | `"__GOFRA_CONFIG__"` | Default JavaScript global variable name |

## Defaults

| Setting | Default |
|---------|---------|
| Config file path | `gofra.yaml` |
| Config path env var | `GOFRA_CONFIG` |
| Environment variable prefix | `GOFRA_` |
| HTTP path | `/_gofra/config.js` |
| JavaScript global | `window.__GOFRA_CONFIG__` |

## Behavior

### YAML File Loading

The YAML file path defaults to `gofra.yaml` but can be overridden by setting
the `GOFRA_CONFIG` environment variable. If the file does not exist, loading
silently succeeds and proceeds to the next layer. If the file exists but
cannot be parsed, `Load` returns an error.

### Environment Variable Mapping

Environment variables map to config struct fields using the configured prefix.
Nesting is expressed with double underscores (`__`). Single underscores are
preserved as literal characters within a key segment. After removing the prefix,
keys are lowercased.

```
GOFRA_APP__PORT=4000       → app.port = 4000
GOFRA_APP__NAME=myapp      → app.name = "myapp"
GOFRA_PUBLIC__APP_NAME=x   → public.app_name = "x"
```

### CLI Flag Overrides

When a `pflag.FlagSet` is provided via `WithFlags`, only flags that were
explicitly set on the command line override config values. Flags that were
not set are ignored, so default values in the FlagSet do not overwrite
YAML or env values.

If no FlagSet is provided, the flags layer is skipped entirely.

### Config HTTP Handler

The handler accepts `GET` and `HEAD` methods. Other methods receive
`405 Method Not Allowed` with an `Allow: GET, HEAD` header.

Response headers:

| Header | Value |
|--------|-------|
| `Content-Type` | `application/javascript; charset=utf-8` |
| `Cache-Control` | `no-store` |

For `GET` requests, the body is a JavaScript assignment:

```javascript
window.__GOFRA_CONFIG__ = {"appName":"myapp","apiBaseUrl":"http://localhost:3000"};
```

For `HEAD` requests, the status code is `200` but the body is omitted.

If the resolver is `nil`, or resolution/JSON serialization fails, the handler
returns `500 Internal Server Error` with a plain-text body.

### Validation

If the config struct pointer implements `Validate() error`, `Load` calls it
after unmarshalling. Errors are wrapped as
`runtimeconfig: validate: <original error>`.

### Error Wrapping

All errors from `Load` are prefixed with `runtimeconfig:` and a phase
identifier:

| Phase | Prefix |
|-------|--------|
| Struct defaults | `runtimeconfig: load defaults:` |
| YAML parsing | `runtimeconfig: load <path>:` |
| Env loading | `runtimeconfig: load env:` |
| Flag parsing | `runtimeconfig: parse flags:` |
| Flag loading | `runtimeconfig: load flags:` |
| Unmarshalling | `runtimeconfig: unmarshal:` |
| Validation | `runtimeconfig: validate:` |

## Errors and Edge Cases

- If the YAML file does not exist, loading silently succeeds (the struct
  defaults and subsequent layers still apply).
- If the YAML file exists but is malformed, `Load` returns an error.
- If `Validate()` returns an error, `Load` returns it wrapped.
- `Resolve()` returns `ErrNilSource` if the source pointer is nil.
- `Resolve()` returns `ErrNilBinder` if the binder function is nil.
- `Resolve()` returns `ErrNilValue` if the binder returns a nil pointer.
- Nil `LoadOption` values are safely ignored.
- Nil `Option[T]` values are safely ignored.

## Examples

```go
type Config struct {
    App struct {
        Port int    `koanf:"port"`
        Name string `koanf:"name"`
    } `koanf:"app"`
}

func (c *Config) Validate() error {
    if c.App.Port == 0 {
        return fmt.Errorf("app.port is required")
    }
    return nil
}

cfg, err := runtimeconfig.Load(Config{
    App: struct {
        Port int    `koanf:"port"`
        Name string `koanf:"name"`
    }{Port: 8080},
}, os.Args[1:])
```

## Related Pages

- [runtime/serve](serve.md) — Uses the loaded config for server address.
- [Config Generator](../cli/generate-config.md) — Generates typed config
  structs and a `Load()` wrapper from protobuf schemas.
- [Generated App Layout](../starter/generated-app-layout.md) — Shows config
  usage in the starter app.
