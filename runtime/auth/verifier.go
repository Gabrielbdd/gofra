package runtimeauth

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	"github.com/zitadel/oidc/v3/pkg/client"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
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
// The mapper receives the validated [oidc.AccessTokenClaims] and must
// return a [User].
func WithClaimMapper(fn ClaimMapperFunc) Option {
	return func(c *jwtVerifierConfig) {
		c.claimMapper = fn
	}
}

// NewJWTVerifier creates a [Verifier] that validates JWT access tokens using
// OIDC discovery. It fetches the provider's metadata and JWKS endpoint on
// construction (one-time network call). If discovery fails, an error is
// returned — this provides fail-fast behaviour on startup.
//
// The verification path follows ZITADEL's recommended JWT resource-server
// pattern: OIDC discovery → remote JWKS key set → [op.VerifyAccessToken]
// with an explicit audience check.
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

	httpClient := http.DefaultClient
	if cfg.httpClient != nil {
		httpClient = cfg.httpClient
	}

	discovery, err := client.Discover(ctx, issuerURL, httpClient)
	if err != nil {
		return nil, fmt.Errorf("runtimeauth: oidc discovery: %w", err)
	}

	keySet := rp.NewRemoteKeySet(httpClient, discovery.JwksURI)
	verifier := op.NewAccessTokenVerifier(discovery.Issuer, keySet)

	return &jwtVerifier{
		verifier:    verifier,
		audience:    audience,
		claimMapper: cfg.claimMapper,
	}, nil
}

type jwtVerifier struct {
	verifier    *op.AccessTokenVerifier
	audience    string
	claimMapper ClaimMapperFunc
}

func (v *jwtVerifier) Verify(ctx context.Context, rawToken string) (User, error) {
	claims, err := op.VerifyAccessToken[*oidc.AccessTokenClaims](ctx, rawToken, v.verifier)
	if err != nil {
		return User{}, fmt.Errorf("runtimeauth: verify token: %w", err)
	}

	// op.VerifyAccessToken checks issuer, signature, and expiry but not
	// audience. Check audience explicitly per ZITADEL's recommended pattern.
	if !slices.Contains(claims.Audience, v.audience) {
		return User{}, fmt.Errorf("runtimeauth: token audience %v does not contain %q", claims.Audience, v.audience)
	}

	return v.claimMapper(claims)
}
