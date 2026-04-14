package runtimeauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockVerifier implements Verifier for testing.
type mockVerifier struct {
	user User
	err  error
}

func (m *mockVerifier) Verify(_ context.Context, _ string) (User, error) {
	return m.user, m.err
}

func TestMiddleware_NonConnectPath_PassesThrough(t *testing.T) {
	mw := NewMiddleware(&mockVerifier{}, nil)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, path := range []string{"/_gofra/config.js", "/", "/favicon.ico", "/assets/app.js"} {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("path %q: got %d, want 200", path, rec.Code)
		}
	}
}

func TestMiddleware_PublicProcedure_PassesThrough(t *testing.T) {
	isPublic := PublicProcedures("/blog.v1.PostsService/ListPosts")
	mw := NewMiddleware(&mockVerifier{}, isPublic)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/blog.v1.PostsService/ListPosts", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("got %d, want 200", rec.Code)
	}
}

func TestMiddleware_MissingBearer_Returns401(t *testing.T) {
	mw := NewMiddleware(&mockVerifier{}, nil)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("POST", "/blog.v1.PostsService/CreatePost", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rec.Code)
	}

	var body connectError
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != "unauthenticated" {
		t.Errorf("code = %q, want %q", body.Code, "unauthenticated")
	}
}

func TestMiddleware_InvalidToken_Returns401(t *testing.T) {
	v := &mockVerifier{err: errForTest("bad token")}
	mw := NewMiddleware(v, nil)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("POST", "/blog.v1.PostsService/CreatePost", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rec.Code)
	}
}

func TestMiddleware_ValidToken_SetsUserContext(t *testing.T) {
	want := User{ID: "user-789"}
	v := &mockVerifier{user: want}
	mw := NewMiddleware(v, nil)

	var gotUser User
	var gotOK bool
	handler := mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotUser, gotOK = UserFromContext(r.Context())
	}))

	req := httptest.NewRequest("POST", "/blog.v1.PostsService/CreatePost", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !gotOK {
		t.Fatal("expected user in context")
	}
	if gotUser != want {
		t.Errorf("got %+v, want %+v", gotUser, want)
	}
}

func TestMiddleware_MalformedAuthHeader_Returns401(t *testing.T) {
	mw := NewMiddleware(&mockVerifier{}, nil)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("POST", "/blog.v1.PostsService/CreatePost", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rec.Code)
	}
}

// --- isConnectProcedure tests ---------------------------------------------

func TestIsConnectProcedure(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/blog.v1.PostsService/CreatePost", true},
		{"/grpc.health.v1.Health/Check", true},
		{"/_gofra/config.js", false},
		{"/", false},
		{"/favicon.ico", false},
		{"/assets/app.js", false},
		{"/healthz/ready", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isConnectProcedure(tt.path)
		if got != tt.want {
			t.Errorf("isConnectProcedure(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// --- PublicProcedures tests -----------------------------------------------

func TestPublicProcedures(t *testing.T) {
	matcher := PublicProcedures(
		"/blog.v1.PostsService/ListPosts",
		"/blog.v1.PostsService/GetPost",
	)

	if !matcher("/blog.v1.PostsService/ListPosts") {
		t.Error("expected ListPosts to be public")
	}
	if !matcher("/blog.v1.PostsService/GetPost") {
		t.Error("expected GetPost to be public")
	}
	if matcher("/blog.v1.PostsService/CreatePost") {
		t.Error("expected CreatePost to NOT be public")
	}
}

// errForTest is a simple error type for test mocks.
type errForTest string

func (e errForTest) Error() string { return string(e) }
