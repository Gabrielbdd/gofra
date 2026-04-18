# runtime/database

> PostgreSQL connection pool management, schema migrations, and health check
> integration.

## Status

Alpha — API may change before v1.

## Import

```go
import "github.com/Gabrielbdd/gofra/runtime/database"
```

The package is named `runtimedatabase` in code.

## API

### Types

```go
type Config struct {
    DSN               string        `koanf:"dsn"`
    MaxConns          int32         `koanf:"max_conns"`
    MinConns          int32         `koanf:"min_conns"`
    MaxConnLifetime   time.Duration `koanf:"max_conn_lifetime"`
    MaxConnIdleTime   time.Duration `koanf:"max_conn_idle_time"`
    HealthCheckPeriod time.Duration `koanf:"health_check_period"`
}
```

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `DSN` | Yes | — | Postgres connection string (URL or keyword/value format) |
| `MaxConns` | No | pgxpool default: `max(4, numCPU)` | Maximum number of pool connections |
| `MinConns` | No | `0` | Minimum idle connections maintained |
| `MaxConnLifetime` | No | `1h` | Maximum time a connection can be reused |
| `MaxConnIdleTime` | No | `30m` | Maximum time a connection can sit idle |
| `HealthCheckPeriod` | No | `1m` | Interval between automatic pool health checks |

Zero-value fields are left at pgxpool defaults. Pool parameters embedded in
the DSN (e.g., `pool_max_conns=25`) are applied first; non-zero `Config`
fields override them.

```go
type MigrateOption func(*migrateSettings)
```

Configures the behaviour of [Migrate].

### Functions

#### Open

```go
func Open(ctx context.Context, cfg Config) (*pgxpool.Pool, error)
```

Creates a `*pgxpool.Pool` from the given config. The sequence is:

1. Validate that `DSN` is non-empty.
2. Parse the DSN via `pgxpool.ParseConfig`.
3. Overlay non-zero `Config` fields onto the parsed pool config.
4. Create the pool via `pgxpool.NewWithConfig`.
5. Ping the database to verify connectivity.

If the ping fails, the pool is closed and the error is returned. This
provides fail-fast behaviour on startup — the application does not silently
start with a bad DSN.

#### Migrate

```go
func Migrate(
    ctx context.Context,
    pool *pgxpool.Pool,
    fsys fs.FS,
    opts ...MigrateOption,
) ([]*goose.MigrationResult, error)
```

Runs pending goose migrations against the database. It uses the goose
Provider API (not the global goose functions) with Postgres session-level
advisory locks for concurrent safety.

The `fsys` parameter should be an `embed.FS` (or any `fs.FS`) containing the
migration SQL files. By default, `Migrate` looks for a `"migrations"`
subdirectory within `fsys`.

Returns a slice describing each applied migration. If no migrations are
pending, the slice is empty and the error is nil.

Internally, `Migrate` obtains a `*sql.DB` from the pool via
`pgx/v5/stdlib.OpenDBFromPool`. This does not create a separate connection
pool — it borrows connections from the pgxpool and returns them. The
`*sql.DB` is closed when `Migrate` returns; the pgxpool is unaffected.

#### HealthCheck

```go
func HealthCheck(pool *pgxpool.Pool) runtimehealth.CheckFunc
```

Returns a `runtimehealth.CheckFunc` that pings the pool. Pass the result to
`runtimehealth.New` as a readiness check.

### Migrate Options

```go
func WithMigrationsDir(dir string) MigrateOption
```

Sets the subdirectory within the provided `fs.FS` that contains migration
files. Default: `"migrations"`.

```go
func WithAllowOutOfOrder() MigrateOption
```

Permits goose to apply migrations that were created before already-applied
migrations. Useful for teams where multiple developers create migrations on
separate branches.

```go
func WithoutSessionLocker() MigrateOption
```

Disables the Postgres advisory lock that serialises concurrent migration
runs. Only disable this if you are certain that a single process runs
migrations at a time.

```go
func WithGooseTableName(name string) MigrateOption
```

Overrides the goose version table name. Default: `"goose_db_version"`.

## Defaults

| Setting | Default |
|---------|---------|
| Migrations subdirectory | `"migrations"` |
| Session locking | Enabled (Postgres advisory lock) |
| Allow out-of-order | Disabled |
| Goose table name | `"goose_db_version"` |

## Behavior

### DSN Parsing

`Open` accepts any connection string format that pgx supports:

- **URL format:** `postgres://user:password@host:5432/dbname?sslmode=disable`
- **Keyword/value:** `host=localhost dbname=myapp sslmode=disable`

Pool-specific parameters can be embedded in the URL:
`pool_max_conns`, `pool_min_conns`, `pool_max_conn_lifetime`,
`pool_max_conn_idle_time`, `pool_health_check_period`,
`pool_max_conn_lifetime_jitter`.

pgx also reads standard PostgreSQL environment variables (`PGHOST`,
`PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD`, `PGSSLMODE`) when the
connection string omits those parameters.

### Fail-Fast Startup

`pgxpool.NewWithConfig` returns before establishing any connections. Without
an explicit check, the application could start "successfully" with a bad DSN
and only fail on the first query. `Open` calls `pool.Ping(ctx)` immediately
after creation to catch configuration and connectivity errors at startup.

### Migration Locking

By default, `Migrate` acquires a Postgres session-level advisory lock before
running migrations. This prevents race conditions when multiple replicas
start simultaneously with auto-migrate enabled. If the lock is held by
another process, `Migrate` waits (respecting the context deadline) until the
lock is released.

### Error Wrapping

All errors from this package are prefixed with `runtimedatabase:` and a
phase identifier:

| Phase | Prefix |
|-------|--------|
| Empty DSN | `runtimedatabase: dsn is required` |
| DSN parsing | `runtimedatabase: parse dsn:` |
| Pool creation | `runtimedatabase: create pool:` |
| Ping | `runtimedatabase: ping:` |
| FS subdirectory | `runtimedatabase: fs.Sub "<dir>":` |
| Locker creation | `runtimedatabase: create session locker:` |
| Provider creation | `runtimedatabase: create goose provider:` |
| Migration execution | `runtimedatabase: goose up:` |

## Errors and Edge Cases

- If `DSN` is empty, `Open` returns an error without attempting to parse.
- If the database is unreachable, `Open` returns a ping error after closing
  the pool.
- If the migrations `fs.FS` does not contain the expected subdirectory,
  `Migrate` returns an `fs.Sub` error.
- If the `fs.FS` contains no migration files, `Migrate` returns an empty
  result slice and nil error.
- `HealthCheck` accepts a nil pool without panicking during construction.
  The returned function will fail when called.
- `Migrate` closes the internal `*sql.DB` bridge on return. The pgxpool
  remains open.

## Examples

### Pool creation with health check

```go
pool, err := runtimedatabase.Open(ctx, runtimedatabase.Config{
    DSN:      cfg.Database.DSN,
    MaxConns: cfg.Database.MaxConns,
    MinConns: cfg.Database.MinConns,
})
if err != nil {
    slog.Error("database connection failed", "error", err)
    os.Exit(1)
}
defer pool.Close()

checker := runtimehealth.New(
    runtimehealth.Check{
        Name: "postgres",
        Fn:   runtimedatabase.HealthCheck(pool),
    },
)
```

### Optional auto-migration on startup

```go
// db/embed.go — app-owned, adjacent to the migrations directory
package db

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
```

```go
// cmd/app/main.go
if cfg.Database.AutoMigrate {
    results, err := runtimedatabase.Migrate(ctx, pool, db.Migrations)
    if err != nil {
        slog.Error("migration failed", "error", err)
        os.Exit(1)
    }
    for _, r := range results {
        slog.Info("migration applied",
            "version", r.Source.Version,
            "duration", r.Duration,
        )
    }
}
```

### Integration with runtime/serve shutdown

```go
err := runtimeserve.Serve(ctx, runtimeserve.Config{
    Handler: mux,
    Addr:    ":3000",
    Health:  checker,
    OnShutdown: func(ctx context.Context) error {
        pool.Close()
        return nil
    },
})
```

## Related Pages

- [runtime/health](health.md) — Accepts the `CheckFunc` returned by
  `HealthCheck` for readiness probes.
- [runtime/serve](serve.md) — `OnShutdown` callback is where the pool is
  closed during graceful shutdown.
- [runtime/config](config.md) — Loads the `DatabaseConfig` that supplies
  pool parameters.
