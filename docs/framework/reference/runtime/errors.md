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

### FieldViolation

```go
type FieldViolation struct {
    Field       string
    Description string
}
```

Describes a single field-level validation failure. A slice is used (not a map)
so that ordering is preserved and multiple violations on the same field are
representable.

### Error Constructors

Each constructor returns a `*connect.Error` with the appropriate Connect error
code. Where noted, structured error details following Google API conventions
are attached.

#### NotFound

```go
func NotFound(resource, identifier string) *connect.Error
```

Returns `connect.CodeNotFound`.

- **Message:** `<resource> "<identifier>" not found`
- **Detail:** `errdetails.ResourceInfo` with `ResourceType`, `ResourceName`,
  and `Description: "The requested <resource> does not exist."`

#### AlreadyExists

```go
func AlreadyExists(resource, identifier string) *connect.Error
```

Returns `connect.CodeAlreadyExists`.

- **Message:** `<resource> "<identifier>" already exists`
- **Detail:** None.

#### InvalidArgument

```go
func InvalidArgument(violations ...FieldViolation) *connect.Error
```

Returns `connect.CodeInvalidArgument`.

- **Message:** `"validation failed"`
- **Detail:** `errdetails.BadRequest` containing one
  `BadRequest_FieldViolation` per input violation, preserving order.

#### PermissionDenied

```go
func PermissionDenied(msg string) *connect.Error
```

Returns `connect.CodePermissionDenied`.

- **Message:** The `msg` argument verbatim.
- **Detail:** None.

#### Aborted

```go
func Aborted(msg string) *connect.Error
```

Returns `connect.CodeAborted`. Use for etag/concurrency conflicts.

- **Message:** The `msg` argument verbatim.
- **Detail:** None.

#### FailedPrecondition

```go
func FailedPrecondition(msg string) *connect.Error
```

Returns `connect.CodeFailedPrecondition`. Use when the system is not in the
required state for the operation.

- **Message:** The `msg` argument verbatim.
- **Detail:** None.

#### Internal

```go
func Internal(ctx context.Context, err error) *connect.Error
```

Returns `connect.CodeInternal`. The original error is **never** sent to the
client.

- **Message:** `"internal error"` (always, regardless of the original error).
- **Detail:** None.
- **Side effect:** Logs the original error via
  `slog.ErrorContext(ctx, "internal error", "error", err)`.

Callers must not log the same error again — this function handles logging
exactly once.

### Panic Recovery

```go
func RecoverHandler(
    ctx context.Context,
    spec connect.Spec,
    _ http.Header,
    r any,
) error
```

A callback for `connect.WithRecover()` that captures panics in Connect
handlers, logs them with a stack trace, and returns a sanitized error.

- **Returns:** `connect.CodeInternal` with message `"internal error"`.
- **Logs:** `slog.ErrorContext` with message `"panic recovered"` and fields:
  - `panic` — the recovered value
  - `stack` — full goroutine stack trace (`debug.Stack()`)
  - `procedure` — the Connect procedure name from `spec.Procedure`

`http.ErrAbortHandler` panics are not intercepted by this handler — Connect's
own recovery machinery handles them before `RecoverHandler` is called.

## Behavior

### Error Detail Attachment

`NotFound` and `InvalidArgument` attach structured error details using
`connect.NewErrorDetail()`. If detail creation fails (which should not happen
with well-formed inputs), the error is returned without the detail. Clients
can inspect details programmatically via the Connect error details API.

`AlreadyExists` does not attach error details — only the message carries the
resource information.

### Internal Error Sanitization

`Internal` is designed to prevent information leakage. The original error
message, type, and stack trace are logged server-side only. The client always
receives the generic `"internal error"` message regardless of what the
original error contained.

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
handler := svcconnect.NewFooServiceHandler(
    &FooService{},
    connect.WithRecover(runtimeerrors.RecoverHandler),
)
```

## Related Pages

- [runtime/serve](serve.md) — The server that hosts Connect handlers.
- [runtime/health](health.md) — Health probes use standard HTTP status codes,
  not Connect errors.
