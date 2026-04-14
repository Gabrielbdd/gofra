# 08 — Authentication & Authorization: Zitadel

> Parent: [Index](00-index.md) | Prev: [Observability](07-observability.md) | Next: [Errors](09-errors.md)


## Addendum to Architecture Design Document
## Last Updated: 2026-04-12

---

## The Decision: Couple with Zitadel

Gofra delegates all identity concerns to Zitadel. The framework does not
implement user registration, login, password hashing, session management,
MFA, social login, or user storage. Zitadel owns these.

Gofra owns authorization enforcement — checking whether an authenticated
user is allowed to perform a specific action on a specific resource. This
is the line: **Zitadel answers "who is this person?" Gofra answers "can this
person do this thing?"**

## Deferred Implementation Note

The high-level auth model in this document remains the intended framework
direction, but the exact `runtime/auth` and `runtime/authz` APIs are still
deferred while the implementation surface is narrowed.

The current proposed MVP package split, persistence model, Laravel comparison,
and "ship later" scope are captured in
[docs/project/auth-authz-mvp-proposal.md](project/auth-authz-mvp-proposal.md).
Use that note when resuming this slice; do not infer a finalized public API
from the illustrative snippets below.

### V1 Client Auth Model

Gofra v1 standardizes on one auth family for all human-operated clients:
direct OpenID Connect Authorization Code flow against Zitadel. The browser
SPA uses Authorization Code + PKCE as a public client. Native mobile and
desktop clients use the same Authorization Code + PKCE pattern with
platform-native redirect handling and OS-managed secure storage.

Gofra does **not** introduce a backend-for-frontend (BFF) session layer for
the default web client in v1. Browser clients authenticate directly with
Zitadel and call Gofra with bearer access tokens. This keeps the browser,
mobile, and desktop stories aligned on one protocol family and one backend
validation path while the framework contract is still being stabilized.

This is an explicit v1 simplification, not a statement that a BFF is never
useful. If Gofra later needs a stronger default web posture around token
handling, that can be revisited as a new architectural decision instead of
remaining ambiguous in the docs.

### Browser SPA Defaults

Gofra keeps the browser contract narrow and explicit:

- `react-oidc-context` is the default React integration layer.
- The browser token set lives in `sessionStorage`.
- The SPA requests `offline_access` and uses rotating refresh tokens.
- On app bootstrap, the SPA restores auth state from `sessionStorage` and
  attempts one token refresh if the access token is expired.
- If bootstrap refresh fails, the SPA clears auth state and treats the user as
  signed out.
- Logout clears browser auth state first and then redirects through Zitadel's
  end-session flow.
- Gofra does not promise cross-device or "log out everywhere" behavior in v1.

**Reason for `sessionStorage`**: it survives a normal page reload, which keeps
the browser UX usable, but it does not persist across full browser restarts the
way `localStorage` does. Gofra is choosing the simpler v1 tradeoff between
reload ergonomics and limiting token persistence.

**Reason for refresh tokens in the browser**: without a BFF, the SPA needs a
renewal path that does not force a full redirect loop every time the access
token expires. Gofra standardizes on rotating refresh tokens rather than
multiple competing browser renewal patterns.

**Reason for one bootstrap refresh attempt**: auth startup should be
deterministic. One refresh attempt covers the normal "token expired while the
tab was closed" case without hiding retry loops inside framework scaffolding.

### Native Client Defaults

Gofra uses the same OIDC code-flow family for native clients, but the storage
and redirect mechanics are platform-native:

- Native apps use the system browser for login.
- Redirects use a custom URI scheme or loopback callback.
- Tokens live in OS-managed secure storage such as Keychain, Keystore, or the
  platform credential manager.
- Embedded webviews are not the Gofra default.

### Why Zitadel specifically

**Protocol alignment.** Zitadel's v2 API speaks Connect RPC — the same
protocol Gofra uses. The API paths are `/zitadel.user.v2.UserService/`,
`/zitadel.org.v2.OrganizationService/`, etc. This means Gofra's backend can
call Zitadel's APIs using the same Connect client libraries it uses internally.
No REST-to-gRPC translation. No separate HTTP client. Same tooling, same
interceptors, same error handling patterns.

**Written in Go.** Zitadel is a Go binary with Postgres. Gofra is a Go binary
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
database (which can be the same Postgres instance Gofra uses, in a separate
database). No Redis, no Kafka, no ElasticSearch. Matches Gofra's minimal
deployment philosophy.

**Everything Gofra would need to build is already there.** User registration,
email verification, password reset, MFA/TOTP/passkeys, social login
(Google/GitHub/Apple/SAML), account lockout policies, password complexity
policies, branding/theming, audit logs (event-sourced — every mutation is
an immutable event), user metadata, service accounts for machine-to-machine.

### What Gofra does NOT delegate

- **Resource-level authorization.** "Can user X edit post Y?" requires
  knowledge of the resource (who owns the post, what's the post's status).
  Zitadel doesn't know about posts. The application checks this.

- **Business-domain roles.** Zitadel manages role assignments (user X has
  role `editor` in project `myapp`). The application decides what `editor`
  means in the context of posts, comments, and settings.

- **Application data.** Users in Zitadel are identity records (name, email,
  credentials). Application-specific user data (preferences, billing, avatar)
  lives in Gofra's Postgres, linked by Zitadel's user ID.

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
│        Gofra API Server          │                    │
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

8. SPA stores the token set in sessionStorage

9. On app bootstrap or page reload:
   - restore the stored token set
   - if access_token is expired and refresh_token exists, attempt one refresh
   - if refresh fails, clear auth state and require login

10. SPA sends API requests with:
   Authorization: Bearer <access_token>

11. User clicks "Logout"

12. SPA clears local auth state, then redirects through Zitadel's
    end-session endpoint, then returns to the application root
```

**Reason for PKCE**: The SPA is a public client (no client_secret). PKCE
prevents authorization code interception attacks. This is the standard for
SPAs per OAuth 2.0 Security Best Current Practice.

**Reason for `urn:zitadel:iam:org:projects:roles` scope**: This makes Zitadel
include the user's project roles in the token claims. Without this scope, the
token contains identity but no role information, and the API would need a
separate call to Zitadel to fetch roles.

**Reason for `offline_access` in the SPA scope**: Gofra's direct-browser
contract depends on refresh tokens for normal session continuity. Without
`offline_access`, the browser would need a full authorization redirect whenever
the access token expired.

**Reason logout clears local state before network logout**: the application
should fail closed. If the browser clears auth state first, an interrupted
redirect to Zitadel does not leave the local app believing the user is still
signed in.

### Token Validation in the AuthInterceptor

```go
// gofra/auth_interceptor.go

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

The deferred MVP proposal keeps this stateless JWT direction, but it narrows
the implementation contract further:

- the protected API application must issue JWT access tokens
- the runtime package should expose a verifier interface so introspection can be
  added later without breaking the package boundary
- the preferred Connect integration point is auth middleware that can support
  both unary and streaming RPCs

### Public RPCs vs Protected RPCs

Gofra treats Connect RPCs as private by default. Public RPCs are an explicit
allowlist:

```go
var publicRPCs = map[string]struct{}{
    "/blog.v1.PublicPostsService/ListPosts": {},
    "/blog.v1.PublicPostsService/GetPost":   {},
}

func isPublicRPC(procedure string) bool {
    _, ok := publicRPCs[procedure]
    return ok
}
```

Everything not in this allowlist requires a valid access token. Health checks
are plain HTTP endpoints outside the Connect auth interceptor.

Admin RPCs are never public. They require normal user authentication plus
application permission checks, and only then does Gofra call Zitadel's
management APIs using a service account.

---

## Authorization: How It Works

### Layer 1: Role-Based (from Zitadel)

Zitadel assigns roles to users within projects. Roles are strings defined
by the application: `admin`, `editor`, `viewer`, `billing_admin`. Zitadel
doesn't know what these strings mean — it just stores and returns them.

The roles are included in the JWT claims when the user authenticates
(via the `urn:zitadel:iam:org:projects:roles` scope).

### Layer 2: Permission Mapping (in Gofra)

Gofra maps roles to permissions. This is application logic, not Zitadel
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

The deferred MVP proposal makes the package split explicit: generated apps own
permission constants, role-permission maps, and policy structs, while
`runtime/authz` stays helper-only and provides the reusable checking mechanics.

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
| App preferences (theme, language, notifications) | Gofra | Gofra's Postgres |
| App profile (bio, avatar, social links) | Gofra | Gofra's Postgres |
| Billing / subscription | Gofra | Gofra's Postgres |
| Content (posts, comments, uploads) | Gofra | Gofra's Postgres |

**Reason for the split**: Zitadel owns identity because it's the single source
of truth for "who is this person and can they prove it." Gofra owns application
data because Zitadel shouldn't know about posts, billing, or app preferences.
The link between them is `user.ID` (Zitadel's user ID), which is stored in
Gofra's tables as a foreign reference.

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

When a user first authenticates and calls a Gofra API, they may not have
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

**Reason for JIT provisioning**: No sync process between Zitadel and Gofra.
No webhook to keep in sync. The user profile is created the first time the
user hits the API. Simple, reliable, no moving parts.

### Admin User Management

For admin panels (listing users, assigning roles, deactivating accounts),
Gofra calls Zitadel's Management API using a service account:

```go
// gofra/zitadel_client.go

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

Admin-facing Connect handlers in Gofra proxy to Zitadel:

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

**Reason for proxying through Gofra instead of calling Zitadel directly from
the SPA**: The SPA shouldn't have admin-level Zitadel credentials. Gofra acts
as a gateway — it enforces authorization (does this user have `user.manage`
permission?) before forwarding the request to Zitadel with service account
credentials.

---

## Zitadel Setup: Concepts Mapping

| Zitadel Concept | Gofra Usage |
|-----------------|-------------|
| **Instance** | One per deployment (dev, staging, prod) |
| **Organization** | One per tenant (B2C: single org for all users. B2B: one org per customer) |
| **Project** | One per Gofra application (contains the app's roles) |
| **Application** | Three: one User Agent app for the browser SPA, one Native app for mobile/desktop, one JWT-profile app for the service account |
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
# gofra.yaml
auth:
  issuer: "http://localhost:8080"
  audience: "myapp-api"
  client_id: "${ZITADEL_BROWSER_CLIENT_ID}"
  scopes:
    - openid
    - profile
    - email
    - offline_access
    - urn:zitadel:iam:org:projects:roles
  redirect_path: "/auth/callback"
  post_logout_redirect_path: "/"
  browser_token_store: session_storage
  use_refresh_tokens: true
  service_account:
    key_path: "${ZITADEL_SERVICE_ACCOUNT_KEY}"
```

---

## What the SPA Needs

### Auth Library

The default React integration is `react-oidc-context`, which wraps
`oidc-client-ts` without forcing Gofra to invent its own client-side auth
abstraction:

```tsx
// web/src/lib/auth.ts
import { AuthProvider } from "react-oidc-context";
import { WebStorageStateStore } from "oidc-client-ts";
import { runtimeConfig } from "./runtime-config";

export const oidcConfig = {
  authority: runtimeConfig.auth.issuer,
  client_id: runtimeConfig.auth.clientId,
  redirect_uri: `${window.location.origin}${runtimeConfig.auth.redirectPath}`,
  post_logout_redirect_uri: `${window.location.origin}${runtimeConfig.auth.postLogoutRedirectPath}`,
  scope: runtimeConfig.auth.scopes.join(" "),
  response_type: "code",
  userStore: new WebStorageStateStore({ store: window.sessionStorage }),
};

export function AppAuthProvider({ children }) {
  return <AuthProvider {...oidcConfig}>{children}</AuthProvider>;
}
```

**Reason for `react-oidc-context`**: Gofra ships a React SPA by default. The
framework should standardize on one React-friendly OIDC integration instead of
describing multiple incompatible frontend auth layers.

**Reason auth config comes from `runtimeConfig` instead of `import.meta.env`**:
OIDC issuer and client IDs are deployment settings. Gofra loads them from the
Go-served public runtime config so the SPA bundle does not need rebuilding per
environment.

### Protecting Routes

```tsx
const PUBLIC_ROUTES = new Set([
  "/",
  "/login",
  "/auth/callback",
]);

function RequireAuth({ children }) {
  const auth = useAuth();
  const location = useLocation();

  if (PUBLIC_ROUTES.has(location.pathname)) return children;
  if (auth.isLoading) return <FullPageSpinner />;
  if (!auth.isAuthenticated) {
    void auth.signinRedirect({ state: { returnTo: location.pathname } });
    return null;
  }

  return children;
}
```

Gofra keeps the route model explicit:

- `/`, `/login`, and `/auth/callback` are public routes.
- Application routes under `/app` require authentication.
- Admin screens are protected routes plus permission checks such as
  `user.manage`.

### Refresh And Logout

- On app startup, restore auth state from `sessionStorage`.
- If the access token is expired and a refresh token exists, attempt one token
  renewal.
- If renewal fails, clear auth state and present signed-out UI.
- On logout, clear local auth state first, then redirect through Zitadel's
  end-session flow, then return to `/`.

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
policy evaluation. Gofra can integrate with them as a future addendum.
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
| 74 | JIT user profile creation | No sync between Zitadel and Gofra. Profile created on first API call. No webhooks, no eventual consistency. |
| 75 | `zitadel_user_id TEXT` as PK | Zitadel IDs are opaque strings. Direct PK avoids surrogate key and simplifies joins. |
| 76 | Admin handlers proxy to Zitadel via service account | SPA doesn't get admin Zitadel credentials. Gofra enforces authz before forwarding. |
| 77 | No BFF/session-cookie default for browser clients | Keep the v1 surface smaller and backend auth validation consistent across browser, mobile, and desktop. Revisit later if needed. |
| 78 | `urn:zitadel:iam:org:projects:roles` scope | Includes roles in the token. No extra API call to fetch roles on every request. |
| 79 | Frontend permission checks are display-only | UI shows/hides buttons. Server always re-checks. Never trust the client. |
| 124 | `react-oidc-context` as the default React auth layer | One supported frontend integration for the generated SPA. Avoids multiple auth stacks in docs and generators. |
| 125 | `sessionStorage` for browser token storage | Survives normal reloads without persisting as broadly as `localStorage`. |
| 126 | `offline_access` + rotating refresh tokens, with one refresh attempt on bootstrap | Keeps the browser session usable without hidden retry loops or full redirects on every expiry. |
| 127 | Logout clears local auth state before Zitadel end-session redirect | Fail closed if network logout is interrupted. |
| 128 | Routes and RPCs are private by default | Public routes and RPCs must be explicitly allowlisted. |
| 129 | Native clients use system browser redirects and OS secure storage | Same OIDC family as the browser, but with platform-native token handling. |
