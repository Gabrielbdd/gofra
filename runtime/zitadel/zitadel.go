package zitadel

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
)

// Config describes how to authenticate against a ZITADEL instance.
//
// This struct is a convenience container for consumer apps that want a
// single place to carry issuer + credentials + org scope. The package does
// not consume Config directly today; it exposes [NewAuthInterceptor] as the
// primary integration point.
type Config struct {
	// Issuer is the ZITADEL base URL (e.g. "http://localhost:8081"). Not
	// used by this package directly — consumer apps pass it to their own
	// Connect client constructors.
	Issuer string
	// PAT is the Personal Access Token used to authenticate requests.
	PAT string
	// OrgID, when non-empty, targets a specific organization via the
	// x-zitadel-orgid header.
	OrgID string
	// HTTPClient overrides the default client used by consumer-constructed
	// Connect clients. Nil means [http.DefaultClient].
	HTTPClient *http.Client
}

// NewAuthInterceptor returns a [connect.Interceptor] that applies
// `Authorization: Bearer <pat>` to every unary and streaming client request.
// When orgID is non-empty, it also sets `x-zitadel-orgid: <orgID>` so the
// request targets a specific organization.
//
// The interceptor is a no-op on the streaming handler side — it is intended
// for outbound calls to ZITADEL.
//
// Empty pat yields an interceptor that still sets `Authorization: Bearer `;
// callers are responsible for ensuring they pass a non-empty token.
func NewAuthInterceptor(pat, orgID string) connect.Interceptor {
	return &authInterceptor{pat: pat, orgID: orgID}
}

type authInterceptor struct {
	pat   string
	orgID string
}

func (a *authInterceptor) setHeaders(h http.Header) {
	h.Set("Authorization", "Bearer "+a.pat)
	if a.orgID != "" {
		h.Set("x-zitadel-orgid", a.orgID)
	}
}

func (a *authInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		a.setHeaders(req.Header())
		return next(ctx, req)
	}
}

func (a *authInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		a.setHeaders(conn.RequestHeader())
		return conn
	}
}

func (a *authInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}
