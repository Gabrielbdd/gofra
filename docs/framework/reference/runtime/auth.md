# runtime/auth

> JWT-based authentication for Connect RPC services. Extracts and validates
> Bearer tokens, attaches the authenticated user to request context, and
> enforces private-by-default access on Connect procedures.

## Status

Alpha — API may change before v1.

## Import

```go
import "databit.com.br/gofra/runtime/auth"
```

The package is named `runtimeauth` in code.

## API

### Types

```go
type User struct {
    ID string
}
```

The authenticated identity extracted from a JWT access token. `ID` is the
`sub` claim — the unique user identifier from the identity provider.

```go
type Verifier interface {
    Verify(ctx context.Context, rawToken string) (User, error)
}
```

Validates a raw bearer token and returns the authenticated user.

```go
type ProcedureMatcher func(procedure string) bool
```

Reports whether a Connect RPC procedure path should be accessible without
authentication.

```go
type ClaimMapperFunc func(claims json.RawMessage) (User, error)
```

Extracts a User from raw JWT claims. The default mapper handles ZITADEL's
claim format.

```go
type Option func(*jwtVerifierConfig)
```

Configures [NewJWTVerifier].

### Functions

#### NewJWTVerifier

```go
func NewJWTVerifier(
    ctx context.Context,
    issuerURL, audience string,
    opts ...Option,
) (Verifier, error)
```

Creates a Verifier that validates JWT access tokens using OIDC discovery.
The sequence is:

1. Validate that `issuerURL` and `audience` are non-empty.
2. Fetch the provider's OpenID Connect metadata from
   `<issuerURL>/.well-known/openid-configuration`.
3. Locate the JWKS endpoint from the metadata.
4. Return a verifier that validates token signature, issuer, audience, and
   expiry against the discovered keys.

OIDC discovery is a one-time network call at construction. If discovery
fails, an error is returned — this provides fail-fast behaviour on startup.

The `audience` parameter is the expected `aud` claim in the access token.
For ZITADEL this is the project ID of the API application.

**Important:** The protected API application in ZITADEL must be configured
to issue JWT access tokens (not opaque tokens). A JWKS-only verifier cannot
validate opaque tokens.

#### WithHTTPClient

```go
func WithHTTPClient(client *http.Client) Option
```

Sets the HTTP client used for OIDC discovery and JWKS fetching. Useful for
testing or when the IdP is behind a proxy.

#### WithClaimMapper

```go
func WithClaimMapper(fn ClaimMapperFunc) Option
```

Overrides the default ZITADEL-aware claim extraction. The mapper receives
the raw JSON claims from the validated token and must return a User.

#### WithUser

```go
func WithUser(ctx context.Context, user User) context.Context
```

Returns a copy of ctx with the given user attached.

#### UserFromContext

```go
func UserFromContext(ctx context.Context) (User, bool)
```

Retrieves the authenticated user from ctx. The second return value is false
when no user is present (unauthenticated request or public procedure).

#### PublicProcedures

```go
func PublicProcedures(procedures ...string) ProcedureMatcher
```

Returns a ProcedureMatcher that matches any of the listed procedure paths.
Procedure paths use the Connect convention: `/<package>.<Service>/<Method>`.

#### NewMiddleware

```go
func NewMiddleware(
    verifier Verifier,
    isPublic ProcedureMatcher,
) func(http.Handler) http.Handler
```

Returns HTTP middleware that enforces authentication on Connect RPC
procedures.

The middleware distinguishes Connect RPC paths from non-RPC paths by
checking whether the first path segment contains a dot and is followed by a
second segment (e.g. `/blog.v1.PostsService/CreatePost`). Non-RPC paths
such as `/_gofra/config.js`, static assets, and SPA routes pass through
without authentication.

For Connect procedures the middleware:

1. Checks if the procedure is public via `isPublic`.
2. Extracts the Bearer token from the Authorization header.
3. Validates the token using the provided Verifier.
4. Attaches the authenticated User to the request context.

## Defaults

| Setting | Default |
|---------|---------|
| Default claim mapper | ZITADEL-aware: extracts `sub` as `User.ID` |
| Non-Connect paths | Pass through without auth |
| Connect procedures | Private by default (require valid Bearer token) |

## Starter Integration

Auth is **opt-in** in the generated starter. The starter's `main.go` checks
`cfg.Auth.Issuer` and `cfg.Auth.Audience` — when either is empty, auth
middleware is not mounted and the server starts without JWT enforcement. This
keeps a fresh `gofra new` app runnable before ZITADEL infrastructure is set
up.

To enable auth, uncomment and configure the `auth` section in `gofra.yaml`:

```yaml
auth:
  issuer: "http://localhost:8081"
  audience: "<ZITADEL project ID for API application>"
```

## Behavior

### Connect Procedure Detection

A request path is treated as a Connect procedure when it has the form
`/<segment-with-dot>/<method>`. Examples:

- `/blog.v1.PostsService/CreatePost` — Connect procedure, auth enforced
- `/_gofra/config.js` — not a procedure (no second segment), passes through
- `/healthz/ready` — not a procedure (no dot in first segment), passes through
- `/assets/app.js` — not a procedure (no dot in first segment), passes through

Health endpoints mounted on the root `http.ServeMux` (outside the chi
router) are structurally unreachable by middleware mounted on the chi
router.

### Token Extraction

The middleware reads the `Authorization` header and expects the format
`Bearer <token>`. The prefix comparison is case-insensitive. If the header
is missing, empty, or uses a different scheme (e.g. `Basic`), the request
is rejected.

### Error Responses

Missing or invalid tokens produce a Connect-compatible JSON error response:

```json
{"code": "unauthenticated", "message": "missing bearer token"}
```

- HTTP status: 401
- Content-Type: `application/json`
- The `message` is either `"missing bearer token"` or `"invalid token"`.
  Detailed verification errors are logged server-side but not sent to the
  client.

### ZITADEL Claim Extraction

The default claim mapper extracts:

| JWT Claim | User Field |
|-----------|------------|
| `sub` | `ID` |

The `sub` claim is required. If missing, verification fails.

### Fail-Fast Startup

`NewJWTVerifier` performs OIDC discovery during construction. If the issuer
is unreachable or returns invalid metadata, the application fails to start
rather than silently accepting all requests.

## Errors and Edge Cases

- If `issuerURL` is empty, `NewJWTVerifier` returns an error without
  attempting discovery.
- If `audience` is empty, `NewJWTVerifier` returns an error.
- If OIDC discovery fails (network error, invalid metadata),
  `NewJWTVerifier` returns a wrapped error.
- If a token's signature is invalid, expired, or has the wrong
  issuer/audience, `Verify` returns an error.
- If the claim mapper fails to extract required claims, `Verify` returns an
  error.
- A nil `ProcedureMatcher` is safe — all Connect procedures require auth.

## Examples

### Verifier setup and middleware wiring (opt-in)

```go
var authMiddleware func(http.Handler) http.Handler

if cfg.Auth.Issuer != "" && cfg.Auth.Audience != "" {
    verifier, err := runtimeauth.NewJWTVerifier(ctx,
        cfg.Auth.Issuer,
        cfg.Auth.Audience,
    )
    if err != nil {
        slog.Error("auth verifier setup failed", "error", err)
        os.Exit(1)
    }

    isPublic := runtimeauth.PublicProcedures(
        "/blog.v1.PostsService/ListPosts",
    )

    authMiddleware = runtimeauth.NewMiddleware(verifier, isPublic)
}

app := chi.NewRouter()
if authMiddleware != nil {
    app.Use(authMiddleware)
}
```

### Reading the authenticated user in a handler

```go
func (s *PostsService) CreatePost(
    ctx context.Context,
    req *connect.Request[postsv1.CreatePostRequest],
) (*connect.Response[postsv1.CreatePostResponse], error) {
    user, ok := runtimeauth.UserFromContext(ctx)
    if !ok {
        return nil, runtimeerrors.Internal(ctx,
            fmt.Errorf("expected authenticated user"))
    }

    // user.ID is the ZITADEL subject identifier.
    post, err := s.Queries.CreatePost(ctx, db.CreatePostParams{
        AuthorUserID: user.ID,
        Title:        req.Msg.Title,
    })
    // ...
}
```

## Related Pages

- [runtime/errors](errors.md) — Connect error helpers used alongside auth
  checks.
- [runtime/serve](serve.md) — Manages the HTTP server lifecycle that hosts
  the auth middleware.
- [runtime/config](config.md) — Loads the `AuthConfig` that supplies issuer
  and audience.
