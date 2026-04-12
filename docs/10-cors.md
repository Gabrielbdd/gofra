# 10 — CORS

> Parent: [Index](00-index.md) | Prev: [Errors](09-errors.md) | Next: [Health Checks](11-health-checks.md)


## Addendum to Architecture Design Document
## Last Updated: 2026-04-12

---

## The Problem

Forge serves a React SPA from Vite in development (`:5173`) and Connect RPC
from the Go server (`:3000`). These are different origins. Every API call from
the browser triggers CORS.

Connect RPC has specific CORS requirements beyond typical REST APIs:

1. **All mutations are POST requests.** Connect uses `POST` for all RPCs
   (mutations and queries). POST with `Content-Type: application/json` or
   `application/proto` is not a "simple" CORS request — the browser sends a
   preflight `OPTIONS` first.

2. **Protocol-specific headers.** Connect, gRPC-Web, and gRPC each use
   different request/response headers (`Connect-Protocol-Version`,
   `Grpc-Status`, `Grpc-Message`, etc.). All must be allowed/exposed.

3. **Trailers as headers.** Connect sends response trailers as HTTP headers
   with a `Trailer-` prefix for unary RPCs. These must be explicitly exposed
   or the client can't read them.

4. **GET requests for read RPCs.** Connect supports `GET` for RPCs annotated
   with `idempotency_level = NO_SIDE_EFFECTS`. GET requests can avoid CORS
   preflight entirely (reducing latency), but only if configured.

5. **Auth bearer tokens.** The SPA sends `Authorization: Bearer <JWT>`. This
   header makes cross-origin RPCs preflight, and the server must explicitly
   allow the `Authorization` header. This is different from cookie-based
   browser auth: Forge's default bearer-token flow does not require
   `AllowCredentials`.

---

## Solution: `connectrpc.com/cors` + `rs/cors`

Connect maintains an official Go package (`connectrpc.com/cors`) that provides
the exact list of headers needed for all three protocols. It's designed to
pair with any standard CORS library — we use `rs/cors`, the most widely used
Go CORS middleware.

```go
// forge/cors.go
package forge

import (
    "net/http"

    connectcors "connectrpc.com/cors"
    "github.com/rs/cors"
)

func CORSMiddleware(cfg CORSConfig) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        c := cors.New(cors.Options{
            AllowedOrigins:   cfg.AllowedOrigins,
            AllowedMethods:   connectcors.AllowedMethods(), // GET, POST
            AllowedHeaders: append(
                connectcors.AllowedHeaders(), // Connect + gRPC protocol headers
                "Authorization",              // Bearer JWT
            ),
            ExposedHeaders: connectcors.ExposedHeaders(), // Grpc-Status, Grpc-Message, etc.
            AllowCredentials: false,
            MaxAge:           7200, // 2 hours — Chrome caps at this
        })
        return c.Handler(next)
    }
}

type CORSConfig struct {
    AllowedOrigins []string
}
```

### What `connectcors.AllowedHeaders()` returns

These are the request headers browsers need to send for Connect and gRPC-Web:

- `Connect-Protocol-Version`
- `Connect-Timeout-Ms`
- `Content-Type` (application/json, application/proto, etc.)
- `Grpc-Timeout`
- `X-Grpc-Web`
- `X-User-Agent`

We append `Authorization` because the SPA sends Bearer tokens. If the
application uses additional custom request headers, they must be added here
too.

### What `connectcors.ExposedHeaders()` returns

These are the response headers browsers need to read:

- `Grpc-Status`
- `Grpc-Message`
- `Grpc-Status-Details-Bin`

Without exposing these, gRPC-Web error details are invisible to the client.
If the application sets custom response headers or trailers, those must be
added here too (trailers with the `Trailer-` prefix).

**Reason for `connectrpc.com/cors`**: Connect's protocol headers evolve with
the protocol. Hardcoding them in the application means updating Forge when
Connect adds a new header. The official package tracks the protocol — when
Connect changes, the package updates, and Forge picks it up via `go get`.

**Reason for `rs/cors`**: It's the most used Go CORS library (3600+
importers), handles all CORS edge cases (wildcards, credentials, preflight
caching), and is a standard `net/http` middleware that wraps any `http.Handler`.

**Reason for `AllowCredentials: false`**: Forge's default browser auth flow
uses bearer tokens, not cookies. The browser still preflights because of the
`Authorization` header, but that does not require CORS credential mode.
Leaving credentials disabled keeps the CORS contract aligned with the chosen
auth architecture.

**Reason for `MaxAge: 7200`**: Browsers cache preflight responses. Each RPC
call triggers a preflight; caching avoids redundant OPTIONS requests. Chrome
caps `Access-Control-Max-Age` at 7200 seconds (2 hours). Setting it higher
has no effect.

---

## Configuration

```yaml
# forge.yaml
cors:
  allowed_origins:
    - "http://localhost:5173"   # Vite dev server
    - "http://localhost:3000"   # same-origin (for production)
```

```yaml
# Production (via env var)
# FORGE_CORS_ALLOWED_ORIGINS=https://myapp.com,https://www.myapp.com
```

**Reason for not defaulting to `*`**: bearer-token requests can technically
use wildcard origins when cookies are not involved, but Forge still keeps an
explicit origin allowlist. Authenticated browser APIs are easier to reason
about when the intended frontend origins are named directly.

**Development defaults**: When `app.env` is `development`, the framework
automatically adds `http://localhost:5173` (Vite default) to allowed origins
if no CORS config is provided. This ensures `mise run dev` works out of the
box without CORS errors.

```go
func defaultCORSConfig(env string) CORSConfig {
    if env == "development" {
        return CORSConfig{
            AllowedOrigins: []string{
                "http://localhost:5173", // Vite dev server
                "http://localhost:3000", // Go server (same-origin)
            },
        }
    }
    // Production: must be explicitly configured
    return CORSConfig{}
}
```

---

## Mux Integration

CORS middleware wraps the entire mux — it must handle preflight OPTIONS
requests before they reach Connect handlers or the SPA fallback.

```go
// cmd/app/main.go
func main() {
    cfg := config.Load()

    mux := chi.NewRouter()

    // CORS must be first — preflight OPTIONS must be handled before routing
    mux.Use(forge.CORSMiddleware(cfg.CORS))

    // ... remaining middleware and handlers
}
```

**Reason CORS is the first middleware**: The browser sends an `OPTIONS` request
with CORS headers before the actual `POST`. If CORS middleware isn't first,
the OPTIONS request may hit a route that doesn't handle it (returning 405),
and the browser blocks the actual request. CORS must intercept before any
other routing or middleware.

---

## When CORS Is Not Needed

In production, if the SPA is served from the same Go binary (via `embed.FS`),
both the SPA and the API are on the same origin. CORS is not needed. The
browser makes same-origin requests.

```
Development:
  SPA at http://localhost:5173 → API at http://localhost:3000
  → CORS required (different port = different origin)

Production:
  SPA at https://myapp.com → API at https://myapp.com
  → CORS not required (same origin, SPA embedded in Go binary)
```

The CORS middleware still runs in production (it's a no-op for same-origin
requests). This is harmless and avoids environment-specific middleware stacks.

---

## Connect GET Requests (Avoiding Preflight)

For read-only RPCs (`GetPost`, `ListPosts`), Connect supports `GET` requests
that avoid CORS preflight entirely. This eliminates the preflight round-trip,
reducing latency — especially on high-latency connections (mobile, cellular).

To opt in, annotate the RPC with `idempotency_level = NO_SIDE_EFFECTS`:

```protobuf
service PostsService {
  rpc GetPost(GetPostRequest) returns (Post) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
  rpc ListPosts(ListPostsRequest) returns (ListPostsResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
  // Mutations stay as POST — preflight is required
  rpc CreatePost(CreatePostRequest) returns (Post) {}
}
```

The SPA transport uses `useHttpGet: true`:

```ts
export const transport = createConnectTransport({
  baseUrl: import.meta.env.VITE_API_URL ?? "",
  useHttpGet: true, // enables GET for NO_SIDE_EFFECTS RPCs
});
```

Connect automatically uses `GET` for annotated RPCs and `POST` for
everything else. No code changes in handlers.

**Reason to annotate only truly side-effect-free RPCs**: GET requests can be
cached by browsers and CDNs. An RPC that has side effects (incrementing a
view counter, logging an event) should not be cached. The annotation is a
contract: "this RPC produces the same result regardless of how many times
it's called."

---

## Decision Log (CORS)

| # | Decision | Rationale |
|---|----------|-----------|
| 80 | `connectrpc.com/cors` for header lists | Official package, tracks protocol changes. Don't hardcode protocol headers. |
| 81 | `rs/cors` for CORS middleware | Most widely used, handles all edge cases, standard `net/http` middleware. |
| 82 | `AllowCredentials: false` for the default bearer-token SPA flow | Browser auth uses `Authorization` headers, not cookies. Preflight still happens, but credential mode is not required. |
| 83 | Explicit origins even without cookie auth | Bearer-token CORS can technically use `*`, but Forge keeps an explicit allowlist for a clearer browser contract. |
| 84 | Explicit allowed origins in config | Developer lists origins. Framework adds `localhost:5173` in dev mode automatically. |
| 85 | CORS middleware first in chain | Preflight OPTIONS must be handled before routing, auth, or any other middleware. |
| 86 | `MaxAge: 7200` | Reduces preflight requests. Chrome's maximum. |
| 87 | `idempotency_level = NO_SIDE_EFFECTS` for reads | Enables Connect GET. Avoids CORS preflight for read RPCs. Fewer round-trips. |
