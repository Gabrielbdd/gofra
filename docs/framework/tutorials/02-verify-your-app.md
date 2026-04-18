# 2. Verify your app is alive

In [Tutorial 1](01-your-first-gofra-app.md) you started a Gofra app. Now you
will verify it is running correctly, and you will learn what each probe
endpoint is really for. A production deployment depends on these contracts.

Keep the app running: infra up, `mise run dev` in another terminal.

## The three health probes

Open a new terminal and run:

```bash
curl -i http://localhost:3000/livez
```

You will get:

```
HTTP/1.1 200 OK
Content-Type: application/json
...

{"status":"alive"}
```

Now:

```bash
curl -i http://localhost:3000/readyz
```

You will also get `200 OK`. The response body includes the status of each
registered readiness check:

```json
{"status":"ready","checks":{"postgres":"ok"}}
```

And the startup probe:

```bash
curl -i http://localhost:3000/startupz
```

Also `200 OK` with body `{"status":"started"}`.

All three return 200, but they answer three different questions.

### `/livez` — is this process responsive?

Liveness only checks that the Go process is answering HTTP. It never
inspects external dependencies. This is deliberate: if `/livez` checked
Postgres, a single Postgres outage would cause Kubernetes to restart every
replica at once — a thundering herd that makes recovery slower, not faster.

Kubernetes uses `/livez` to decide whether to **restart** a container.
Restarting won't fix a broken database, so `/livez` deliberately can't fail
because of one.

### `/readyz` — should this process receive traffic?

Readiness checks whether the app can currently serve requests. For the
starter, that means: can it reach Postgres?

Try this. In the terminal where infra is running, stop Postgres:

```bash
mise run infra:stop
```

Now curl `/readyz` again:

```bash
curl -i http://localhost:3000/readyz
```

You should see `503 Service Unavailable`. The `checks` map shows the
`postgres` check failing:

```json
{"status":"not_ready","checks":{"postgres":"failed to connect..."}}
```

Kubernetes uses `/readyz` to decide whether to **route traffic** to this pod.
A failing readiness check removes the pod from the load balancer without
killing the process, so it can recover without restarting.

Bring Postgres back:

```bash
mise run infra
```

Within a few seconds `/readyz` returns `200 OK` again. You did not restart
the Go process — the pool reconnected on its own.

### `/startupz` — has this process finished starting?

Startup probes exist so Kubernetes knows not to run liveness checks against
a process that has not yet finished initializing. For the starter, this
returns 200 as soon as `runtimeserve.Serve` is ready to accept connections.

If your app needed to run a long warm-up (load a model, prime a cache), you
would expose that progress through `/startupz` without affecting `/livez`.

### Why these live outside app middleware

All three endpoints are registered on the **root mux**, before any
application middleware runs. Look at `cmd/app/main.go`:

```go
root := http.NewServeMux()
root.Handle(runtimehealth.DefaultStartupPath, health.StartupHandler())
root.Handle(runtimehealth.DefaultLivenessPath, health.LivenessHandler())
root.Handle(runtimehealth.DefaultReadinessPath, health.ReadinessHandler())

app := chi.NewRouter()
// ...auth middleware attaches here...
root.Handle("/", app)
```

If authentication middleware wrapped the probes, a broken auth provider
would mark the pod unhealthy and Kubernetes would restart it — cascading a
dependency failure into a container crash loop. Keeping probes on the root
mux means they answer independently of app concerns.

## The public config endpoint

Now the runtime config endpoint:

```bash
curl -i http://localhost:3000/_gofra/config.js
```

The response is JavaScript, not JSON:

```javascript
window.__GOFRA_CONFIG__ = {"appName":"hello","apiBaseUrl":"http://localhost:3000","auth":{...}};
```

This single script tag hydrates the browser with everything the frontend
needs to start — without a second round trip. The SPA loads it with a
`<script>` tag before any framework code runs.

Two design choices worth naming:

1. **Only the `public` subtree is served.** Any field under `app.*` or
   `database.*` — including your database DSN — is never reachable from
   here. The proto schema is the boundary.
2. **Camel case on the wire.** Proto fields like `app_name` become
   `appName` in JavaScript. Your SPA code reads `runtimeConfig.appName`,
   not `runtimeConfig.app_name`.

## The web shell

Open <http://localhost:3000> in the browser and view the page source.

The `<head>` loads `/_gofra/config.js` before any inline script. The inline
script at the bottom reads `window.__GOFRA_CONFIG__` and renders the app
name, the API base URL, and the full config as pretty-printed JSON.

This is the default starter web shell. It is intentionally minimal: it
proves the runtime config path end-to-end from proto → Go → browser. When
you plug in a real frontend (React, Vue, plain HTML — anything) you replace
`web/index.html` with your own bundle and keep the same config-loading
contract.

## What you learned

- Three probes exist because they answer three different operational
  questions: process responsiveness, traffic-worthiness, and
  initialization-done.
- Liveness deliberately ignores dependencies. Readiness deliberately
  depends on them.
- Probes live outside app middleware so a broken app layer can't cause
  restart loops.
- `/_gofra/config.js` serves only the `public` subtree of your proto
  schema. The boundary between server-side and browser-safe config is
  enforced by schema, not by discipline.

## Next

- [Tutorial 3: Understanding what was generated](03-understanding-what-was-generated.md)
  — a walkthrough of every file in the generated tree and how they fit
  together.
