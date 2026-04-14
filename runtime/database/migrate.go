package runtimedatabase

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// MigrateOption configures [Migrate].
type MigrateOption func(*migrateSettings)

type migrateSettings struct {
	dir              string
	allowOutOfOrder  bool
	sessionLocker    bool
	gooseTableName   string
}

func defaultMigrateSettings() migrateSettings {
	return migrateSettings{
		dir:           "migrations",
		sessionLocker: true,
	}
}

// WithMigrationsDir sets the subdirectory within the provided [fs.FS] that
// contains the migration files. Default: "migrations".
//
// This matches the convention where the app's db/embed.go contains:
//
//	//go:embed migrations/*.sql
//	var Migrations embed.FS
//
// and the resulting [embed.FS] has "migrations/" as a top-level directory.
func WithMigrationsDir(dir string) MigrateOption {
	return func(s *migrateSettings) {
		s.dir = dir
	}
}

// WithAllowOutOfOrder permits goose to apply migrations that were created
// before already-applied migrations. This is useful for teams where multiple
// developers create migrations on separate branches.
func WithAllowOutOfOrder() MigrateOption {
	return func(s *migrateSettings) {
		s.allowOutOfOrder = true
	}
}

// WithoutSessionLocker disables the Postgres advisory lock that serialises
// concurrent migration runs. Only disable this if you are certain that a
// single process runs migrations at a time.
func WithoutSessionLocker() MigrateOption {
	return func(s *migrateSettings) {
		s.sessionLocker = false
	}
}

// WithGooseTableName overrides the goose version table name. Default is
// goose's built-in default ("goose_db_version").
func WithGooseTableName(name string) MigrateOption {
	return func(s *migrateSettings) {
		s.gooseTableName = name
	}
}

// Migrate runs pending goose migrations against the database pool. It uses
// the goose Provider API with Postgres session-level advisory locks for
// concurrent safety.
//
// The fsys parameter should be an [embed.FS] (or any [fs.FS]) containing the
// migration SQL files. By default, Migrate looks for a "migrations"
// subdirectory within fsys — override this with [WithMigrationsDir].
//
// The returned results describe each migration that was applied. If no
// migrations are pending, the slice is empty and the error is nil.
func Migrate(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS, opts ...MigrateOption) ([]*goose.MigrationResult, error) {
	s := defaultMigrateSettings()
	for _, o := range opts {
		o(&s)
	}

	// Sub into the migrations directory within the FS.
	migrationsFS, err := fs.Sub(fsys, s.dir)
	if err != nil {
		return nil, fmt.Errorf("runtimedatabase: fs.Sub %q: %w", s.dir, err)
	}

	// goose needs a *sql.DB — bridge from the pgxpool without creating a
	// separate connection pool.
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	// Build goose provider options.
	var providerOpts []goose.ProviderOption

	if s.sessionLocker {
		locker, err := lock.NewPostgresSessionLocker()
		if err != nil {
			return nil, fmt.Errorf("runtimedatabase: create session locker: %w", err)
		}
		providerOpts = append(providerOpts, goose.WithSessionLocker(locker))
	}

	if s.allowOutOfOrder {
		providerOpts = append(providerOpts, goose.WithAllowOutofOrder(true))
	}

	if s.gooseTableName != "" {
		providerOpts = append(providerOpts, goose.WithTableName(s.gooseTableName))
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrationsFS, providerOpts...)
	if err != nil {
		return nil, fmt.Errorf("runtimedatabase: create goose provider: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return results, fmt.Errorf("runtimedatabase: goose up: %w", err)
	}

	return results, nil
}
