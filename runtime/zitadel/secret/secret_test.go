package zitadelsecret_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	zitadelsecret "github.com/Gabrielbdd/gofra/runtime/zitadel/secret"
)

func TestRead_FileWinsOverEnv(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "pat")
	if err := os.WriteFile(file, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOFRA_TEST_PAT", "from-env")

	got, err := zitadelsecret.Read(zitadelsecret.Source{
		FilePath: file,
		EnvVar:   "GOFRA_TEST_PAT",
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != "from-file" {
		t.Errorf("Read = %q; want %q", got, "from-file")
	}
}

func TestRead_MissingFileFallsBackToEnv(t *testing.T) {
	t.Setenv("GOFRA_TEST_PAT", "from-env")

	got, err := zitadelsecret.Read(zitadelsecret.Source{
		FilePath: "/nonexistent/path/pat",
		EnvVar:   "GOFRA_TEST_PAT",
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != "from-env" {
		t.Errorf("Read = %q; want %q", got, "from-env")
	}
}

func TestRead_EmptyFileErrors(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "pat")
	if err := os.WriteFile(file, []byte("   \n\t"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := zitadelsecret.Read(zitadelsecret.Source{FilePath: file})
	if err == nil {
		t.Fatal("Read succeeded on empty file; want error")
	}
	if errors.Is(err, zitadelsecret.ErrNotFound) {
		t.Errorf("empty file should not return ErrNotFound; got %v", err)
	}
}

func TestRead_MissingBothReturnsErrNotFound(t *testing.T) {
	t.Setenv("GOFRA_TEST_PAT", "")

	_, err := zitadelsecret.Read(zitadelsecret.Source{
		FilePath: "/nonexistent/path/pat",
		EnvVar:   "GOFRA_TEST_PAT",
	})
	if !errors.Is(err, zitadelsecret.ErrNotFound) {
		t.Errorf("Read = %v; want ErrNotFound", err)
	}
}

func TestRead_TrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "pat")
	if err := os.WriteFile(file, []byte("  token-value  \n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := zitadelsecret.Read(zitadelsecret.Source{FilePath: file})
	if err != nil {
		t.Fatal(err)
	}
	if got != "token-value" {
		t.Errorf("Read = %q; want %q", got, "token-value")
	}
}

func TestRead_EmptyEnvFallsToErrNotFound(t *testing.T) {
	t.Setenv("GOFRA_TEST_PAT", "  \t ")

	_, err := zitadelsecret.Read(zitadelsecret.Source{EnvVar: "GOFRA_TEST_PAT"})
	if !errors.Is(err, zitadelsecret.ErrNotFound) {
		t.Errorf("Read = %v; want ErrNotFound", err)
	}
}
