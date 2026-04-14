// Package runtimeauth provides JWT-based authentication for Connect RPC
// services. It extracts and validates Bearer tokens, attaches the
// authenticated user to [context.Context], and enforces private-by-default
// access on Connect procedures.
package runtimeauth

import "context"

// User represents an authenticated identity extracted from a JWT access token.
type User struct {
	// ID is the subject claim (sub) — the unique user identifier from the
	// identity provider.
	ID string
}

type contextKey struct{}

// WithUser returns a copy of ctx with the given user attached.
func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, contextKey{}, user)
}

// UserFromContext retrieves the authenticated user from ctx. The second return
// value is false when no user is present (unauthenticated request or public
// procedure).
func UserFromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(contextKey{}).(User)
	return u, ok
}
