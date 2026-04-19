# 13 — Frontend: React SPA

> Parent: [Index](00-index.md) | Prev: [Graceful Shutdown](12-graceful-shutdown.md) | Next: [Tooling](14-tooling.md)

---

## Stack

| Concern | Tool | Reason |
|---------|------|--------|
| Build | Vite | Sub-second HMR. Optimized production bundles. |
| UI Framework | React | Largest ecosystem. Most available components and libraries. |
| Routing | TanStack Router (file-based) | Type-safe routes. File-based convention. |
| Data Fetching | TanStack Query + Connect-Query | Caching, refetching, optimistic updates. Connect-Query generates hooks from proto. |
| Components | shadcn/ui | Copied into project (not a dependency). Fully customizable. Accessible. |
| Styling | Tailwind CSS 4 | Utility-first. New engine requires no config file. |
| API Types | Generated from protobuf | Zero manual type definitions. End-to-end type safety. |

## How Types Flow

Two generators feed the browser:

```
posts.proto
    │
    ├── protoc-gen-es         → web/src/gen/.../posts_pb.ts        (TS message types)
    └── protoc-gen-connect-query → web/src/gen/.../posts_connectquery.ts (TanStack hooks)

config.proto
    │
    └── gofra generate config -ts-out web/src/gen
                              → web/src/gen/runtime-config.ts (public config types + loader)
```

**Decision #22.** The frontend developer writes `const { data } = useQuery(listPosts, { pageSize: 20 })` and `data` has the exact type of `ListPostsResponse`. No manual type definitions. No `any`. No `fetch` wrappers.

**Decision #Ncfg.** Public runtime config types come from the same
`config.proto` the backend uses — one proto, both sides. The generator
emits plain TS interfaces (no proto-es dependency for the runtime-config
path), plus `runtimeConfig` and `loadRuntimeConfig` accessors.

## Transport Configuration

```ts
// web/src/lib/transport.ts
import { createConnectTransport } from "@connectrpc/connect-web";
import { runtimeConfig } from "./runtime-config";

export const transport = createConnectTransport({
  baseUrl: runtimeConfig.apiBaseUrl,
  useHttpGet: true, // GET for NO_SIDE_EFFECTS RPCs (avoids CORS preflight)
});
```

## Runtime Browser Config

The browser does not read deploy-time settings from `import.meta.env`. Gofra
serves a generated public runtime config asset from the Go server:

```html
<!-- web/index.html -->
<script src="/_gofra/config.js"></script>
<script type="module" src="/src/main.tsx"></script>
```

The contract lives in its own proto:

```proto
// proto/myapp/runtime/v1/runtime_config.proto
syntax = "proto3";

package myapp.runtime.v1;

message RuntimeConfig {
  string api_base_url = 1;
  string sentry_dsn = 2;
  AuthConfig auth = 3;
}

message AuthConfig {
  string issuer = 1;
  string client_id = 2;
  repeated string scopes = 3;
  string redirect_path = 4;
  string post_logout_redirect_path = 5;
}
```

The user workflow is:

1. Add a field to `proto/myapp/runtime/v1/runtime_config.proto`.
2. Set the value under `public.*` in YAML, env vars, or flags.
3. Regenerate code.
4. Import the new typed field in the SPA.

Gofra generates the frontend loader, so application code does not touch
`window.__GOFRA_CONFIG__` directly:

```ts
// web/src/lib/runtime-config.ts
import { runtimeConfig } from "@/gen/runtime/runtime-config";

export { runtimeConfig };
```

The generator currently emits a single TS module:

- `web/src/gen/runtime-config.ts` — public config TS interfaces + a typed
  `RuntimeConfig` alias + `Window.__GOFRA_CONFIG__` augmentation +
  `runtimeConfig` and `loadRuntimeConfig` accessors.

App code imports from that file directly (no separate re-export layer is
required today):

```ts
// web/src/some-feature.tsx
import { runtimeConfig, loadRuntimeConfig } from "@/gen/runtime-config";
```

The generated loader reads `window.__GOFRA_CONFIG__`. `runtimeConfig` is a
`Partial<RuntimeConfig>` for the common case (falls back to `{}` when the
global is absent — useful during SSR or tests). `loadRuntimeConfig()`
fails fast with a clear error when the global is absent.

The generated TypeScript APIs are:

```ts
export interface PublicConfig { /* ...fields from proto public subtree... */ }
export type RuntimeConfig = PublicConfig;
export const runtimeConfig: Partial<RuntimeConfig>;
export function loadRuntimeConfig(): RuntimeConfig;
```

**Runtime schema validation is out of scope for this generator.** Apps that
need it can pair the output with a proto-es pipeline emitting `*_pb.ts`
and validate `window.__GOFRA_CONFIG__` against that schema before use.

On the backend, Gofra also generates a convention-first resolver. Proto fields
bind to the generated `cfg.Public` subtree by matching nested names:

- `runtime_config.api_base_url` -> `cfg.Public.APIBaseURL`
- `runtime_config.sentry_dsn` -> `cfg.Public.SentryDSN`
- `runtime_config.auth.client_id` -> `cfg.Public.Auth.ClientID`

The common case is zero handwritten mapping code:

```go
resolver := runtimeconfig.NewResolver(appCfg, BindPublicConfig)
mux.Handle("/_gofra/config.js", runtimeconfig.Handler(resolver))
```

For derived or request-aware values, the app can opt into a small mutator hook:

```go
resolver := runtimeconfig.NewResolver(
    appCfg,
    BindPublicConfig,
    runtimeconfig.WithMutator(func(ctx context.Context, r *http.Request, cfg *runtimev1.RuntimeConfig) error {
        // optional request-aware overrides
        return nil
    }),
)
```

Only browser-safe fields defined in `runtime_config.proto` can reach the
browser. Secrets, service-account keys, database URLs, and any other
server-only settings remain in Go memory only.

**Reason for a generated abstraction on both sides**: the browser contract is
typed once in proto, then consumed from generated Go and TypeScript. The user
adds a proto field, sets `public.*`, regenerates, and gets one coherent result.

**Reason for convention-first binding into `cfg.Public`**: the proto already
defines the browser allowlist. Generating a dedicated `public.*` config subtree
keeps the common case low-boilerplate without requiring manual edits to the
handwritten root config.

**Reason for `config.js` instead of a public RPC**: the browser gets runtime
config synchronously before the SPA boots, with the same path in dev and prod.

**Reason for fail-fast runtime-config loading**: the SPA should not mount with
missing or malformed deployment config. Invalid public config is a startup
error, not a partially recoverable UI state.

## Development Mode

**Decision #23.** Browser connects to the Go server (`:3000`) in development.
The Go server serves `/_gofra/config.js`, handles API routes directly, and
proxies frontend asset and page requests to Vite (`:5173`) for HMR.

```ts
// cmd/app/dev_proxy.go — conceptual route split
switch {
case strings.HasPrefix(path, "/_gofra/"):
    serveRuntimeConfig(w, r)
case strings.HasPrefix(path, "/myapp."):
    apiMux.ServeHTTP(w, r)
default:
    viteProxy.ServeHTTP(w, r)
}
```

Vite still owns HMR and the frontend asset pipeline, but it sits behind the Go
server in development so the browser uses the same origin in dev and prod.

## Production Mode

**Decision #25.** `web/dist/` is compiled into the Go binary via `//go:embed`.
The Go server serves the API, `/_gofra/config.js`, and the compiled SPA from the
same origin. No separate static file server. No CDN required (though one can be
placed in front).

```go
//go:embed all:dist
var Assets embed.FS
```

The SPA handler falls back to `index.html` for client-side routing — any path
that doesn't match a static file serves `index.html` so TanStack Router handles
the route.

## Project Structure

```
web/
├── package.json
├── vite.config.ts
├── tsconfig.json
├── index.html
├── src/
│   ├── main.tsx
│   ├── routes/              # TanStack Router file-based routes
│   │   ├── __root.tsx
│   │   ├── index.tsx
│   │   ├── posts/
│   │   │   ├── index.tsx
│   │   │   └── $slug.tsx
│   │   └── dashboard.tsx
│   ├── components/          # shadcn/ui components
│   │   └── ui/
│   ├── lib/
│   │   ├── transport.ts     # Connect transport config
│   │   ├── runtime-config.ts # Re-export of generated runtime config loader
│   │   ├── auth.ts          # OIDC client
│   │   └── errors.ts        # Error parsing helpers
│   └── gen/                 # Generated (from proto)
│       └── myapp/posts/v1/
│           ├── posts_pb.ts
│           └── posts-PostsService_connectquery.ts
│       └── runtime/
│           ├── runtime_config_pb.ts
│           ├── runtime-config.ts
│           └── runtime-config.global.d.ts
└── dist/                    # Built assets (gitignored, embedded in Go)
```

## Decisions in This Section

| # | Decision | Rationale |
|---|----------|-----------|
| 22 | React + TanStack + shadcn | Largest ecosystem. Connect-Query gives E2E type safety from proto. |
| 23 | Vite | Sub-second HMR. Proxy in dev, embed in prod. |
| 24 | SPA (no SSR) | Decouples frontend and backend. Contract is the proto file. |
| 25 | `embed.FS` for production | Single binary deployment. No separate file server. |
| 29 | No server-side rendering or templates | API-first. Frontend is replaceable. |
| 130 | Public browser runtime config via generated `/_gofra/config.js` loader | Runtime values come from Go without rebuilding the SPA per environment, while staying typed in Go and TS. |
| 131 | Go is the browser entrypoint in dev and proxies Vite | Same browser origin in dev and prod. Vite still provides HMR behind the proxy. |
