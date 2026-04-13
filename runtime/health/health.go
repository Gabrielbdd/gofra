// Package runtimehealth provides HTTP health check handlers for startup,
// liveness, and readiness probes. It is stdlib-only and does not depend on
// any specific database, queue, or service mesh implementation.
package runtimehealth

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

// readinessCheckTimeout is the per-probe timeout applied to readiness checks.
const readinessCheckTimeout = 2 * time.Second

// CheckFunc is a function that reports whether a dependency is healthy.
// Return nil for healthy, a non-nil error describing the failure otherwise.
type CheckFunc func(ctx context.Context) error

// Check pairs a human-readable name with a health check function.
type Check struct {
	Name string
	Fn   CheckFunc
}

// Checker manages startup, liveness, and readiness state and exposes HTTP
// handlers for each probe type.
type Checker struct {
	checks   []Check
	started  atomic.Bool
	notReady atomic.Bool
}

// New creates a Checker with the given readiness checks. Checks are evaluated
// in order on every readiness probe request.
func New(checks ...Check) *Checker {
	return &Checker{checks: checks}
}

// MarkStarted signals that application initialization is complete. After this
// call the startup probe returns 200 and readiness checks begin running.
func (c *Checker) MarkStarted() {
	c.started.Store(true)
}

// SetNotReady forces the readiness probe to return 503. This is intended to be
// called at the beginning of the shutdown sequence so that load balancers stop
// sending new traffic.
func (c *Checker) SetNotReady() {
	c.notReady.Store(true)
}

// StartupHandler returns an http.Handler for the startup probe.
//
//	200 {"status":"started"}   — after MarkStarted()
//	503 {"status":"starting"}  — before MarkStarted()
func (c *Checker) StartupHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowGetHead(w, r) {
			return
		}

		if c.started.Load() {
			writeJSON(w, r, http.StatusOK, response{Status: "started"})
		} else {
			writeJSON(w, r, http.StatusServiceUnavailable, response{Status: "starting"})
		}
	})
}

// LivenessHandler returns an http.Handler for the liveness probe.
// It always returns 200 — the only signal is whether the process can respond
// to HTTP at all. It intentionally does not check external dependencies.
func (c *Checker) LivenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowGetHead(w, r) {
			return
		}
		writeJSON(w, r, http.StatusOK, response{Status: "alive"})
	})
}

// ReadinessHandler returns an http.Handler for the readiness probe.
//
//	200 {"status":"ready","checks":{...}}      — started and all checks pass
//	503 {"status":"not_ready","checks":{...}}  — check failed or shutting down
//	503 {"status":"starting"}                  — not yet started
func (c *Checker) ReadinessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowGetHead(w, r) {
			return
		}

		if !c.started.Load() {
			writeJSON(w, r, http.StatusServiceUnavailable, response{Status: "starting"})
			return
		}

		if c.notReady.Load() {
			writeJSON(w, r, http.StatusServiceUnavailable, response{Status: "shutting_down"})
			return
		}

		checks := make(map[string]string, len(c.checks))
		healthy := true

		for _, ch := range c.checks {
			ctx, cancel := context.WithTimeout(r.Context(), readinessCheckTimeout)
			err := ch.Fn(ctx)
			cancel()

			if err != nil {
				checks[ch.Name] = err.Error()
				healthy = false
			} else {
				checks[ch.Name] = "ok"
			}
		}

		if healthy {
			writeJSON(w, r, http.StatusOK, response{Status: "ready", Checks: checks})
		} else {
			writeJSON(w, r, http.StatusServiceUnavailable, response{Status: "not_ready", Checks: checks})
		}
	})
}

// response is the JSON body returned by all health handlers.
type response struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

// allowGetHead checks the HTTP method and writes a 405 if it is not GET or
// HEAD. Returns true if the caller should continue handling the request.
func allowGetHead(w http.ResponseWriter, r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		return true
	default:
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return false
	}
}

// writeJSON marshals v as JSON and writes it with the given status code.
// For HEAD requests the body is omitted.
func writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
