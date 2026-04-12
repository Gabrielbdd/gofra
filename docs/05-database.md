# 05 — Database: sqlc & goose

> Parent: [Index](00-index.md) | Prev: [Restate](04-restate.md) | Next: [Configuration](06-configuration.md)


## Addendum to Architecture Design Document
## Last Updated: 2026-04-12

---

## What Changes

The previous architecture document described a custom query builder (`query.From[Post](db)`)
and a framework-owned migration system. This addendum replaces both with
established, focused tools:

- **sqlc** replaces the custom query builder. You write SQL queries in `.sql`
  files, sqlc generates type-safe Go code from them.
- **goose** replaces the framework-owned migrator. It provides SQL migration
  files with `-- +goose Up` / `-- +goose Down` annotations, a CLI, and a Go
  library for programmatic execution.

Both are pure SQL tools. No Go DSL for queries. No Go DSL for schema. SQL
is the language for talking to the database.

---

## Why sqlc Over a Custom Query Builder

### The problem with query builders

The previous design had:

```go
posts, err := query.From[Post](db).
    Where("status", "published").
    Where("created_at >", lastWeek).
    Preload("Author", "Tags").
    Paginate(page, 25)
```

This looks clean but has real issues:

1. **Column names are strings.** `"status"` and `"created_at"` are unchecked
   at compile time. Rename a column, the query builder compiles fine, fails at
   runtime.

2. **The builder is a framework to build and maintain.** Implementing `Where`,
   `WhereIn`, `OrWhere`, `Preload`, `OrderBy`, `Paginate`, `Scope`,
   `SoftDeletes`, `Chunk` — with correct SQL generation for Postgres — is
   thousands of lines of code. Every edge case (NULL handling, subqueries,
   JSON operators, CTEs, window functions) requires framework changes.

3. **It's an abstraction over SQL that leaks.** Complex queries eventually
   need raw SQL. The builder encourages simple queries and punishes complex
   ones, which is backwards — simple queries don't need a builder, complex
   queries need SQL.

4. **The "Preload" system is an ORM in disguise.** Eager loading relationships
   requires the framework to understand foreign keys, join tables, and
   polymorphism. This is a full ORM, just without the name.

### Why sqlc is better

sqlc takes the opposite approach: **you write SQL, it generates Go.**

```sql
-- db/queries/posts.sql

-- name: GetPost :one
SELECT p.*, u.name as author_name, u.email as author_email
FROM posts p
JOIN users u ON u.id = p.author_id
WHERE p.slug = $1 AND p.delete_time IS NULL;

-- name: ListPosts :many
SELECT p.*, u.name as author_name
FROM posts p
JOIN users u ON u.id = p.author_id
WHERE (p.delete_time IS NULL OR @show_deleted::bool = true)
  AND (sqlc.narg('status')::text IS NULL OR p.status = @status)
ORDER BY p.create_time DESC
LIMIT @page_size OFFSET @page_offset;

-- name: CreatePost :one
INSERT INTO posts (title, slug, body, status, author_id, create_time, update_time)
VALUES (@title, @slug, @body, @status, @author_id, now(), now())
RETURNING *;

-- name: UpdatePost :one
UPDATE posts
SET title = COALESCE(sqlc.narg('title'), title),
    slug = COALESCE(sqlc.narg('slug'), slug),
    body = COALESCE(sqlc.narg('body'), body),
    status = COALESCE(sqlc.narg('status'), status),
    update_time = now()
WHERE id = @id AND delete_time IS NULL
RETURNING *;

-- name: SoftDeletePost :one
UPDATE posts
SET delete_time = now(), update_time = now()
WHERE id = @id AND delete_time IS NULL
RETURNING *;

-- name: UndeletePost :one
UPDATE posts
SET delete_time = NULL, update_time = now()
WHERE id = @id AND delete_time IS NOT NULL
RETURNING *;

-- name: CountPostsByStatus :one
SELECT count(*) FROM posts
WHERE status = @status AND delete_time IS NULL;
```

sqlc generates:

```go
// db/sqlc/posts.sql.go (generated — do not edit)

type GetPostParams struct {
    Slug string
}

type GetPostRow struct {
    ID          int64
    Title       string
    Slug        string
    Body        string
    Status      string
    AuthorID    int64
    CreateTime  time.Time
    UpdateTime  time.Time
    DeleteTime  *time.Time
    AuthorName  string
    AuthorEmail string
}

func (q *Queries) GetPost(ctx context.Context, arg GetPostParams) (GetPostRow, error) {
    // ... generated implementation
}

type ListPostsParams struct {
    ShowDeleted bool
    Status      *string
    PageSize    int32
    PageOffset  int32
}

func (q *Queries) ListPosts(ctx context.Context, arg ListPostsParams) ([]ListPostsRow, error) {
    // ... generated implementation
}

type CreatePostParams struct {
    Title    string
    Slug     string
    Body     string
    Status   string
    AuthorID int64
}

func (q *Queries) CreatePost(ctx context.Context, arg CreatePostParams) (Post, error) {
    // ... generated implementation
}
```

**What sqlc gives us that the query builder didn't:**

1. **Compile-time SQL validation.** sqlc parses the SQL against the schema. If
   a column doesn't exist or a type doesn't match, `sqlc generate` fails. Not
   at runtime — at generation time.

2. **Zero framework code to maintain.** The query builder was thousands of lines.
   sqlc is a third-party tool we don't maintain.

3. **Real SQL.** Joins, subqueries, CTEs, window functions, Postgres-specific
   operators — all work. No builder limitations. No escape hatch needed because
   SQL IS the interface.

4. **Generated types match the query, not the table.** `GetPostRow` includes
   `AuthorName` and `AuthorEmail` because the query joins users. The type is
   exactly what the query returns — no manual struct mapping.

5. **Named parameters.** `@title`, `@slug`, `@status` — the generated `Params`
   struct has readable field names, not positional `$1`, `$2`.

### What we lose (and why it's acceptable)

1. **No dynamic query building.** You can't conditionally add WHERE clauses at
   runtime. The queries are static SQL. For filtering, use `sqlc.narg()` with
   `COALESCE` or `CASE` patterns:

   ```sql
   WHERE (sqlc.narg('status')::text IS NULL OR p.status = @status)
     AND (sqlc.narg('author_id')::bigint IS NULL OR p.author_id = @author_id)
   ```

   The parameter is nullable. If the caller passes nil, the condition is
   skipped. This covers 90% of filtering use cases. For the remaining 10%
   (truly dynamic filters with arbitrary field combinations), write the SQL
   by hand using `db.Query()`.

2. **No automatic relationship loading.** No `Preload("Author", "Tags")`. You
   write explicit JOINs in SQL, or you make two queries. This is more work but
   produces better SQL — you never accidentally N+1 because there's no implicit
   lazy loading.

3. **No model structs with methods.** sqlc generates flat structs from queries.
   There's no `Post.ToProto()` method on the generated struct. The conversion
   lives in the Connect handler or a dedicated conversion function.

---

## Why goose Over a Custom Migrator

### The previous design

The previous architecture described a framework-owned migrator:

```bash
forge migrate create create_posts_table
forge migrate
forge migrate rollback
```

With plain SQL up/down files tracked in a `schema_migrations` table.

### Why goose is better

goose does exactly the same thing — but it already exists, is battle-tested,
and has features we'd have to build ourselves:

1. **Embeddable migrations.** `//go:embed migrations/*.sql` + `goose.Up(db, "migrations")`
   means migrations are compiled into the binary. No file shipping. This is
   critical for Forge's single-binary deployment story.

2. **Programmatic API.** goose is both a CLI tool and a Go library. We can
   call `goose.Up()` from `main()` on startup, or use the CLI for manual
   operations. Both use the same migration files.

3. **Go migrations.** For complex migrations that need application logic (data
   backfills, encryption key rotation), goose supports Go functions alongside
   SQL files. Same numbering, same versioning.

4. **Out-of-order migrations.** With `--allow-missing`, goose can apply
   migrations that were created on a branch and merged after newer migrations
   were already applied. Essential for teams with multiple developers.

5. **Environment variable substitution.** `${ENV_VAR}` in SQL migration files.
   Useful for environment-specific schema (e.g., different extension names in
   dev vs. prod).

6. **No framework code to maintain.** We'd have to build all of the above.
   goose has been doing it since 2014.

---

## How They Compose: sqlc + goose

sqlc reads the database schema to validate queries. goose manages the schema.
The two connect naturally:

```yaml
# sqlc.yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "db/queries/"
    schema: "db/migrations/"    # sqlc reads goose migration files as schema
    gen:
      go:
        package: "sqlc"
        out: "db/sqlc"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_interface: true
```

**The key line**: `schema: "db/migrations/"` — sqlc parses the goose migration
files to understand the current schema. No separate `schema.sql` to keep in
sync. The migration files ARE the schema definition. When you add a migration
that creates a new column, `sqlc generate` immediately knows about it and
validates all queries against the updated schema.

---

## Project Structure (Updated)

```
myapp/
├── db/
│   ├── migrations/                    # goose SQL migrations
│   │   ├── 00001_create_users.sql
│   │   ├── 00002_create_posts.sql
│   │   └── 00003_add_slug_to_posts.sql
│   ├── queries/                       # sqlc query files
│   │   ├── posts.sql
│   │   ├── users.sql
│   │   └── auth.sql
│   ├── sqlc/                          # generated Go code (do not edit)
│   │   ├── db.go
│   │   ├── models.go
│   │   ├── posts.sql.go
│   │   ├── users.sql.go
│   │   └── auth.sql.go
│   └── seeds/                         # seed data (goose --no-versioning)
│       └── seed.sql
├── sqlc.yaml                          # sqlc configuration
└── ...
```

---

## Migration Files

```sql
-- db/migrations/00002_create_posts.sql

-- +goose Up
CREATE TABLE posts (
    id          BIGSERIAL PRIMARY KEY,
    title       VARCHAR(255) NOT NULL,
    slug        VARCHAR(255) NOT NULL UNIQUE,
    body        TEXT NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'draft',
    author_id   BIGINT NOT NULL REFERENCES users(id),
    etag        VARCHAR(64),
    create_time TIMESTAMPTZ NOT NULL DEFAULT now(),
    update_time TIMESTAMPTZ NOT NULL DEFAULT now(),
    delete_time TIMESTAMPTZ,
    publish_time TIMESTAMPTZ
);

CREATE INDEX idx_posts_slug ON posts(slug);
CREATE INDEX idx_posts_author_id ON posts(author_id);
CREATE INDEX idx_posts_status ON posts(status) WHERE delete_time IS NULL;

-- +goose Down
DROP TABLE IF EXISTS posts;
```

**Reason for `TIMESTAMPTZ` over `TIMESTAMP`**: Postgres `TIMESTAMP` stores
without timezone. `TIMESTAMPTZ` stores in UTC and converts on read. Since
the application may run in different timezones, `TIMESTAMPTZ` prevents
subtle time bugs.

**Reason for partial index `WHERE delete_time IS NULL`**: Most queries filter
out soft-deleted records. The partial index covers only active records,
making it smaller and faster.

---

## Startup: Optional Auto-Migration

```go
// cmd/app/main.go
func main() {
    cfg := config.Load()

    db, err := pgx.New(cfg.Database.DSN)
    if err != nil {
        slog.Error("database connection failed", "err", err)
        os.Exit(1)
    }
    defer db.Close()

    // Optional: run migrations on startup
    if cfg.Database.AutoMigrate {
        if err := runMigrations(db); err != nil {
            slog.Error("migration failed", "err", err)
            os.Exit(1)
        }
    }

    // ... rest of application setup
}

//go:embed db/migrations/*.sql
var migrations embed.FS

func runMigrations(db *sql.DB) error {
    goose.SetBaseFS(migrations)
    if err := goose.SetDialect("postgres"); err != nil {
        return fmt.Errorf("goose dialect: %w", err)
    }
    if err := goose.Up(db, "db/migrations"); err != nil {
        return fmt.Errorf("goose up: %w", err)
    }
    return nil
}
```

**Reason auto-migrate is opt-in, not default**: In production with multiple
replicas, auto-migrating on startup causes races — 10 replicas all trying to
run `CREATE TABLE` simultaneously. goose uses a lock table to serialize, but
it can still cause startup delays and unexpected failures. The safe default
is to run migrations as a separate step before deployment (`mise run migrate`).
Auto-migrate is convenient for development and single-instance deployments.

```yaml
# forge.yaml
database:
  dsn: "${DATABASE_URL}"
  auto_migrate: true   # convenient for dev, disable in prod
```

**Reason migrations are embedded**: The production binary must be self-contained.
`//go:embed db/migrations/*.sql` compiles the migration files into the binary.
Even with auto-migrate disabled, the binary carries its migrations — useful for
running `./myapp migrate` as a standalone command without needing the source tree.

---

## How Connect Handlers Use sqlc

```go
// app/rpc/posts_service.go
type PostsService struct {
    Queries *sqlc.Queries    // generated by sqlc
    Restate *forge.RestateClient
}

func (s *PostsService) GetPost(
    ctx context.Context,
    req *connect.Request[postsv1.GetPostRequest],
) (*connect.Response[postsv1.Post], error) {

    row, err := s.Queries.GetPost(ctx, sqlc.GetPostParams{
        Slug: req.Msg.Slug,
    })
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("post not found"))
        }
        return nil, connect.NewError(connect.CodeInternal, err)
    }

    return connect.NewResponse(postRowToProto(row)), nil
}

func (s *PostsService) ListPosts(
    ctx context.Context,
    req *connect.Request[postsv1.ListPostsRequest],
) (*connect.Response[postsv1.ListPostsResponse], error) {

    pageSize := int32(25)
    if req.Msg.PageSize > 0 {
        pageSize = min(req.Msg.PageSize, 100)
    }
    offset := decodePageToken(req.Msg.PageToken)

    var status *string
    if req.Msg.Filter != "" {
        status = parseStatusFilter(req.Msg.Filter)
    }

    rows, err := s.Queries.ListPosts(ctx, sqlc.ListPostsParams{
        ShowDeleted: req.Msg.ShowDeleted,
        Status:      status,
        PageSize:    pageSize + 1, // fetch one extra to detect next page
        PageOffset:  offset,
    })
    if err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }

    hasNext := len(rows) > int(pageSize)
    if hasNext {
        rows = rows[:pageSize]
    }

    posts := make([]*postsv1.Post, len(rows))
    for i, row := range rows {
        posts[i] = listPostRowToProto(row)
    }

    var nextToken string
    if hasNext {
        nextToken = encodePageToken(offset + pageSize)
    }

    return connect.NewResponse(&postsv1.ListPostsResponse{
        Posts:         posts,
        NextPageToken: nextToken,
    }), nil
}

func (s *PostsService) CreatePost(
    ctx context.Context,
    req *connect.Request[postsv1.CreatePostRequest],
) (*connect.Response[postsv1.Post], error) {

    userID, _ := forge.UserIDFromContext(ctx)

    post, err := s.Queries.CreatePost(ctx, sqlc.CreatePostParams{
        Title:    req.Msg.Title,
        Slug:     slugify(req.Msg.Title),
        Body:     req.Msg.Body,
        Status:   "draft",
        AuthorID: userID,
    })
    if err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }

    // Dispatch durable indexing
    if err := s.Restate.Service("SearchIndexer", "Index").Send(
        ctx,
        IndexPostRequest{PostID: post.ID},
    ); err != nil {
        return nil, connect.NewError(connect.CodeUnavailable, err)
    }

    return connect.NewResponse(postToProto(post)), nil
}
```

**Reason for `*sqlc.Queries` as a struct field**: Explicit dependency injection.
The `Queries` struct wraps a `*pgx.Pool` (or `*sql.DB`). It's created once in
`main()` and passed to each handler. No global database accessor.

### Conversion Functions

sqlc generates flat row structs. Proto messages are separate types. Conversion
is explicit:

```go
// app/rpc/converters.go

func postToProto(p sqlc.Post) *postsv1.Post {
    pb := &postsv1.Post{
        Id:         p.ID,
        Title:      p.Title,
        Slug:       p.Slug,
        Body:       p.Body,
        Status:     postsv1.PostStatus(postsv1.PostStatus_value["POST_STATUS_"+strings.ToUpper(p.Status)]),
        AuthorId:   p.AuthorID,
        CreateTime: timestamppb.New(p.CreateTime),
        UpdateTime: timestamppb.New(p.UpdateTime),
    }
    if p.DeleteTime != nil {
        pb.DeleteTime = timestamppb.New(*p.DeleteTime)
    }
    if p.PublishTime != nil {
        pb.PublishTime = timestamppb.New(*p.PublishTime)
    }
    return pb
}

func postRowToProto(r sqlc.GetPostRow) *postsv1.Post {
    pb := postToProto(sqlc.Post{
        ID: r.ID, Title: r.Title, Slug: r.Slug, Body: r.Body,
        Status: r.Status, AuthorID: r.AuthorID,
        CreateTime: r.CreateTime, UpdateTime: r.UpdateTime,
        DeleteTime: r.DeleteTime, PublishTime: r.PublishTime,
    })
    pb.Author = &usersv1.User{
        Name:  r.AuthorName,
        Email: r.AuthorEmail,
    }
    return pb
}
```

**Reason for explicit conversion functions (not methods)**: sqlc-generated
structs are in the `sqlc` package. Proto messages are in `postsv1`. Conversion
functions live in the `rpc` package that imports both. This keeps the generated
code untouched — no methods added to generated types.

---

## How Restate Handlers Use sqlc

```go
// app/services/search_indexer.go
type SearchIndexer struct {
    Queries *sqlc.Queries
}

func (s SearchIndexer) Index(ctx restate.Context, req IndexPostRequest) error {
    post, err := restate.Run(ctx, func(ctx restate.RunContext) (sqlc.Post, error) {
        return s.Queries.GetPostByID(context.Background(), req.PostID)
    }, restate.WithName("load-post"))
    if err != nil {
        return err
    }

    _, err = restate.Run(ctx, func(ctx restate.RunContext) (restate.Void, error) {
        return restate.Void{}, search.Index("posts", post.ID, map[string]any{
            "title": post.Title,
            "body":  post.Body,
        })
    }, restate.WithName("index"))
    return err
}
```

**Note**: Inside `restate.Run()`, you cannot use the Restate context. You must
use `context.Background()` or a standard `context.Context` for database calls.
This is a Restate SDK constraint — `RunContext` is not a `context.Context` for
Restate operations, only for regular Go operations.

---

## Transactions

sqlc generates a `WithTx` method on the Queries struct:

```go
func (s *PostsService) PublishPost(
    ctx context.Context,
    req *connect.Request[postsv1.PublishPostRequest],
) (*connect.Response[postsv1.Post], error) {

    tx, err := s.DB.Begin(ctx)
    if err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }
    defer tx.Rollback(ctx)

    qtx := s.Queries.WithTx(tx)

    post, err := qtx.SoftPublishPost(ctx, sqlc.SoftPublishPostParams{
        ID:     req.Msg.Id,
        Status: "published",
    })
    if err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }

    // additional transactional work...

    if err := tx.Commit(ctx); err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }

    return connect.NewResponse(postToProto(post)), nil
}
```

**Reason for explicit transaction management**: No hidden transaction middleware.
The handler starts, commits, and rolls back transactions with standard
`database/sql` or `pgx` methods. sqlc's `WithTx` wraps the transaction for
use with generated queries. This is transparent — you can see every transaction
boundary by reading the handler code.

---

## Mise Tasks (Updated)

```toml
# mise.toml additions

[tasks."gen:sql"]
description = "Generate Go code from SQL queries"
run = "sqlc generate"
sources = ["db/queries/*.sql", "db/migrations/*.sql", "sqlc.yaml"]
outputs = ["db/sqlc/*.go"]

[tasks.gen]
description = "Generate all code"
depends = ["gen:go", "gen:ts", "gen:sql"]

[tasks.migrate]
description = "Run pending migrations"
run = "goose -dir db/migrations postgres $DATABASE_URL up"

[tasks."migrate:create"]
description = "Create a new migration"
run = "goose -dir db/migrations create {{arg(i=0)}} sql"

[tasks."migrate:down"]
description = "Rollback last migration"
run = "goose -dir db/migrations postgres $DATABASE_URL down"

[tasks."migrate:status"]
description = "Show migration status"
run = "goose -dir db/migrations postgres $DATABASE_URL status"

[tasks.seed]
description = "Seed database"
run = "goose -dir db/seeds -no-versioning postgres $DATABASE_URL up"
```

**Reason for `gen:sql` in the gen dependency chain**: When a developer changes
a query or adds a migration, `mise run gen` regenerates all code — proto stubs,
TypeScript hooks, AND sqlc Go code. One command, everything in sync.

**Reason goose uses `-dir db/migrations` explicitly**: No hidden migration
directory. The mise task shows exactly where migrations live. Any developer
can read the task and understand the operation.

---

## Code Generation Flow (Updated)

```
db/migrations/*.sql   ──┐
                        ├──→ sqlc generate ──→ db/sqlc/*.go (Go query functions)
db/queries/*.sql      ──┘

db/migrations/*.sql   ──→ goose up        ──→ applies schema to database

proto/**/*.proto      ──→ buf generate    ──→ gen/**/*.go (Connect stubs)
                                          ──→ web/src/gen/**/*.ts (TS hooks)
```

Three code generators, three input directories, no overlap:
- **buf**: proto → Go server stubs + TypeScript client
- **sqlc**: SQL queries + migrations → Go database functions
- **goose**: migrations → database schema

---

## Decision Log (Database Layer)

| # | Decision | Rationale |
|---|----------|-----------|
| 48 | sqlc over custom query builder | SQL is validated at generation time against the schema. Zero framework code to maintain. No abstraction leak — SQL IS the query language. |
| 49 | goose over custom migrator | Battle-tested, embeddable, supports Go migrations, out-of-order, env vars. No framework code to maintain. |
| 50 | sqlc reads goose migration files as schema | One source of truth for schema. No separate `schema.sql` to keep in sync. |
| 51 | Migrations embedded via `//go:embed` | Binary carries its own migrations. Single-artifact deployment. |
| 52 | Auto-migrate opt-in, not default | Multiple replicas racing on `CREATE TABLE` causes startup failures. Safe for dev, dangerous for prod. |
| 53 | pgx/v5 as sql_package for sqlc | pgx is the most performant Postgres driver for Go. Native support for Postgres types (arrays, JSONB, intervals). |
| 54 | Explicit conversion functions over methods on generated types | Generated code stays untouched. Conversion logic lives in the layer that knows both types (rpc package). |
| 55 | No dynamic query builder | sqlc queries are static SQL. Dynamic filtering uses `sqlc.narg()` + `COALESCE`. Truly dynamic queries use `db.Query()` directly. Covers 90% of cases without a builder. |
| 56 | Transactions via `WithTx` + explicit Begin/Commit | No hidden transaction middleware. Every transaction boundary is visible in the handler code. |
| 57 | Seeds via goose `--no-versioning` | Seed files are SQL, applied without version tracking. Can be re-run safely if written idempotently (INSERT ON CONFLICT DO NOTHING). |
