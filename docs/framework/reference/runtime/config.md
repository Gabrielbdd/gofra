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

If `*T` implements `interface{ Validate() error }`, validation runs
automatically after loading. Returns the validated config or the first error
encountered.

### LoadOption

```go
type LoadOption func(*loadSettings)
```

| Option | Effect | Default |
|--------|--------|---------|
| `WithConfigPath(path string)` | Sets the YAML config file path | `"gofra.yaml"` |
| `WithEnvPrefix(prefix string)` | Sets the environment variable prefix | `"GOFRA_"` |
| `WithFlags(flags *flag.FlagSet)` | Provides a pflag.FlagSet for CLI flag overrides | auto-created |
| `WithoutYAML()` | Disables YAML file loading | YAML enabled |
| `WithoutEnv()` | Disables environment variable loading | env enabled |
| `WithoutFlags()` | Disables CLI flag loading | flags enabled |

### Resolver and Binding

```go
type Resolver[T any] interface {
    Resolve(context.Context, *http.Request) (*T, error)
}

type Binder[C, T any] func(*C) (*T, error)
type Mutator[T any]   func(context.Context, *http.Request, *T) error
type Option[T any]    func(*settings[T])

func NewResolver[C, T any](source *C, bind Binder[C, T], opts ...Option[T]) Resolver[T]
func WithMutator[T any](mutator Mutator[T]) Option[T]
```

`NewResolver` creates a resolver that binds a source config struct to an
output type. Mutators run after binding to modify the resolved config
per-request (e.g., injecting request-specific values).

### HTTP Handler

```go
func Handler[T any](resolver Resolver[T]) http.Handler
func HandlerWithOptions[T any](resolver Resolver[T], opts HandlerOptions) http.Handler
```

Returns an HTTP handler that serves the resolved config as a JavaScript
snippet, assigning it to a global variable on `window`.

```go
type HandlerOptions struct {
    GlobalName string
}
```

### Constants

| Constant | Value | Description |
|----------|-------|-------------|
| `DefaultPath` | `"/_gofra/config.js"` | Default HTTP path for the config endpoint |
| `DefaultGlobalName` | `"__GOFRA_CONFIG__"` | Default JavaScript global variable name |

## Defaults

- Config file path: `gofra.yaml`
- Environment variable prefix: `GOFRA_`
- HTTP path: `/_gofra/config.js`
- JavaScript global: `window.__GOFRA_CONFIG__`

## Behavior

### Environment Variable Mapping

Environment variables map to config struct fields using the configured prefix.
Nesting is expressed with double underscores (`__`). Single underscores are
preserved as literal characters.

```
GOFRA_APP__PORT=4000    → app.port = 4000
GOFRA_APP_NAME=myapp    → app_name = "myapp"
```

### Config Serving

The HTTP handler resolves the config on each request, serializes it to JSON,
and wraps it in a JavaScript assignment:

```javascript
window.__GOFRA_CONFIG__ = {"app":{"port":4000}};
```

The frontend loads this via a `<script>` tag.

## Errors and Edge Cases

- If the YAML file does not exist and YAML loading is enabled, `Load` returns
  an error.
- If `Validate()` is implemented and returns an error, `Load` returns that
  error.
- If environment variable parsing fails for a typed field, `Load` returns an
  error.

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
- [Generated App Layout](../starter/generated-app-layout.md) — Shows config
  usage in the starter app.
