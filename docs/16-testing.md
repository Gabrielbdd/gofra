# 16 — Testing

> Parent: [Index](00-index.md) | Prev: [Docker Compose](15-docker-compose.md) | Next: [Decision Log](17-decision-log.md)

---

## Testing Strategy

Forge has three test layers, each testing different things at different speeds.

### Unit Tests (fast, no Docker)

Test business logic extracted from handlers:

```go
func TestSlugify(t *testing.T) {
    assert.Equal(t, "hello-world", slugify("Hello World"))
    assert.Equal(t, "my-post-title", slugify("My Post Title!"))
}
```

### Connect Handler Tests (fast, no Docker)

**Decision #30.** Use `httptest.Server` with the generated Connect client.
Tests the full request-response path (serialization, validation, handler
logic, response shape) in-process:

```go
func TestListPosts(t *testing.T) {
    db := forge.TestDB(t)
    factory.CreateMany[models.Post](db, 5)

    recorder := forge.NewRestateRecorder()
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

`RestateRecorder` captures Restate dispatch calls for assertions without
running Restate:

```go
assert.Equal(t, 1, recorder.SendCount("SearchIndexer", "Index"))
```

### Integration Tests (slower, Docker required)

**Decision #31.** Restate handlers need the real journal to test correctly.
Tagged `integration`, run separately:

```go
//go:build integration

func TestSearchIndexer(t *testing.T) {
    db := forge.TestDB(t)
    svc := services.SearchIndexer{Queries: sqlc.New(db)}

    env := restatetest.Start(t, restate.Reflect(svc))
    client := env.Ingress()

    _, err := restateingress.Service[IndexRequest, restate.Void](
        client, "SearchIndexer", "Index",
    ).Request(t.Context(), IndexRequest{PostID: 42})

    require.NoError(t, err)
}
```

### Frontend Tests

The SPA uses mock transports for testing against generated types:

```ts
import { createRouterTransport } from "@connectrpc/connect";
import { PostsService } from "../gen/myapp/posts/v1/posts_connectweb";

const mockTransport = createRouterTransport(({ service }) => {
  service(PostsService, {
    listPosts: () => ({
      posts: [{ id: 1n, title: "Test", slug: "test" }],
      total: 1, page: 1, perPage: 10,
    }),
  });
});
```

### Runtime Config Tests

The generated runtime-config feature has its own test surface on both sides.

Go-side tests:

- generator golden tests for nested field binding, slices, and missing-field
  rejection
- resolver tests for clone semantics, mutator ordering, and error propagation
- handler tests for `GET`/`HEAD`, content type, `Cache-Control: no-store`, and
  the JavaScript envelope `window.__FORGE_CONFIG__ = ...`
- `httptest` integration tests that resolve a real `config.Config`, fetch
  `/_forge/config.js`, and parse the payload back into the runtime proto

Frontend tests:

- unit tests for `loadRuntimeConfig()` and `validateRuntimeConfig()` with a
  valid payload
- failure tests for missing `window.__FORGE_CONFIG__`, wrong field types, and
  missing required auth fields
- unit tests confirming `auth.ts` and `transport.ts` read from
  `runtimeConfig`, not `import.meta.env`
- app-boot tests that fail fast on invalid runtime config instead of mounting a
  partially configured SPA

**Reason for dedicated runtime-config tests**: this feature sits between Go
config, generated code, JavaScript bootstrapping, and browser auth. It needs
tests at the generator, handler, and frontend-loader boundaries.

## Test Database

`forge.TestDB(t)` creates an isolated test database for each test:
- Creates a temporary database (or uses a transaction that rolls back)
- Runs migrations via embedded goose
- Returns a `*sql.DB` scoped to the test

## Factories

Factories produce test data using sqlc-generated types:

```go
factory.Create[models.Post](db, factory.With{
    "Title":  "Test Post",
    "Status": "published",
})

factory.CreateMany[models.Post](db, 10) // 10 posts with default values
```

## Decisions in This Section

| # | Decision | Rationale |
|---|----------|-----------|
| 30 | `RestateRecorder` for handler tests | Fast HTTP tests without Docker. Verify dispatch, not execution. |
| 31 | Docker-based Restate integration tests | Durable handlers need real journal. Tagged `integration`. |
| 133 | Runtime config tested at generator, handler, and frontend-loader boundaries | Public browser config spans Go, generated code, and SPA startup. One test layer is not enough. |
