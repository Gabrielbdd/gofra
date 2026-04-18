package runtimeconfig_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Gabrielbdd/gofra/runtime/config"
)

type appConfig struct {
	Name string
}

type publicConfig struct {
	Name string `json:"name"`
}

func TestNewResolverAppliesMutator(t *testing.T) {
	t.Parallel()

	source := &appConfig{Name: "basic"}
	resolver := runtimeconfig.NewResolver(
		source,
		func(cfg *appConfig) (*publicConfig, error) {
			return &publicConfig{Name: cfg.Name}, nil
		},
		runtimeconfig.WithMutator(func(_ context.Context, _ *http.Request, cfg *publicConfig) error {
			cfg.Name = strings.ToUpper(cfg.Name)
			return nil
		}),
	)

	value, err := resolver.Resolve(context.Background(), httptest.NewRequest(http.MethodGet, runtimeconfig.DefaultPath, nil))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if value.Name != "BASIC" {
		t.Fatalf("Resolve() name = %q, want %q", value.Name, "BASIC")
	}
}

func TestNewResolverRejectsNilSource(t *testing.T) {
	t.Parallel()

	resolver := runtimeconfig.NewResolver(
		(*appConfig)(nil),
		func(cfg *appConfig) (*publicConfig, error) {
			return &publicConfig{Name: cfg.Name}, nil
		},
	)

	_, err := resolver.Resolve(context.Background(), httptest.NewRequest(http.MethodGet, runtimeconfig.DefaultPath, nil))
	if !errors.Is(err, runtimeconfig.ErrNilSource) {
		t.Fatalf("Resolve() error = %v, want %v", err, runtimeconfig.ErrNilSource)
	}
}

func TestHandlerReturnsJavaScript(t *testing.T) {
	t.Parallel()

	resolver := runtimeconfig.NewResolver(
		&appConfig{Name: "basic"},
		func(cfg *appConfig) (*publicConfig, error) {
			return &publicConfig{Name: cfg.Name}, nil
		},
	)

	req := httptest.NewRequest(http.MethodGet, runtimeconfig.DefaultPath, nil)
	rec := httptest.NewRecorder()

	runtimeconfig.Handler(resolver).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `window.__GOFRA_CONFIG__ = {"name":"basic"};`) {
		t.Fatalf("body = %q, want wrapped JS payload", body)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want %q", got, "no-store")
	}
}

func TestHandlerSupportsHead(t *testing.T) {
	t.Parallel()

	resolver := runtimeconfig.NewResolver(
		&appConfig{Name: "basic"},
		func(cfg *appConfig) (*publicConfig, error) {
			return &publicConfig{Name: cfg.Name}, nil
		},
	)

	req := httptest.NewRequest(http.MethodHead, runtimeconfig.DefaultPath, nil)
	rec := httptest.NewRecorder()

	runtimeconfig.Handler(resolver).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("body length = %d, want 0", rec.Body.Len())
	}
}

func TestHandlerRejectsInvalidMethod(t *testing.T) {
	t.Parallel()

	resolver := runtimeconfig.NewResolver(
		&appConfig{Name: "basic"},
		func(cfg *appConfig) (*publicConfig, error) {
			return &publicConfig{Name: cfg.Name}, nil
		},
	)

	req := httptest.NewRequest(http.MethodPost, runtimeconfig.DefaultPath, nil)
	rec := httptest.NewRecorder()

	runtimeconfig.Handler(resolver).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if got := rec.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("Allow = %q, want %q", got, "GET, HEAD")
	}
}
