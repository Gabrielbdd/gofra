package runtimeerrors_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"connectrpc.com/connect"

	runtimeerrors "github.com/Gabrielbdd/gofra/runtime/errors"
)

func TestRecoverHandler_ReturnsInternal(t *testing.T) {
	t.Parallel()

	spec := connect.Spec{Procedure: "/test.v1.TestService/Crash"}
	err := runtimeerrors.RecoverHandler(context.Background(), spec, http.Header{}, "something broke")

	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("error type = %T, want *connect.Error", err)
	}
	if connErr.Code() != connect.CodeInternal {
		t.Fatalf("code = %v, want %v", connErr.Code(), connect.CodeInternal)
	}
	if got := connErr.Message(); got != "internal error" {
		t.Fatalf("message = %q, want %q — panic value must not leak", got, "internal error")
	}
}

func TestRecoverHandler_NilPanic(t *testing.T) {
	t.Parallel()

	spec := connect.Spec{Procedure: "/test.v1.TestService/NilPanic"}
	err := runtimeerrors.RecoverHandler(context.Background(), spec, http.Header{}, nil)

	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("error type = %T, want *connect.Error", err)
	}
	if connErr.Code() != connect.CodeInternal {
		t.Fatalf("code = %v, want %v", connErr.Code(), connect.CodeInternal)
	}
}
