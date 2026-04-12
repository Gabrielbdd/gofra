# 04 — Durable Execution: Restate

> Parent: [Index](00-index.md) | Prev: [API Layer](03-api-layer.md) | Next: [Database](05-database.md)


## Design Principle

Restate is not an optional backend. It is the runtime for all durable operations
in Forge. The framework uses Restate's SDK directly — no wrappers, no abstraction
layers. Forge provides the HTTP layer, ORM, templating, validation, and developer
tooling. Restate provides durable execution, state machines, workflows, scheduling,
and reliable messaging.

The developer writes Restate handlers using the Restate Go SDK. Forge's job is to
make this ergonomic: scaffolding, auto-registration, dev server management, testing
helpers, and glue between HTTP and Restate.

---

## Architecture

```
                Browser / API Client
                        │
                        ▼
              ┌─────────────────────┐
              │   Forge HTTP Server │  :3000
              │   (chi router)      │
              │                     │
              │ • Routes & Middleware│
              │ • Sessions & Auth   │
              │ • Templ rendering   │
              │ • Validation        │
              │ • Static files      │
              │ • WebSocket proxy   │
              └──────┬──────────────┘
                     │ Restate Ingress Client
                     ▼
              ┌─────────────────────┐
              │   Restate Server    │  :8080 (ingress) / :9070 (UI)
              │   (single binary)   │
              │                     │
              │ • Journal / Log     │
              │ • K/V state store   │
              │ • Timer management  │
              │ • Invocation mgmt   │
              │ • Kafka ingress     │
              └──────┬──────────────┘
                     │ HTTP/2 bidirectional
                     ▼
              ┌─────────────────────┐
              │ Forge Service       │  :9080
              │ Endpoint            │
              │ (same Go binary)    │
              │                     │
              │ • Jobs (Services)   │
              │ • Entities (VOs)    │
              │ • Workflows         │
              │ • Scheduled tasks   │
              └─────────────────────┘
                     │
                     ▼
              ┌─────────────────────┐
              │    PostgreSQL       │
              │ (source of truth)   │
              └─────────────────────┘
```

One Go binary runs three servers: the HTTP server (for browsers/APIs), the Restate
service endpoint (receives invocations from Restate), and optionally auto-starts the
Restate Server in development.

---

## What Restate Replaces

| Old Forge Subsystem | Replaced By | Restate Primitive |
|---------------------|-------------|-------------------|
| Queue + Workers | Restate Services | `ServiceSend` → durable handler |
| Job retry/backoff | Restate retry policy | Service-level config |
| Job batching | `RunAsync` + futures | Concurrent durable tasks |
| Job chaining | Sequential `Run` calls | Journaled steps |
| Failed jobs table | Restate invocation state | Admin API + UI |
| Event dispatcher | `ServiceSend` / `ObjectSend` | One-way durable messages |
| Queued listeners | Restate Services | Each listener = a Service handler |
| Task scheduling / cron | Virtual Object + delayed self-messages | Durable timers |
| Feature flags (stateful) | Virtual Object with K/V state | `Get`/`Set` per flag entity |
| Rate limiting (per-entity) | Virtual Object single-writer | One execution at a time per key |
| Saga / compensation | `defer` + journaled steps | Terminal errors trigger compensation |
| Webhook waiting | Awakeables | Suspend, resolve from HTTP |
| Email verification flow | Workflow + Durable Promise | Suspend until user clicks link |
| Password reset flow | Workflow + Durable Promise | Suspend until user submits form |

## What Forge Keeps

| Subsystem | Why It Stays |
|-----------|-------------|
| HTTP Router (chi-based) | Restate doesn't serve HTML to browsers |
| Middleware pipeline | Sessions, CSRF, CORS, auth — HTTP concerns |
| ORM / Query Builder | Postgres is the source of truth for domain data. Restate K/V is for operational state. |
| Migrations & Seeds | Schema management is a DB concern |
| Validation | Request validation happens before durable execution |
| Templ rendering | Server-side HTML |
| Auth (sessions, tokens) | HTTP-layer identity |
| Authorization (policies) | Business rules, checked in HTTP handlers |
| File Storage | S3/local — orthogonal to execution |
| HTTP Client | Making external API calls (wrapped in `restate.Run` when inside handlers) |
| Cache (Redis/memory) | Response caching, fragment caching — HTTP perf concern |
| I18n | Localization is a rendering concern |
| Mail (building messages) | Template + send logic. Sending happens inside a Restate `Run` for durability. |
| Full-text Search | Scout-like abstraction over Postgres FTS / Meilisearch |
| Testing framework | Forge provides test helpers, wraps Restate's test environment |
| CLI tooling | Generators, dev server, deployment |
| Observability | Structured logging. Restate UI handles job/workflow visibility. |

---

## Project Structure

```
myapp/
├── cmd/app/main.go              # Boots HTTP + Restate endpoint
├── forge.yaml                   # Framework config
├── .env
│
├── app/
│   ├── handlers/                # HTTP handlers (controllers)
│   │   ├── posts_handler.go
│   │   ├── auth_handler.go
│   │   └── api/
│   │       └── posts_handler.go
│   │
│   ├── models/                  # Domain models + ORM
│   │   ├── user.go
│   │   └── post.go
│   │
│   ├── services/                # Restate Services (durable background work)
│   │   ├── mail_service.go      # Sends emails durably
│   │   ├── search_service.go    # Updates search indexes
│   │   └── webhook_service.go   # Delivers webhooks with retry
│   │
│   ├── objects/                 # Restate Virtual Objects (stateful entities)
│   │   ├── notification_feed.go # Per-user notification state
│   │   ├── rate_limiter.go      # Per-key rate limiting
│   │   └── scheduler.go         # Cron job scheduler
│   │
│   ├── workflows/               # Restate Workflows (multi-step processes)
│   │   ├── user_signup.go       # Registration + email verification
│   │   ├── password_reset.go    # Reset flow with token
│   │   └── order_checkout.go    # Payment + fulfillment
│   │
│   ├── middleware/
│   ├── policies/
│   ├── rules/                   # Custom validation rules
│   ├── resources/               # API resource transformers
│   └── mail/                    # Mail message builders
│
├── config/
│   ├── routes.go
│   └── app.go                   # Typed config struct
│
├── db/
│   ├── migrations/
│   ├── seeds/
│   └── factories/
│
├── resources/
│   ├── views/                   # Templ templates
│   ├── mail/                    # Email templates
│   └── lang/                    # i18n files
│
└── tests/
```

**Key convention**: `services/`, `objects/`, `workflows/` use the Restate SDK
directly. Everything else is normal Go. The folder name tells you what Restate
primitive it uses.

---

## Boot Sequence

```go
// cmd/app/main.go
package main

import (
    "myapp/app/handlers"
    "myapp/app/services"
    "myapp/app/objects"
    "myapp/app/workflows"
    "myapp/config"

    "github.com/myapp/forge"
)

func main() {
    app := forge.New()

    // Load config from .env + forge.yaml
    app.LoadConfig()

    // Setup database (ORM, migrations check)
    app.SetupDB()

    // Register HTTP routes
    app.HTTP(config.Routes)

    // Register Restate services — all in one place
    app.Restate(func(r *forge.Restate) {
        // Services (stateless durable handlers)
        r.Service(services.MailService{})
        r.Service(services.SearchIndexer{})
        r.Service(services.WebhookDelivery{})

        // Virtual Objects (stateful, keyed)
        r.Object(objects.NotificationFeed{})
        r.Object(objects.Scheduler{})

        // Workflows (exactly-once, multi-step)
        r.Workflow(workflows.UserSignup{})
        r.Workflow(workflows.PasswordReset{})
        r.Workflow(workflows.OrderCheckout{})
    })

    // Start everything
    //   - HTTP server on :3000
    //   - Restate service endpoint on :9080
    //   - In dev: auto-starts Restate Server, auto-registers services
    app.Start()
}
```

---

## How The Developer Writes Each Piece

### Background Jobs → Restate Services

Every "background job" in Laravel/Rails becomes a Restate Service handler.
The developer uses the Restate Go SDK directly.

```go
// app/services/mail_service.go
package services

import (
    "fmt"
    restate "github.com/restatedev/sdk-go"
    "myapp/app/mail"
    "myapp/app/models"
    "myapp/pkg/mailer"
    "myapp/pkg/db"
)

type MailService struct{}

type SendMailRequest struct {
    To       string         `json:"to"`
    Template string         `json:"template"`
    Data     map[string]any `json:"data"`
}

func (MailService) Send(ctx restate.Context, req SendMailRequest) error {
    // This entire operation is durable.
    // If it crashes after building the message but before sending,
    // Restate replays, skips the build, retries only the send.

    msg, err := restate.Run(ctx, func(ctx restate.RunContext) (*mailer.Message, error) {
        return mail.Build(req.Template, req.Data)
    }, restate.WithName("build-message"))
    if err != nil {
        return err
    }

    _, err = restate.Run(ctx, func(ctx restate.RunContext) (restate.Void, error) {
        return restate.Void{}, mailer.Send(req.To, msg)
    }, restate.WithName("send-smtp"))

    return err
}
```

**Dispatching from an HTTP handler:**

```go
// app/handlers/auth_handler.go
func (h AuthHandler) Register(c *forge.Context) error {
    var input RegisterInput
    if err := c.BindAndValidate(&input); err != nil {
        return err
    }

    user, err := h.createUser(c, input)
    if err != nil {
        return err
    }

    // Start the signup workflow (exactly-once per user ID)
    c.Restate().Workflow("UserSignup", fmt.Sprint(user.ID), "Run").
        Send(workflows.SignupRequest{
            UserID: user.ID,
            Email:  user.Email,
        })

    // Send welcome email (durable, fire-and-forget)
    c.Restate().Service("MailService", "Send").
        Send(services.SendMailRequest{
            To:       user.Email,
            Template: "welcome",
            Data:     map[string]any{"name": user.Name},
        })

    return c.Redirect("/check-your-email")
}
```

`c.Restate()` returns a thin wrapper around the Restate ingress client,
pre-configured with the Restate server URL from `forge.yaml`. It provides:

```go
type RestateClient struct { /* ... */ }

// Fire-and-forget to a Service
func (r *RestateClient) Service(name, handler string) *ServiceSender
// Fire-and-forget to a Virtual Object
func (r *RestateClient) Object(name, key, handler string) *ObjectSender
// Start/signal a Workflow
func (r *RestateClient) Workflow(name, key, handler string) *WorkflowSender

// Each sender has:
func (s *Sender) Send(payload any, opts ...restate.CallOption) error
func (s *Sender) Request(ctx context.Context, payload any) ([]byte, error) // wait for response
```

This is a **very thin wrapper** — it just hides the `restateingress.NewClient(url)`
boilerplate and reads the URL from config. No magic.

---

### Events & Listeners → Durable One-Way Messages

In Laravel, you define events and map them to listeners. In Forge+Restate,
an "event" is just a one-way message to one or more Restate Services.

```go
// config/events.go
package config

func Events() forge.EventMap {
    return forge.EventMap{
        "post.created": {
            {Service: "SearchIndexer", Handler: "IndexPost"},
            {Service: "WebhookDelivery", Handler: "DeliverPostCreated"},
            {Service: "NotificationFeed", Handler: "OnNewPost", ObjectKey: "author:{AuthorID}"},
        },
        "post.published": {
            {Service: "MailService", Handler: "Send"},   // needs transform
            {Service: "SearchIndexer", Handler: "IndexPost"},
        },
        "user.created": {
            // The signup workflow handles everything — no extra listeners needed
        },
    }
}
```

**Dispatching:**

```go
// In an HTTP handler or anywhere with access to the Restate client:
c.Events().Dispatch("post.created", PostCreatedPayload{
    PostID:   post.ID,
    AuthorID: post.AuthorID,
    Title:    post.Title,
})

// Under the hood, this sends one-way messages to each registered listener:
// → ServiceSend("SearchIndexer", "IndexPost").Send(payload)
// → ServiceSend("WebhookDelivery", "DeliverPostCreated").Send(payload)
// → ObjectSend("NotificationFeed", "author:42", "OnNewPost").Send(payload)
```

Every listener execution is individually durable. If `WebhookDelivery` crashes,
`SearchIndexer` isn't affected. Restate retries only the failed one.

**Inline listeners** (synchronous, need to run before HTTP response) stay as
plain Go function calls — no Restate involvement:

```go
// In the HTTP handler itself:
func (h PostsHandler) Create(c *forge.Context) error {
    post := // ... save to DB ...

    // Inline: must happen before response (e.g., update cache, set flash)
    cache.Forget("posts:latest")
    c.Flash("success", "Post created!")

    // Async: durable delivery via Restate
    c.Events().Dispatch("post.created", PostCreatedPayload{...})

    return c.Redirect("/posts/" + post.Slug)
}
```

---

### Notifications → Virtual Object Per User

```go
// app/objects/notification_feed.go
package objects

import (
    restate "github.com/restatedev/sdk-go"
    "myapp/pkg/db"
    "myapp/pkg/mailer"
    "myapp/pkg/slack"
)

type NotificationFeed struct{}

type NotifyRequest struct {
    Title    string         `json:"title"`
    Body     string         `json:"body"`
    Channel  string         `json:"channel,omitempty"` // "", "mail", "slack", "all"
    Data     map[string]any `json:"data,omitempty"`
}

// Send is an exclusive handler — serialized per user key.
// Two notifications for the same user NEVER race.
func (NotificationFeed) Send(ctx restate.ObjectContext, req NotifyRequest) error {
    userID := restate.Key(ctx)

    // Always store in DB notification log
    _, err := restate.Run(ctx, func(ctx restate.RunContext) (restate.Void, error) {
        return restate.Void{}, db.StoreNotification(userID, req.Title, req.Body, req.Data)
    }, restate.WithName("store"))
    if err != nil {
        return err
    }

    // Increment unread count in Restate K/V state
    unread, _ := restate.Get[int](ctx, "unread_count")
    restate.Set(ctx, "unread_count", unread+1)

    // Send to external channels
    if req.Channel == "mail" || req.Channel == "all" {
        restate.Run(ctx, func(ctx restate.RunContext) (restate.Void, error) {
            email, _ := db.GetUserEmail(userID)
            return restate.Void{}, mailer.Send(email, req.Title, req.Body)
        }, restate.WithName("email"))
    }

    if req.Channel == "slack" || req.Channel == "all" {
        restate.Run(ctx, func(ctx restate.RunContext) (restate.Void, error) {
            webhook, _ := db.GetUserSlackWebhook(userID)
            if webhook != "" {
                return restate.Void{}, slack.Send(webhook, req.Title, req.Body)
            }
            return restate.Void{}, nil
        }, restate.WithName("slack"))
    }

    return nil
}

// GetUnreadCount — shared handler, concurrent reads allowed
func (NotificationFeed) GetUnreadCount(ctx restate.ObjectSharedContext) (int, error) {
    return restate.Get[int](ctx, "unread_count")
}

// MarkRead — exclusive handler
func (NotificationFeed) MarkRead(ctx restate.ObjectContext) error {
    restate.Set(ctx, "unread_count", 0)
    return nil
}
```

**From HTTP handlers:**

```go
// Send notification
c.Restate().Object("NotificationFeed", userID, "Send").Send(NotifyRequest{
    Title:   "New comment on your post",
    Body:    comment.Body[:100],
    Channel: "all",
})

// Read notification count (for navbar badge)
func (h DashboardHandler) Show(c *forge.Context) error {
    // Request-response call to Restate (waits for result)
    count, err := forge.RestateRequest[int](c, "NotificationFeed", c.UserID(), "GetUnreadCount", nil)
    if err != nil {
        count = 0 // graceful degradation
    }
    return c.Render("dashboard", forge.Map{"unread": count})
}
```

---

### Auth Flows → Workflows

Email verification as a Restate Workflow — the cleanest version of this pattern:

```go
// app/workflows/user_signup.go
package workflows

import (
    "fmt"
    restate "github.com/restatedev/sdk-go"
    "myapp/pkg/db"
    "myapp/pkg/mailer"
)

type UserSignup struct{}

type SignupRequest struct {
    UserID int64  `json:"user_id"`
    Email  string `json:"email"`
}

// Run executes exactly once per workflow key (= user ID string)
func (UserSignup) Run(ctx restate.WorkflowContext, req SignupRequest) (bool, error) {
    // Step 1: Generate verification token (deterministic — same on replay)
    token := restate.UUID(ctx).String()

    // Step 2: Store token in DB
    _, err := restate.Run(ctx, func(ctx restate.RunContext) (restate.Void, error) {
        return restate.Void{}, db.StoreVerificationToken(req.UserID, token)
    }, restate.WithName("store-token"))
    if err != nil {
        return false, err
    }

    // Step 3: Send verification email
    _, err = restate.Run(ctx, func(ctx restate.RunContext) (restate.Void, error) {
        link := fmt.Sprintf("https://myapp.com/verify/%d/%s", req.UserID, token)
        return restate.Void{}, mailer.Send(req.Email, "Verify your email", link)
    }, restate.WithName("send-email"))
    if err != nil {
        return false, err
    }

    // Step 4: SUSPEND — wait for the user to click the link
    // No resources consumed. On FaaS, the function shuts down.
    // Restate wakes it when the promise is resolved.
    clickedToken, err := restate.Promise[string](ctx, "email-verified").Result()
    if err != nil {
        return false, err
    }

    // Step 5: Validate
    if clickedToken != token {
        return false, restate.TerminalErrorf("invalid token")
    }

    // Step 6: Mark verified
    _, err = restate.Run(ctx, func(ctx restate.RunContext) (restate.Void, error) {
        return restate.Void{}, db.MarkUserVerified(req.UserID)
    }, restate.WithName("mark-verified"))

    return err == nil, err
}

// VerifyEmail — called when user clicks the link. Resolves the promise.
func (UserSignup) VerifyEmail(ctx restate.WorkflowSharedContext, token string) error {
    return restate.Promise[string](ctx, "email-verified").Resolve(token)
}
```

**The HTTP handler that receives the verification click:**

```go
// config/routes.go
r.Get("/verify/{userID}/{token}", handlers.VerifyEmail)

// app/handlers/auth_handler.go
func VerifyEmail(c *forge.Context) error {
    userID := c.Param("userID")
    token := c.Param("token")

    // Signal the workflow — resolves the durable promise
    err := c.Restate().Workflow("UserSignup", userID, "VerifyEmail").Send(token)
    if err != nil {
        return c.Error(err)
    }

    c.Flash("success", "Email verified! You can now log in.")
    return c.Redirect("/login")
}
```

No tokens table, no expiration cron job, no race conditions. Six steps, one file.

---

### Scheduling → Virtual Object

```go
// app/objects/scheduler.go
package objects

import (
    "time"
    restate "github.com/restatedev/sdk-go"
    "github.com/robfig/cron/v3"
)

type Scheduler struct{}

type ScheduleRequest struct {
    CronExpr string `json:"cron_expr"`
    Service  string `json:"service"`
    Handler  string `json:"handler"`
    Payload  any    `json:"payload,omitempty"`
}

type ScheduleInfo struct {
    Request      ScheduleRequest `json:"request"`
    NextRun      time.Time       `json:"next_run"`
    InvocationID string          `json:"invocation_id"`
}

func (Scheduler) Create(ctx restate.ObjectContext, req ScheduleRequest) (*ScheduleInfo, error) {
    if existing, _ := restate.Get[*ScheduleInfo](ctx, "info"); existing != nil {
        return nil, restate.TerminalErrorf("schedule already exists for key %s", restate.Key(ctx))
    }
    return scheduleNext(ctx, req)
}

func (Scheduler) Tick(ctx restate.ObjectContext) error {
    info, _ := restate.Get[*ScheduleInfo](ctx, "info")
    if info == nil {
        return restate.TerminalErrorf("no schedule found")
    }

    // Execute the scheduled work (durable one-way message)
    restate.ServiceSend(ctx, info.Request.Service, info.Request.Handler).
        Send(info.Request.Payload)

    _, err := scheduleNext(ctx, info.Request)
    return err
}

func (Scheduler) Cancel(ctx restate.ObjectContext) error {
    info, _ := restate.Get[*ScheduleInfo](ctx, "info")
    if info != nil {
        restate.CancelInvocation(ctx, info.InvocationID)
    }
    restate.ClearAll(ctx)
    return nil
}

func (Scheduler) GetInfo(ctx restate.ObjectSharedContext) (*ScheduleInfo, error) {
    return restate.Get[*ScheduleInfo](ctx, "info")
}

func scheduleNext(ctx restate.ObjectContext, req ScheduleRequest) (*ScheduleInfo, error) {
    parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
    sched, err := parser.Parse(req.CronExpr)
    if err != nil {
        return nil, restate.TerminalErrorf("invalid cron: %v", err)
    }

    now, _ := restate.Run(ctx, func(ctx restate.RunContext) (time.Time, error) {
        return time.Now(), nil
    })

    next := sched.Next(now)
    delay := next.Sub(now)

    handle := restate.ObjectSend(ctx, "Scheduler", restate.Key(ctx), "Tick").
        Send(nil, restate.WithDelay(delay))

    info := &ScheduleInfo{
        Request:      req,
        NextRun:      next,
        InvocationID: handle.GetInvocationId(),
    }
    restate.Set(ctx, "info", info)
    return info, nil
}
```

**Forge registers schedules at boot from config:**

```go
// config/schedules.go
func Schedules() []forge.Schedule {
    return []forge.Schedule{
        {Key: "cleanup", Cron: "0 2 * * *", Service: "CleanupService", Handler: "PruneTokens"},
        {Key: "digest", Cron: "0 9 * * 1", Service: "DigestService", Handler: "SendWeekly"},
        {Key: "health", Cron: "*/15 * * * *", Service: "HealthService", Handler: "CheckAPIs"},
    }
}

// On app.Start(), Forge calls:
// for _, s := range config.Schedules() {
//     restateClient.Object("Scheduler", s.Key, "Create").Send(ScheduleRequest{...})
// }
// The Create handler is idempotent — "schedule already exists" is fine.
```

---

## The HTTP ↔ Restate Bridge

The glue between Forge's HTTP layer and Restate is minimal. The `forge.Context`
gets a `.Restate()` method that returns a pre-configured ingress client:

```go
// forge/context.go
func (c *Context) Restate() *RestateClient {
    return c.app.restateClient // initialized once at boot from forge.yaml
}

// forge/restate_client.go
type RestateClient struct {
    client *restateingress.Client
}

func NewRestateClient(ingressURL string) *RestateClient {
    return &RestateClient{
        client: restateingress.NewClient(ingressURL),
    }
}

func (r *RestateClient) Service(name, handler string) *Sender {
    return &Sender{kind: "service", client: r.client, name: name, handler: handler}
}

func (r *RestateClient) Object(name, key, handler string) *Sender {
    return &Sender{kind: "object", client: r.client, name: name, key: key, handler: handler}
}

func (r *RestateClient) Workflow(name, key, handler string) *Sender {
    return &Sender{kind: "workflow", client: r.client, name: name, key: key, handler: handler}
}

type Sender struct {
    kind, name, key, handler string
    client *restateingress.Client
}

func (s *Sender) Send(payload any, opts ...restate.CallOption) error {
    ctx := context.Background()
    switch s.kind {
    case "service":
        _, err := restateingress.ServiceSend[any](s.client, s.name, s.handler).
            Send(ctx, payload, opts...)
        return err
    case "object":
        _, err := restateingress.ObjectSend[any](s.client, s.name, s.key, s.handler).
            Send(ctx, payload, opts...)
        return err
    case "workflow":
        _, err := restateingress.WorkflowSend[any](s.client, s.name, s.key, s.handler).
            Send(ctx, payload, opts...)
        return err
    }
    return nil
}
```

That's it. ~60 lines of glue code. No framework magic.

---

## The CLI

```bash
# === Project ===
forge new myapp                              # scaffold (includes Restate in docker-compose)
forge dev                                    # HTTP + Restate service endpoint + auto-start Restate Server
forge build                                  # compile binary (HTTP + Restate endpoint)

# === Generators ===
forge generate model Post title:string body:text
forge generate handler Posts --resource
forge generate migration add_views_to_posts

# Restate-specific generators:
forge generate service ProcessPodcast        # → app/services/process_podcast.go
forge generate object ShoppingCart           # → app/objects/shopping_cart.go
forge generate workflow OrderCheckout        # → app/workflows/order_checkout.go
forge generate schedule daily-cleanup        # → adds to config/schedules.go

# === Database ===
forge migrate
forge migrate rollback
forge db seed

# === Restate ===
forge restate status                         # show registered services, pending invocations
forge restate invocations                    # list active/failed invocations
forge restate retry <invocation-id>          # retry a failed invocation
forge restate cancel <invocation-id>         # cancel an invocation
forge restate purge <invocation-id>          # purge completed invocation
forge restate ui                             # open Restate UI in browser

# These are thin wrappers around the `restate` CLI / admin API.
# The Restate UI at :9070 is the primary debugging tool.

# === Other ===
forge routes                                 # list HTTP routes
forge tinker                                 # REPL
forge build                                  # single binary
```

### What `forge generate service` produces:

```bash
$ forge generate service ProcessPodcast
```

```go
// app/services/process_podcast.go
package services

import (
    restate "github.com/restatedev/sdk-go"
)

type ProcessPodcast struct{}

type ProcessPodcastRequest struct {
    // TODO: define your input fields
}

func (ProcessPodcast) Handle(ctx restate.Context, req ProcessPodcastRequest) error {
    // TODO: implement your durable logic
    //
    // Use restate.Run(ctx, func(ctx restate.RunContext) (T, error) { ... })
    // to wrap side effects (DB calls, HTTP requests, etc.)
    //
    // Each Run block is journaled — if this handler crashes,
    // completed Run blocks are NOT re-executed on retry.

    return nil
}
```

And auto-adds to `cmd/app/main.go`:

```go
r.Service(services.ProcessPodcast{})
```

### What `forge generate workflow` produces:

```bash
$ forge generate workflow OrderCheckout
```

```go
// app/workflows/order_checkout.go
package workflows

import (
    restate "github.com/restatedev/sdk-go"
)

type OrderCheckout struct{}

type OrderCheckoutRequest struct {
    // TODO: define your input fields
}

// Run executes exactly once per workflow ID.
func (OrderCheckout) Run(ctx restate.WorkflowContext, req OrderCheckoutRequest) (bool, error) {
    // TODO: implement your workflow steps
    //
    // Use restate.Run(ctx, ...) for durable side effects
    // Use restate.Promise[T](ctx, "name").Result() to wait for external signals
    // Use restate.Sleep(ctx, duration) for durable timers
    //
    // This handler runs EXACTLY ONCE per workflow key.

    return true, nil
}

// TODO: Add shared handlers for signaling/querying the workflow
// func (OrderCheckout) Cancel(ctx restate.WorkflowSharedContext) error { ... }
```

---

## Testing

### Testing HTTP Handlers (unchanged from original design)

```go
func TestCreatePost(t *testing.T) {
    app := forge.TestApp(t)
    user := factory.Create[User](app.DB)

    // Restate calls are captured, not executed
    app.FakeRestate()

    resp := app.ActingAs(user).
        PostJSON("/api/v1/posts", map[string]any{
            "title": "My Post",
            "body":  "Hello world",
        })

    resp.AssertStatus(201)
    app.AssertDatabaseHas("posts", map[string]any{"title": "My Post"})

    // Assert the right Restate messages were sent
    app.AssertRestateSent("SearchIndexer", "IndexPost")
    app.AssertRestateSent("MailService", "Send")
}
```

`app.FakeRestate()` replaces the Restate ingress client with one that captures
all messages instead of sending them. Like Laravel's `Queue::fake()`.

### Testing Restate Handlers (durable logic)

```go
func TestProcessPodcast(t *testing.T) {
    // Starts a real Restate Server in Docker via testcontainers
    env := restatetest.Start(t, restate.Reflect(services.ProcessPodcast{}))
    client := env.Ingress()

    _, err := restateingress.Service[services.ProcessPodcastRequest, restate.Void](
        client, "ProcessPodcast", "Handle",
    ).Request(t.Context(), services.ProcessPodcastRequest{
        PodcastID: 1,
        FileURL:   "s3://test-bucket/test.wav",
    })

    require.NoError(t, err)
    // Assert side effects happened (DB state, files created, etc.)
}
```

This uses the Restate Go SDK's testing package directly. No wrapper needed.
Tests are slower (Docker) but test real durable execution behavior.

### Testing Workflows

```go
func TestUserSignupWorkflow(t *testing.T) {
    env := restatetest.Start(t, restate.Reflect(workflows.UserSignup{}))
    client := env.Ingress()

    // Start the workflow (async)
    handle, err := restateingress.WorkflowSend[workflows.SignupRequest](
        client, "UserSignup", "user-42", "Run",
    ).Send(t.Context(), workflows.SignupRequest{
        UserID: 42,
        Email:  "alice@example.com",
    })
    require.NoError(t, err)

    // Simulate user clicking verification link
    time.Sleep(100 * time.Millisecond) // let workflow reach the promise
    _, err = restateingress.Workflow[string, restate.Void](
        client, "UserSignup", "user-42", "VerifyEmail",
    ).Request(t.Context(), "the-expected-token")
    require.NoError(t, err)

    // Wait for workflow completion
    result, err := restateingress.InvocationById[bool](client, handle.Id()).
        Attach(t.Context())
    require.NoError(t, err)
    require.True(t, result)
}
```

---

## Development Experience

```bash
$ forge dev

  ╭──────────────────────────────────────────────╮
  │  Forge v1.0                                  │
  │                                              │
  │  HTTP:     http://localhost:3000              │
  │  Restate:  http://localhost:8080  (ingress)   │
  │  UI:       http://localhost:9070  (dashboard) │
  │  Services: http://localhost:9080  (endpoint)  │
  │                                              │
  │  Restate services registered:                │
  │    ✓ MailService           (Service)         │
  │    ✓ SearchIndexer         (Service)         │
  │    ✓ WebhookDelivery       (Service)         │
  │    ✓ NotificationFeed      (Virtual Object)  │
  │    ✓ Scheduler             (Virtual Object)  │
  │    ✓ UserSignup            (Workflow)        │
  │    ✓ PasswordReset         (Workflow)        │
  │    ✓ OrderCheckout         (Workflow)        │
  │                                              │
  │  Schedules:                                  │
  │    ✓ cleanup  0 2 * * *  → CleanupService    │
  │    ✓ digest   0 9 * * 1  → DigestService     │
  │    ✓ health   */15 * * * * → HealthService   │
  │                                              │
  │  Watching for changes...                     │
  ╰──────────────────────────────────────────────╯
```

`forge dev` does:
1. Starts the Restate Server binary (downloads on first run)
2. Starts the HTTP server with hot reload
3. Starts the Restate service endpoint
4. Auto-registers the service endpoint with the Restate Server
5. Registers scheduled tasks

The Restate UI at `:9070` gives you:
- Every invocation (pending, running, completed, failed, suspended)
- Journal entries for each invocation (each `Run` step)
- K/V state for every Virtual Object
- Pending timers and delayed messages
- Cancel, retry, or purge any invocation

This replaces Laravel Telescope and Horizon for background job visibility, with
**much deeper introspection** (step-level instead of job-level).

---

## Production Deployment

```bash
forge build
# Produces a single binary that runs:
#   - HTTP server
#   - Restate service endpoint
#
# Does NOT include the Restate Server — that runs separately in production.
```

Production setup:
1. Run the Restate Server (single binary or HA cluster)
2. Run your Forge app (single binary)
3. The Forge app connects to Restate on startup and registers its services

```yaml
# forge.yaml (production)
app:
  env: production
  port: 3000

restate:
  ingress_url: "http://restate-server:8080"
  service_port: 9080
  auto_start: false  # Restate Server runs separately in production
```

Deployment is two binaries + Postgres. No Redis, no queue workers, no cron daemons,
no message broker. The Restate Server replaces all of them.

---

## What Developers Need to Learn

Coming from Laravel/Rails, developers need to internalize three concepts:

1. **Side effects must be wrapped in `restate.Run()`** — Any operation that has
   external effects (DB writes, API calls, sending emails) must go inside a `Run`
   block when used in a Restate handler. This is the journal checkpoint. If you
   forget, the operation re-executes on every retry, which means duplicate writes,
   duplicate charges, duplicate emails.

2. **Restate handlers must be deterministic** — No `time.Now()`, no `rand.Int()`,
   no reading environment variables that might change. Use `restate.UUID(ctx)` for
   UUIDs, capture time inside `restate.Run()`. This is because Restate replays
   your code on recovery, and non-deterministic operations would produce different
   results on replay.

3. **Virtual Objects serialize per key** — If you send 10 messages to the same
   Virtual Object key, they execute one at a time, in order. This is powerful
   (no race conditions) but means you shouldn't do expensive work in a VO handler
   if you need high throughput per key. Delegate heavy work to a Service via
   one-way message.

Everything else — HTTP routing, ORM, templates, validation, auth — works exactly
like any other Go web framework. Restate only touches the "durable operations" layer.
