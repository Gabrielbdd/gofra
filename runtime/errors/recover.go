package runtimeerrors

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"connectrpc.com/connect"
)

// RecoverHandler is a callback for [connect.WithRecover] that logs the panic
// with its stack trace and returns a sanitized [connect.CodeInternal] error.
//
// Wire it when creating Connect service handlers:
//
//	mux.Handle(svcconnect.NewFooServiceHandler(
//	    &FooService{},
//	    connect.WithRecover(runtimeerrors.RecoverHandler),
//	))
//
// Connect's recovery machinery preserves standard library semantics for
// [http.ErrAbortHandler] — it is not intercepted by this handler.
func RecoverHandler(ctx context.Context, spec connect.Spec, _ http.Header, r any) error {
	slog.ErrorContext(ctx, "panic recovered",
		"panic", r,
		"stack", string(debug.Stack()),
		"procedure", spec.Procedure,
	)
	return connect.NewError(connect.CodeInternal, fmt.Errorf("internal error"))
}
