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

```
posts.proto
    │
    ├── protoc-gen-es         → web/src/gen/.../posts_pb.ts        (TS message types)
    └── protoc-gen-connect-query → web/src/gen/.../posts_connectquery.ts (TanStack hooks)
```

**Decision #22.** The frontend developer writes `const { data } = useQuery(listPosts, { pageSize: 20 })` and `data` has the exact type of `ListPostsResponse`. No manual type definitions. No `any`. No `fetch` wrappers.

## Transport Configuration

```ts
// web/src/lib/transport.ts
import { createConnectTransport } from "@connectrpc/connect-web";

export const transport = createConnectTransport({
  baseUrl: import.meta.env.VITE_API_URL ?? "",
  useHttpGet: true, // GET for NO_SIDE_EFFECTS RPCs (avoids CORS preflight)
});
```

## Development Mode

**Decision #23.** Browser connects to Vite (`:5173`). Vite serves the SPA with
HMR. Connect RPC calls proxy to the Go server (`:3000`).

```ts
// web/vite.config.ts — proxy configuration
server: {
  port: 5173,
  proxy: {
    "/myapp.": { target: "http://localhost:3000", changeOrigin: true },
  },
},
```

## Production Mode

**Decision #25.** `web/dist/` is compiled into the Go binary via `//go:embed`.
The Go server serves both the API and the SPA from the same origin. No separate
static file server. No CDN required (though one can be placed in front).

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
│   │   ├── auth.ts          # OIDC client (oidc-client-ts)
│   │   └── errors.ts        # Error parsing helpers
│   └── gen/                 # Generated (from proto)
│       └── myapp/posts/v1/
│           ├── posts_pb.ts
│           └── posts-PostsService_connectquery.ts
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
