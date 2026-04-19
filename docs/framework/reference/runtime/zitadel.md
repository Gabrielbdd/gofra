# runtime/zitadel

> Helpers for calling the ZITADEL Management and v2 Connect RPC APIs from
> consumer applications built on Gofra. Pairs with a small companion package
> that reads a Personal Access Token from a file or environment variable.

## Status

Alpha — API may change before v1.

The generated starter does **not** import this package. It exists for
consumer apps (e.g., a product built on Gofra) that need to provision
organizations, users, projects, or applications in ZITADEL on behalf of
the deployment.

## Import

```go
import (
    "github.com/Gabrielbdd/gofra/runtime/zitadel"
    zitadelsecret "github.com/Gabrielbdd/gofra/runtime/zitadel/secret"
)
```

Packages are named `zitadel` and `zitadelsecret` in code.

## API

### `zitadel`

```go
type Config struct {
    Issuer     string
    PAT        string
    OrgID      string
    HTTPClient *http.Client
}
```

Convenience container for consumer apps carrying issuer + credentials +
org scope. The package does not consume `Config` directly; it is a shape
convention for downstream code.

```go
func NewAuthInterceptor(pat, orgID string) connect.Interceptor
```

Returns a `connect.Interceptor` that applies `Authorization: Bearer <pat>`
on every unary and streaming client request. When `orgID` is non-empty it
also sets `x-zitadel-orgid: <orgID>` so the request targets a specific
organization.

The interceptor is a no-op on the streaming handler side — it is intended
for outbound calls to ZITADEL, not for receiving them.

Empty `pat` yields an interceptor that still sets `Authorization: Bearer `;
callers are responsible for ensuring they pass a non-empty token (use
[`zitadelsecret.Read`](#zitadelsecret) to source one).

### `zitadelsecret`

```go
type Source struct {
    FilePath string
    EnvVar   string
}

var ErrNotFound = errors.New("zitadelsecret: no PAT found in file or env")

func Read(src Source) (string, error)
```

Resolves a PAT. `FilePath` is tried first; on missing file (ENOENT) the
reader falls through to `EnvVar`. Other filesystem errors are returned
as-is. Returned values have surrounding whitespace trimmed. An empty file
is treated as a hard error — callers almost certainly want to know rather
than silently fall through to the environment.

## Defaults

- `HTTPClient` defaults to `http.DefaultClient` when nil.
- Empty `OrgID` omits the `x-zitadel-orgid` header entirely.

## Behavior

### Connect client wiring

This package deliberately does not bundle generated Connect service
clients (Organization, User, Project, Application). Consumer apps pick
their preferred source for those stubs — the upstream ZITADEL `.proto`
files compiled with `buf`, the `zitadel-go` module, or hand-written
wrappers. Gofra stays out of that decision so ZITADEL module cadence does
not drive framework releases.

### File-or-env precedence

`zitadelsecret.Read` reads the file first. If the file is missing the
reader falls through to the environment. If the file exists but is empty,
an error is returned — this catches the common misconfiguration where a
volume mount exists but was not yet populated.

### Unary and streaming

`NewAuthInterceptor` wraps both unary and streaming **client** calls. The
streaming **handler** path is a no-op — this package is a client-side
helper, not a server-side enforcer. For server-side authentication use
[`runtime/auth`](auth.md).

## Errors

- `zitadelsecret.ErrNotFound`: neither `FilePath` nor `EnvVar` yielded a
  value.
- Wrapped `os.PathError`: filesystem read error on the PAT file.
- `zitadelsecret: %q is empty`: the PAT file exists but contains only
  whitespace.

## Example

```go
pat, err := zitadelsecret.Read(zitadelsecret.Source{
    FilePath: os.Getenv("APP_ZITADEL_PROVISIONER_PAT_FILE"),
})
if err != nil {
    return err
}

interceptor := zitadel.NewAuthInterceptor(pat, "")

orgs := orgv2connect.NewOrganizationServiceClient(
    http.DefaultClient,
    "http://localhost:8081",
    connect.WithInterceptors(interceptor),
)

_, err = orgs.AddOrganization(ctx, connect.NewRequest(&orgv2.AddOrganizationRequest{
    Name: "Acme Corp",
}))
```

## Related pages

- [`runtime/auth`](auth.md) — server-side JWT validation for inbound
  requests. Pairs with `runtime/zitadel` when the same app both accepts
  user tokens and makes provisioning calls.
- [`docs/08-auth.md`](../../../08-auth.md) — framework authentication
  design and tradeoffs.
