package runtimeerrors_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"

	runtimeerrors "databit.com.br/gofra/runtime/errors"
)

func TestNotFound(t *testing.T) {
	t.Parallel()

	err := runtimeerrors.NotFound("post", "abc-123")

	if err.Code() != connect.CodeNotFound {
		t.Fatalf("code = %v, want %v", err.Code(), connect.CodeNotFound)
	}
	if got := err.Message(); got != `post "abc-123" not found` {
		t.Fatalf("message = %q, want %q", got, `post "abc-123" not found`)
	}

	details := err.Details()
	if len(details) != 1 {
		t.Fatalf("details count = %d, want 1", len(details))
	}
	msg, unmarshalErr := details[0].Value()
	if unmarshalErr != nil {
		t.Fatalf("unmarshal detail: %v", unmarshalErr)
	}
	ri, ok := msg.(*errdetails.ResourceInfo)
	if !ok {
		t.Fatalf("detail type = %T, want *errdetails.ResourceInfo", msg)
	}
	if ri.ResourceType != "post" {
		t.Errorf("ResourceType = %q, want %q", ri.ResourceType, "post")
	}
	if ri.ResourceName != "abc-123" {
		t.Errorf("ResourceName = %q, want %q", ri.ResourceName, "abc-123")
	}
}

func TestAlreadyExists(t *testing.T) {
	t.Parallel()

	err := runtimeerrors.AlreadyExists("user", "jane@example.com")

	if err.Code() != connect.CodeAlreadyExists {
		t.Fatalf("code = %v, want %v", err.Code(), connect.CodeAlreadyExists)
	}
	if got := err.Message(); got != `user "jane@example.com" already exists` {
		t.Fatalf("message = %q, want %q", got, `user "jane@example.com" already exists`)
	}
}

func TestInvalidArgument_SingleViolation(t *testing.T) {
	t.Parallel()

	err := runtimeerrors.InvalidArgument(
		runtimeerrors.FieldViolation{Field: "email", Description: "is required"},
	)

	if err.Code() != connect.CodeInvalidArgument {
		t.Fatalf("code = %v, want %v", err.Code(), connect.CodeInvalidArgument)
	}

	violations := extractFieldViolations(t, err)
	if len(violations) != 1 {
		t.Fatalf("violations count = %d, want 1", len(violations))
	}
	if violations[0].Field != "email" || violations[0].Description != "is required" {
		t.Errorf("violation = %+v, want {email, is required}", violations[0])
	}
}

func TestInvalidArgument_MultipleViolations(t *testing.T) {
	t.Parallel()

	err := runtimeerrors.InvalidArgument(
		runtimeerrors.FieldViolation{Field: "email", Description: "is required"},
		runtimeerrors.FieldViolation{Field: "email", Description: "must be valid"},
		runtimeerrors.FieldViolation{Field: "name", Description: "is required"},
	)

	violations := extractFieldViolations(t, err)
	if len(violations) != 3 {
		t.Fatalf("violations count = %d, want 3", len(violations))
	}
	// Verify ordering is preserved.
	if violations[0].Field != "email" || violations[0].Description != "is required" {
		t.Errorf("violations[0] = %+v", violations[0])
	}
	if violations[1].Field != "email" || violations[1].Description != "must be valid" {
		t.Errorf("violations[1] = %+v", violations[1])
	}
	if violations[2].Field != "name" || violations[2].Description != "is required" {
		t.Errorf("violations[2] = %+v", violations[2])
	}
}

func TestInvalidArgument_NoViolations(t *testing.T) {
	t.Parallel()

	err := runtimeerrors.InvalidArgument()

	if err.Code() != connect.CodeInvalidArgument {
		t.Fatalf("code = %v, want %v", err.Code(), connect.CodeInvalidArgument)
	}
	// Detail is still attached (empty BadRequest).
	violations := extractFieldViolations(t, err)
	if len(violations) != 0 {
		t.Fatalf("violations count = %d, want 0", len(violations))
	}
}

func TestPermissionDenied(t *testing.T) {
	t.Parallel()

	err := runtimeerrors.PermissionDenied("not allowed to delete posts")

	if err.Code() != connect.CodePermissionDenied {
		t.Fatalf("code = %v, want %v", err.Code(), connect.CodePermissionDenied)
	}
	if got := err.Message(); got != "not allowed to delete posts" {
		t.Fatalf("message = %q, want %q", got, "not allowed to delete posts")
	}
}

func TestAborted(t *testing.T) {
	t.Parallel()

	err := runtimeerrors.Aborted("etag mismatch")

	if err.Code() != connect.CodeAborted {
		t.Fatalf("code = %v, want %v", err.Code(), connect.CodeAborted)
	}
	if got := err.Message(); got != "etag mismatch" {
		t.Fatalf("message = %q, want %q", got, "etag mismatch")
	}
}

func TestFailedPrecondition(t *testing.T) {
	t.Parallel()

	err := runtimeerrors.FailedPrecondition("post has no body")

	if err.Code() != connect.CodeFailedPrecondition {
		t.Fatalf("code = %v, want %v", err.Code(), connect.CodeFailedPrecondition)
	}
	if got := err.Message(); got != "post has no body" {
		t.Fatalf("message = %q, want %q", got, "post has no body")
	}
}

func TestInternal_SanitizesMessage(t *testing.T) {
	t.Parallel()

	origErr := errors.New("SELECT * FROM users WHERE id=$1: connection refused")
	connErr := runtimeerrors.Internal(context.Background(), origErr)

	if connErr.Code() != connect.CodeInternal {
		t.Fatalf("code = %v, want %v", connErr.Code(), connect.CodeInternal)
	}
	if got := connErr.Message(); got != "internal error" {
		t.Fatalf("message = %q, want %q — original error must not leak", got, "internal error")
	}
}

func TestInternal_DoesNotWrapOriginal(t *testing.T) {
	t.Parallel()

	origErr := errors.New("secret database error")
	connErr := runtimeerrors.Internal(context.Background(), origErr)

	// The returned connect.Error must not wrap the original error —
	// otherwise Unwrap chains could leak the message.
	if errors.Is(connErr, origErr) {
		t.Fatal("Internal() must not wrap the original error")
	}
}

// extractFieldViolations unwraps a BadRequest detail from a connect.Error.
func extractFieldViolations(t *testing.T, err *connect.Error) []*errdetails.BadRequest_FieldViolation {
	t.Helper()
	details := err.Details()
	if len(details) != 1 {
		t.Fatalf("details count = %d, want 1", len(details))
	}
	msg, unmarshalErr := details[0].Value()
	if unmarshalErr != nil {
		t.Fatalf("unmarshal detail: %v", unmarshalErr)
	}
	br, ok := msg.(*errdetails.BadRequest)
	if !ok {
		t.Fatalf("detail type = %T, want *errdetails.BadRequest", msg)
	}
	return br.FieldViolations
}
