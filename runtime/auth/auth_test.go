package runtimeauth

import (
	"context"
	"encoding/json"
	"testing"
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
	claims := json.RawMessage(`{
		"sub": "user-456",
		"email": "alice@example.com",
		"urn:zitadel:iam:org:id": "org-abc",
		"urn:zitadel:iam:org:project:789:roles": {
			"admin": {"org-abc": "org-abc"},
			"editor": {"org-abc": "org-abc"}
		}
	}`)

	user, err := defaultClaimMapper(claims)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "user-456" {
		t.Errorf("ID = %q, want %q", user.ID, "user-456")
	}
}

func TestDefaultClaimMapper_MissingSub(t *testing.T) {
	claims := json.RawMessage(`{"email": "alice@example.com"}`)

	_, err := defaultClaimMapper(claims)
	if err == nil {
		t.Fatal("expected error for missing sub claim")
	}
}

func TestDefaultClaimMapper_InvalidJSON(t *testing.T) {
	_, err := defaultClaimMapper(json.RawMessage(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
