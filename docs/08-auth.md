# 08 — Authentication & Authorization: Zitadel

> Parent: [Index](00-index.md) | Prev: [Observability](07-observability.md) | Next: [Errors](09-errors.md)


## Addendum to Architecture Design Document
## Last Updated: 2026-04-12

---

## The Decision: Couple with Zitadel

Forge delegates all identity concerns to Zitadel. The framework does not
implement user registration, login, password hashing, session management,
MFA, social login, or user storage. Zitadel owns these.

Forge owns authorization enforcement — checking whether an authenticated
user is allowed to perform a specific action on a specific resource. This
is the line: **Zitadel answers "who is this person?" Forge answers "can this
person do this thing?"**

### V1 Client Auth Model

Forge v1 standardizes on one auth family for all human-operated clients:
direct OpenID Connect Authorization Code flow against Zitadel. The browser
SPA uses Authorization Code + PKCE as a public client. Native mobile and
desktop clients use the same Authorization Code + PKCE pattern with
platform-native redirect handling and OS-managed secure storage.

Forge does **not** introduce a backend-for-frontend (BFF) session layer for
the default web client in v1. Browser clients authenticate directly with
Zitadel and call Forge with bearer access tokens. This keeps the browser,
mobile, and desktop stories aligned on one protocol family and one backend
validation path while the framework contract is still being stabilized.

This is an explicit v1 simplification, not a statement that a BFF is never
useful. If Forge later needs a stronger default web posture around token
handling, that can be revisited as a new architectural decision instead of
remaining ambiguous in the docs.

### Why Zitadel specifically

**Protocol alignment.** Zitadel's v2 API speaks Connect RPC — the same
protocol Forge uses. The API paths are `/zitadel.user.v2.UserService/`,
`/zitadel.org.v2.OrganizationService/`, etc. This means Forge's backend can
call Zitadel's APIs using the same Connect client libraries it uses internally.
No REST-to-gRPC translation. No separate HTTP client. Same tooling, same
interceptors, same error handling patterns.

**Written in Go.** Zitadel is a Go binary with Postgres. Forge is a Go binary
with Postgres. Same language, same runtime, same deployment model. The
`zitadel/oidc` library is the most complete OIDC implementation in Go — it
supports both Relying Party (client) and OpenID Provider (server) roles, is
certified by the OpenID Foundation, and is what Zitadel itself uses.

**Organizations and multi-tenancy are native.** Zitadel's hierarchy —
Instance > Organization > Project > Application — maps well to general web
applications that need users, teams, org boundaries, or full multi-tenancy.
If an app is simple, it can still live inside one default organization. If it
grows into a multi-tenant SaaS later, the identity model is already there.

**Single binary, self-hostable.** Zitadel runs as one binary with one Postgres
database (which can be the same Postgres instance Forge uses, in a separate
database). No Redis, no Kafka, no ElasticSearch. Matches Forge's minimal
deployment philosophy.

**Everything Forge would need to build is already there.** User registration,
email verification, password reset, MFA/TOTP/passkeys, social login
(Google/GitHub/Apple/SAML), account lockout policies, password complexity
policies, branding/theming, audit logs (event-sourced — every mutation is
an immutable event), user metadata, service accounts for machine-to-machine.

### What Forge does NOT delegate

- **Resource-level authorization.** "Can user X edit post Y?" requires
  knowledge of the resource (who owns the post, what's the post's status).
  Zitadel doesn't know about posts. The application checks this.

- **Business-domain roles.** Zitadel manages role assignments (user X has
  role `editor` in project `myapp`). The application decides what `editor`
  means in the context of posts, comments, and settings.

- **Application data.** Users in Zitadel are identity records (name, email,
  credentials). Application-specific user data (preferences, billing, avatar)
  lives in Forge's Postgres, linked by Zitadel's user ID.

---

## Architecture

```
┌──────────────┐     OIDC / Connect RPC      ┌──────────────────┐
│   React SPA  │ ◄──────────────────────────► │     Zitadel      │
│              │   redirect to login          │                  │
│  - login     │   receive tokens             │  - Users         │
│  - register  │   refresh tokens             │  - Organizations │
│  - profile   │                              │  - Projects      │
└──────┬───────┘                              │  - Roles         │
       │                                      │  - MFA/Passkeys  │
       │ Authorization: Bearer <JWT>          │  - Social Login  │
       │                                      │  - Audit Log     │
       ▼                                      └──────────────────┘
┌──────────────────────────────────┐                    │
│        Forge API Server          │                    │
│                                  │   JWKS validation  │
│  ┌───────────────────────────┐   │◄───────────────────┘
│  │ AuthInterceptor           │   │   (fetch public keys)
│  │                           │   │
│  │ 1. Extract Bearer token   │   │
│  │ 2. Validate JWT signature │   │
│  │    via Zitadel JWKS       │   │
│  │ 3. Check audience, expiry │   │
│  │ 4. Extract claims:        │   │
│  │    - sub (user ID)        │   │
│  │    - roles []string       │   │
│  │    - org_id               │   │
│  │ 5. Set in context         │   │
│  └───────────────────────────┘   │
│                                  │
│  ┌───────────────────────────┐   │
│  │ Connect RPC Handler       │   │
│  │                           │   │
│  │ userID := auth.UserID(ctx)│   │
│  │ roles := auth.Roles(ctx)  │   │
│  │                           │   │
│  │ // Authz: app-level check │   │
│  │ if !canEdit(roles, post) {│   │
│  │   return PermissionDenied │   │
│  │ }                         │   │
│  └───────────────────────────┘   │
│                                  │
│  ┌───────────────────────────┐   │
│  │ Zitadel Management Client │   │   Connect RPC to Zitadel
│  │ (service account)         │ ──┼──────────────────────────►
│  │                           │   │   Create users, assign roles,
│  │ Used by admin handlers    │   │   list organizations, etc.
│  └───────────────────────────┘   │
└──────────────────────────────────┘
```

---

## Authentication Flow

### SPA Login (Authorization Code Flow with PKCE)

```
1. User clicks "Login" in the SPA
2. SPA generates code_verifier + code_challenge (PKCE)
3. SPA redirects to Zitadel's /authorize endpoint:
   GET https://auth.myapp.com/oauth/v2/authorize?
     client_id=<app_client_id>
     &redirect_uri=https://myapp.com/callback
     &response_type=code
     &scope=openid profile email urn:zitadel:iam:org:projects:roles
     &code_challenge=<challenge>
     &code_challenge_method=S256

4. Zitadel presents its login UI (or custom login UI)
   - Email/password, passkey, social login, MFA — all handled by Zitadel

5. On success, Zitadel redirects back:
   GET https://myapp.com/callback?code=<auth_code>&state=<state>

6. SPA exchanges code for tokens:
   POST https://auth.myapp.com/oauth/v2/token
   - code, code_verifier, client_id, redirect_uri

7. Zitadel returns:
   - access_token (JWT, short-lived, ~15min)
   - id_token (user identity claims)
   - refresh_token (long-lived, rotating, for token renewal)

8. SPA manages the returned token set in frontend-controlled storage
   (Forge v1 does not convert this into a backend session cookie)

9. SPA sends API requests with:
   Authorization: Bearer <access_token>
```

**Reason for PKCE**: The SPA is a public client (no client_secret). PKCE
prevents authorization code interception attacks. This is the standard for
SPAs per OAuth 2.0 Security Best Current Practice.

**Reason for `urn:zitadel:iam:org:projects:roles` scope**: This makes Zitadel
include the user's project roles in the token claims. Without this scope, the
token contains identity but no role information, and the API would need a
separate call to Zitadel to fetch roles.

**Reason Forge does not use a BFF/session-cookie layer for the browser by
default**: Forge is choosing one direct OIDC code-flow architecture across
browser, mobile, and desktop clients for v1. That reduces framework surface
area while the rest of the auth model is still being hardened. The exact
browser token storage, refresh lifecycle, and logout contract remain tracked
in the readiness checklist.

### Token Validation in the AuthInterceptor

```go
// forge/auth_interceptor.go

type AuthInterceptor struct {
    verifier *auth.AccessTokenVerifier
}

func NewAuthInterceptor(issuerURL, audience string) (*AuthInterceptor, error) {
    verifier, err := auth.NewAccessTokenVerifier(issuerURL, audience)
    if err != nil {
        return nil, fmt.Errorf("jwt verifier setup: %w", err)
    }

    return &AuthInterceptor{verifier: verifier}, nil
}

func (a *AuthInterceptor) Unary() connect.UnaryInterceptorFunc {
    return func(next connect.UnaryFunc) connect.UnaryFunc {
        return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
            // Skip auth for public RPCs
            if isPublicRPC(req.Spec().Procedure) {
                return next(ctx, req)
            }

            token := extractBearerToken(req.Header())
            if token == "" {
                return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing token"))
            }

            // Validate access token: signature (via JWKS), audience, expiry, issuer
            claims, err := a.verifier.Verify(ctx, token)
            if err != nil {
                return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid token: %w", err))
            }

            // Flatten Zitadel role structure to []string
            roles := flattenRoles(claims.Roles)

            // Set auth context
            ctx = auth.WithUser(ctx, auth.User{
                ID:    claims.Sub,
                Email: claims.Email,
                Roles: roles,
                OrgID: claims.OrgID,
            })

            return next(ctx, req)
        }
    }
}
```

**Reason for an access-token verifier backed by OIDC discovery + JWKS**: the
API receives bearer access tokens, not ID tokens. The verifier should use
Zitadel's discovery metadata to find the JWKS endpoint, cache public keys,
validate JWT signatures locally, and parse claims without a network round trip
per request.

**Reason for stateless JWT validation**: The interceptor validates the token
locally using Zitadel's public keys (fetched once and cached via JWKS). No
network call to Zitadel per request. This is fast and doesn't create a
dependency on Zitadel availability for every API call.

---

## Authorization: How It Works

### Layer 1: Role-Based (from Zitadel)

Zitadel assigns roles to users within projects. Roles are strings defined
by the application: `admin`, `editor`, `viewer`, `billing_admin`. Zitadel
doesn't know what these strings mean — it just stores and returns them.

The roles are included in the JWT claims when the user authenticates
(via the `urn:zitadel:iam:org:projects:roles` scope).

### Layer 2: Permission Mapping (in Forge)

Forge maps roles to permissions. This is application logic, not Zitadel
configuration.

```go
// app/authz/permissions.go

// Permissions are fine-grained actions
type Permission string

const (
    PermPostCreate  Permission = "post.create"
    PermPostEdit    Permission = "post.edit"
    PermPostDelete  Permission = "post.delete"
    PermPostPublish Permission = "post.publish"
    PermUserList    Permission = "user.list"
    PermUserManage  Permission = "user.manage"
    PermSettingsView Permission = "settings.view"
    PermSettingsEdit Permission = "settings.edit"
)

// RolePermissions maps Zitadel roles to application permissions
var RolePermissions = map[string][]Permission{
    "admin": {
        PermPostCreate, PermPostEdit, PermPostDelete, PermPostPublish,
        PermUserList, PermUserManage,
        PermSettingsView, PermSettingsEdit,
    },
    "editor": {
        PermPostCreate, PermPostEdit, PermPostPublish,
    },
    "viewer": {
        // no write permissions
    },
}

func HasPermission(roles []string, perm Permission) bool {
    for _, role := range roles {
        for _, p := range RolePermissions[role] {
            if p == perm {
                return true
            }
        }
    }
    return false
}
```

**Reason permissions are defined in Go, not Zitadel**: Zitadel doesn't
have a native permission concept (as of 2025 — this is a frequently
requested feature, see zitadel/zitadel#9768). Zitadel has roles, which
are opaque strings. The role-to-permission mapping is business logic that
belongs in the application. Defining it in Go means it's compile-time
checked, testable, and version-controlled with the app code.

**Reason for a static map, not a database table**: For most applications,
the permission set is small (10-50 permissions) and changes with code
deployments, not at runtime. A static Go map is the simplest correct
implementation. If runtime-configurable permissions are needed later, this
can be moved to a database table without changing the checking interface.

### Layer 3: Resource-Level Checks (in Handlers)

Some authorization decisions depend on the resource, not just the role.
"Can this user edit THIS post?" requires knowing who owns the post.

```go
func (s *PostsService) UpdatePost(
    ctx context.Context,
    req *connect.Request[postsv1.UpdatePostRequest],
) (*connect.Response[postsv1.Post], error) {

    user := auth.UserFromContext(ctx)

    // Permission check: does the user's role allow editing posts at all?
    if !authz.HasPermission(user.Roles, authz.PermPostEdit) {
        return nil, connect.NewError(connect.CodePermissionDenied, errors.New("insufficient permissions"))
    }

    post, err := s.Queries.GetPostByID(ctx, req.Msg.Post.Id)
    if err != nil {
        return nil, connect.NewError(connect.CodeNotFound, err)
    }

    // Resource check: non-admins can only edit their own posts
    if post.AuthorID != user.ID && !authz.HasPermission(user.Roles, authz.PermUserManage) {
        return nil, connect.NewError(connect.CodePermissionDenied, errors.New("can only edit own posts"))
    }

    // ... proceed with update
}
```

**Reason for inline checks, not a middleware/interceptor**: Resource-level
authorization requires loading the resource from the database. You can't
check "can this user edit this post" without knowing who owns the post. The
interceptor handles authentication and role extraction. The handler handles
resource-level authorization. This is the natural separation.

---

## User Management

### Where User Data Lives

| Data | Owner | Storage |
|------|-------|---------|
| Identity (email, name, credentials) | Zitadel | Zitadel's Postgres |
| Authentication method (password, passkey, social) | Zitadel | Zitadel's Postgres |
| Roles (admin, editor, viewer) | Zitadel | Zitadel's Postgres |
| Organization membership | Zitadel | Zitadel's Postgres |
| MFA enrollment | Zitadel | Zitadel's Postgres |
| Audit log (login events, role changes) | Zitadel | Zitadel's event store |
| App preferences (theme, language, notifications) | Forge | Forge's Postgres |
| App profile (bio, avatar, social links) | Forge | Forge's Postgres |
| Billing / subscription | Forge | Forge's Postgres |
| Content (posts, comments, uploads) | Forge | Forge's Postgres |

**Reason for the split**: Zitadel owns identity because it's the single source
of truth for "who is this person and can they prove it." Forge owns application
data because Zitadel shouldn't know about posts, billing, or app preferences.
The link between them is `user.ID` (Zitadel's user ID), which is stored in
Forge's tables as a foreign reference.

### App-Side User Record

```sql
-- db/migrations/00001_create_user_profiles.sql
-- +goose Up
CREATE TABLE user_profiles (
    zitadel_user_id  TEXT PRIMARY KEY,    -- Zitadel's user ID (sub claim)
    display_name     TEXT,
    bio              TEXT,
    avatar_url       TEXT,
    locale           TEXT DEFAULT 'en',
    create_time      TIMESTAMPTZ NOT NULL DEFAULT now(),
    update_time      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**Reason for `zitadel_user_id TEXT`**: Zitadel user IDs are opaque strings.
Using them directly as the primary key avoids a surrogate key and simplifies
joins. The field name makes the data source explicit.

### User Provisioning: Just-In-Time

When a user first authenticates and calls a Forge API, they may not have
a `user_profiles` row yet. The handler creates one on first access:

```go
func ensureUserProfile(ctx context.Context, queries *sqlc.Queries, user auth.User) error {
    _, err := queries.GetUserProfile(ctx, user.ID)
    if err == nil {
        return nil // already exists
    }
    if !errors.Is(err, pgx.ErrNoRows) {
        return err // real error
    }

    // First-time user — create profile from Zitadel claims
    _, err = queries.CreateUserProfile(ctx, sqlc.CreateUserProfileParams{
        ZitadelUserID: user.ID,
        DisplayName:   user.Email, // default, user can change later
    })
    return err
}
```

**Reason for JIT provisioning**: No sync process between Zitadel and Forge.
No webhook to keep in sync. The user profile is created the first time the
user hits the API. Simple, reliable, no moving parts.

### Admin User Management

For admin panels (listing users, assigning roles, deactivating accounts),
Forge calls Zitadel's Management API using a service account:

```go
// forge/zitadel_client.go

type ZitadelClient struct {
    userService    zitadeluserv2.UserServiceClient
    orgService     zitadelorgv2.OrganizationServiceClient
    projectService zitadelprojectv2.ProjectServiceClient
}

func NewZitadelClient(issuerURL, serviceAccountKeyPath string) (*ZitadelClient, error) {
    // Authenticate as service account using JWT profile
    // This gives the client admin-level access to Zitadel APIs
    // ...
}
```

Admin-facing Connect handlers in Forge proxy to Zitadel:

```go
// app/rpc/admin_service.go

func (s *AdminService) ListUsers(
    ctx context.Context,
    req *connect.Request[adminv1.ListUsersRequest],
) (*connect.Response[adminv1.ListUsersResponse], error) {

    user := auth.UserFromContext(ctx)
    if !authz.HasPermission(user.Roles, authz.PermUserList) {
        return nil, connect.NewError(connect.CodePermissionDenied, nil)
    }

    // Call Zitadel's User API
    zitadelResp, err := s.Zitadel.userService.ListUsers(ctx, &zitadeluserv2.ListUsersRequest{
        // map pagination, filters
    })
    if err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }

    // Map Zitadel users to our proto
    users := make([]*adminv1.User, len(zitadelResp.Result))
    for i, zu := range zitadelResp.Result {
        users[i] = zitadelUserToProto(zu)
    }

    return connect.NewResponse(&adminv1.ListUsersResponse{Users: users}), nil
}
```

**Reason for proxying through Forge instead of calling Zitadel directly from
the SPA**: The SPA shouldn't have admin-level Zitadel credentials. Forge acts
as a gateway — it enforces authorization (does this user have `user.manage`
permission?) before forwarding the request to Zitadel with service account
credentials.

---

## Zitadel Setup: Concepts Mapping

| Zitadel Concept | Forge Usage |
|-----------------|-------------|
| **Instance** | One per deployment (dev, staging, prod) |
| **Organization** | One per tenant (B2C: single org for all users. B2B: one org per customer) |
| **Project** | One per Forge application (contains the app's roles) |
| **Application** | Two: one PKCE app for the SPA, one JWT-profile app for the service account |
| **Roles** | Application-level roles: `admin`, `editor`, `viewer`, etc. |
| **User Grants** | Assigns a user to a role within the project |
| **Actions** | Optional: custom claims injection (e.g., add org metadata to tokens) |

### Development Setup

```yaml
# docker-compose.yml (additions for Zitadel)
services:
  zitadel:
    image: ghcr.io/zitadel/zitadel:latest
    command: start-from-init --masterkeyFromEnv --tlsMode disabled
    environment:
      ZITADEL_MASTERKEY: "MasterkeyNeedsToHave32Characters"
      ZITADEL_DATABASE_POSTGRES_HOST: postgres
      ZITADEL_DATABASE_POSTGRES_PORT: 5432
      ZITADEL_DATABASE_POSTGRES_DATABASE: zitadel
      ZITADEL_DATABASE_POSTGRES_USER: zitadel
      ZITADEL_DATABASE_POSTGRES_PASSWORD: zitadel
      ZITADEL_EXTERNALSECURE: "false"
      ZITADEL_EXTERNALPORT: 8080
      ZITADEL_EXTERNALDOMAIN: localhost
    ports:
      - "8080:8080"  # Zitadel API + login UI
    depends_on:
      postgres:
        condition: service_healthy
```

```yaml
# forge.yaml
auth:
  issuer: "http://localhost:8080"
  client_id: "${ZITADEL_CLIENT_ID}"
  project_id: "${ZITADEL_PROJECT_ID}"

  # Service account for admin operations (user management, role assignment)
  service_account:
    key_path: "${ZITADEL_SERVICE_ACCOUNT_KEY}"
```

---

## What the SPA Needs

### Auth Library

The SPA uses `@zitadel/react` or plain `oidc-client-ts` (standard OIDC
client for JavaScript):

```tsx
// web/src/lib/auth.ts
import { UserManager } from "oidc-client-ts";

export const userManager = new UserManager({
  authority: import.meta.env.VITE_ZITADEL_ISSUER,
  client_id: import.meta.env.VITE_ZITADEL_CLIENT_ID,
  redirect_uri: `${window.location.origin}/callback`,
  post_logout_redirect_uri: window.location.origin,
  scope: "openid profile email urn:zitadel:iam:org:projects:roles",
  response_type: "code",
});
```

### Protecting Routes

```tsx
// web/src/routes/__root.tsx
function AuthGuard({ children }) {
  const user = useAuth();
  if (!user) return <RedirectToLogin />;
  return children;
}
```

### Using Roles in the UI

```tsx
function PostActions({ post }) {
  const { roles } = useAuth();
  const canEdit = hasPermission(roles, "post.edit") ||
                  post.authorId === currentUserId;

  return (
    <>
      {canEdit && <Button onClick={onEdit}>Edit</Button>}
      {hasPermission(roles, "post.delete") && <Button onClick={onDelete}>Delete</Button>}
    </>
  );
}
```

**Reason for duplicating permission checks in the frontend**: UI/UX — don't
show buttons the user can't use. The server ALWAYS re-checks permissions.
Frontend checks are for display only, never for security.

---

## Future: Fine-Grained Authorization

Zitadel provides roles. For most apps, role → permission mapping in Go is
sufficient. When an application outgrows simple RBAC (e.g., "user X can
access resource Y because they belong to department Z and it's during
business hours"), Zitadel integrates with external authorization services:

- **OpenFGA** (Google Zanzibar-based, relationship-based access control)
- **Cerbos** (policy engine, policy-as-code)
- **Permify** (Google Zanzibar-based)

These services consume Zitadel's user/role data and add fine-grained
policy evaluation. Forge can integrate with them as a future addendum.
The current architecture doesn't preclude this — the `HasPermission`
function is the seam where a policy engine can be plugged in.

---

## Decision Log (Auth & Authz)

| # | Decision | Rationale |
|---|----------|-----------|
| 67 | Couple with Zitadel for identity | Go-native, Connect RPC API, single binary, multi-tenant native, OIDC certified. Eliminates thousands of lines of security-critical code. |
| 68 | Direct OIDC Authorization Code flow for browser and native clients | One auth family across human clients. Browser uses PKCE as a public client. Native clients use PKCE with platform redirect handling. |
| 69 | Stateless JWT validation via JWKS | No per-request call to Zitadel. Fast. Keys cached and auto-rotated. |
| 70 | OIDC discovery + local JWT verification for access tokens | Uses Zitadel discovery metadata and cached JWKS keys. Verifies bearer access tokens locally. |
| 71 | Roles in Zitadel, permissions in Go | Zitadel manages role assignment. Application defines what roles mean (permissions). Clean separation. |
| 72 | Static role→permission map | Small, changes with code deploys, testable. Can move to DB later without changing the interface. |
| 73 | Resource-level authz in RPC handlers, not interceptor | Requires loading the resource. Can't check "can edit this post" without knowing who owns it. |
| 74 | JIT user profile creation | No sync between Zitadel and Forge. Profile created on first API call. No webhooks, no eventual consistency. |
| 75 | `zitadel_user_id TEXT` as PK | Zitadel IDs are opaque strings. Direct PK avoids surrogate key and simplifies joins. |
| 76 | Admin handlers proxy to Zitadel via service account | SPA doesn't get admin Zitadel credentials. Forge enforces authz before forwarding. |
| 77 | No BFF/session-cookie default for browser clients | Keep the v1 surface smaller and backend auth validation consistent across browser, mobile, and desktop. Revisit later if needed. |
| 78 | `urn:zitadel:iam:org:projects:roles` scope | Includes roles in the token. No extra API call to fetch roles on every request. |
| 79 | Frontend permission checks are display-only | UI shows/hides buttons. Server always re-checks. Never trust the client. |
