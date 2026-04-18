// Package runtimedatabase provides PostgreSQL connection pool management and
// a health check integration. It wraps pgxpool for pool creation and exposes
// a [runtimehealth.CheckFunc] for readiness probes.
package runtimedatabase

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	runtimehealth "github.com/Gabrielbdd/gofra/runtime/health"
)

// Config holds pool parameters for [Open]. Fields with zero values are left
// at the pgxpool defaults, which are derived from the DSN or from pgxpool's
// built-in defaults.
type Config struct {
	// DSN is the Postgres connection string. It may be a URL
	// ("postgres://user:pass@host/db") or a keyword/value string
	// ("host=localhost dbname=myapp"). Required.
	DSN string `koanf:"dsn"`

	// MaxConns is the maximum number of connections in the pool.
	// pgxpool default: max(4, runtime.NumCPU()).
	MaxConns int32 `koanf:"max_conns"`

	// MinConns is the minimum number of idle connections the pool maintains.
	// pgxpool default: 0.
	MinConns int32 `koanf:"min_conns"`

	// MaxConnLifetime is the maximum duration a connection can be reused.
	// pgxpool default: 1 hour.
	MaxConnLifetime time.Duration `koanf:"max_conn_lifetime"`

	// MaxConnIdleTime is the maximum duration a connection can sit idle
	// before being closed. pgxpool default: 30 minutes.
	MaxConnIdleTime time.Duration `koanf:"max_conn_idle_time"`

	// HealthCheckPeriod is the interval between automatic pool health
	// checks. pgxpool default: 1 minute.
	HealthCheckPeriod time.Duration `koanf:"health_check_period"`
}

// Open creates a [*pgxpool.Pool] from the given config. It parses the DSN,
// overlays any non-zero Config fields, creates the pool, and pings the
// database to verify connectivity. If the ping fails the pool is closed and
// the error is returned — this provides fail-fast behaviour on startup.
func Open(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("runtimedatabase: dsn is required")
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("runtimedatabase: parse dsn: %w", err)
	}

	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		poolCfg.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	}
	if cfg.HealthCheckPeriod > 0 {
		poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("runtimedatabase: create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("runtimedatabase: ping: %w", err)
	}

	return pool, nil
}

// HealthCheck returns a [runtimehealth.CheckFunc] that pings the pool. It is
// intended to be passed as a readiness check to [runtimehealth.New]:
//
//	runtimehealth.New(runtimehealth.Check{
//	    Name: "postgres",
//	    Fn:   runtimedatabase.HealthCheck(pool),
//	})
func HealthCheck(pool *pgxpool.Pool) runtimehealth.CheckFunc {
	return func(ctx context.Context) error {
		return pool.Ping(ctx)
	}
}
