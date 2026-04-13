package runtimehealth_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	runtimehealth "databit.com.br/gofra/runtime/health"
)

func TestStartupBeforeMarkStarted(t *testing.T) {
	t.Parallel()
	c := runtimehealth.New()

	rec := get(t, c.StartupHandler())

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	expectStatus(t, rec, "starting")
}

func TestStartupAfterMarkStarted(t *testing.T) {
	t.Parallel()
	c := runtimehealth.New()
	c.MarkStarted()

	rec := get(t, c.StartupHandler())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	expectStatus(t, rec, "started")
}

func TestLivenessAlwaysOK(t *testing.T) {
	t.Parallel()
	c := runtimehealth.New()

	rec := get(t, c.LivenessHandler())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	expectStatus(t, rec, "alive")
}

func TestReadinessBeforeStartup(t *testing.T) {
	t.Parallel()
	c := runtimehealth.New()

	rec := get(t, c.ReadinessHandler())

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	expectStatus(t, rec, "starting")
}

func TestReadinessNoChecks(t *testing.T) {
	t.Parallel()
	c := runtimehealth.New()
	c.MarkStarted()

	rec := get(t, c.ReadinessHandler())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	expectStatus(t, rec, "ready")
}

func TestReadinessAllChecksPass(t *testing.T) {
	t.Parallel()
	c := runtimehealth.New(
		runtimehealth.Check{Name: "db", Fn: func(context.Context) error { return nil }},
		runtimehealth.Check{Name: "cache", Fn: func(context.Context) error { return nil }},
	)
	c.MarkStarted()

	rec := get(t, c.ReadinessHandler())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	expectStatus(t, rec, "ready")
	expectCheck(t, rec, "db", "ok")
	expectCheck(t, rec, "cache", "ok")
}

func TestReadinessCheckFails(t *testing.T) {
	t.Parallel()
	c := runtimehealth.New(
		runtimehealth.Check{Name: "db", Fn: func(context.Context) error { return errors.New("connect: connection refused") }},
		runtimehealth.Check{Name: "cache", Fn: func(context.Context) error { return nil }},
	)
	c.MarkStarted()

	rec := get(t, c.ReadinessHandler())

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	expectStatus(t, rec, "not_ready")
	expectCheck(t, rec, "db", "connect: connection refused")
	expectCheck(t, rec, "cache", "ok")
}

func TestReadinessAfterSetNotReady(t *testing.T) {
	t.Parallel()
	c := runtimehealth.New()
	c.MarkStarted()
	c.SetNotReady()

	rec := get(t, c.ReadinessHandler())

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	expectStatus(t, rec, "shutting_down")
}

func TestHandlerRejectsPost(t *testing.T) {
	t.Parallel()
	c := runtimehealth.New()

	handlers := []struct {
		name    string
		handler http.Handler
	}{
		{"startup", c.StartupHandler()},
		{"liveness", c.LivenessHandler()},
		{"readiness", c.ReadinessHandler()},
	}

	for _, h := range handlers {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		h.handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: status = %d, want %d", h.name, rec.Code, http.StatusMethodNotAllowed)
		}
		if allow := rec.Header().Get("Allow"); allow != "GET, HEAD" {
			t.Errorf("%s: Allow = %q, want %q", h.name, allow, "GET, HEAD")
		}
	}
}

func TestHandlerHeadReturnsStatusNoBody(t *testing.T) {
	t.Parallel()
	c := runtimehealth.New()
	c.MarkStarted()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodHead, "/", nil)
	c.LivenessHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}
}

// helpers

func get(t *testing.T, h http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json decode: %v (body: %q)", err, rec.Body.String())
	}
	return body
}

func expectStatus(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	body := decodeBody(t, rec)
	if got, _ := body["status"].(string); got != want {
		t.Errorf("status = %q, want %q", got, want)
	}
}

func expectCheck(t *testing.T, rec *httptest.ResponseRecorder, name, want string) {
	t.Helper()
	body := decodeBody(t, rec)
	checks, ok := body["checks"].(map[string]any)
	if !ok {
		t.Fatalf("checks not present in body: %v", body)
	}
	if got, _ := checks[name].(string); got != want {
		t.Errorf("checks[%q] = %q, want %q", name, got, want)
	}
}
