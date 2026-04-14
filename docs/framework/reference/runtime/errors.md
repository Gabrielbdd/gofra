# runtime/errors

> Connect RPC error helpers with structured error details.

## Status

Alpha — API may change before v1.

## Import

```go
import "databit.com.br/gofra/runtime/errors"
```

The package is named `runtimeerrors` in code.

## API

### Error Constructors

Each constructor returns a `*connect.Error` with the appropriate Connect error
code and, where applicable, structured error details following Google API
conventions.

```go
func NotFound(resource, identifier string) *connect.Error
```

Returns `CodeNotFound` with a `ResourceInfo` detail containing the resource
type and identifier.

```go
func AlreadyExists(resource, identifier string) *connect.Error
```

Returns `CodeAlreadyExists` with a `ResourceInfo` detail.

```go
func InvalidArgument(violations ...FieldViolation) *connect.Error
```

Returns `CodeInvalidArgument` with a `BadRequest` detail containing field
violations.

```go
func PermissionDenied(msg string) *connect.Error
```

Returns `CodePermissionDenied` with the given message.

```go
func Aborted(msg string) *connect.Error
```

Returns `CodeAborted` with the given message. Use for etag/concurrency
conflicts.

```go
func FailedPrecondition(msg string) *connect.Error
```

Returns `CodeFailedPrecondition` with the given message.

```go
func Internal(ctx context.Context, err error) *connect.Error
```

Wraps an unexpected error as `CodeInternal`. Logs the original error via
`slog.ErrorContext` but returns a generic `"internal error"` message to the
client. Never leaks internal details.

### FieldViolation

```go
type FieldViolation struct {
    Field       string
    Description string
}
```

Describes a single field-level validation failure. Used with
`InvalidArgument`.

### Panic Recovery

```go
func RecoverHandler(ctx context.Context, spec connect.Spec, _ http.Header, r any) error
```

A callback for `connect.WithRecover()` that logs the panic value and stack
trace, then returns a sanitized `CodeInternal` error to the client.

## Behavior

### Error Detail Attachment

`NotFound`, `AlreadyExists`, and `InvalidArgument` attach structured error
details to the Connect error using `errdetails` from the
`google.golang.org/genproto` package. Clients can inspect these details
programmatically.

### Internal Error Sanitization

`Internal` always sends `"internal error"` to the client regardless of the
original error message. The original error and its message are logged
server-side only.

### Panic Recovery

`RecoverHandler` is designed to be passed to `connect.WithRecover()` as an
interceptor option. It captures panics in Connect handlers, logs them with a
stack trace, and converts them to a `CodeInternal` error.

## Examples

```go
// Not found
return nil, runtimeerrors.NotFound("user", userID)

// Validation errors
return nil, runtimeerrors.InvalidArgument(
    runtimeerrors.FieldViolation{Field: "email", Description: "must not be empty"},
    runtimeerrors.FieldViolation{Field: "name", Description: "must be at least 2 characters"},
)

// Internal error (logs original, sends generic message)
if err != nil {
    return nil, runtimeerrors.Internal(ctx, err)
}

// Panic recovery interceptor
handler := connect.NewUnaryHandler(
    "/example",
    exampleFn,
    connect.WithRecover(runtimeerrors.RecoverHandler),
)
```

## Related Pages

- [runtime/serve](serve.md) — The server that hosts Connect handlers.
- [runtime/health](health.md) — Health probes use standard HTTP status codes,
  not Connect errors.
