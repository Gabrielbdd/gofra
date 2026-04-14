package runtimeauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
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

// TestJWTVerifier_RoundTrip spins up a mock OIDC provider (discovery + JWKS)
// and verifies the full flow: create verifier → sign a JWT → verify → extract
// user.
func TestJWTVerifier_RoundTrip(t *testing.T) {
	// Generate a test RSA key pair.
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	const keyID = "test-key-1"

	// Build the JWKS document.
	jwks := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key:       &privKey.PublicKey,
				KeyID:     keyID,
				Algorithm: string(jose.RS256),
				Use:       "sig",
			},
		},
	}

	// Serve mock OIDC discovery and JWKS endpoints.
	mux := http.NewServeMux()
	var issuerURL string

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":   issuerURL,
			"jwks_uri": issuerURL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	issuerURL = srv.URL

	const audience = "my-api"

	// Create verifier.
	ctx := context.Background()
	verifier, err := NewJWTVerifier(ctx, issuerURL, audience,
		WithHTTPClient(srv.Client()),
	)
	if err != nil {
		t.Fatalf("NewJWTVerifier: %v", err)
	}

	// Sign a JWT.
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: privKey},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", keyID),
	)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	now := time.Now()
	claims := jwt.Claims{
		Issuer:    issuerURL,
		Subject:   "user-42",
		Audience:  jwt.Audience{audience},
		IssuedAt:  jwt.NewNumericDate(now),
		Expiry:    jwt.NewNumericDate(now.Add(5 * time.Minute)),
		NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
	}

	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}

	// Verify the token.
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

	jwks := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key:       &privKey.PublicKey,
				KeyID:     keyID,
				Algorithm: string(jose.RS256),
				Use:       "sig",
			},
		},
	}

	mux := http.NewServeMux()
	var issuerURL string

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":   issuerURL,
			"jwks_uri": issuerURL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	issuerURL = srv.URL

	const audience = "my-api"

	ctx := context.Background()
	verifier, err := NewJWTVerifier(ctx, issuerURL, audience,
		WithHTTPClient(srv.Client()),
	)
	if err != nil {
		t.Fatalf("NewJWTVerifier: %v", err)
	}

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: privKey},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", keyID),
	)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	// Issue an already-expired token.
	past := time.Now().Add(-1 * time.Hour)
	claims := jwt.Claims{
		Issuer:   issuerURL,
		Subject:  "user-42",
		Audience: jwt.Audience{audience},
		IssuedAt: jwt.NewNumericDate(past),
		Expiry:   jwt.NewNumericDate(past.Add(5 * time.Minute)),
	}

	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}

	_, err = verifier.Verify(ctx, raw)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

// TestJWTVerifier_WrongAudience verifies that tokens with wrong audience are
// rejected.
func TestJWTVerifier_WrongAudience(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	const keyID = "test-key-1"

	jwks := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key:       &privKey.PublicKey,
				KeyID:     keyID,
				Algorithm: string(jose.RS256),
				Use:       "sig",
			},
		},
	}

	mux := http.NewServeMux()
	var issuerURL string

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":   issuerURL,
			"jwks_uri": issuerURL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	issuerURL = srv.URL

	ctx := context.Background()
	verifier, err := NewJWTVerifier(ctx, issuerURL, "my-api",
		WithHTTPClient(srv.Client()),
	)
	if err != nil {
		t.Fatalf("NewJWTVerifier: %v", err)
	}

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: privKey},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", keyID),
	)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	now := time.Now()
	claims := jwt.Claims{
		Issuer:   issuerURL,
		Subject:  "user-42",
		Audience: jwt.Audience{"wrong-audience"},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(5 * time.Minute)),
	}

	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}

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

	jwks := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key:       &privKey.PublicKey,
				KeyID:     keyID,
				Algorithm: string(jose.RS256),
				Use:       "sig",
			},
		},
	}

	mux := http.NewServeMux()
	var issuerURL string

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":   issuerURL,
			"jwks_uri": issuerURL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	issuerURL = srv.URL

	const audience = "my-api"

	customMapper := func(raw json.RawMessage) (User, error) {
		var c struct {
			CustomID string `json:"custom_id"`
		}
		if err := json.Unmarshal(raw, &c); err != nil {
			return User{}, err
		}
		return User{ID: fmt.Sprintf("custom-%s", c.CustomID)}, nil
	}

	ctx := context.Background()
	verifier, err := NewJWTVerifier(ctx, issuerURL, audience,
		WithHTTPClient(srv.Client()),
		WithClaimMapper(customMapper),
	)
	if err != nil {
		t.Fatalf("NewJWTVerifier: %v", err)
	}

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: privKey},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", keyID),
	)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	now := time.Now()
	standardClaims := jwt.Claims{
		Issuer:   issuerURL,
		Subject:  "user-42",
		Audience: jwt.Audience{audience},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(5 * time.Minute)),
	}
	extraClaims := struct {
		CustomID string `json:"custom_id"`
	}{CustomID: "abc"}

	raw, err := jwt.Signed(signer).Claims(standardClaims).Claims(extraClaims).Serialize()
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}

	user, err := verifier.Verify(ctx, raw)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if user.ID != "custom-abc" {
		t.Errorf("ID = %q, want %q", user.ID, "custom-abc")
	}
}
