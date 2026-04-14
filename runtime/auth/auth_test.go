package runtimeauth

import (
	"context"
	"testing"

	"github.com/zitadel/oidc/v3/pkg/oidc"
)

func TestWithUserAndUserFromContext(t *testing.T) {
	want := User{ID: "user-123"}
	ctx := WithUser(context.Background(), want)

	got, ok := UserFromContext(ctx)
	if !ok {
		t.Fatal("expected user in context")
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestUserFromContext_Empty(t *testing.T) {
	_, ok := UserFromContext(context.Background())
	if ok {
		t.Error("expected no user in empty context")
	}
}

// --- Claim extraction tests -----------------------------------------------

func TestDefaultClaimMapper(t *testing.T) {
	claims := &oidc.AccessTokenClaims{
		TokenClaims: oidc.TokenClaims{
			Subject: "user-456",
		},
	}

	user, err := defaultClaimMapper(claims)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "user-456" {
		t.Errorf("ID = %q, want %q", user.ID, "user-456")
	}
}

func TestDefaultClaimMapper_MissingSub(t *testing.T) {
	claims := &oidc.AccessTokenClaims{}

	_, err := defaultClaimMapper(claims)
	if err == nil {
		t.Fatal("expected error for missing sub claim")
	}
}
