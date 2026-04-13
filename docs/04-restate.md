# 04 — Durable Execution: Restate

> Parent: [Index](00-index.md) | Prev: [API Layer](03-api-layer.md) | Next: [Database](05-database.md)


## Design Principle

Restate is not an optional backend. It is the runtime for all durable operations
in Gofra. The framework uses Restate's SDK directly — no wrappers, no abstraction
layers. Gofra provides the HTTP/RPC layer, data access, validation, auth/authz
integration, static asset serving, and developer tooling. Restate provides
durable execution, state machines, workflows, scheduling, and reliable
messaging.

The developer writes Restate handlers using the Restate Go SDK. Gofra's job is to
make this ergonomic: scaffolding, auto-registration, dev server management, testing
helpers, and glue between HTTP and Restate.

---

## Architecture

```
                Browser / API Client
                        │
                        ▼
              ┌─────────────────────┐
              │   Gofra HTTP Server │  :3000
              │   (chi router)      │
              │                     │
              │ • Routes & Middleware│
              │ • Connect handlers  │
              │ • Sessions & Auth   │
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
              │ Gofra Service       │  :9080
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

| Old Gofra Subsystem | Replaced By | Restate Primitive |
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
| Payment confirmation flow | Workflow + Durable Promise | Suspend until provider webhook arrives |
| Manual approval flow | Workflow + Durable Promise | Suspend until a user or admin confirms |

## What Gofra Keeps

| Subsystem | Why It Stays |
|-----------|-------------|
| HTTP/RPC layer (chi + Connect) | Restate doesn't serve public API traffic or static assets |
| Middleware pipeline | Sessions, CSRF, CORS, auth — HTTP concerns |
| Data access / query layer | Postgres is the source of truth for domain data. Restate K/V is for operational state. |
| Migrations & Seeds | Schema management is a DB concern |
| Validation | Request validation happens before durable execution |
| Auth (sessions, tokens) | HTTP-layer identity |
| Authorization (policies) | Business rules, checked in RPC or HTTP handlers |
| File Storage | S3/local — orthogonal to execution |
| HTTP Client | Making external API calls (wrapped in `restate.Run` when inside handlers) |
| Cache (Redis/memory) | Response caching, fragment caching — HTTP perf concern |
| I18n | Localization is a rendering concern |
| Mail (building messages) | Template + send logic. Sending happens inside a Restate `Run` for durability. |
| Full-text Search | Scout-like abstraction over Postgres FTS / Meilisearch |
| Testing framework | Gofra provides test helpers, wraps Restate's test environment |
| CLI tooling | Generators, dev server, deployment |
| Observability | Structured logging. Restate UI handles job/workflow visibility. |

---

## Project Structure

```
myapp/
├── cmd/app/main.go              # Boots HTTP + Restate endpoint
├── gofra.yaml                   # Framework config
├── sqlc.yaml                    # sqlc configuration
├── .env
│
├── app/
│   ├── rpc/                     # Connect RPC service implementations
│   │   ├── posts_service.go
│   │   ├── auth_service.go
│   │   ├── users_service.go
│   │   └── converters.go        # sqlc rows -> proto messages
│   │
│   ├── http/                    # Plain HTTP handlers for webhooks/callbacks
│   │   └── payment_webhook.go
│   │
│   ├── models/                  # Optional domain helpers, not the query layer
│   │   ├── user.go
│   │   └── post.go
│   │
│   ├── authz/
│   │   └── permissions.go
│   │
│   ├── services/                # Restate Services (durable background work)
│   │   ├── mail_service.go      # Sends emails durably
│   │   ├── search_indexer.go    # Updates search indexes
│   │   └── webhook_service.go   # Delivers webhooks with retry
│   │
│   ├── objects/                 # Restate Virtual Objects (stateful entities)
│   │   ├── notification_feed.go # Per-user notification state
│   │   └── scheduler.go         # Cron job scheduler
│   │
│   ├── workflows/               # Restate Workflows (multi-step processes)
│   │   ├── order_checkout.go    # Payment callback + fulfillment
│   │   ├── payout_approval.go   # Human approval flow
│   │   └── report_export.go     # Long-running export pipeline
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
│   ├── queries/
│   ├── sqlc/
│   ├── seeds/
│   └── factories/
│
├── resources/
│   ├── mail/                    # Email templates
│   └── lang/                    # i18n files
│
├── web/                         # React SPA
│   ├── src/
│   └── dist/
│
└── tests/
```

**Key convention**: `services/`, `objects/`, `workflows/` use the Restate SDK
directly. Database access still goes through sqlc-generated queries. The folder
name tells you which Restate primitive owns the durable logic.

---

## Boot Sequence

```go
// cmd/app/main.go
package main

// imports omitted for brevity

func main() {
    ctx := context.Background()
    cfg, _ := config.Load()
    db, _ := gofra.OpenDB(cfg.Database)
    queries := sqlc.New(db)
    restateClient := gofra.NewRestateClient(cfg.Restate.IngressURL)

    mux := chi.NewRouter()
    interceptors := connect.WithInterceptors(
        gofra.OTELInterceptor(),
        protovalidate.NewInterceptor(),
        gofra.AuthInterceptor(cfg.Auth),
    )

    postsPath, postsHandler := postsv1connect.NewPostsServiceHandler(
        &rpc.PostsService{Queries: queries, Restate: restateClient},
        interceptors,
    )
    mux.Mount(postsPath, postsHandler)

    gofra.Serve(ctx, gofra.ServeConfig{
        HTTPHandler: mux,
        HTTPAddr:    fmt.Sprintf(":%d", cfg.App.Port),
        RestateAddr: fmt.Sprintf(":%d", cfg.Restate.ServicePort),
        RestateSetup: func() (*server.Restate, error) {
            rs := server.NewRestate()
            rs.Bind(restate.Reflect(services.MailService{Queries: queries}))
            rs.Bind(restate.Reflect(services.SearchIndexer{Queries: queries}))
            rs.Bind(restate.Reflect(services.WebhookDelivery{Queries: queries}))
            rs.Bind(restate.Reflect(objects.NotificationFeed{}))
            rs.Bind(restate.Reflect(objects.Scheduler{}))
            rs.Bind(restate.Reflect(workflows.OrderCheckout{Queries: queries}))
            rs.Bind(restate.Reflect(workflows.PayoutApproval{Queries: queries}))
            return rs, nil
        },
    })
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

**Dispatching from a Connect RPC handler:**

```go
// app/rpc/posts_service.go
func (s *PostsService) CreatePost(
    ctx context.Context,
    req *connect.Request[postsv1.CreatePostRequest],
) (*connect.Response[postsv1.Post], error) {
    post, err := s.createPost(ctx, req.Msg.Post)
    if err != nil {
        return nil, err
    }

    if err := s.Restate.Service("SearchIndexer", "IndexPost").Send(
        ctx,
        services.IndexPostRequest{PostID: post.ID},
    ); err != nil {
        return nil, err
    }

    if err := s.Restate.Object("NotificationFeed", fmt.Sprint(post.AuthorID), "Send").Send(
        ctx,
        objects.NotifyRequest{
            Title:   "Post published",
            Body:    post.Title,
            Channel: "all",
        },
    ); err != nil {
        return nil, err
    }

    return connect.NewResponse(postToProto(post)), nil
}
```

The injected `RestateClient` is a thin wrapper around the Restate ingress client,
pre-configured with the Restate server URL from `gofra.yaml`. It provides:

```go
type RestateClient struct { /* ... */ }

// Fire-and-forget to a Service
func (r *RestateClient) Service(name, handler string) *ServiceSender
// Fire-and-forget to a Virtual Object
func (r *RestateClient) Object(name, key, handler string) *ObjectSender
// Start/signal a Workflow
func (r *RestateClient) Workflow(name, key, handler string) *WorkflowSender

// Each sender has:
func (s *Sender) Send(ctx context.Context, payload any, opts ...restate.CallOption) error
func (s *Sender) Request(ctx context.Context, payload any) ([]byte, error) // wait for response
```

This is a **very thin wrapper** — it just hides the `restateingress.NewClient(url)`
boilerplate and reads the URL from config. No magic.

---

### Events & Listeners → Durable One-Way Messages

In Laravel, you define events and map them to listeners. In Gofra+Restate,
an "event" is just a one-way message to one or more Restate Services.

```go
// config/events.go
package config

func Events() gofra.EventMap {
    return gofra.EventMap{
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
            // Zitadel + JIT profile creation handle identity bootstrap
        },
    }
}
```

**Dispatching:**

```go
// In a Connect RPC handler or anywhere with access to the Restate client:
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
// In the RPC handler itself:
func (s *PostsService) CreatePost(
    ctx context.Context,
    req *connect.Request[postsv1.CreatePostRequest],
) (*connect.Response[postsv1.Post], error) {
    post := // ... save to DB ...

    // Inline: must happen before response (e.g., update cache)
    cache.Forget("posts:latest")

    // Async: durable delivery via Restate
    s.events.Dispatch("post.created", PostCreatedPayload{...})

    return connect.NewResponse(postToProto(post)), nil
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

**From RPC or HTTP handlers:**

```go
// Send notification
if err := s.Restate.Object("NotificationFeed", userID, "Send").Send(ctx, NotifyRequest{
    Title:   "New comment on your post",
    Body:    preview,
    Channel: "all",
}); err != nil {
    return nil, err
}

// Read notification count for an API badge endpoint
func (s *NotificationsService) GetUnreadCount(
    ctx context.Context,
    req *connect.Request[notificationsv1.GetUnreadCountRequest],
) (*connect.Response[notificationsv1.GetUnreadCountResponse], error) {
    count, err := gofra.RestateRequest[int](ctx, s.Restate, "NotificationFeed", req.Msg.UserId, "GetUnreadCount", nil)
    if err != nil {
        count = 0 // graceful degradation
    }
    return connect.NewResponse(&notificationsv1.GetUnreadCountResponse{
        Count: int32(count),
    }), nil
}
```

---

### Checkout & External Callbacks → Workflows

Order checkout as a Restate Workflow — the cleanest version of this pattern:

```go
// app/workflows/order_checkout.go
package workflows

import (
    restate "github.com/restatedev/sdk-go"
    "myapp/pkg/db"
)

type OrderCheckout struct{}

type CheckoutRequest struct {
    OrderID int64  `json:"order_id"`
    PaymentRef string `json:"payment_ref"`
}

type PaymentResult struct {
    Succeeded bool   `json:"succeeded"`
    ProviderID string `json:"provider_id"`
}

// Run executes exactly once per workflow key (= order ID string)
func (OrderCheckout) Run(ctx restate.WorkflowContext, req CheckoutRequest) (bool, error) {
    // Step 1: mark the order as waiting for payment confirmation
    _, err := restate.Run(ctx, func(ctx restate.RunContext) (restate.Void, error) {
        return restate.Void{}, db.MarkOrderPending(req.OrderID, req.PaymentRef)
    }, restate.WithName("mark-pending"))
    if err != nil {
        return false, err
    }

    // Step 2: suspend until the payment provider confirms or rejects the charge
    result, err := restate.Promise[PaymentResult](ctx, "payment-confirmed").Result()
    if err != nil {
        return false, err
    }

    // Step 3: fail permanently if the payment did not succeed
    if !result.Succeeded {
        return false, restate.TerminalErrorf("payment failed")
    }

    // Step 4: mark the order as paid
    _, err = restate.Run(ctx, func(ctx restate.RunContext) (restate.Void, error) {
        return restate.Void{}, db.MarkOrderPaid(req.OrderID, result.ProviderID)
    }, restate.WithName("mark-paid"))

    return err == nil, err
}

// ConfirmPayment — called by a webhook handler. Resolves the promise.
func (OrderCheckout) ConfirmPayment(ctx restate.WorkflowSharedContext, result PaymentResult) error {
    return restate.Promise[PaymentResult](ctx, "payment-confirmed").Resolve(result)
}
```

**The plain HTTP handler that receives the provider webhook:**

```go
// config/routes.go
r.Post("/webhooks/payments", http.PaymentWebhook)

// app/http/payment_webhook.go
func PaymentWebhook(w http.ResponseWriter, r *http.Request) {
    event := decodeProviderEvent(r)

    _ = restateClient.Workflow(
        "OrderCheckout",
        strconv.FormatInt(event.OrderID, 10),
        "ConfirmPayment",
    ).Send(workflows.PaymentResult{
        Succeeded: event.Status == "paid",
        ProviderID: event.PaymentID,
    })

    w.WriteHeader(http.StatusAccepted)
}
```

No polling worker, no retry cron, no "restart from step 1" failure mode. One
durable workflow, one callback entry point.

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

**Gofra registers schedules at boot from config:**

```go
// config/schedules.go
func Schedules() []gofra.Schedule {
    return []gofra.Schedule{
        {Key: "cleanup", Cron: "0 2 * * *", Service: "CleanupService", Handler: "PruneTokens"},
        {Key: "digest", Cron: "0 9 * * 1", Service: "DigestService", Handler: "SendWeekly"},
        {Key: "health", Cron: "*/15 * * * *", Service: "HealthService", Handler: "CheckAPIs"},
    }
}

// On app.Start(), Gofra calls:
// for _, s := range config.Schedules() {
//     restateClient.Object("Scheduler", s.Key, "Create").Send(ScheduleRequest{...})
// }
// The Create handler is idempotent — "schedule already exists" is fine.
```

---

## The HTTP ↔ Restate Bridge

The glue between Gofra's request layer and Restate is minimal. RPC services and
plain HTTP handlers both use the same pre-configured ingress client:

```go
// gofra/restate_client.go
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

func (s *Sender) Send(ctx context.Context, payload any, opts ...restate.CallOption) error {
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

RPC services and plain HTTP handlers both receive the same `*RestateClient`
instance from `main()`. That's it. A thin wrapper for dependency injection, no
framework-owned queue abstraction.

---

## The CLI

```bash
# === Project ===
gofra new myapp                              # scaffold the current starter (minimal today)
gofra dev                                    # HTTP + Restate service endpoint + auto-start Restate Server
gofra build                                  # compile binary (HTTP + Restate endpoint)

# === Generators ===
gofra generate model Post title:string body:text
gofra generate rpc Posts
gofra generate migration add_views_to_posts

# Restate-specific generators:
gofra generate service ProcessPodcast        # → app/services/process_podcast.go
gofra generate object ShoppingCart           # → app/objects/shopping_cart.go
gofra generate workflow OrderCheckout        # → app/workflows/order_checkout.go
gofra generate schedule daily-cleanup        # → adds to config/schedules.go

# === Database ===
gofra migrate
gofra migrate rollback
gofra db seed

# === Restate ===
gofra restate status                         # show registered services, pending invocations
gofra restate invocations                    # list active/failed invocations
gofra restate retry <invocation-id>          # retry a failed invocation
gofra restate cancel <invocation-id>         # cancel an invocation
gofra restate purge <invocation-id>          # purge completed invocation
gofra restate ui                             # open Restate UI in browser

# These are thin wrappers around the `restate` CLI / admin API.
# The Restate UI at :9070 is the primary debugging tool.

# === Other ===
gofra routes                                 # list HTTP routes
gofra tinker                                 # REPL
gofra build                                  # single binary
```

### What `gofra generate service` produces:

```bash
$ gofra generate service ProcessPodcast
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

### What `gofra generate workflow` produces:

```bash
$ gofra generate workflow OrderCheckout
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

### Testing API Handlers

```go
func TestListPosts(t *testing.T) {
    db := gofra.TestDB(t)
    factory.CreateMany[models.Post](db, 5)

    recorder := gofra.NewRestateRecorder()
    svc := &rpc.PostsService{Queries: sqlc.New(db), Restate: recorder}

    _, handler := postsv1connect.NewPostsServiceHandler(svc)
    srv := httptest.NewServer(handler)
    defer srv.Close()

    client := postsv1connect.NewPostsServiceClient(http.DefaultClient, srv.URL)
    resp, err := client.ListPosts(context.Background(),
        connect.NewRequest(&postsv1.ListPostsRequest{PageSize: 10}),
    )

    require.NoError(t, err)
    assert.Len(t, resp.Msg.Posts, 5)
}
```

`RestateRecorder` captures durable dispatches for assertions without starting a
real Restate server.

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
func TestOrderCheckoutWorkflow(t *testing.T) {
    env := restatetest.Start(t, restate.Reflect(workflows.OrderCheckout{}))
    client := env.Ingress()

    // Start the workflow (async)
    handle, err := restateingress.WorkflowSend[workflows.CheckoutRequest](
        client, "OrderCheckout", "order-42", "Run",
    ).Send(t.Context(), workflows.CheckoutRequest{
        OrderID:    42,
        PaymentRef: "pay_ref_123",
    })
    require.NoError(t, err)

    // Simulate the provider webhook confirming the charge
    time.Sleep(100 * time.Millisecond)
    _, err = restateingress.Workflow[workflows.PaymentResult, restate.Void](
        client, "OrderCheckout", "order-42", "ConfirmPayment",
    ).Request(t.Context(), workflows.PaymentResult{
        Succeeded:  true,
        ProviderID: "ch_123",
    })
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
$ gofra dev

  ╭──────────────────────────────────────────────╮
  │  Gofra v1.0                                  │
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
  │    ✓ OrderCheckout         (Workflow)        │
  │    ✓ PayoutApproval        (Workflow)        │
  │    ✓ ReportExport          (Workflow)        │
  │                                              │
  │  Schedules:                                  │
  │    ✓ cleanup  0 2 * * *  → CleanupService    │
  │    ✓ digest   0 9 * * 1  → DigestService     │
  │    ✓ health   */15 * * * * → HealthService   │
  │                                              │
  │  Watching for changes...                     │
  ╰──────────────────────────────────────────────╯
```

`gofra dev` does:
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
gofra build
# Produces a single binary that runs:
#   - HTTP server
#   - Restate service endpoint
#
# Does NOT include the Restate Server — that runs separately in production.
```

Production setup:
1. Run the Restate Server (single binary or HA cluster)
2. Run your Gofra app (single binary)
3. The Gofra app connects to Restate on startup and registers its services

```yaml
# gofra.yaml (production)
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

Everything else — HTTP routing, Connect handlers, data access, validation,
auth, frontend integration — works like any other Go web framework. Restate
only touches the "durable operations" layer.
