package runtimeauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
)

// Verifier validates a raw bearer token and returns the authenticated [User].
type Verifier interface {
	Verify(ctx context.Context, rawToken string) (User, error)
}

// Option configures [NewJWTVerifier].
type Option func(*jwtVerifierConfig)

type jwtVerifierConfig struct {
	httpClient  *http.Client
	claimMapper ClaimMapperFunc
}

// WithHTTPClient sets the HTTP client used for OIDC discovery and JWKS
// fetching. Useful for testing or when the IdP is behind a proxy.
func WithHTTPClient(client *http.Client) Option {
	return func(c *jwtVerifierConfig) {
		c.httpClient = client
	}
}

// WithClaimMapper overrides the default ZITADEL-aware claim extraction.
// The mapper receives the raw JSON claims from the validated token and must
// return a [User].
func WithClaimMapper(fn ClaimMapperFunc) Option {
	return func(c *jwtVerifierConfig) {
		c.claimMapper = fn
	}
}

// NewJWTVerifier creates a [Verifier] that validates JWT access tokens using
// OIDC discovery. It fetches the provider's metadata and JWKS on
// construction (one-time network call). If discovery fails, an error is
// returned — this provides fail-fast behaviour on startup.
//
// The audience parameter is the expected "aud" claim in the access token.
// For ZITADEL this is the project ID of the API application.
func NewJWTVerifier(ctx context.Context, issuerURL, audience string, opts ...Option) (Verifier, error) {
	if issuerURL == "" {
		return nil, fmt.Errorf("runtimeauth: issuer URL is required")
	}
	if audience == "" {
		return nil, fmt.Errorf("runtimeauth: audience is required")
	}

	cfg := &jwtVerifierConfig{
		claimMapper: defaultClaimMapper,
	}
	for _, o := range opts {
		o(cfg)
	}

	if cfg.httpClient != nil {
		ctx = oidc.ClientContext(ctx, cfg.httpClient)
	}

	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("runtimeauth: oidc discovery: %w", err)
	}

	// oidc.IDTokenVerifier is named for ID tokens, but its Verify method is a
	// generic JWT validator: it checks signature, issuer, audience, and expiry.
	// It does NOT enforce ID-token-specific claims (nonce, at_hash). Using it
	// for JWT access tokens is the standard pattern across the Go ecosystem.
	// ClientID is set to the expected audience of the access token.
	verifier := provider.Verifier(&oidc.Config{
		ClientID: audience,
	})

	return &jwtVerifier{
		verifier:    verifier,
		claimMapper: cfg.claimMapper,
	}, nil
}

type jwtVerifier struct {
	verifier    *oidc.IDTokenVerifier
	claimMapper ClaimMapperFunc
}

func (v *jwtVerifier) Verify(ctx context.Context, rawToken string) (User, error) {
	token, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		return User{}, fmt.Errorf("runtimeauth: verify token: %w", err)
	}

	var raw json.RawMessage
	if err := token.Claims(&raw); err != nil {
		return User{}, fmt.Errorf("runtimeauth: extract claims: %w", err)
	}

	return v.claimMapper(raw)
}
