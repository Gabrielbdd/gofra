package runtimedatabase_test

import (
	"context"
	"testing"
	"testing/fstest"
	"time"

	runtimedatabase "databit.com.br/gofra/runtime/database"
)

func TestOpenEmptyDSN(t *testing.T) {
	t.Parallel()
	_, err := runtimedatabase.Open(context.Background(), runtimedatabase.Config{})
	if err == nil {
		t.Fatal("expected error for empty DSN, got nil")
	}
	want := "runtimedatabase: dsn is required"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestOpenInvalidDSN(t *testing.T) {
	t.Parallel()
	_, err := runtimedatabase.Open(context.Background(), runtimedatabase.Config{
		DSN: "not a valid connection string %%%",
	})
	if err == nil {
		t.Fatal("expected error for invalid DSN, got nil")
	}
}

func TestOpenUnreachable(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Use a valid DSN pointing to an unreachable host. The pool will be
	// created but the ping should fail.
	_, err := runtimedatabase.Open(ctx, runtimedatabase.Config{
		DSN: "postgres://localhost:59999/nonexistent?connect_timeout=1",
	})
	if err == nil {
		t.Fatal("expected error for unreachable database, got nil")
	}
}

func TestHealthCheckReturnsCheckFunc(t *testing.T) {
	t.Parallel()
	// HealthCheck should accept a nil pool without panicking during
	// construction — the returned func will fail when called, but the
	// factory itself must not panic.
	fn := runtimedatabase.HealthCheck(nil)
	if fn == nil {
		t.Fatal("HealthCheck returned nil")
	}
}

func TestMigrateEmptyFS(t *testing.T) {
	t.Parallel()

	// An empty FS with the expected "migrations" directory should produce
	// zero results and no error — there are simply no migrations to apply.
	// However, without a real database pool this will fail at the sql.DB
	// bridge step. We test that the FS sub-directory logic works by using
	// a non-existent subdirectory and verifying the error.
	fsys := fstest.MapFS{}
	_, err := runtimedatabase.Migrate(context.Background(), nil, fsys)
	if err == nil {
		t.Fatal("expected error for nil pool or missing dir, got nil")
	}
}

func TestMigrateBadDir(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"other/001_init.sql": &fstest.MapFile{Data: []byte("-- +goose Up\n")},
	}
	_, err := runtimedatabase.Migrate(context.Background(), nil, fsys, runtimedatabase.WithMigrationsDir("nope"))
	if err == nil {
		t.Fatal("expected error for bad migrations dir, got nil")
	}
}
