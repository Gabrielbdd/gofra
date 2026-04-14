# Deferred Auth/Authz MVP Proposal

This note captures the current proposed implementation shape for Gofra's
authentication and authorization runtime packages. It exists so maintainers can
refer back to one concrete proposal later without treating the API as finalized
or ready to ship in v1 today.

Status: deferred. This is a design note, not a committed public contract.

Related design docs:

- [08-auth.md](../08-auth.md) — source of truth for the high-level auth model
- [19-implementation-gaps.md](../19-implementation-gaps.md) — tracks what is
  still missing in code
- [17-decision-log.md](../17-decision-log.md) — durable auth decisions already
  accepted

## Summary

The proposed v1 implementation should stay intentionally small:

- `runtime/auth` owns bearer-token validation, auth context propagation, and
  private-by-default Connect RPC enforcement.
- `runtime/authz` owns only reusable authorization mechanics such as compiled
  role-to-permission lookups and decision helpers.
- Generated apps own permission constants, role-permission maps, policy
  structs, and resource-level checks.
- Postgres stores authorization facts such as ownership, membership, and
  sharing. It does not store the framework's default global role-permission
  model.

The practical effect is simple: Gofra should ship a minimal authz API similar
in spirit to Laravel's core policies, not a runtime-managed permission system
similar to Spatie's database-backed package.

## Goals

- Ship a usable default auth/authz API without adding another always-on
  service.
- Keep the generated app story explicit and easy to debug.
- Scale cleanly across multiple app instances.
- Preserve room for future integrations such as introspection, OpenFGA, or
  Cedar without forcing them into the core v1 path.

## Non-Goals

- Runtime-editable global permissions.
- Framework-managed `roles`, `permissions`, or `user_roles` tables.
- A Zanzibar-style authz service in core.
- Automatic policy discovery, reflection, or magic registries.
- Postgres Row Level Security as the framework default.

## Proposed `runtime/auth`

`runtime/auth` should be a transport/security package, not a ZITADEL SDK
wrapper.

### Responsibilities

- Extract bearer tokens from requests.
- Validate access tokens.
- Normalize the authenticated user into a small `User` struct.
- Attach that user to `context.Context`.
- Enforce "private by default" for Connect procedures.

### Public Shape

```go
package auth

type User struct {
	ID    string
	Email string
	OrgID string
	Roles []string
}

func WithUser(ctx context.Context, user User) context.Context
func UserFromContext(ctx context.Context) (User, bool)

type Verifier interface {
	Verify(ctx context.Context, raw string) (User, error)
}

func NewJWTVerifier(issuerURL, audience string, opts ...Option) (Verifier, error)
```

### Validation Mode

The framework should standardize on local JWT validation for v1:

- OIDC discovery to locate metadata
- JWKS-backed signature validation
- issuer, audience, expiry, and token-type checks
- no per-request network call to ZITADEL

Important contract: the protected API application in ZITADEL must issue JWT
access tokens. If the application is configured for opaque access tokens, a
JWKS-only verifier will not work correctly.

Because that token-type assumption is operationally important, Gofra should
document it explicitly and fail clearly when the configured app cannot satisfy
the contract.

### Integration Point

Prefer Connect authentication middleware that runs before request body decoding
and supports both unary and streaming RPCs. A plain unary interceptor is
acceptable as the initial implementation shape if the middleware option is not
yet wired, but the package boundary should not depend on unary-only behavior.

### Public Procedures

Connect procedures stay private by default. Public procedures are an explicit
allowlist.

The initial implementation can use a plain procedure matcher or generated map.
Longer term, this should become generated metadata rather than a handwritten
string table.

## Proposed `runtime/authz`

`runtime/authz` should stay helper-only. It should not own application
permissions or policy meaning.

### Responsibilities

- Compile role-to-permission maps into fast lookup structures.
- Answer `Has`, `HasAny`, and `HasAll`.
- Represent authorization decisions in a reusable way.
- Provide a small helper to turn decisions into errors.

### Public Shape

```go
package authz

type Compiled[P comparable] struct { /* ... */ }

func Compile[P comparable](defs map[string][]P) (Compiled[P], error)
func MustCompile[P comparable](defs map[string][]P) Compiled[P]

func (c Compiled[P]) Has(roles []string, perm P) bool
func (c Compiled[P]) HasAny(roles []string, perms ...P) bool
func (c Compiled[P]) HasAll(roles []string, perms ...P) bool

type Decision struct { /* ... */ }

func Allow() Decision
func Deny(reason string) Decision
func DenyAsNotFound(reason string) Decision
func Authorize(d Decision) error
```

This API is intentionally small. Most application code should not depend on the
generic helper directly; it should use app-owned wrappers and policy methods.

## App-Owned Authorization Layer

Generated applications should own:

- the `Permission` type
- permission constants
- the role-to-permission map
- policy structs such as `PostPolicy`
- resource-specific helpers such as `CanEditPost`

Example:

```go
// app/authz/permissions.go
type Permission string

const (
	PermPostCreate Permission = "post.create"
	PermPostEdit   Permission = "post.edit"
	PermUserManage Permission = "user.manage"
)

var permissions = runtimeauthz.MustCompile(map[string][]Permission{
	"admin": {
		PermPostCreate,
		PermPostEdit,
		PermUserManage,
	},
	"editor": {
		PermPostCreate,
		PermPostEdit,
	},
	"viewer": {},
})

func HasPermission(roles []string, perm Permission) bool {
	return permissions.Has(roles, perm)
}
```

```go
// app/authz/posts.go
type PostPolicy struct{}

func (PostPolicy) Update(user auth.User, post Post) runtimeauthz.Decision {
	if post.OrgID != user.OrgID {
		return runtimeauthz.DenyAsNotFound("post not found")
	}

	if HasPermission(user.Roles, PermUserManage) {
		return runtimeauthz.Allow()
	}

	if !HasPermission(user.Roles, PermPostEdit) {
		return runtimeauthz.Deny("missing post.edit permission")
	}

	if post.AuthorUserID != user.ID {
		return runtimeauthz.Deny("can only edit own posts")
	}

	return runtimeauthz.Allow()
}
```

This gives Gofra a Laravel-like policy model without adding a registry,
reflection, or database-managed permission subsystem.

## Persistence Model

The database should store authorization facts, not application policy.

### What Postgres Should Store

- local user projection rows for joins, ownership, and audit
- org or tenant identifiers on tenant-scoped resources
- ownership columns such as `author_user_id`
- relationship facts such as collaborators, memberships, and shares
- audit logs when needed

Example tables:

```sql
CREATE TABLE app_users (
  zitadel_user_id TEXT PRIMARY KEY,
  email TEXT NOT NULL,
  display_name TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen_at TIMESTAMPTZ
);
```

```sql
CREATE TABLE posts (
  id BIGSERIAL PRIMARY KEY,
  org_id TEXT NOT NULL,
  author_user_id TEXT NOT NULL REFERENCES app_users(zitadel_user_id),
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  status TEXT NOT NULL,
  is_public BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

```sql
CREATE TABLE post_collaborators (
  post_id BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES app_users(zitadel_user_id),
  role TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (post_id, user_id)
);
```

### What Postgres Should Not Store By Default

- framework-managed `roles`
- framework-managed `permissions`
- framework-managed `role_permissions`
- framework-managed `user_roles`

Those tables duplicate either ZITADEL role assignment or app-owned policy code
and create unnecessary synchronization problems for the default framework path.

### Runtime Decision Flow

The expected request flow is:

1. Validate the bearer access token.
2. Extract `user_id`, `email`, `org_id`, and `roles`.
3. Optionally upsert the local user projection.
4. Check coarse permission in app code.
5. Load resource facts from Postgres.
6. Apply resource-level policy.

This keeps coarse policy in code and dynamic resource facts in the database.

### RLS Position

Postgres Row Level Security should not be the framework default. It can be a
valid opt-in for specific applications later, but it introduces hidden behavior
and a more complex debugging story than Gofra should adopt as the baseline.

## Comparison To Laravel

The closest Laravel core concepts are:

- Laravel Gate -> explicit helper or app-owned wrapper
- Laravel Policy -> app-owned policy structs such as `PostPolicy`
- Laravel authorize helpers -> `runtime/authz.Authorize`
- Laravel deny / deny as not found -> `Deny` / `DenyAsNotFound`

What this proposal intentionally does not copy from Laravel or common Laravel
packages:

- global gate registry
- reflection-based policy discovery
- route-level authorization DSL in core
- database-backed permission management
- wildcard permissions
- direct per-user overrides in the framework default

The goal is to capture Laravel's productive policy ergonomics without pulling
in the complexity of a runtime-managed permissions subsystem.

## Scaling Behavior

The proposed default scales cleanly across multiple stateless API instances:

- JWT validation is local to each process.
- The permission map is code loaded into each process.
- Authorization facts live in Postgres and are visible to all instances after
  commit.
- There is no cross-instance policy-cache invalidation problem in the default
  path.

The main tradeoff is token freshness: if a user's ZITADEL role assignment
changes, existing access tokens may continue to reflect the old role set until
expiry or refresh. This is a normal consequence of stateless JWT validation.

## Heavier Alternatives Considered

### Casbin

Casbin is the easiest path to runtime-editable RBAC or ABAC without adding a
dedicated authz service. It scales across instances if policies are stored in a
shared adapter and changes are synchronized with watchers or dispatchers.

Tradeoff: once adapters, watchers, and cache invalidation enter the picture,
the operational and mental overhead rises quickly. This is likely too much for
Gofra's default path.

### Cedar

Cedar is a strong policy-as-code option with explicit `permit` and `forbid`
semantics and a good conceptual model. Embedded evaluation scales well because
each process can evaluate locally.

Tradeoff: Gofra would still need to define how policies and entities are stored,
distributed, and tested in Go-first applications. It is a strong future option,
but not the simplest MVP.

### OPA

OPA is mature and flexible, especially in sidecar deployments and bundle-based
policy distribution.

Tradeoff: Rego and bundle management solve a larger class of problems than the
default app authz story currently needs. This would add power before the
framework has validated the simpler model.

### OpenFGA / SpiceDB

These systems are the right direction when the application truly needs
relationship-based authorization across many resource types and many app
instances.

Tradeoff: they introduce an additional service, schema/model lifecycle, and a
different mental model. That complexity is not justified for the framework's
default v1 story.

## DX Recommendation

If developer experience remains the priority, the order of preference is:

1. App-owned policies plus helper-only `runtime/authz`
2. Optional future OpenFGA integration for apps that truly need ReBAC
3. Optional future Cedar integration if policy-as-code becomes a framework
   priority

Casbin, OPA, and SpiceDB remain valid tools, but they should not shape the
framework default until real product requirements justify the extra complexity.

## What To Revisit Later

When the auth/authz slice returns to the active roadmap, revisit these open
questions before implementation:

- Should `runtime/auth` ship with both JWT and introspection backends or only
  JWT first?
- Should public Connect procedures be declared by generated metadata instead of
  a handwritten allowlist?
- Should the starter scaffold one or two example policy files by default?
- Does the frontend starter need a stronger default around token lifecycle
  observability or explicit expiry UX?
- Is there enough demand to justify a first-party OpenFGA or Cedar integration?

Until those questions are settled, this note is the preferred reference for the
proposed implementation shape.
