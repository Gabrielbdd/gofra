# 17 — Decision Log

> Parent: [Index](00-index.md) | Prev: [Testing](16-testing.md) | Next: [V1 Readiness Checklist](18-readiness-checklist.md)
>
> This is the complete list of all architectural decisions. Each decision has
> a number, a short description, and a rationale. Search this document to
> understand why any choice was made.

---

## Foundations (Decisions #1–34, #130–131)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 1 | Go | Compiled binary. Static types. Goroutines. Standard library. | [01](01-foundations.md) |
| 2 | Connect RPC | Type-safe API from proto. Works in browsers. Standard `net/http`. | [01](01-foundations.md) |
| 3 | buf for codegen | Manages proto deps, lints, detects breaking changes. Replaces protoc. | [01](01-foundations.md) |
| 4 | AIP conventions | Consistent naming, pagination, errors. Enforced by linting. | [03](03-api-layer.md) |
| 5 | `{action}_time` fields | AIP-148. `create_time` not `created_at`. One convention. | [03](03-api-layer.md) |
| 6 | Pagination on all List methods | AIP-158. Adding later is breaking. `page_token` is opaque. | [03](03-api-layer.md) |
| 7 | `request_id` as an operation key, not a blanket mutation guarantee | AIP-155. Client-supplied correlation key. Forwarded to Restate idempotency where applicable, but direct Connect-handler mutations are not deduplicated by the framework. | [03](03-api-layer.md) |
| 8 | `FieldMask` on Update | AIP-161. Prevents overwrite with proto zero values. | [03](03-api-layer.md) |
| 9 | `validate_only` on mutations | AIP-163. Enables frontend dry-run validation. | [03](03-api-layer.md) |
| 10 | Soft delete with `delete_time` | AIP-164. Undelete via custom method. | [03](03-api-layer.md) |
| 11 | `etag` for concurrency | AIP-154. Optimistic locking without DB locks. | [03](03-api-layer.md) |
| 12 | Connect error codes | AIP-193. Fixed set. Generic frontend handling. | [09](09-errors.md) |
| 13 | Restate for durable ops | Crash recovery at step level. Replaces queue + scheduler + saga. | [04](04-restate.md) |
| 14 | Restate SDK direct (no wrapper) | One API to learn. Type-safe context hierarchy. | [04](04-restate.md) |
| 15 | Virtual Objects for stateful entities | Per-key single-writer. No DB locks. | [04](04-restate.md) |
| 16 | Workflows for long-running business flows | Suspend on Durable Promise. Resume on external callback or user action. | [04](04-restate.md) |
| 17 | Postgres for domain data | Complex queries, joins, FTS. Restate K/V for operational state. | [05](05-database.md) |
| 18 | Explicit DI via struct fields | No service container. No globals. | [01](01-foundations.md) |
| 19 | Explicit conversion functions between DB rows and API messages | Database structs and proto messages serve different concerns. Keep the mapping explicit. | [02](02-system-architecture.md) |
| 20 | chi router | `net/http` compatible. Connect handlers mount directly. | [02](02-system-architecture.md) |
| 21 | SQL migrations (plain SQL) | SQL is the schema language. No DSL translation layer. | [05](05-database.md) |
| 22 | React + TanStack + shadcn | Largest ecosystem. Connect-Query for E2E type safety. | [13](13-frontend.md) |
| 23 | Vite | Sub-second HMR. Proxy in dev, embed in prod. | [13](13-frontend.md) |
| 24 | SPA (no SSR) | Decouples frontend and backend. Contract is proto. | [13](13-frontend.md) |
| 25 | `embed.FS` for production | Single binary deployment. | [13](13-frontend.md) |
| 26 | mise for tools + tasks | Pins versions. Replaces Makefile. Incremental builds. | [14](14-tooling.md) |
| 27 | gofra CLI for project bootstrap and generators | `gofra new` and generators need Go-aware project structure logic. Tasks are TOML. | [14](14-tooling.md) |
| 28 | Events as map + loop | ~15 lines. Each listener durable via Restate. | [04](04-restate.md) |
| 29 | No server-side rendering or templates | API-first. Frontend is replaceable. | [13](13-frontend.md) |
| 130 | Public browser runtime config via generated `/_gofra/config.js` loader | Runtime browser values come from Go without rebuilding the SPA per environment, while staying typed in Go and TS. | [13](13-frontend.md) |
| 131 | Go is the browser entrypoint in dev and proxies Vite | Same browser origin in dev and prod. Vite still provides HMR behind the proxy. | [13](13-frontend.md) |
| 30 | `RestateRecorder` for handler tests | Fast HTTP tests without Docker. | [16](16-testing.md) |
| 31 | Docker-based Restate integration tests | Durable handlers need real journal. | [16](16-testing.md) |
| 32 | Skip AIP-122 (resource name hierarchy) | Web apps use `id`/`slug`, not `publishers/123/books/456`. | [03](03-api-layer.md) |
| 33 | Skip AIP-151 (long-running operations) | Restate Workflows are strictly more capable. | [03](03-api-layer.md) |
| 34 | Skip AIP-127 (HTTP transcoding) | Connect handles HTTP mapping automatically. | [03](03-api-layer.md) |

## Observability (Decisions #35–47)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 35 | slog for logging | Standard library since Go 1.21. Structured. Context-aware. | [07](07-observability.md) |
| 36 | Custom OTELHandler over otelslog bridge | Enriches stdout logs with trace_id/span_id. Works with existing pipelines. | [07](07-observability.md) |
| 37 | JSON logs in prod, text in dev | JSON for machines. Text for humans. | [07](07-observability.md) |
| 38 | otelconnect interceptor | Per-RPC spans + metrics. Standard OTEL semantic conventions. | [07](07-observability.md) |
| 39 | otelconnect first in interceptor chain | Must wrap auth + validation to capture full request duration. | [07](07-observability.md) |
| 40 | `WithTrustRemote()` for internal services | Connected traces instead of disconnected roots. | [07](07-observability.md) |
| 41 | `WithoutServerPeerAttributes()` | Reduces metric cardinality. | [07](07-observability.md) |
| 42 | `ctx.Log()` in Restate handlers | Suppresses duplicate logs during journal replay. | [07](07-observability.md) |
| 43 | Restate Server exports OTLP | Same collector. Correlated traces via W3C TraceContext. | [07](07-observability.md) |
| 44 | AlwaysSample in dev, ratio-based in prod | See everything locally. Control volume in production. | [07](07-observability.md) |
| 45 | W3C TraceContext propagator | Standard. Restate and Connect both use it. | [07](07-observability.md) |
| 46 | `gofra.` metric prefix | Distinguish app metrics from otelconnect and Restate metrics. | [07](07-observability.md) |
| 47 | Jaeger in dev via mise task | One command for local trace viewing. | [07](07-observability.md) |

## Database (Decisions #48–57)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 48 | sqlc over custom query builder | SQL validated at generation time. Zero framework code. | [05](05-database.md) |
| 49 | goose over custom migrator | Battle-tested, embeddable, Go migrations, out-of-order. | [05](05-database.md) |
| 50 | sqlc reads goose migrations as schema | One source of truth. No separate schema.sql. | [05](05-database.md) |
| 51 | Migrations embedded via `//go:embed` | Binary carries its own migrations. Single-artifact deployment. | [05](05-database.md) |
| 52 | Auto-migrate opt-in, not default | Multiple replicas racing on CREATE TABLE. Safe for dev, risky for prod. | [05](05-database.md) |
| 53 | pgx/v5 as SQL driver | Best Postgres performance. Native type support. | [05](05-database.md) |
| 54 | Explicit conversion functions (not methods on generated types) | Generated code stays untouched. Conversion in the rpc package. | [05](05-database.md) |
| 55 | No dynamic query builder | sqlc.narg() + COALESCE for optional filters. Raw SQL for truly dynamic. | [05](05-database.md) |
| 56 | Transactions via WithTx + explicit Begin/Commit | No hidden middleware. Every boundary visible. | [05](05-database.md) |
| 57 | Seeds via goose --no-versioning | SQL seeds, applied without version tracking. | [05](05-database.md) |

## Configuration (Decisions #58–66, #132)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 58 | koanf over viper | No forced lowercasing. Modular deps. Correct merge semantics. | [06](06-configuration.md) |
| 59 | YAML over TOML | Supports comments. Familiar from Docker/K8s ecosystem. | [06](06-configuration.md) |
| 60 | Four-layer precedence: defaults → YAML → env → flags | 12-factor app standard. | [06](06-configuration.md) |
| 61 | `GOFRA_` prefix for env vars | Prevents collisions with platform vars like PORT. | [06](06-configuration.md) |
| 62 | Single gofra.yaml (no per-environment files) | Env vars handle deployment config. Per-env files drift. | [06](06-configuration.md) |
| 63 | Typed struct, not k.String() calls | Compile-time checking. Single place for all options. | [06](06-configuration.md) |
| 64 | No global config singleton | Config passed from main(). Same DI pattern as everything else. | [06](06-configuration.md) |
| 65 | Manual validation over struct tags | Startup-time concern. Simple rules. 10-line function. | [06](06-configuration.md) |
| 66 | Secrets only via env vars | YAML is in version control. Secrets in VCS = security incident. | [06](06-configuration.md) |
| 132 | Generated public runtime config for the browser | Browser gets an explicit proto-defined safe subset at `/_gofra/config.js`, loaded from generated `public.*` config and emitted as typed Go and TS APIs. | [06](06-configuration.md) |
| 133 | Double-underscore env var nesting (`GOFRA_APP__PORT`) | Single-underscore transform is ambiguous for keys with underscores (`auto_migrate`, `client_id`). `__` is industry standard (Docker Compose, .NET). | [06](06-configuration.md) |
| 134 | Generic `runtimeconfig.Load[T]` in the framework | Config loading is framework logic, not app logic. Shipping 160 lines of loading boilerplate into every scaffold prevents bug propagation and requires manual wiring for each new field. | [06](06-configuration.md) |
| 135 | Proto-driven config generation (`gofra generate config`) | One proto file is the single source of truth for config types, defaults, descriptions, and public/private separation. Go structs, flag registration, and public config wiring are generated. | [06](06-configuration.md) |
| 136 | Typed defaults via gofra.config.v1 proto annotations | Proto3 has no field defaults. Only two custom options: `default_value` (typed oneof) and `secret`. Descriptions use proto comments, validation uses buf/validate. | [06](06-configuration.md) |
| 137 | Two top-level config fields: `app` and `public` | Server-side config nests under `app`. Browser-safe config lives under `public`. The `public` subtree convention determines what gets served at `/_gofra/config.js`. | [06](06-configuration.md) |

## Auth & Authz (Decisions #67–79, #124–129)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 67 | Couple with Zitadel for identity | Go-native, Connect RPC API, single binary, multi-tenant, OIDC certified. | [08](08-auth.md) |
| 68 | Direct OIDC Authorization Code flow for browser and native clients | One auth family across human clients. Browser uses PKCE as a public client. Native clients use PKCE with platform redirects. | [08](08-auth.md) |
| 69 | Stateless JWT validation via JWKS | No per-request call to Zitadel. Keys cached and auto-rotated. | [08](08-auth.md) |
| 70 | OIDC discovery + local JWT verification for access tokens | Uses Zitadel discovery metadata and cached JWKS keys. | [08](08-auth.md) |
| 71 | Roles in Zitadel, permissions in Go | Clean separation. Zitadel manages assignment, app defines meaning. | [08](08-auth.md) |
| 72 | Static role→permission map | Small, testable, version-controlled. Can move to DB later. | [08](08-auth.md) |
| 73 | Resource-level authz in RPC handlers | Requires loading the resource. Can't check in interceptor. | [08](08-auth.md) |
| 74 | JIT user profile creation | No sync. Profile created on first API call. | [08](08-auth.md) |
| 75 | `zitadel_user_id TEXT` as PK | Zitadel IDs are opaque strings. Direct PK avoids surrogate. | [08](08-auth.md) |
| 76 | Admin handlers proxy to Zitadel | SPA doesn't get admin credentials. Gofra enforces authz. | [08](08-auth.md) |
| 77 | No BFF/session-cookie default for browser clients | Keep the v1 surface smaller and backend validation consistent. Revisit later if needed. | [08](08-auth.md) |
| 78 | `urn:zitadel:iam:org:projects:roles` scope | Includes roles in token. No extra API call. | [08](08-auth.md) |
| 79 | Frontend permission checks display-only | Server always re-checks. Never trust the client. | [08](08-auth.md) |
| 124 | `react-oidc-context` as the default React auth layer | One supported frontend integration for the generated SPA. Avoids multiple auth stacks in docs and generators. | [08](08-auth.md) |
| 125 | `sessionStorage` for browser token storage | Survives normal reloads without persisting as broadly as `localStorage`. | [08](08-auth.md) |
| 126 | `offline_access` + rotating refresh tokens with one bootstrap refresh attempt | Keeps the browser session usable without hidden retry loops or full redirects on every expiry. | [08](08-auth.md) |
| 127 | Logout clears local auth state before Zitadel end-session redirect | Fail closed if network logout is interrupted. | [08](08-auth.md) |
| 128 | Routes and RPCs private by default | Public routes and RPCs must be explicitly allowlisted. | [08](08-auth.md) |
| 129 | Native clients use system browser redirects and OS secure storage | Same OIDC family as the browser, with platform-native token handling. | [08](08-auth.md) |

## CORS (Decisions #80–87)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 80 | `connectrpc.com/cors` for header lists | Official package. Tracks protocol changes. | [10](10-cors.md) |
| 81 | `rs/cors` for middleware | Most widely used. Standard `net/http` middleware. | [10](10-cors.md) |
| 82 | `AllowCredentials: false` for the default bearer-token SPA flow | Browser auth uses `Authorization` headers, not cookies. Preflight still happens, but credential mode is not required. | [10](10-cors.md) |
| 83 | Explicit origins even without cookie auth | Bearer-token CORS can technically use `*`, but Gofra keeps an explicit allowlist for a clearer browser contract. | [10](10-cors.md) |
| 84 | Explicit allowed origins in config | Standard Gofra dev is same-origin through Go. Separate browser origins must be listed explicitly. | [10](10-cors.md) |
| 85 | CORS middleware first in chain | Preflight OPTIONS must be handled before routing. | [10](10-cors.md) |
| 86 | `MaxAge: 7200` | Chrome's maximum. Reduces preflight requests. | [10](10-cors.md) |
| 87 | `idempotency_level = NO_SIDE_EFFECTS` for reads | Enables Connect GET. Avoids CORS preflight. | [10](10-cors.md) |

## Health Checks (Decisions #88–95)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 88 | Three separate endpoints (startup/live/ready) | Different questions, different failure consequences. | [11](11-health-checks.md) |
| 89 | Plain HTTP routes, not Connect RPC | Kubernetes and load balancers use plain HTTP GET. | [11](11-health-checks.md) |
| 90 | Liveness checks nothing except process responsiveness | Checking deps causes thundering herd restarts. | [11](11-health-checks.md) |
| 91 | Readiness checks database + Restate | If unreachable, stop routing traffic until recovery. | [11](11-health-checks.md) |
| 92 | Health endpoints before middleware | Must not go through CORS, auth, or rate limiting. | [11](11-health-checks.md) |
| 93 | 2-second timeout on dependency checks | Prevents goroutine pile-up. | [11](11-health-checks.md) |
| 94 | JSON response bodies | For human debugging. Orchestrators use status codes. | [11](11-health-checks.md) |
| 95 | `successThreshold: 2` on readiness | Prevents flapping. | [11](11-health-checks.md) |

## Error Handling (Decisions #96–105)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 96 | Connect error codes only, no custom codes | Standard set. Same as gRPC and AIP-193. | [09](09-errors.md) |
| 97 | `runtimeerrors.NotFound()`, `runtimeerrors.Internal()` helpers | Consistent construction with proper error details. Package naming follows `runtime*` convention. | [09](09-errors.md) |
| 98 | `Internal(ctx, err)` logs but hides original error | Security. No stack traces reaching clients. Accepts context for trace_id correlation. Callers must not double-log. | [09](09-errors.md) |
| 99 | `BadRequest` with `FieldViolation` for app validation; protovalidate uses its own `Violations` detail | Google's standard type for app-level validation. Protovalidate's native type differs — normalize later or handle both in frontend. | [09](09-errors.md) |
| 100 | Restate: terminal vs retryable framework | "Will retrying fix this?" Terminal for logic errors. | [09](09-errors.md) |
| 101 | No custom failed-jobs dashboard | Restate UI + admin API + OTEL traces cover this. | [09](09-errors.md) |
| 102 | Generate error_details.proto for frontend | Enables `findDetails(BadRequestSchema)` in TypeScript. | [09](09-errors.md) |
| 103 | Transport interceptor for global error handling | Auth expiry, server unavailable. Handled once. | [09](09-errors.md) |
| 104 | `connect.WithRecover` for RPC panic recovery | Correct for Connect, gRPC, and gRPC-Web protocols. HTTP middleware only for non-RPC routes. | [09](09-errors.md) |
| 105 | Warn for validation, Debug for NotFound | Spikes indicate bugs. NotFound is normal navigation. | [09](09-errors.md) |

## Docker Compose (Decisions #106–114)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 106 | App on host, infra in Docker | Hot reload for app. Reproducible versions for infra. | [15](15-docker-compose.md) |
| 107 | Single Postgres for app + Zitadel | Saves resources. Separate databases in one instance. | [15](15-docker-compose.md) |
| 108 | Restate single-node in dev | Cluster features not needed for development. | [15](15-docker-compose.md) |
| 109 | Jaeger behind `--profile tracing` | Not every session needs tracing. | [15](15-docker-compose.md) |
| 110 | Named volumes for persistence | `down` keeps data. `down -v` for clean reset. | [15](15-docker-compose.md) |
| 111 | `host.docker.internal` for Restate → host | Restate in Docker must reach Go on host :9080. | [15](15-docker-compose.md) |
| 112 | Separate `infra` and `dev` mise tasks | Infra starts once. App restarts frequently. | [15](15-docker-compose.md) |
| 113 | ~~Manual Zitadel bootstrap~~ (superseded by #150) | Chicken-and-egg for API automation. 2-min manual setup. | [15](15-docker-compose.md) |
| 114 | `.env` for machine-specific values | Zitadel client IDs differ per developer. Not in git. | [15](15-docker-compose.md) |

## Graceful Shutdown (Decisions #115–123)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 115 | `signal.NotifyContext` for SIGTERM/SIGINT | Standard Go pattern. Context-based cancellation. | [12](12-graceful-shutdown.md) |
| 116 | Four-phase: drain → HTTP → Restate → resources | HTTP clients wait. Restate invocations are durable. DB closes last. | [12](12-graceful-shutdown.md) |
| 117 | 2-second readiness drain delay | LB deregistration race. Prevents connection-refused errors. | [12](12-graceful-shutdown.md) |
| 118 | `http.Server.Shutdown()` for HTTP | Standard library. Stops accepting, waits for in-flight, respects deadline. | [12](12-graceful-shutdown.md) |
| 119 | Context cancellation for Restate endpoint | `rs.Start(ctx, addr)` respects context cancellation. | [12](12-graceful-shutdown.md) |
| 120 | Restate invocations safe to interrupt | Completed steps journaled. Interrupted invocations retried on another instance. | [12](12-graceful-shutdown.md) |
| 121 | 25-second budget within 30-second K8s grace | 5-second safety margin before SIGKILL. | [12](12-graceful-shutdown.md) |
| 122 | Second signal force-kills | `stop()` restores default. Ctrl+C twice = immediate exit. | [12](12-graceful-shutdown.md) |
| 123 | `OnShutdown` callback for resource cleanup | OTEL flush + DB close. Framework provides hook, main.go provides logic. | [12](12-graceful-shutdown.md) |

## Error Handling Addendum (Decisions #124–125)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 124 | `InvalidArgument` takes `...FieldViolation` not `map[string]string` | Preserves field order, allows multiple violations per field. Matches Rails/Laravel/Phoenix DX. | [09](09-errors.md) |
| 125 | `FailedPrecondition` helper included in first cut | Listed in error code table, handlers need it immediately (e.g., publish draft without body). | [09](09-errors.md) |

## Testing (Decisions #133)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 133 | Runtime config tested at generator, handler, and frontend-loader boundaries | Public browser config spans Go, generated code, and SPA startup. One test layer is not enough. | [16](16-testing.md) |

## Framework Development (Decisions #134, #138)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 134 | Framework repo has three surfaces: one public CLI, public runtime packages, and a canonical starter | Keeps tooling, app-facing library code, and generated-app contract distinct while preserving one versioned repo and module during the early phase. | [02](02-system-architecture.md) |
| 138 | Scaffold output contains no generated code | Generated files are derived artifacts that couple `gofra new` to every generator. Config gen moves to `mise run generate` in the generated app. | [14](14-tooling.md) |

## Starter Infra Slice (Decisions #139–142)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 139 | Generated apps use root `compose.yaml` | Canonical Compose file name. Works with modern Docker Compose and Podman Compose without a Docker-specific file name. | [15](15-docker-compose.md) |
| 140 | Pin starter Postgres to `postgres:18.3-alpine3.23` | Avoid floating `latest` while staying on the current official stable image line with a small Alpine base. | [15](15-docker-compose.md) |
| 141 | `scripts/compose.sh` auto-detects Docker vs Podman compose providers | One `mise run infra` command surface across the common local container engines. | [14](14-tooling.md) |
| 142 | `scripts/load-env.sh` derives one local DB contract for Compose, goose, and app runtime | Prevents config drift between `compose.yaml`, migration tasks, and `go run ./cmd/app`. | [14](14-tooling.md) |

## Public Distribution (Decisions #143–146)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 143 | Canonical module path is `github.com/Gabrielbdd/gofra` | The framework is published on GitHub; the module path must match so apps resolve through the Go proxy. Previous internal path `databit.com.br/gofra` was migrated in full in v0.1.0. | [14](14-tooling.md) |
| 144 | Starter ships `Dockerfile`, `.dockerignore`, and `.github/workflows/ci.yml` by default | Day-1 deployability is a framework promise: a freshly generated app must be runnable, buildable as a binary, buildable as an image, and testable in CI without extra tooling decisions. | [14](14-tooling.md) |
| 145 | Starter Dockerfile uses multi-stage build with `gcr.io/distroless/static-debian12:nonroot` as runtime base | Static binary, no shell, non-root by default, minimal attack surface. Works without CGO. | [14](14-tooling.md) |
| 146 | Generated apps depend on a published Gofra release through the Go module proxy — no `replace` directive in `go.mod.tmpl` | A generated app must build reproducibly from its own `go.mod` alone, without depending on the filesystem layout that created it. Any local override for simultaneous framework development is the caller's responsibility, not a framework-committed coupling. | [14](14-tooling.md) |

## Public Documentation Site (Decisions #147–149)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 147 | MkDocs + Material for the public documentation site | Content is already Markdown under `docs/framework/` and already Diataxis-organized. MkDocs builds a static site with zero frontend app, and Material provides search, navigation, and code-copy out of the box. No runtime dependency is added to the framework itself. | [documentation-system](project/documentation-system.md) |
| 148 | GitHub Pages deployment via GitHub Actions on `main` | The repo is already public on GitHub. Actions + Pages removes any separate hosting contract, keeps deploys reproducible from source, and requires no external credentials beyond the repo itself. PRs run `mkdocs build --strict` to catch broken links and dangling references before merge. | [documentation-system](project/documentation-system.md) |
| 149 | The public site publishes only tutorials and reference in v0 — no stubs, no placeholders | The cost of publishing empty How-to and Explanation sections is worse than not publishing them: readers click, find nothing, and lose trust. How-to and Explanation enter the nav only when each has real content that reflects behavior shipping today. | [documentation-system](project/documentation-system.md) |

## Identity & Frontend Default (Decisions #150–153)

| # | Decision | Rationale | Doc |
|---|----------|-----------|-----|
| 150 | ZITADEL is a default service in the starter `compose.yaml`; runtime auth stays opt-in | Identity is a standard dependency of the apps Gofra targets. Shipping ZITADEL alongside Postgres removes a setup step and lets consumer apps treat ZITADEL as a ready neighbor. The Go binary keeps its opt-in JWT middleware (installs only when `auth.issuer` and `auth.audience` are set) so a fresh starter still runs without any ZITADEL configuration. Supersedes #113. | [15](15-docker-compose.md) |
| 151 | Starter frontend is Vite + React 19 + TanStack Router/Query/Form + Tailwind v4 + shadcn/ui, with no lint or formatting tool by default | Matches Gofra's documented v1 stack in [13](13-frontend.md). Copying shadcn/ui into the tree (rather than depending on it) keeps customization cheap. Skipping Biome/Prettier/ESLint by default avoids imposing a style choice on every consumer app — those can be added per project. | [13](13-frontend.md) |
| 152 | TS runtime-config types are emitted by `gofra generate config -ts-out`, from the same parser that emits Go | One proto, one parser, two languages — the frontend and backend cannot drift on public config shape. Runtime schema validation is intentionally out of scope (consumers can pair with a proto-es pipeline). | [framework/reference/cli/generate-config.md](framework/reference/cli/generate-config.md) |
| 153 | `runtime/zitadel` + `runtime/zitadel/secret` are consumer-facing helper packages; the starter never imports them | The framework ships the ZITADEL service but not any hard dependency on it at runtime. Consumer apps that need to call ZITADEL management APIs opt into the helpers explicitly. Keeps smoke:new Go-only and the starter binary free of opinionated provisioning flows. | [framework/reference/runtime/zitadel.md](framework/reference/runtime/zitadel.md) |
