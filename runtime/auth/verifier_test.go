package runtimeauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

// TestNewJWTVerifier_MissingIssuer verifies that an empty issuer URL is
// rejected at construction time.
func TestNewJWTVerifier_MissingIssuer(t *testing.T) {
	_, err := NewJWTVerifier(context.Background(), "", "aud")
	if err == nil {
		t.Fatal("expected error for empty issuer")
	}
}

// TestNewJWTVerifier_MissingAudience verifies that an empty audience is
// rejected at construction time.
func TestNewJWTVerifier_MissingAudience(t *testing.T) {
	_, err := NewJWTVerifier(context.Background(), "http://example.com", "")
	if err == nil {
		t.Fatal("expected error for empty audience")
	}
}

// TestNewJWTVerifier_InvalidIssuer verifies that an unreachable issuer fails
// fast during OIDC discovery.
func TestNewJWTVerifier_InvalidIssuer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := NewJWTVerifier(ctx, "http://127.0.0.1:0/not-a-real-issuer", "aud")
	if err == nil {
		t.Fatal("expected error for unreachable issuer")
	}
}

// mockOIDCServer starts an httptest.Server that serves OIDC discovery and JWKS
// endpoints using the given RSA public key.
type mockOIDCServer struct {
	Server *httptest.Server
	URL    string
}

func newMockOIDCServer(t *testing.T, pubKey *rsa.PublicKey, keyID string) *mockOIDCServer {
	t.Helper()

	jwks := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key:       pubKey,
				KeyID:     keyID,
				Algorithm: string(jose.RS256),
				Use:       "sig",
			},
		},
	}

	mux := http.NewServeMux()
	m := &mockOIDCServer{}

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":   m.URL,
			"jwks_uri": m.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})

	m.Server = httptest.NewServer(mux)
	m.URL = m.Server.URL
	t.Cleanup(m.Server.Close)
	return m
}

func signJWT(t *testing.T, privKey *rsa.PrivateKey, keyID string, claims jwt.Claims, extra ...any) string {
	t.Helper()

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: privKey},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", keyID),
	)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	builder := jwt.Signed(signer).Claims(claims)
	for _, c := range extra {
		builder = builder.Claims(c)
	}

	raw, err := builder.Serialize()
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}
	return raw
}

// TestJWTVerifier_RoundTrip spins up a mock OIDC provider and verifies the
// full flow: create verifier → sign a JWT → verify → extract user.
func TestJWTVerifier_RoundTrip(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	const keyID = "test-key-1"
	const audience = "my-api"

	m := newMockOIDCServer(t, &privKey.PublicKey, keyID)

	ctx := context.Background()
	verifier, err := NewJWTVerifier(ctx, m.URL, audience,
		WithHTTPClient(m.Server.Client()),
	)
	if err != nil {
		t.Fatalf("NewJWTVerifier: %v", err)
	}

	now := time.Now()
	raw := signJWT(t, privKey, keyID, jwt.Claims{
		Issuer:    m.URL,
		Subject:   "user-42",
		Audience:  jwt.Audience{audience},
		IssuedAt:  jwt.NewNumericDate(now),
		Expiry:    jwt.NewNumericDate(now.Add(5 * time.Minute)),
		NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
	})

	user, err := verifier.Verify(ctx, raw)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if user.ID != "user-42" {
		t.Errorf("ID = %q, want %q", user.ID, "user-42")
	}
}

// TestJWTVerifier_ExpiredToken verifies that expired tokens are rejected.
func TestJWTVerifier_ExpiredToken(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	const keyID = "test-key-1"
	const audience = "my-api"

	m := newMockOIDCServer(t, &privKey.PublicKey, keyID)

	ctx := context.Background()
	verifier, err := NewJWTVerifier(ctx, m.URL, audience,
		WithHTTPClient(m.Server.Client()),
	)
	if err != nil {
		t.Fatalf("NewJWTVerifier: %v", err)
	}

	past := time.Now().Add(-1 * time.Hour)
	raw := signJWT(t, privKey, keyID, jwt.Claims{
		Issuer:   m.URL,
		Subject:  "user-42",
		Audience: jwt.Audience{audience},
		IssuedAt: jwt.NewNumericDate(past),
		Expiry:   jwt.NewNumericDate(past.Add(5 * time.Minute)),
	})

	_, err = verifier.Verify(ctx, raw)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

// TestJWTVerifier_WrongAudience verifies that tokens with wrong audience are
// rejected by the explicit audience check.
func TestJWTVerifier_WrongAudience(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	const keyID = "test-key-1"

	m := newMockOIDCServer(t, &privKey.PublicKey, keyID)

	ctx := context.Background()
	verifier, err := NewJWTVerifier(ctx, m.URL, "my-api",
		WithHTTPClient(m.Server.Client()),
	)
	if err != nil {
		t.Fatalf("NewJWTVerifier: %v", err)
	}

	now := time.Now()
	raw := signJWT(t, privKey, keyID, jwt.Claims{
		Issuer:   m.URL,
		Subject:  "user-42",
		Audience: jwt.Audience{"wrong-audience"},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(5 * time.Minute)),
	})

	_, err = verifier.Verify(ctx, raw)
	if err == nil {
		t.Fatal("expected error for wrong audience")
	}
}

// TestJWTVerifier_CustomClaimMapper verifies that WithClaimMapper overrides
// the default claim extraction.
func TestJWTVerifier_CustomClaimMapper(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	const keyID = "test-key-1"
	const audience = "my-api"

	m := newMockOIDCServer(t, &privKey.PublicKey, keyID)

	customMapper := func(claims *oidc.AccessTokenClaims) (User, error) {
		// Read a custom claim from the extra claims map.
		customID, _ := claims.Claims["custom_id"].(string)
		return User{ID: "custom-" + customID}, nil
	}

	ctx := context.Background()
	verifier, err := NewJWTVerifier(ctx, m.URL, audience,
		WithHTTPClient(m.Server.Client()),
		WithClaimMapper(customMapper),
	)
	if err != nil {
		t.Fatalf("NewJWTVerifier: %v", err)
	}

	now := time.Now()
	raw := signJWT(t, privKey, keyID, jwt.Claims{
		Issuer:   m.URL,
		Subject:  "user-42",
		Audience: jwt.Audience{audience},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(5 * time.Minute)),
	}, struct {
		CustomID string `json:"custom_id"`
	}{CustomID: "abc"})

	user, err := verifier.Verify(ctx, raw)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if user.ID != "custom-abc" {
		t.Errorf("ID = %q, want %q", user.ID, "custom-abc")
	}
}
