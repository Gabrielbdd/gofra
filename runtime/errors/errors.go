// Package runtimeerrors provides Connect RPC error construction helpers.
//
// Each helper produces a [connect.Error] with the correct code, a
// human-readable message, and standard protobuf error details where
// applicable. Handlers call these instead of constructing errors manually
// so that every error of the same class has a consistent shape.
package runtimeerrors

import (
	"context"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
)

// FieldViolation describes a single field-level validation failure.
// A slice of FieldViolation is used instead of map[string]string so that
// ordering is preserved and multiple violations on the same field are
// representable.
type FieldViolation struct {
	Field       string
	Description string
}

// NotFound returns a [connect.CodeNotFound] error with a [errdetails.ResourceInfo]
// detail identifying the missing resource.
func NotFound(resource, identifier string) *connect.Error {
	err := connect.NewError(
		connect.CodeNotFound,
		fmt.Errorf("%s %q not found", resource, identifier),
	)
	detail, detailErr := connect.NewErrorDetail(&errdetails.ResourceInfo{
		ResourceType: resource,
		ResourceName: identifier,
		Description:  fmt.Sprintf("The requested %s does not exist.", resource),
	})
	if detailErr == nil {
		err.AddDetail(detail)
	}
	return err
}

// AlreadyExists returns a [connect.CodeAlreadyExists] error indicating a
// duplicate resource.
func AlreadyExists(resource, identifier string) *connect.Error {
	return connect.NewError(
		connect.CodeAlreadyExists,
		fmt.Errorf("%s %q already exists", resource, identifier),
	)
}

// InvalidArgument returns a [connect.CodeInvalidArgument] error with a
// [errdetails.BadRequest] detail containing the supplied field violations.
func InvalidArgument(violations ...FieldViolation) *connect.Error {
	err := connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("validation failed"))
	br := &errdetails.BadRequest{}
	for _, v := range violations {
		br.FieldViolations = append(br.FieldViolations, &errdetails.BadRequest_FieldViolation{
			Field:       v.Field,
			Description: v.Description,
		})
	}
	detail, detailErr := connect.NewErrorDetail(br)
	if detailErr == nil {
		err.AddDetail(detail)
	}
	return err
}

// PermissionDenied returns a [connect.CodePermissionDenied] error.
func PermissionDenied(msg string) *connect.Error {
	return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("%s", msg))
}

// Aborted returns a [connect.CodeAborted] error for etag/concurrency conflicts.
func Aborted(msg string) *connect.Error {
	return connect.NewError(connect.CodeAborted, fmt.Errorf("%s", msg))
}

// FailedPrecondition returns a [connect.CodeFailedPrecondition] error for
// operations rejected because the system is not in the required state.
func FailedPrecondition(msg string) *connect.Error {
	return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%s", msg))
}

// Internal wraps an unexpected error as [connect.CodeInternal]. The original
// error is logged via [slog.ErrorContext] but NOT sent to the client — the
// client receives a generic "internal error" message.
//
// Callers must not log the same error again; this function handles logging
// exactly once.
func Internal(ctx context.Context, err error) *connect.Error {
	slog.ErrorContext(ctx, "internal error", "error", err)
	return connect.NewError(connect.CodeInternal, fmt.Errorf("internal error"))
}
