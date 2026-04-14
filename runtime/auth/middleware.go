package runtimeauth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// ProcedureMatcher reports whether a Connect RPC procedure should be
// accessible without authentication.
type ProcedureMatcher func(procedure string) bool

// PublicProcedures returns a [ProcedureMatcher] that matches any of the
// listed procedure paths. Procedure paths use the Connect convention:
// "/<package>.<Service>/<Method>".
func PublicProcedures(procedures ...string) ProcedureMatcher {
	set := make(map[string]struct{}, len(procedures))
	for _, p := range procedures {
		set[p] = struct{}{}
	}
	return func(procedure string) bool {
		_, ok := set[procedure]
		return ok
	}
}

// NewMiddleware returns HTTP middleware that enforces authentication on
// Connect RPC procedures. Non-RPC paths (static assets, SPA routes,
// /_gofra/* endpoints) pass through without authentication.
//
// A path is treated as a Connect procedure when the first path segment
// contains a dot (e.g. "/blog.v1.PostsService/CreatePost"). All other
// paths are passed through unconditionally.
//
// For Connect procedures, the middleware:
//  1. Checks if the procedure is public via isPublic.
//  2. Extracts the Bearer token from the Authorization header.
//  3. Validates the token using the provided [Verifier].
//  4. Attaches the authenticated [User] to the request context.
//
// Missing or invalid tokens produce a Connect-compatible JSON error response
// with HTTP 401 and code "unauthenticated".
func NewMiddleware(verifier Verifier, isPublic ProcedureMatcher) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			// Non-Connect paths pass through without auth.
			if !isConnectProcedure(path) {
				next.ServeHTTP(w, r)
				return
			}

			// Public Connect procedures pass through without auth.
			if isPublic != nil && isPublic(path) {
				next.ServeHTTP(w, r)
				return
			}

			token := extractBearer(r)
			if token == "" {
				writeUnauthenticated(w, "missing bearer token")
				return
			}

			user, err := verifier.Verify(r.Context(), token)
			if err != nil {
				slog.WarnContext(r.Context(), "auth: token verification failed", "error", err)
				writeUnauthenticated(w, "invalid token")
				return
			}

			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
		})
	}
}

// isConnectProcedure reports whether the path looks like a Connect RPC
// procedure. Connect procedures have the form
// "/<package>.<Service>/<Method>" — the first path segment contains a dot
// AND is followed by a second segment (the method name).
func isConnectProcedure(path string) bool {
	if len(path) < 2 || path[0] != '/' {
		return false
	}
	// Find the first segment: everything between the leading "/" and the
	// next "/" (or end of string).
	rest := path[1:]
	i := strings.IndexByte(rest, '/')
	if i < 0 {
		// No second segment — not a procedure (e.g. "/favicon.ico").
		return false
	}
	seg := rest[:i]
	return strings.ContainsRune(seg, '.')
}

// extractBearer returns the token from an "Authorization: Bearer <token>"
// header. Returns empty string if the header is missing or malformed.
func extractBearer(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return ""
	}
	return auth[len(prefix):]
}

// connectError is the JSON wire format for Connect error responses.
type connectError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeUnauthenticated(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(connectError{
		Code:    "unauthenticated",
		Message: msg,
	})
}
