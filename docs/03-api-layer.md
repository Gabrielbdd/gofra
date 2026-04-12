# 03 — API Layer: Connect RPC, Protobuf & AIP Conventions

> Parent: [Index](00-index.md) | Prev: [System Architecture](02-system-architecture.md) | Next: [Restate](04-restate.md)


## What AIP Is

Google's API Improvement Proposals (AIPs) are a corpus of ~70 approved design
documents covering how to build protobuf/gRPC APIs consistently. They emerged
from Google's internal API review process and represent years of accumulated
decisions about naming, structure, behavior, and error handling across thousands
of APIs.

The AIPs are organized into categories: resource design, standard methods,
fields, design patterns, compatibility, and polish. Not all are relevant to
Forge — some are deeply Google-specific (resource names with `googleapis.com`
prefixes, `google.api.http` transcoding annotations). But the core design
principles are universal and well-suited to a Connect RPC framework.

---

## Guidelines We Should Adopt

### AIP-121: Resource-Oriented Design

**Core principle**: APIs are modeled as resources (nouns) with a small set of
standard methods (verbs). Resources live in hierarchies. The schema of a
resource is the same across all methods that return it.

**Why it fits**: This is exactly how we want Forge APIs to look. A `Post` has
the same shape whether returned from `GetPost`, `ListPosts`, or `CreatePost`.
Connect RPC already encourages this — the proto message is the contract.

**What we adopt**:
- Every API entity is a protobuf message (the resource)
- Standard CRUD methods operate on resources
- Create returns the resource. Get returns the resource. Update returns the
  resource. Delete returns empty (or the resource for soft delete). List
  returns a collection of resources.
- The resource schema is consistent across all methods

**What we skip**:
- `google.api.resource` annotations (Google-specific for resource name resolution)
- Full resource name hierarchy with `publishers/123/books/456` patterns (useful
  for large APIs but overkill for most web apps; we use simple IDs)

---

### AIP-130 through AIP-136: Standard Methods + Custom Methods

**Adopt — this is our RPC naming convention.**

| AIP | Method | Our Convention |
|-----|--------|----------------|
| 131 | Get | `GetPost(GetPostRequest) returns (Post)` |
| 132 | List | `ListPosts(ListPostsRequest) returns (ListPostsResponse)` |
| 133 | Create | `CreatePost(CreatePostRequest) returns (Post)` |
| 134 | Update | `UpdatePost(UpdatePostRequest) returns (Post)` |
| 135 | Delete | `DeletePost(DeletePostRequest) returns (DeletePostResponse)` |
| 136 | Custom | `PublishPost(PublishPostRequest) returns (PublishPostResponse)` |

**Naming rules we adopt**:
- RPC names: `{Verb}{Resource}` (e.g., `GetPost`, `ListPosts`)
- Request messages: `{Verb}{Resource}Request`
- Response messages: `{Verb}{Resource}Response` (or the resource itself for
  Get/Create/Update)
- List responses contain `repeated {Resource}` + pagination fields
- Custom methods use descriptive verbs: `ArchivePost`, `PublishPost`,
  `TranslateText` — not `DoAction` or `ProcessPost`

**What we skip**:
- `google.api.http` annotations (Connect RPC handles HTTP mapping automatically)
- `google.api.method_signature` annotations (Google-specific for generated client method overloads)

---

### AIP-140: Field Names

**Adopt fully.** This is universal good practice.

- Fields use `lower_snake_case` in proto (maps to camelCase in JSON automatically)
- Same name for same concept across all services: `create_time`, not
  `created_at` in one place and `creation_date` in another
- Repeated fields use plural names: `repeated string tags`, not `repeated string tag`
- Boolean fields are prefixed with `is_`, `has_`, or `can_` where natural:
  `is_published`, `has_attachments`
- Avoid abbreviations unless universally understood (`id`, `url`, `uri` are fine;
  `desc` for `description` is not)
- No prepositions in field names: `author_id`, not `id_of_author`

---

### AIP-142: Time and Duration

**Adopt.** Use `google.protobuf.Timestamp` for absolute times and
`google.protobuf.Duration` for durations. Not `int64 created_at_unix` or
`string created_at`.

```protobuf
google.protobuf.Timestamp create_time = 5;
google.protobuf.Timestamp update_time = 6;
google.protobuf.Timestamp delete_time = 7;    // for soft delete
google.protobuf.Timestamp publish_time = 8;
google.protobuf.Timestamp expire_time = 9;    // when resource expires
```

Field naming for timestamps: `{action}_time` (e.g., `create_time`, `update_time`,
`delete_time`). Not `created_at` (Rails convention) or `createdAt`.

---

### AIP-148: Standard Fields

**Adopt selectively.** These standardized field names ensure consistency:

| Field | Type | Usage |
|-------|------|-------|
| `name` | `string` | Resource name / identifier (we may use `id` for simple cases) |
| `display_name` | `string` | User-visible name |
| `create_time` | `Timestamp` | When created |
| `update_time` | `Timestamp` | When last modified |
| `delete_time` | `Timestamp` | When soft-deleted |
| `etag` | `string` | Optimistic concurrency token |
| `uid` | `string` | System-assigned unique identifier |

**Adaptation**: Google uses `name` as the primary resource identifier in the
format `publishers/123/books/456`. For Forge, we use a simpler model:
- `int64 id` for database primary keys
- `string uid` when a globally unique string identifier is needed
- `string slug` for URL-friendly human-readable identifiers

We don't adopt the full `name` hierarchy because most web applications don't
need deeply nested resource paths. Our resources are identified by `id` or
`slug` in the proto, with parent references via explicit `_id` fields.

---

### AIP-158: Pagination

**Adopt fully.** Every List method must support pagination from day one (adding
it later is a breaking change).

```protobuf
message ListPostsRequest {
  int32 page_size = 1;     // max items to return
  string page_token = 2;   // opaque token from previous response
  string order_by = 3;     // ordering expression
  string filter = 4;       // filtering expression
}

message ListPostsResponse {
  repeated Post posts = 1;
  string next_page_token = 2;   // empty = no more pages
  int32 total_size = 3;         // total count (optional, may be expensive)
}
```

**Key rules**:
- `page_size` has a maximum (e.g., 100) and a default (e.g., 25)
- `page_token` is opaque — clients must not construct or parse it
- `next_page_token` is empty when there are no more results
- Server may return fewer items than `page_size`
- `total_size` is optional because counting can be expensive on large tables

---

### AIP-160: Filtering

**Adopt the concept, simplify the syntax.** AIP-160 defines a filtering grammar
based on Google's Common Expression Language (CEL). The full syntax is complex
(supports logical operators, functions, traversals).

For Forge, we adopt a simplified version:
- Support basic field-value filters: `status = "published"`
- Support logical AND: `status = "published" AND author_id = 42`
- Support common operators: `=`, `!=`, `>`, `<`, `>=`, `<=`
- Support `IN`: `status IN ["draft", "published"]`
- Pass the filter string to the handler, which parses and applies it to the
  generated query layer

The filter string is a `string filter` field on the List request. The server
is responsible for parsing and validating it. Invalid filters return
`INVALID_ARGUMENT`.

---

### AIP-161: Field Masks (Update)

**Adopt for Update methods.** Field masks solve the problem of partial updates:
how does the server know which fields the client intends to change vs. which
are just empty/default?

```protobuf
message UpdatePostRequest {
  Post post = 1;
  google.protobuf.FieldMask update_mask = 2;
}
```

If `update_mask` is set, only the specified fields are updated. If not set, all
non-empty fields are updated (full replacement).

This is important for generated clients — when a TypeScript client sends an
update with `{ title: "New Title" }`, the other fields (body, status) are
empty strings/zero values in the proto. Without a field mask, the server would
overwrite body with an empty string.

**Connect RPC and protobuf handle FieldMask natively.** This works out of the box.

---

### AIP-163: Change Validation (validate_only)

**Adopt.** Add `bool validate_only` to Create and Update requests. When true,
the server validates the request but doesn't persist anything. Returns the
resource as it would look after the change, or validation errors.

```protobuf
message CreatePostRequest {
  Post post = 1;
  bool validate_only = 2;  // if true, validate but don't persist
}
```

This is trivial to implement and valuable for UX — the frontend can validate
before submitting by sending the same request with `validate_only = true`.

---

### AIP-164: Soft Delete

**Adopt.** Resources that support soft delete should:

1. Set `delete_time` and `purge_time` instead of actually removing
2. Exclude soft-deleted resources from List by default
3. Add `bool show_deleted` to ListRequest to include them
4. Provide `UndeletePost` as a custom method
5. Get should still return soft-deleted resources (not 404)

```protobuf
service PostsService {
  rpc DeletePost(DeletePostRequest) returns (Post) {}       // returns updated resource
  rpc UndeletePost(UndeletePostRequest) returns (Post) {}   // custom method
}
```

This aligns with the database layer's soft-delete conventions.

---

### AIP-155: Request Identification (request_id)

**Adopt.** Add `string request_id` to mutating requests. It is the client-
supplied idempotency key for that mutation attempt.

```protobuf
message CreatePostRequest {
  Post post = 1;
  string request_id = 2;   // client-generated UUID for idempotency
}
```

The framework can forward `request_id` to Restate when dispatching durable
work. The exact end-to-end mutation semantics for database writes remain a
framework design responsibility and should not be overstated in the API
contract alone.

---

### AIP-193: Errors

**Adopt the error model.** Use Connect's standard error codes (which map to
gRPC status codes):

| Code | When to use |
|------|-------------|
| `InvalidArgument` | Validation failed, bad input |
| `NotFound` | Resource doesn't exist |
| `AlreadyExists` | Conflict (duplicate create) |
| `PermissionDenied` | Authenticated but not authorized |
| `Unauthenticated` | No valid credentials |
| `FailedPrecondition` | Operation rejected due to system state |
| `Aborted` | Concurrency conflict (etag mismatch) |
| `Internal` | Server bug |
| `Unavailable` | Transient, safe to retry |

**Error details**: Use Connect's `ErrorDetail` for structured error info:
- `BadRequest` with field violations for validation errors
- `ErrorInfo` with reason code and metadata for machine-readable errors

This replaces ad-hoc JSON error formats. Every error from every service uses
the same structure. The frontend can handle errors generically.

---

### AIP-154: Resource Freshness (etag)

**Adopt for resources that need optimistic concurrency.** Add `string etag` to
the resource. Update and Delete check the etag; if it doesn't match, return
`Aborted`.

```protobuf
message Post {
  // ...
  string etag = 15;  // server-computed, changes on every update
}

message UpdatePostRequest {
  Post post = 1;
  google.protobuf.FieldMask update_mask = 2;
  // etag is inside post — server checks post.etag against stored value
}
```

This integrates with Restate's Virtual Objects naturally — the etag can be
part of the Restate K/V state for entities that use it.

---

### AIP-190: Naming Conventions

**Adopt.**

- Proto package names: `myapp.posts.v1` (lowercase, versioned)
- Service names: `PostsService` (plural resource + Service)
- Message names: PascalCase (`CreatePostRequest`)
- Field names: snake_case (`author_id`)
- Enum values: UPPER_SNAKE_CASE (`POST_STATUS_DRAFT`)
- RPC names: PascalCase verb+noun (`ListPosts`)

---

### AIP-191: File and Directory Structure

**Adopt.** Proto files organized by package:

```
proto/
  myapp/
    posts/v1/
      posts.proto
    auth/v1/
      auth.proto
    users/v1/
      users.proto
```

One service per file. Service name matches file name.

---

### AIP-185: API Versioning

**Adopt the package-based approach.** Version is part of the proto package
name: `myapp.posts.v1`. When breaking changes are needed, create `v2`.

For most web apps, `v1` is sufficient for years. But having the version in
the package from day one means the option is always there.

---

### AIP-231/233/234/235: Batch Methods

**Adopt when needed.** Batch methods follow a consistent pattern:

```protobuf
rpc BatchGetPosts(BatchGetPostsRequest) returns (BatchGetPostsResponse) {}
rpc BatchCreatePosts(BatchCreatePostsRequest) returns (BatchCreatePostsResponse) {}
```

These are not required by default but the naming pattern should be followed
when a batch operation is added.

---

## Guidelines We Skip

### AIP-122: Resource Names (Full Hierarchy)

Google uses `publishers/123/books/456` as resource identifiers. This is
powerful for deeply hierarchical APIs (Cloud resources) but overengineered for
typical web apps where resources are identified by simple IDs or slugs.

**We use**: `int64 id`, `string slug`, `string uid` — not hierarchical name
strings.

### AIP-127: HTTP and gRPC Transcoding

Google annotates RPCs with `google.api.http` to define REST mappings. Connect
RPC handles this automatically — each RPC is callable via HTTP POST with a
JSON body at a predictable path (`/package.Service/Method`). No transcoding
annotations needed.

### AIP-128: Declarative-Friendly Interfaces

This is about making APIs compatible with Terraform/Kubernetes. Relevant for
infrastructure APIs, not web applications.

### AIP-151: Long-Running Operations

Google's LRO pattern uses `google.longrunning.Operation` with polling. We use
Restate Workflows instead, which are far more capable. Long-running operations
in Forge are Restate invocations — the client can attach to them by invocation
ID or idempotency key using Restate's ingress API. No need for a separate
Operation resource.

### AIP-152: Jobs

Google's Jobs pattern is for scheduled/repeatable tasks with a lifecycle. We
use Restate's Virtual Object scheduler pattern instead, which is more powerful
(durable timers, self-rescheduling).

### AIP-157: Partial Responses (read_mask)

Allows clients to request only specific fields. This adds complexity to every
handler and is rarely needed in web apps where the frontend controls what it
renders. If needed, it can be added per-service — but it's not a framework
default.

### AIP-203: Field Behavior Documentation

Google annotates fields with `google.api.field_behavior` (OUTPUT_ONLY,
REQUIRED, IMMUTABLE). We use `buf/validate` for validation constraints instead.
Output-only semantics are documented in proto comments and enforced in the
handler.

---

## Summary: Our AIP-Derived Proto Conventions

```protobuf
syntax = "proto3";
package myapp.posts.v1;

import "buf/validate/validate.proto";
import "google/protobuf/field_mask.proto";
import "google/protobuf/timestamp.proto";

service PostsService {
  // Standard methods (AIP-131 through AIP-135)
  rpc GetPost(GetPostRequest) returns (Post) {}
  rpc ListPosts(ListPostsRequest) returns (ListPostsResponse) {}
  rpc CreatePost(CreatePostRequest) returns (Post) {}
  rpc UpdatePost(UpdatePostRequest) returns (Post) {}
  rpc DeletePost(DeletePostRequest) returns (Post) {}  // soft delete returns resource

  // Custom methods (AIP-136)
  rpc PublishPost(PublishPostRequest) returns (Post) {}
  rpc UndeletePost(UndeletePostRequest) returns (Post) {}  // AIP-164

  // Batch methods (AIP-231+)
  rpc BatchGetPosts(BatchGetPostsRequest) returns (BatchGetPostsResponse) {}
}

// The resource (AIP-121)
message Post {
  int64 id = 1;                                       // primary key
  string title = 2;
  string slug = 3;
  string body = 4;
  PostStatus status = 5;
  int64 author_id = 6;
  string etag = 7;                                    // AIP-154
  google.protobuf.Timestamp create_time = 8;          // AIP-148
  google.protobuf.Timestamp update_time = 9;          // AIP-148
  google.protobuf.Timestamp delete_time = 10;         // AIP-164
  google.protobuf.Timestamp publish_time = 11;
  // Relations (populated when requested)
  User author = 12;
  repeated Tag tags = 13;
}

enum PostStatus {
  POST_STATUS_UNSPECIFIED = 0;                        // AIP-126
  POST_STATUS_DRAFT = 1;
  POST_STATUS_PUBLISHED = 2;
  POST_STATUS_ARCHIVED = 3;
}

// Standard method messages

message GetPostRequest {
  // Fetch by ID or slug
  oneof identifier {
    int64 id = 1;
    string slug = 2;
  }
}

message ListPostsRequest {
  int32 page_size = 1 [(buf.validate.field).int32 = {gte: 1, lte: 100}];  // AIP-158
  string page_token = 2;                                                    // AIP-158
  string order_by = 3;                                                      // AIP-132
  string filter = 4;                                                        // AIP-160
  bool show_deleted = 5;                                                    // AIP-164
}

message ListPostsResponse {
  repeated Post posts = 1;
  string next_page_token = 2;                         // AIP-158
  int32 total_size = 3;                               // optional
}

message CreatePostRequest {
  string title = 1 [(buf.validate.field).string = {min_len: 1, max_len: 255}];
  string body = 2 [(buf.validate.field).string.min_len = 10];
  repeated string tags = 3;
  string request_id = 4;                              // AIP-155 (client-supplied idempotency key)
  bool validate_only = 5;                             // AIP-163
}

message UpdatePostRequest {
  Post post = 1 [(buf.validate.field).required = true];
  google.protobuf.FieldMask update_mask = 2;          // AIP-161
  bool validate_only = 3;                             // AIP-163
}

message DeletePostRequest {
  int64 id = 1;
  string etag = 2;                                    // AIP-154 (optional)
}

message UndeletePostRequest {
  int64 id = 1;
}

message PublishPostRequest {
  int64 id = 1;
  string etag = 2;                                    // AIP-154 (optional)
}

// Batch messages follow AIP-231 pattern
message BatchGetPostsRequest {
  repeated int64 ids = 1 [(buf.validate.field).repeated = {min_items: 1, max_items: 100}];
}

message BatchGetPostsResponse {
  repeated Post posts = 1;
}
```

This proto file follows AIP conventions where they improve consistency and
skips them where they add complexity without value for web applications. The
generated code (Go Connect stubs, TypeScript Connect-Query hooks) inherits all
of these patterns automatically.

---

## Integration with Forge

### Proto → Generated Code → Handler → Restate

The AIP patterns compose with our existing architecture:

1. **Proto defines the API** with AIP naming, pagination, filtering, field masks
2. **buf generates** Go Connect stubs + TypeScript clients
3. **Connect handler** implements the generated interface and uses sqlc-
   generated queries for database access
4. **request_id may be forwarded to Restate idempotency keys** when dispatching
   durable work
5. **validate_only** is checked early in the handler — run validation, return
   result without persisting
6. **etag** is computed from `update_time` or a hash — checked in the handler
   before mutation
7. **Error codes** use Connect's typed errors, which match AIP-193 exactly
8. **Soft delete** sets `delete_time` in the database, handler checks
   `show_deleted` on List, returns deleted resources on Get

### Linting

The `api-linter` tool can be integrated into `mise run lint` to check proto
files against AIP rules:

```toml
[tasks."lint:proto"]
run = "buf lint && api-linter proto/**/*.proto"
```

This catches naming mistakes, missing pagination, incorrect field types, and
other AIP violations at development time — before code generation.
