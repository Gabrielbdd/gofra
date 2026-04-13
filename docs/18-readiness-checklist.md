# 18 — V1 Readiness Checklist

> Parent: [Index](00-index.md) | Prev: [Decision Log](17-decision-log.md)
>
> This document defines which promises Gofra should keep for v1, which claims
> should be softened or removed, and what must be true before calling the
> framework usable in production.

---

## Why This Document Exists

Gofra's design docs are broad. That is useful during exploration, but it is not
enough for a v1 release. A usable framework needs a smaller set of promises
that are actually solved end to end.

This checklist is the scope authority for v1:

- If another doc promises more than this checklist, the checklist wins for v1.
- If two docs conflict, use this checklist to decide which contract to keep.
- A feature is not "part of Gofra v1" because it appears in an example. It is
  part of v1 only if this document keeps it and the supporting docs make it
  operationally credible.

---

## V1 Promises Worth Keeping

Gofra v1 should keep these promises and solve them fully:

- Gofra is an API-first Go framework built around Connect RPC, PostgreSQL,
  Restate, Zitadel, and a default React SPA.
- API contracts are defined in `proto/` and generate both Go server types and
  TypeScript client types.
- Durable background work and long-running workflows are first-class through
  Restate, with clear handler patterns and operational visibility.
- Authentication is delegated to Zitadel. Authorization is enforced in Gofra.
- Production deployment is one application binary plus required infrastructure,
  with documented health checks, shutdown, configuration, and observability.
- Local development is reproducible with pinned tools, one default workflow,
  and minimal manual guesswork.

These are strong enough to differentiate the framework and narrow enough to
implement coherently.

---

## Promises To Soften Or Drop For V1

Gofra v1 should not promise the following until they are actually solved:

- End-to-end exactly-once behavior for mutating requests.
- Multi-tenant application data isolation by default.
- A framework-owned ORM or query builder.
- Multiple first-class auth/session models for the SPA.
- Fine-grained external policy engines as part of the core architecture.
- "Batteries included" coverage for every adjacent subsystem such as uploads,
  rate limiting, search backends, or webhook ecosystems.

These may still appear as extension points or future work, but they should not
be part of the v1 product claim.

---

## Release-Critical Checklist

The following items are release blockers for a credible v1.

### 1. One Coherent Architecture Contract

- [x] Standardize the docs on `Connect + sqlc + Restate + Zitadel + React SPA`.
- [x] Remove stale references to the old query builder, ORM language,
  server-side templating, and obsolete workflow examples.
- [x] Make generated-code examples match the kept architecture exactly.
- [x] Ensure the project structure, handler examples, and test examples all use
  the same package layout and dependency story.

**Why this is release-critical**: Gofra is still documentation-first. If the
docs disagree, the framework contract itself is unstable.

**Progress**: Completed by the architecture-alignment pass that normalized the
core system, API, Restate, database, shutdown, and decision-log docs on the
same `Connect + sqlc + Restate + Zitadel + React SPA` contract. The
architecture docs now also distinguish the repo's three surfaces: the public
`gofra` CLI, public runtime packages, and starter-owned generated app files,
with `internal/scaffold/starter/full/` documented as the canonical current
starter source and `internal/scaffold` / `internal/generate` recorded as the
internal direction.

### 2. One Auth Model

- [x] Choose one SPA auth default and document it precisely.
- [x] Define token storage, refresh behavior, logout behavior, and route
  protection rules.
- [x] Add the missing canonical config for auth to the main config document.
- [x] Define which endpoints are public, which require authentication, and how
  admin access flows through the backend to Zitadel.

**Why this is release-critical**: auth is not a plugin in Gofra's story. It is
one of the framework's core opinions.

**Progress**: Completed. Gofra now has one explicit auth model: direct OIDC
Authorization Code flow on all human clients, with browser-specific defaults
for `sessionStorage`, refresh handling, logout, route protection, and
private-by-default RPCs, plus a canonical `auth` config block in the main
configuration document.

### 3. Explicit Mutation Boundary

- [x] Replace vague `request_id` claims with an explicit boundary.
- [x] State clearly that Gofra v1 does not provide general request-layer
  deduplication for direct Connect-handler mutations.
- [x] State clearly that `request_id` has framework-defined idempotency
  semantics only when the mutation is handed off to Restate.
- [ ] Define the preferred synchronous and asynchronous patterns for mutations
  that require Restate-owned retry-safe execution.
- [ ] Add tests and examples that distinguish direct handler mutations from
  Restate-owned mutation flows.

**Why this is release-critical**: the mutation boundary is part of the
framework promise. If Gofra implies retry safety in places where it does not
exist, applications will ship duplicate writes and unsafe crash behavior.

**Progress**: The docs now make one architectural claim: Restate owns retry and
deduplication semantics for work it executes; direct Connect-handler mutations
do not get framework-level idempotency. What remains open is documenting the
canonical patterns and tests for putting mutation-critical paths behind that
boundary.

### 4. Explicit Tenancy Decision

- [ ] Decide whether Gofra v1 is single-tenant or multi-tenant by default.
- [ ] If single-tenant, remove multi-tenant positioning from the product story.
- [ ] If multi-tenant, define tenant keys, query scoping, authz boundaries,
  uniqueness rules, and test coverage for isolation.

**Why this is release-critical**: Zitadel organizations do not automatically
create safe application-level tenant isolation.

### 5. Request-Layer To Restate Boundary

- [ ] Preserve request context when dispatching to Restate.
- [ ] Preserve trace context and cancellation semantics.
- [ ] Define when dispatch errors fail the request versus being logged and
  retried elsewhere.
- [ ] Make helper APIs use the same error-handling and context rules as the raw
  ingress client.

**Why this is release-critical**: the Connect-to-Restate bridge is one of the
framework's main differentiators. It must be correct, not just convenient.

### 6. Truthful Startup And Shutdown Semantics

- [x] Make startup readiness reflect real listener bind success and dependency
  readiness.
- [x] Define the precise contract for health endpoints and shutdown drains.
- [ ] Document how Restate endpoint lifecycle interacts with application
  readiness and rolling deploys.

**Why this is release-critical**: broken lifecycle semantics produce bad
rollouts even when the application code is correct.

**Progress**: `runtime/health` provides startup/liveness/readiness probes with
generic `CheckFunc` checks. `runtime/serve` owns `net.Listen` and marks
startup ready only after bind succeeds, then runs a three-phase shutdown
(readiness drain → HTTP shutdown → resource cleanup). The starter mounts
health endpoints on a root `http.ServeMux` structurally outside the chi app
router. Restate lifecycle integration is still open — it will add a fourth
shutdown phase when the Restate package lands.

### 7. Production Security Baseline

- [ ] Add webhook signature verification and replay-protection guidance.
- [ ] Decide whether rate limiting is in scope for v1. If yes, define it. If
  not, remove implied claims.
- [ ] Define the minimal secret-handling, TLS, and admin-credential guidance
  needed for real deployments.

**Why this is release-critical**: these are common failure points for modern web
applications and they sit in Gofra's claimed area of responsibility.

### 8. Reproducible Tooling And Local Workflow

- [ ] Pin tool versions and container images instead of relying on `latest`.
- [ ] Make the local Restate registration story use one correct endpoint.
- [ ] Keep `mise`, generators, Docker Compose, and config examples in sync.

**Why this is release-critical**: "works on a clean machine" is a baseline
requirement for framework usability.

**Progress**: The config DX is now proto-driven. A single
`proto/<app>/config/v1/config.proto` file defines the full config schema with
typed defaults (`gofra.config.v1.field` annotations) and secret marking.
`gofra generate config` produces Go structs, flag registration, loading, and
public config wiring — the starter ships zero hand-written config code.
`gofra new` does a pure file copy; `mise run generate` handles code generation;
the developer workflow is `gofra new myapp && cd myapp && mise trust && mise run dev`.
Browser config comes from `/_gofra/config.js` via the `public`
subtree convention. Tool/version pinning and Restate endpoint cleanup are
still open.

### 9. Operational Baseline

- [ ] Define the minimum logs, traces, metrics, and alerts required in
  production.
- [ ] Add runbooks for migrations, rollout, rollback, backup/restore, and
  durable job failure inspection.
- [ ] State what Gofra expects the operator to provide versus what the framework
  provides directly.

**Why this is release-critical**: operational clarity is part of the framework's
value proposition, not an afterthought.

### 10. Test Matrix For The Promises Kept

- [ ] Keep unit tests, Connect handler tests, and Restate integration tests.
- [ ] Add tests for auth refresh/logout, direct-vs-Restate mutation semantics,
  async failure handling, workflow compensation, and migration safety.
- [ ] If multi-tenancy is kept, add tenancy isolation tests before release.

**Why this is release-critical**: Gofra should only promise behavior it can
verify repeatedly.

**Progress**: The runtime-config feature now has a documented test shape across
generator output, Go resolver/handler behavior, and frontend loader/bootstrap
behavior. That test shape now also covers the generated `public.*` config
subtree and the binder from `cfg.Public` to the runtime proto. The initial
scaffold includes basic Go tests for the shared runtime-config resolver,
handler, and generator renderers. The broader auth, mutation-boundary,
workflow, and tenancy matrix is still open.

---

## Suggested Order Of Work

Do the work in this order:

1. Stabilize the docs contract around one architecture.
2. Choose and document the auth model.
3. Choose and document the tenancy scope.
4. Define the mutation boundary and the preferred Restate-owned safe mutation
   patterns.
5. Fix config, health, and shutdown semantics.
6. Lock down tooling reproducibility.
7. Add production security and ops guidance.
8. Expand the test matrix to cover the promises that remain.

This order is deliberate. It avoids building helper code on top of unresolved
contracts.

---

## Ship Gate

Gofra v1 is not ready to ship until all of the following are true:

- [ ] A new application can be scaffolded and run locally without undocumented
  decisions.
- [ ] The documentation describes one coherent framework, not multiple
  competing designs.
- [ ] Auth works end to end with one documented refresh and logout story.
- [ ] Mutation safety semantics are explicit: direct handler mutations are not
  promised idempotent, and Restate-owned mutation flows have documented retry
  and deduplication behavior.
- [ ] A durable workflow can succeed, retry, fail terminally, and be inspected
  operationally.
- [ ] Production deployment has a credible runbook for startup, migration,
  shutdown, rollback, and failure inspection.

If one of these is false, the framework is still in design or pre-release
hardening, not "usable".

---

## Deferred Until After V1

These are reasonable follow-up tracks once the core contract is stable:

- Runtime-configurable permission models.
- First-class multi-tenant application primitives, if not included in v1.
- Richer admin tooling around Zitadel.
- Additional frontend templates or alternate frontend stacks.
- Optional integrations for search engines, object storage, and policy engines.

The right v1 is smaller and more coherent than the maximal design space.
