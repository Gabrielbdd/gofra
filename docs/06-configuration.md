# 06 — Configuration: koanf

> Parent: [Index](00-index.md) | Prev: [Database](05-database.md) | Next: [Observability](07-observability.md)


## Addendum to Architecture Design Document
## Last Updated: 2026-04-12

---

## The Problem

A Gofra application needs configuration from three sources with clear
precedence:

1. **YAML file** (`gofra.yaml`) — project defaults, checked into version
   control. Defines the shape of the config and documents every option.
2. **Environment variables** — deployment-specific overrides. Secrets, DSNs,
   API keys. Set by the platform (Docker, Kubernetes, systemd).
3. **CLI flags** — one-off overrides for development and debugging.
   `--port=4000`, `--log-level=debug`.

Precedence: **flags > env > yaml file > defaults**. A flag overrides an env
var, which overrides the YAML file, which overrides a hardcoded default.

---

## Why koanf

### Options considered

| Library | Verdict |
|---------|---------|
| **viper** | Most popular. But: forcibly lowercases all keys (breaks YAML/JSON spec), bloated dependencies (pulls in every parser even if unused), returns references to internal maps (mutation footguns), no semantic versioning. |
| **envconfig** | Only reads env vars. No file support. No flags. Too narrow. |
| **ff** | Clean but minimal. No YAML support without custom parser. No nested structs. |
| **koanf** | Modular providers (file, env, flags). Modular parsers (YAML, JSON, TOML). Dependencies are opt-in per-provider. Correct key casing. Unmarshal to typed structs. Active, well-maintained. |

### Why koanf wins

**Modular deps.** koanf's core has zero external dependencies. You `go get`
only the providers and parsers you use: `koanf/providers/file`,
`koanf/providers/env`, `koanf/parsers/yaml`. No parser you don't use ends up
in your binary.

**Correct key handling.** Viper lowercases `DATABASE_URL` to `database_url`
and breaks round-tripping. koanf preserves keys exactly as they are in each
source. The delimiter (`.` by default) maps nested YAML keys to flat env vars
via a transform function — you control the mapping.

**Clean merge semantics.** You load sources in order. Each `Load()` call
merges into the existing config. Last writer wins per key. The order is
explicit in your code, not imposed by the library.

**Unmarshal to structs.** `k.Unmarshal("", &cfg)` produces a typed Go struct
from the merged config. Fields use `koanf:"field_name"` tags. Nested structs
map to nested YAML keys naturally.

---

## Design

### Config Struct

The config is a typed Go struct. Every field has a `koanf` tag and a comment.
This struct IS the documentation — a developer reads it to know every
configurable option.

```go
// config/config.go
package config

import "time"

type Config struct {
    App      AppConfig      `koanf:"app"`
    Database DatabaseConfig `koanf:"database"`
    Restate  RestateConfig  `koanf:"restate"`
    Auth     AuthConfig     `koanf:"auth"`
    Public   PublicConfig   `koanf:"public"`        // Generated from proto/myapp/runtime/v1/runtime_config.proto
    OTEL     OTELConfig     `koanf:"observability"`
    Mail     MailConfig     `koanf:"mail"`
    Storage  StorageConfig  `koanf:"storage"`
}

type AppConfig struct {
    Name    string `koanf:"name"`     // Application name, used in logs and OTEL resource
    Env     string `koanf:"env"`      // development, staging, production
    Port    int    `koanf:"port"`     // HTTP server port
    Version string `koanf:"version"`  // Set at build time via ldflags
}

func (a AppConfig) IsProduction() bool { return a.Env == "production" }
func (a AppConfig) IsDevelopment() bool { return a.Env == "development" }

type DatabaseConfig struct {
    DSN          string        `koanf:"dsn"`           // Postgres connection string
    MaxOpenConns int           `koanf:"max_open_conns"` // Max open connections
    MaxIdleConns int           `koanf:"max_idle_conns"` // Max idle connections
    MaxLifetime  time.Duration `koanf:"max_lifetime"`   // Connection max lifetime
    AutoMigrate  bool          `koanf:"auto_migrate"`   // Run goose up on startup
}

type RestateConfig struct {
    IngressURL  string `koanf:"ingress_url"`  // Restate server ingress endpoint
    ServicePort int    `koanf:"service_port"` // Port for Restate service endpoint
    AutoStart   bool   `koanf:"auto_start"`   // Auto-start restate-server in dev
}

type AuthConfig struct {
    Issuer                 string                    `koanf:"issuer"`                    // Zitadel issuer URL
    Audience               string                    `koanf:"audience"`                  // Expected access token audience
    ClientID               string                    `koanf:"client_id"`                 // Browser SPA OIDC client ID
    Scopes                 []string                  `koanf:"scopes"`                    // OIDC scopes requested by browser clients
    RedirectPath           string                    `koanf:"redirect_path"`             // Browser callback path
    PostLogoutRedirectPath string                    `koanf:"post_logout_redirect_path"` // Browser logout return path
    BrowserTokenStore      string                    `koanf:"browser_token_store"`       // session_storage
    UseRefreshTokens       bool                      `koanf:"use_refresh_tokens"`        // Request offline_access and rotating refresh tokens
    ServiceAccount         AuthServiceAccountConfig  `koanf:"service_account"`           // Zitadel management API credentials
}

type AuthServiceAccountConfig struct {
    KeyPath string `koanf:"key_path"` // Path to Zitadel JWT-profile service account key
}

// PublicConfig is generated from the runtime-config proto so the app-owned
// root config only needs one stable `Public` field.
type PublicConfig struct {
    APIBaseURL string           `koanf:"api_base_url"`
    SentryDSN  string           `koanf:"sentry_dsn"`
    Auth       PublicAuthConfig `koanf:"auth"`
}

type PublicAuthConfig struct {
    Issuer                 string   `koanf:"issuer"`
    ClientID               string   `koanf:"client_id"`
    Scopes                 []string `koanf:"scopes"`
    RedirectPath           string   `koanf:"redirect_path"`
    PostLogoutRedirectPath string   `koanf:"post_logout_redirect_path"`
}

type OTELConfig struct {
    Endpoint        string  `koanf:"endpoint"`          // OTLP collector endpoint
    LogLevel        string  `koanf:"log_level"`         // debug, info, warn, error
    TraceSampleRate float64 `koanf:"trace_sample_rate"` // 0.0 to 1.0
    ServiceName     string  `koanf:"service_name"`      // OTEL resource service name
}

type MailConfig struct {
    Driver   string `koanf:"driver"`    // smtp, log
    From     string `koanf:"from"`      // Default from address
    SMTPHost string `koanf:"smtp_host"`
    SMTPPort int    `koanf:"smtp_port"`
    SMTPUser string `koanf:"smtp_user"`
    SMTPPass string `koanf:"smtp_pass"`
}

type StorageConfig struct {
    Driver    string `koanf:"driver"`     // local, s3
    LocalPath string `koanf:"local_path"` // Path for local storage
    S3Bucket  string `koanf:"s3_bucket"`
    S3Region  string `koanf:"s3_region"`
}
```

**Reason for a concrete struct (not `k.String("app.port")` calls throughout
the code)**: A struct is type-checked at compile time. If someone adds a field
to the YAML but forgets to add it to the struct, `Unmarshal` ignores it — but
the field is never used, so the developer notices. If someone removes a field
from the struct, every caller that referenced it fails to compile. Scattered
`k.String()` calls are stringly-typed and impossible to refactor safely.

### YAML File

```yaml
# gofra.yaml — project defaults (checked into version control)

app:
  name: myapp
  env: development
  port: 3000

database:
  dsn: "postgres://localhost/myapp_dev?sslmode=disable"
  max_open_conns: 25
  max_idle_conns: 5
  max_lifetime: 5m
  auto_migrate: true

restate:
  ingress_url: "http://localhost:8080"
  service_port: 9080
  auto_start: true

auth:
  issuer: "http://localhost:8080"
  audience: "myapp-api"
  client_id: "${ZITADEL_BROWSER_CLIENT_ID}"
  scopes:
    - openid
    - profile
    - email
    - offline_access
    - urn:zitadel:iam:org:projects:roles
  redirect_path: "/auth/callback"
  post_logout_redirect_path: "/"
  browser_token_store: session_storage
  use_refresh_tokens: true
  service_account:
    key_path: "${ZITADEL_SERVICE_ACCOUNT_KEY}"

public:
  api_base_url: "http://localhost:3000"
  auth:
    issuer: "http://localhost:9000"
    client_id: "${ZITADEL_BROWSER_CLIENT_ID}"
    scopes:
      - openid
      - profile
      - email
      - offline_access
      - urn:zitadel:iam:org:projects:roles
    redirect_path: "/auth/callback"
    post_logout_redirect_path: "/"

observability:
  endpoint: "localhost:4317"
  log_level: debug
  trace_sample_rate: 1.0
  service_name: myapp

mail:
  driver: log
  from: "noreply@myapp.local"

storage:
  driver: local
  local_path: ./storage/app
```

**Reason for YAML over TOML or JSON**: YAML supports comments (JSON doesn't).
YAML is more readable for nested config than TOML. Most developers in the
web ecosystem are already familiar with YAML from Docker Compose, Kubernetes,
and GitHub Actions. TOML is fine too — koanf supports both — but YAML is the
default.

**Reason gofra.yaml is checked into version control**: It contains project
defaults, not secrets. `database.dsn` points to `localhost` for development.
Production overrides come from environment variables.

### Environment Variables

Env vars override YAML values. The mapping convention:

```
YAML path             → env var
app.name              → GOFRA_APP_NAME
app.port              → GOFRA_APP_PORT
database.dsn          → GOFRA_DATABASE_DSN
database.auto_migrate → GOFRA_DATABASE_AUTO_MIGRATE
restate.ingress_url   → GOFRA_RESTATE_INGRESS_URL
auth.issuer           → GOFRA_AUTH_ISSUER
auth.client_id        → GOFRA_AUTH_CLIENT_ID
public.api_base_url   → GOFRA_PUBLIC_API_BASE_URL
public.sentry_dsn     → GOFRA_PUBLIC_SENTRY_DSN
public.auth.issuer    → GOFRA_PUBLIC_AUTH_ISSUER
public.auth.client_id → GOFRA_PUBLIC_AUTH_CLIENT_ID
auth.redirect_path    → GOFRA_AUTH_REDIRECT_PATH
observability.endpoint→ GOFRA_OBSERVABILITY_ENDPOINT
mail.smtp_pass        → GOFRA_MAIL_SMTP_PASS
```

The transform: uppercase, replace `.` with `_`, prefix with `GOFRA_`.

**Reason for the `GOFRA_` prefix**: Prevents collisions with other env vars.
`PORT` is commonly set by platforms (Heroku, Cloud Run). `GOFRA_APP_PORT`
is unambiguous.

**Reason env vars override YAML**: The YAML file has development defaults.
In production, the platform sets `GOFRA_DATABASE_DSN` to the real connection
string. The developer doesn't need a separate `gofra.production.yaml` — env
vars handle environment-specific config.

### CLI Flags

Flags override everything, for one-off development use:

```bash
# Override port for this run
./myapp --app.port=4000

# Override log level
./myapp --observability.log_level=debug

# Override database DSN
./myapp --database.dsn="postgres://localhost/myapp_test"
```

**Reason flags use the same dotted key paths as YAML**: No separate flag
naming convention to learn. `--app.port=4000` maps to `app.port` in the YAML,
which maps to `GOFRA_APP_PORT` in env. One key path, three sources.

**Reason flags are not the primary config mechanism**: Flags are awkward for
12+ config values. They're useful for quick overrides during development
(`--app.port=4000`) but not for production config. Production uses env vars.

---

## Loading Implementation

```go
// config/load.go
package config

import (
    "log/slog"
    "os"
    "strings"

    "github.com/knadh/koanf/v2"
    "github.com/knadh/koanf/parsers/yaml"
    "github.com/knadh/koanf/providers/env"
    "github.com/knadh/koanf/providers/file"
    "github.com/knadh/koanf/providers/posflag"
    flag "github.com/spf13/pflag"
)

func Load() (*Config, error) {
    k := koanf.New(".")

    // Layer 1: Defaults (hardcoded)
    // Embedded in the struct via zero values + explicit defaults below
    k.Load(confmap.Provider(map[string]interface{}{
        "app.port":                  3000,
        "app.env":                   "development",
        "database.max_open_conns":   25,
        "database.max_idle_conns":   5,
        "database.max_lifetime":     "5m",
        "database.auto_migrate":     false,
        "restate.service_port":      9080,
        "auth.redirect_path":        "/auth/callback",
        "auth.post_logout_redirect_path": "/",
        "auth.browser_token_store":  "session_storage",
        "auth.use_refresh_tokens":   true,
        "auth.scopes": []string{
            "openid",
            "profile",
            "email",
            "offline_access",
            "urn:zitadel:iam:org:projects:roles",
        },
        "observability.log_level":   "info",
        "observability.trace_sample_rate": 0.1,
        "mail.driver":               "log",
        "storage.driver":            "local",
        "storage.local_path":        "./storage/app",
    }, "."), nil)

    // Layer 2: YAML file
    configPath := "gofra.yaml"
    if p := os.Getenv("GOFRA_CONFIG"); p != "" {
        configPath = p
    }
    if _, err := os.Stat(configPath); err == nil {
        if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
            return nil, fmt.Errorf("loading %s: %w", configPath, err)
        }
    }

    // Layer 3: Environment variables
    // GOFRA_APP_PORT → app.port
    k.Load(env.Provider("GOFRA_", ".", func(s string) string {
        return strings.Replace(
            strings.ToLower(strings.TrimPrefix(s, "GOFRA_")),
            "_", ".", -1,
        )
    }), nil)

    // Layer 4: CLI flags (highest precedence)
    f := flag.NewFlagSet("gofra", flag.ContinueOnError)
    f.Int("app.port", 0, "HTTP server port")
    f.String("app.env", "", "Environment (development, staging, production)")
    f.String("database.dsn", "", "Database connection string")
    f.String("auth.issuer", "", "OIDC issuer URL")
    f.String("auth.client_id", "", "OIDC browser client ID")
    f.String("observability.log_level", "", "Log level")
    f.Bool("database.auto_migrate", false, "Run migrations on startup")
    f.Parse(os.Args[1:])

    // Only load flags that were explicitly set (not defaults)
    k.Load(posflag.Provider(f, ".", k), nil)

    // Unmarshal into typed struct
    var cfg Config
    if err := k.Unmarshal("", &cfg); err != nil {
        return nil, fmt.Errorf("unmarshaling config: %w", err)
    }

    return &cfg, nil
}
```

**Reason for four layers in this order**: Defaults provide sane values when
nothing is configured. YAML overrides defaults with project-specific settings.
Env vars override YAML with deployment-specific values. Flags override
everything for ad-hoc debugging. This is the standard precedence in the
12-factor app methodology.

**Reason for `GOFRA_CONFIG` env var**: Allows overriding the config file path
for testing or alternative configurations without changing code.

**Reason posflag loads only explicitly-set flags**: Without this, the default
value of a flag (e.g., `0` for `--app.port`) would override the YAML and env
values. koanf's `posflag.Provider` with the koanf instance as the second
argument ensures only flags the user actually typed on the command line take
effect.

---

## Usage in Application

```go
// cmd/app/main.go
func main() {
    cfg, err := config.Load()
    if err != nil {
        slog.Error("failed to load config", "err", err)
        os.Exit(1)
    }

    db, err := gofra.OpenDB(cfg.Database)
    // cfg.Database is a typed DatabaseConfig — not a string lookup

    if cfg.Database.AutoMigrate {
        runMigrations(db)
    }

    // ...
}
```

**No global config singleton.** The `*Config` is created in `main()` and
passed explicitly to everything that needs it. This is the same explicit
dependency injection pattern used for the database connection, Restate client,
and every other dependency.

## Public Runtime Config For The Browser

The browser must not read raw environment variables directly. Gofra exposes an
explicit public runtime-config contract at `GET /_gofra/config.js`.

The public browser contract is not the same type as `config.Config`. It lives
in its own proto so the browser-safe allowlist is explicit:

```proto
// proto/myapp/runtime/v1/runtime_config.proto
message RuntimeConfig {
  string api_base_url = 1;
  string sentry_dsn = 2;
  AuthConfig auth = 3;
}

message AuthConfig {
  string issuer = 1;
  string client_id = 2;
  repeated string scopes = 3;
  string redirect_path = 4;
  string post_logout_redirect_path = 5;
}
```

Adding a new browser-safe field follows one path:

1. Add the field to `proto/myapp/runtime/v1/runtime_config.proto`.
2. Regenerate code.
3. Set the value under `public.*` via YAML, env vars, or CLI flags.
4. Consume the new typed field from the generated frontend API.

The common case should require no handwritten Go mapping and no parallel
TypeScript edits.

Gofra generates a dedicated public config subtree from the runtime-config proto
and keeps the app-owned root config stable:

```go
type Config struct {
    App    AppConfig    `koanf:"app"`
    Auth   AuthConfig   `koanf:"auth"`
    Public PublicConfig `koanf:"public"` // Generated type
}
```

For the example proto above, the generated config values are loaded from the
reserved `public.*` namespace:

```yaml
public:
  api_base_url: "http://localhost:3000"
  sentry_dsn: "https://example.ingest.sentry.io/123"
  auth:
    issuer: "http://localhost:9000"
    client_id: "myapp-web"
```

```bash
GOFRA_PUBLIC_API_BASE_URL=http://localhost:3000
GOFRA_PUBLIC_SENTRY_DSN=https://example.ingest.sentry.io/123
GOFRA_PUBLIC_AUTH_ISSUER=http://localhost:9000
GOFRA_PUBLIC_AUTH_CLIENT_ID=myapp-web
```

```bash
./myapp \
  --public.api_base_url=http://localhost:3000 \
  --public.sentry_dsn=https://example.ingest.sentry.io/123
```

Generated and handwritten responsibilities are split cleanly:

- `proto/myapp/runtime/v1/runtime_config.proto` is the app-owned browser
  contract
- `config/config.go` remains the handwritten server config root with one stable
  `Public PublicConfig` field
- `config/public_config_types_gen.go` contains generated `PublicConfig` and
  nested types derived from the runtime-config proto
- `config/public_config_gen.go` contains the generated binder from `cfg.Public`
  to `runtimev1.RuntimeConfig`
- `config/public_config.go` contains app-owned wiring and optional custom logic
- `runtime/config/` contains the reusable resolver and HTTP handler behavior
- `web/src/gen/runtime/` contains the generated frontend loader and
  `Window.__GOFRA_CONFIG__` typing
- the intended public generator shape is `gofra generate runtime-config`; the
  framework repo currently also carries a temporary dedicated slice entrypoint
  while that wiring lands

```go
resolver := runtimeconfig.NewResolver(appCfg, BindPublicConfig)
mux.Handle("/_gofra/config.js", runtimeconfig.Handler(resolver))
```

This generated code uses typed Go field access, not runtime reflection. The
generated binder reads `cfg.Public`, so adding a runtime-config proto field does
not require manual edits to the handwritten root `Config` type.

The generator contract is:

- input: runtime-config proto descriptor
- output: generated Go `PublicConfig` types, a binder from `cfg.Public` to the
  runtime proto, and the frontend runtime-config loader/types
- failure mode: generation stops if the public contract cannot be rendered into
  valid generated code

The reusable Go API is generic over the application config type and the public
runtime-config type:

```go
type Resolver[T any] interface {
    Resolve(context.Context, *http.Request) (*T, error)
}

type Binder[C any, T any] func(*C) (*T, error)

type Mutator[T any] func(context.Context, *http.Request, *T) error
type Option[T any] func(*settings[T])

func NewResolver[C any, T any](source *C, bind Binder[C, T], opts ...Option[T]) Resolver[T]
func WithMutator[T any](Mutator[T]) Option[T]
func Handler[T any](r Resolver[T]) http.Handler
```

The common case is zero handwritten mapping code. For derived or request-aware
public config, the application can add a mutator:

```go
resolver := runtimeconfig.NewResolver(
    appCfg,
    BindPublicConfig,
    runtimeconfig.WithMutator(func(ctx context.Context, r *http.Request, cfg *runtimev1.RuntimeConfig) error {
        // optional dynamic overrides
        return nil
    }),
)
```

This is the escape hatch for values such as `api_base_url` derived from
`app.port`, per-request tenant branding, or environment-dependent CDN origins.
It should be the exception, not the default path.

The handler serializes the resolved value to JavaScript:

```go
value, err := resolver.Resolve(r.Context(), r)
payload, err := json.Marshal(value)
fmt.Fprintf(w, "window.__GOFRA_CONFIG__ = %s;\n", payload)
```

The handler accepts only `GET` and `HEAD`. Handler failures return `500` and do
not emit partial config.

Until the full proto-driven runtime-config generation is wired into the starter,
`gofra new` checks in starter-owned placeholder files under `gen/`,
`config/public_config_types_gen.go`, and `config/public_config_gen.go` that
mirror the future generated output shape.

**Reason for a dedicated public proto instead of exposing env vars directly**:
the server config contains secrets and server-only settings. The browser should
receive only an explicit allowlist of safe values.

**Reason for a generated `public.*` config subtree instead of handwritten root
config fields**: changing the public browser contract should not require manual
edits to the app-owned root `Config` type. The user edits the proto, sets
`public.*` values, regenerates, and gets matching Go and TypeScript types.

**Reason for keeping `public.*` explicit even when some values overlap with
server config**: the browser allowlist is its own product contract. Gofra
should not infer browser-visible fields from arbitrary server config.

**Reason for `/_gofra/config.js` instead of build-time `VITE_*` variables**:
the same frontend bundle can run in different environments without a rebuild.
The deployment changes server config, not compiled frontend assets.

**Reason for JavaScript instead of HTML templating**: Gofra keeps the HTML shell
static in both dev and prod. The public runtime config path stays the same
whether the browser is loading Vite-served assets or embedded production files.

---

## Validation

After loading and unmarshaling, validate required fields:

```go
func (c *Config) Validate() error {
    var errs []error
    if c.Database.DSN == "" {
        errs = append(errs, fmt.Errorf("database.dsn is required"))
    }
    if c.Auth.Issuer == "" {
        errs = append(errs, fmt.Errorf("auth.issuer is required"))
    }
    if c.Auth.ClientID == "" {
        errs = append(errs, fmt.Errorf("auth.client_id is required"))
    }
    if c.App.Port < 1 || c.App.Port > 65535 {
        errs = append(errs, fmt.Errorf("app.port must be between 1 and 65535"))
    }
    if c.Restate.IngressURL == "" {
        errs = append(errs, fmt.Errorf("restate.ingress_url is required"))
    }
    return errors.Join(errs...)
}
```

**Reason for manual validation over struct tags**: Config validation is a
startup concern, not a request-time concern. It runs once. The rules are
simple (non-empty, valid range). A 10-line function is clearer than struct
tags with a validation library.

---

## Environment-Specific Config

No multiple YAML files (`gofra.dev.yaml`, `gofra.prod.yaml`). The pattern is:

- `gofra.yaml` contains development defaults (checked in)
- Production sets env vars: `GOFRA_DATABASE_DSN`, `GOFRA_APP_ENV=production`,
  `GOFRA_OBSERVABILITY_TRACE_SAMPLE_RATE=0.1`, etc.
- Staging is the same as production with different env var values

**Reason for no per-environment YAML files**: Per-environment files proliferate
and drift. A developer adds a field to `gofra.yaml` but forgets
`gofra.production.yaml`. Env vars are the standard for deployment-specific
config in containerized environments. The YAML file is for project structure,
not deployment configuration.

---

## Sensitive Values

Secrets (database passwords, SMTP credentials, API keys) must come from env
vars, never from `gofra.yaml`:

```yaml
# gofra.yaml — NO secrets here
auth:
  issuer: "https://auth.myapp.com"
  client_id: "myapp-browser" # public OIDC client ID, safe to check in
  service_account:
    key_path: "" # set via GOFRA_AUTH_SERVICE_ACCOUNT_KEY_PATH

mail:
  driver: smtp
  smtp_host: smtp.example.com
  smtp_port: 587
  smtp_user: ""    # set via GOFRA_MAIL_SMTP_USER
  smtp_pass: ""    # set via GOFRA_MAIL_SMTP_PASS
```

```bash
# Production env
export GOFRA_MAIL_SMTP_USER="noreply@myapp.com"
export GOFRA_MAIL_SMTP_PASS="s3cret"
export GOFRA_DATABASE_DSN="postgres://user:pass@db.internal/myapp"
```

**Reason**: `gofra.yaml` is in version control. Secrets in version control
is a security incident. Env vars are the standard mechanism for secrets in
Docker, Kubernetes, and every PaaS.

---

## Decision Log (Configuration)

| # | Decision | Rationale |
|---|----------|-----------|
| 58 | koanf over viper | No forced lowercasing. Modular deps (only import what you use). Correct merge semantics. Active maintenance. |
| 59 | YAML over TOML | Supports comments (unlike JSON). Familiar from Docker/K8s ecosystem. More readable for nested config. |
| 60 | Four-layer precedence: defaults → YAML → env → flags | 12-factor app standard. YAML for project defaults. Env for deployment. Flags for debugging. |
| 61 | `GOFRA_` prefix for env vars | Prevents collisions with platform env vars like `PORT`, `DATABASE_URL`. |
| 62 | Single `gofra.yaml` (no per-environment files) | Per-environment files drift. Env vars are the standard for deployment-specific config. |
| 63 | Typed struct, not `k.String()` calls | Compile-time type checking. Single place to see all config options. Refactor-safe. |
| 64 | No global config singleton | Config is a value passed in `main()`. Follows the same DI pattern as every other dependency. |
| 65 | Manual validation over struct tags | Startup-time concern. Simple rules. 10-line function is clearer than a validation framework. |
| 66 | Secrets only via env vars | YAML is in version control. Secrets in VCS is a security incident. |
| 132 | Generated public runtime config for the browser | Browser gets an explicit proto-defined safe subset at `/_gofra/config.js`, loaded from generated `public.*` config and emitted as typed Go and TS APIs. |
