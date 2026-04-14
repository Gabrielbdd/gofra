# 09 — Error Handling

> Parent: [Index](00-index.md) | Prev: [Auth](08-auth.md) | Next: [CORS](10-cors.md)


## Addendum to Architecture Design Document
## Last Updated: 2026-04-12

---

## The Problem

Errors in Gofra cross three boundaries:

1. **Handler → Client**: A Connect RPC handler returns an error to the SPA.
   The SPA needs the error code, a human-readable message, and optionally
   structured details (which fields failed validation, what resource wasn't
   found).

2. **Restate → Handler**: A Restate durable handler encounters a failure. It
   must decide: retry (transient) or stop (terminal)? Terminal errors
   propagate back to the caller. Retried errors don't.

3. **Handler → Restate → Eventually back to someone**: A Connect handler
   dispatches durable work via Restate. The durable work fails terminally.
   The user who triggered the original action doesn't see this error
   synchronously — it happened after the HTTP response was sent.

Each boundary has different error types, different semantics, and different
consumers. The framework must define patterns for all three.

---

## Layer 1: Connect RPC Errors (Handler → Client)

### Error Codes

Connect uses a fixed set of error codes (identical to gRPC's). Each code
maps to an HTTP status code. The framework adopts this set with no additions:

| Code | HTTP | When to use |
|------|------|-------------|
| `InvalidArgument` | 400 | Request validation failed. Bad input. |
| `Unauthenticated` | 401 | No valid credentials. Token missing or expired. |
| `PermissionDenied` | 403 | Authenticated but not authorized for this action. |
| `NotFound` | 404 | Resource doesn't exist (or caller lacks permission to know it exists). |
| `AlreadyExists` | 409 | Conflict. Duplicate create (e.g., unique slug collision). |
| `Aborted` | 409 | Concurrency conflict. Etag mismatch. |
| `FailedPrecondition` | 400 | System state prevents the operation (e.g., publish a draft that has no body). |
| `ResourceExhausted` | 429 | Rate limit exceeded. |
| `Internal` | 500 | Server bug. Unexpected error. |
| `Unavailable` | 503 | Transient. Retry later. Database temporarily unreachable. |

**Reason for using Connect's codes directly**: They're the same as gRPC's,
which are the same as AIP-193's. The frontend can handle them generically
by code number. No custom error codes to define or document.

### Error Construction Helpers

The framework provides helper functions that produce consistent errors with
proper details. Handlers call these instead of constructing errors manually.

```go
// runtime/errors/errors.go
package runtimeerrors

import (
    "context"
    "fmt"
    "log/slog"

    "connectrpc.com/connect"
    "google.golang.org/genproto/googleapis/rpc/errdetails"
)

// FieldViolation describes a single field-level validation failure.
// Using a struct instead of map[string]string preserves order and allows
// multiple violations on the same field.
type FieldViolation struct {
    Field       string
    Description string
}

// NotFound returns a CodeNotFound error with resource information.
func NotFound(resource, identifier string) *connect.Error {
    err := connect.NewError(connect.CodeNotFound,
        fmt.Errorf("%s %q not found", resource, identifier),
    )
    detail, _ := connect.NewErrorDetail(&errdetails.ResourceInfo{
        ResourceType: resource,
        ResourceName: identifier,
        Description:  fmt.Sprintf("The requested %s does not exist.", resource),
    })
    err.AddDetail(detail)
    return err
}

// AlreadyExists returns a CodeAlreadyExists error.
func AlreadyExists(resource, identifier string) *connect.Error {
    return connect.NewError(connect.CodeAlreadyExists,
        fmt.Errorf("%s %q already exists", resource, identifier),
    )
}

// InvalidArgument returns a CodeInvalidArgument error with field violations.
// Accepts a variadic list of FieldViolation structs so callers can express
// multiple violations per field and control ordering.
func InvalidArgument(violations ...FieldViolation) *connect.Error {
    err := connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("validation failed"))
    br := &errdetails.BadRequest{}
    for _, v := range violations {
        br.FieldViolations = append(br.FieldViolations, &errdetails.BadRequest_FieldViolation{
            Field:       v.Field,
            Description: v.Description,
        })
    }
    detail, _ := connect.NewErrorDetail(br)
    err.AddDetail(detail)
    return err
}

// PermissionDenied returns a CodePermissionDenied error.
func PermissionDenied(msg string) *connect.Error {
    return connect.NewError(connect.CodePermissionDenied, fmt.Errorf(msg))
}

// Aborted returns a CodeAborted error for etag/concurrency conflicts.
func Aborted(msg string) *connect.Error {
    return connect.NewError(connect.CodeAborted, fmt.Errorf(msg))
}

// FailedPrecondition returns a CodeFailedPrecondition error for operations
// rejected because the system is not in the required state.
func FailedPrecondition(msg string) *connect.Error {
    return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf(msg))
}

// Internal wraps an unexpected error as CodeInternal.
// The original error is logged via slog.ErrorContext but NOT sent to
// the client. Callers must not log the same error again.
func Internal(ctx context.Context, err error) *connect.Error {
    slog.ErrorContext(ctx, "internal error", "error", err)
    return connect.NewError(connect.CodeInternal, fmt.Errorf("internal error"))
}
```

**Reason for helper functions**: Raw `connect.NewError(connect.CodeNotFound, err)`
is verbose and doesn't include error details. Helpers enforce consistency:
every NotFound error has a `ResourceInfo` detail, every validation error has
`BadRequest` with field violations. The pattern is always the same.

**Reason `Internal()` logs but doesn't expose the original error**: Stack
traces and internal messages are security-sensitive. The client gets
"internal error". The server log gets the full error with trace_id for
correlation. `Internal()` accepts a `context.Context` so that
`slog.ErrorContext` can extract trace_id/span_id when the observability
package is wired in. Callers must not log the same error again — the
helper handles logging exactly once.

### Handler Usage

```go
func (s *PostsService) GetPost(
    ctx context.Context,
    req *connect.Request[postsv1.GetPostRequest],
) (*connect.Response[postsv1.Post], error) {

    post, err := s.Queries.GetPost(ctx, sqlc.GetPostParams{Slug: req.Msg.Slug})
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, runtimeerrors.NotFound("post", req.Msg.Slug)
        }
        return nil, runtimeerrors.Internal(ctx, err)
    }

    return connect.NewResponse(postRowToProto(post)), nil
}

func (s *PostsService) CreatePost(
    ctx context.Context,
    req *connect.Request[postsv1.CreatePostRequest],
) (*connect.Response[postsv1.Post], error) {

    post, err := s.Queries.CreatePost(ctx, sqlc.CreatePostParams{
        Title: req.Msg.Title,
        Slug:  slugify(req.Msg.Title),
        Body:  req.Msg.Body,
    })
    if err != nil {
        if isUniqueViolation(err, "posts_slug_key") {
            return nil, runtimeerrors.AlreadyExists("post", slugify(req.Msg.Title))
        }
        return nil, runtimeerrors.Internal(ctx, err)
    }

    return connect.NewResponse(postToProto(post)), nil
}
```

### Validation Errors from protovalidate

The `connectrpc.com/validate` interceptor handles validation errors from
`buf/validate` annotations automatically. When validation fails, it
returns `CodeInvalidArgument` with a **protovalidate `Violations`
detail** (not a `google.rpc.BadRequest`). No handler code needed for
proto-level validation.

**Note**: The protovalidate detail type differs from the `BadRequest`
detail that the framework's `InvalidArgument()` helper produces. The
frontend must handle both detail types, or the framework can provide a
validation-normalizing interceptor that converts protovalidate
`Violations` into `BadRequest` field violations for a uniform frontend
contract. This normalizer is deferred until protovalidate is wired into
the starter.

For application-level validation (business rules not expressible in proto
annotations), handlers use `runtimeerrors.InvalidArgument()`:

```go
if post.Status == "published" && post.Body == "" {
    return nil, runtimeerrors.InvalidArgument(
        runtimeerrors.FieldViolation{Field: "body", Description: "published posts must have a body"},
    )
}
```

---

## Layer 2: Restate Errors (Durable Handlers)

### Terminal vs. Retryable

Restate retries all errors by default (infinite retries with exponential
backoff). To stop retries and propagate the error, use `restate.TerminalError`:

```go
func (s SearchIndexer) Index(ctx restate.Context, req IndexPostRequest) error {
    post, err := restate.Run(ctx, func(ctx restate.RunContext) (sqlc.Post, error) {
        return s.Queries.GetPostByID(context.Background(), req.PostID)
    }, restate.WithName("load-post"))
    if err != nil {
        // Post not found is terminal — retrying won't help
        if errors.Is(err, pgx.ErrNoRows) {
            return restate.TerminalError(fmt.Errorf("post %d not found", req.PostID), 404)
        }
        // Database connection error is retryable — Restate retries automatically
        return err
    }

    _, err = restate.Run(ctx, func(ctx restate.RunContext) (restate.Void, error) {
        return restate.Void{}, search.Index("posts", post.ID, post.ToSearchableMap())
    }, restate.WithName("index"))
    return err // retryable if search is temporarily down
}
```

### The Decision Framework

| Error Type | Action | Example |
|------------|--------|---------|
| Resource doesn't exist | `restate.TerminalError` | Post deleted between dispatch and execution |
| Invalid input | `restate.TerminalError` | Malformed data that can't be fixed by retry |
| Business rule violation | `restate.TerminalError` | Can't publish post without body |
| Database temporarily down | Return `err` (retryable) | Restate retries with backoff |
| External API rate limited | Return `err` (retryable) | Restate retries after backoff |
| External API 500 | Return `err` (retryable) | Transient, may recover |
| External API 400 | `restate.TerminalError` | Bad request won't change on retry |

**Reason for this split**: The question is always "will retrying fix this?"
If the database is down, retrying in 5 seconds might work. If the post
doesn't exist, retrying in 5 seconds won't create it. Terminal errors stop
wasting resources on impossible retries.

### Saga Compensation on Terminal Errors

When a multi-step handler fails terminally after some steps have succeeded,
earlier steps may need compensation:

```go
func (w OrderCheckout) Run(ctx restate.WorkflowContext, req CheckoutRequest) (OrderResult, error) {
    // Step 1: Reserve inventory
    _, err := restate.Run(ctx, func(ctx restate.RunContext) (restate.Void, error) {
        return restate.Void{}, inventory.Reserve(req.Items)
    }, restate.WithName("reserve-inventory"))
    if err != nil {
        return OrderResult{}, err
    }

    // Step 2: Charge payment
    chargeID, err := restate.Run(ctx, func(ctx restate.RunContext) (string, error) {
        return payments.Charge(req.PaymentMethod, req.Total)
    }, restate.WithName("charge-payment"))
    if err != nil {
        // Compensate step 1
        restate.Run(ctx, func(ctx restate.RunContext) (restate.Void, error) {
            return restate.Void{}, inventory.Release(req.Items)
        }, restate.WithName("release-inventory"))
        return OrderResult{}, restate.TerminalError(
            fmt.Errorf("payment failed: %w", err), 402,
        )
    }

    // Step 3: Create order record
    // ...
}
```

**Reason compensation is explicit**: Restate journals every step. If the
handler crashes after charging but before creating the order, Restate
replays steps 1 and 2 from the journal (no re-execution) and continues
from step 3. Compensation only runs when the handler explicitly decides
the operation has failed — not on crashes.

---

## Layer 3: Async Error Visibility

When a Connect handler dispatches durable work and returns a response
before the durable work completes, the client doesn't see durable errors
synchronously. This is inherent to async processing.

### Pattern: Check Status Later

For operations where the client needs to know the outcome:

```protobuf
service PostsService {
  rpc PublishPost(PublishPostRequest) returns (PublishPostResponse) {}
  rpc GetPublishStatus(GetPublishStatusRequest) returns (PublishStatus) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
}
```

The handler returns immediately with a Restate invocation ID. The client
polls or subscribes for the result:

```go
func (s *PostsService) PublishPost(ctx context.Context, req *connect.Request[...]) (...) {
    // Dispatch as a workflow so the client can check status
    invocationID, err := restateingress.WorkflowSubmit[PublishRequest, PublishResult](
        s.Restate, "PublishWorkflow", req.Msg.Id,
    ).Submit(ctx, PublishRequest{PostID: req.Msg.Id})

    return connect.NewResponse(&postsv1.PublishPostResponse{
        InvocationId: invocationID,
        Status:       "processing",
    }), nil
}
```

### Pattern: Notify on Failure

For fire-and-forget operations (search indexing, email sending), failures
are visible through:

1. **Restate UI** (`:9070`) — shows failed invocations with error details
2. **Restate admin API** — queryable via SQL: `SELECT * FROM sys_invocation WHERE status = 'backing-off'`
3. **OTEL traces** — the failed invocation span carries the error
4. **Application logging** — `ctx.Log().Error(...)` in the handler

The framework does not build a custom "failed jobs dashboard." Restate's
admin API and UI already provide this. Gofra's observability layer (slog +
OTEL) ensures errors are visible in the same tools the team uses for
everything else.

---

## Frontend Error Handling

### Parsing Connect Errors in TypeScript

Connect-ES provides `ConnectError` with typed access to error codes and
details:

```ts
// web/src/lib/errors.ts
import { ConnectError, Code } from "@connectrpc/connect";
import { BadRequestSchema } from "./gen/google/rpc/error_details_pb";

export type FieldErrors = Record<string, string>;

export function extractFieldErrors(err: unknown): FieldErrors | null {
  if (!(err instanceof ConnectError)) return null;
  if (err.code !== Code.InvalidArgument) return null;

  const details = err.findDetails(BadRequestSchema);
  if (details.length === 0) return null;

  const errors: FieldErrors = {};
  for (const detail of details) {
    for (const violation of detail.fieldViolations) {
      errors[violation.field] = violation.description;
    }
  }
  return errors;
}

export function isNotFound(err: unknown): boolean {
  return err instanceof ConnectError && err.code === Code.NotFound;
}

export function isUnauthenticated(err: unknown): boolean {
  return err instanceof ConnectError && err.code === Code.Unauthenticated;
}

export function isPermissionDenied(err: unknown): boolean {
  return err instanceof ConnectError && err.code === Code.PermissionDenied;
}

export function getUserMessage(err: unknown): string {
  if (err instanceof ConnectError) {
    return err.rawMessage;
  }
  return "An unexpected error occurred.";
}
```

**Reason for `err.findDetails(BadRequestSchema)`**: Connect sends error
details as serialized protobuf `Any` messages. `findDetails` deserializes
them using the generated schema. This requires generating the Google RPC
error details protos into the frontend:

```bash
npx buf generate buf.build/googleapis/googleapis --path google/rpc/error_details.proto
```

### Using in React Components

```tsx
// web/src/components/post-form.tsx
import { useMutation } from "@connectrpc/connect-query";
import { createPost } from "../gen/.../posts-PostsService_connectquery";
import { extractFieldErrors, getUserMessage } from "../lib/errors";

function PostForm() {
  const mutation = useMutation(createPost);
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});

  const onSubmit = async (data: FormData) => {
    setFieldErrors({});
    try {
      await mutation.mutateAsync({
        title: data.get("title") as string,
        body: data.get("body") as string,
      });
    } catch (err) {
      const fields = extractFieldErrors(err);
      if (fields) {
        setFieldErrors(fields); // show inline field errors
      } else {
        toast.error(getUserMessage(err)); // show generic toast
      }
    }
  };

  return (
    <form onSubmit={onSubmit}>
      <Input name="title" error={fieldErrors.title} />
      <Textarea name="body" error={fieldErrors.body} />
      <Button type="submit">Create Post</Button>
    </form>
  );
}
```

### Global Error Handling via Connect Transport Interceptor

For errors that apply globally (auth expired, server unavailable), a
transport interceptor handles them before individual components:

```ts
// web/src/lib/transport.ts
import { createConnectTransport } from "@connectrpc/connect-web";
import { ConnectError, Code, Interceptor } from "@connectrpc/connect";

const authRedirectInterceptor: Interceptor = (next) => async (req) => {
  try {
    return await next(req);
  } catch (err) {
    if (err instanceof ConnectError && err.code === Code.Unauthenticated) {
      // Token expired — redirect to login
      window.location.href = "/login";
    }
    throw err;
  }
};

export const transport = createConnectTransport({
  baseUrl: import.meta.env.VITE_API_URL ?? "",
  interceptors: [authRedirectInterceptor],
});
```

---

## Panic Recovery

Connect provides a built-in `WithRecover` handler option that catches
panics inside RPC handlers. The framework provides a `RecoverHandler`
function that plugs into this option:

```go
// runtime/errors/recover.go
package runtimeerrors

// RecoverHandler is a connect.WithRecover callback that logs the panic
// and returns a sanitized CodeInternal error. It does not re-panic
// http.ErrAbortHandler (standard library semantics are preserved by
// Connect's recovery machinery).
func RecoverHandler(ctx context.Context, spec connect.Spec, header http.Header, r any) error {
    slog.ErrorContext(ctx, "panic recovered",
        "panic", r,
        "stack", string(debug.Stack()),
        "procedure", spec.Procedure,
    )
    return connect.NewError(connect.CodeInternal, fmt.Errorf("internal error"))
}
```

Wire it when creating Connect handlers:

```go
mux.Handle(postsv1connect.NewPostsServiceHandler(
    &PostsService{},
    connect.WithRecover(runtimeerrors.RecoverHandler),
))
```

**Reason for `connect.WithRecover` instead of HTTP middleware**:
`WithRecover` produces a proper Connect error response that is correct
across all three Connect wire protocols (Connect, gRPC, gRPC-Web). A
hand-rolled HTTP middleware writing raw JSON would only work for the
Connect protocol and would break gRPC and gRPC-Web clients.

For non-RPC HTTP routes (health checks, config endpoint, static files),
panics are less likely and chi's `middleware.Recoverer` is sufficient
since those routes always serve plain HTTP responses.

---

## Logging Conventions

| Scenario | Level | What to log |
|----------|-------|-------------|
| Validation error (client's fault) | `Warn` | Field violations, user ID |
| NotFound (normal operation) | `Debug` | Resource, identifier |
| PermissionDenied (authn ok, authz failed) | `Warn` | User ID, action, resource |
| Unauthenticated | `Info` | Request path (no user ID by definition) |
| AlreadyExists | `Info` | Resource, identifier |
| Internal (server bug) | `Error` | Full error, stack trace |
| Panic | `Error` | Panic value, stack trace |
| Restate terminal error | `Error` | Invocation ID, error |
| Restate retryable error | `Warn` | Invocation ID, attempt count, error |

**Reason for Warn on validation errors**: They're common and expected (users
submit bad input). They shouldn't page anyone. But they're not Debug because
a sudden spike in validation errors might indicate a frontend bug or an
attack.

**Reason for Debug on NotFound**: A user navigating to `/posts/nonexistent`
is normal. Logging it at Info or higher creates noise.

---

## Error Handling Conventions Summary

1. **Always use the `runtimeerrors` helpers** (`runtimeerrors.NotFound`,
   `runtimeerrors.Internal`, `runtimeerrors.InvalidArgument`) — they produce
   consistent errors with proper details.

2. **Never send internal error messages to clients.**
   `runtimeerrors.Internal(ctx, err)` logs the real error and returns
   "internal error" to the client. Callers must not log the same error again.

3. **In Restate handlers, decide: terminal or retryable.** Ask "will retrying
   fix this?" If yes, return the error (Restate retries). If no, return
   `restate.TerminalError`.

4. **Generate `google/rpc/error_details.proto` in the frontend.** This enables
   `err.findDetails(BadRequestSchema)` for typed field-level errors.

5. **Use `extractFieldErrors()` for form validation.** The helper extracts
   `BadRequest` field violations into a `Record<string, string>` for inline
   error display.

6. **Use a transport interceptor for global errors.** `Unauthenticated`
   redirects to login. `Unavailable` shows a retry banner. Individual
   component error handling is for business-specific errors only.

7. **Panic recovery uses `connect.WithRecover`**, which produces correct
   responses for all Connect wire protocols (Connect, gRPC, gRPC-Web).

---

## Decision Log (Error Handling)

| # | Decision | Rationale |
|---|----------|-----------|
| 96 | Connect error codes only, no custom codes | Standard set. Frontend handles generically. Same as gRPC and AIP-193. |
| 97 | `runtimeerrors.NotFound()`, `runtimeerrors.Internal()` helpers | Enforce consistent error construction with proper details. Package naming follows `runtime*` convention. |
| 98 | `Internal(ctx, err)` logs but hides original error | Security. Stack traces and internal messages don't reach clients. Accepts context for trace_id correlation. Callers must not double-log. |
| 99 | `BadRequest` with `FieldViolation` for app validation; protovalidate uses its own `Violations` detail | Google's standard type for app-level validation. Protovalidate's native type differs — normalize later or handle both in frontend. |
| 100 | Restate: terminal vs. retryable decision framework | "Will retrying fix this?" is the only question. Terminal for logic errors, retryable for transient failures. |
| 101 | No custom "failed jobs" dashboard | Restate UI + admin API + OTEL traces cover this. Don't rebuild what exists. |
| 102 | Generate `google/rpc/error_details.proto` for frontend | Enables `err.findDetails(BadRequestSchema)` for typed error detail extraction in TypeScript. |
| 103 | Transport interceptor for global error handling | Auth expiry, server unavailable. Handled once, not in every component. |
| 104 | `connect.WithRecover` for RPC panic recovery | Produces correct responses for Connect, gRPC, and gRPC-Web protocols. HTTP middleware only needed for non-RPC routes. |
| 105 | Warn for validation errors, Debug for NotFound | Validation spikes may indicate bugs/attacks. NotFound is normal navigation. |
| 124 | `InvalidArgument` takes `...FieldViolation` not `map[string]string` | Preserves field order, allows multiple violations per field. Matches Rails/Laravel/Phoenix DX. |
| 125 | `FailedPrecondition` helper included in first cut | Listed in error code table, handlers need it immediately (e.g., publish draft without body). |
