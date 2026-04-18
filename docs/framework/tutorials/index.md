# Tutorials

A guided introduction to Gofra. Work through the four tutorials in order —
they build on each other. Together they take less than an hour and leave you
with a real app running on your machine and an accurate mental model of how
Gofra is put together.

## The track

1. **[Your first Gofra app](01-your-first-gofra-app.md)** — install the CLI,
   generate a starter, boot Postgres, and run the app. You finish with a
   browser tab pointing at your local app.
2. **[Verify your app is alive](02-verify-your-app.md)** — the three health
   probes, the public config endpoint, and the web shell's data flow.
   You learn what each probe is really for, and why they live outside
   application middleware.
3. **[Understanding what was generated](03-understanding-what-was-generated.md)**
   — a file-by-file walkthrough of the generated tree. You learn where
   the framework ends and your code begins.
4. **[Changing configuration](04-changing-configuration.md)** — exercise
   the four configuration layers (defaults → YAML → env → flags) by
   changing the HTTP port three different ways.

## What the tutorials don't cover

Gofra is in early alpha. The tutorials cover **only what ships today**:

- Scaffolding a starter app.
- The runtime packages the starter wires up: `runtime/config`,
  `runtime/database`, `runtime/auth` (opt-in), `runtime/health`,
  `runtime/serve`.
- Local development against Postgres in Docker or Podman.
- Building a binary and a container image locally.

Not yet covered, because the surfaces are not yet stable:

- Adding a Connect RPC service.
- Adding a Restate durable workflow.
- Integrating ZITADEL authentication end-to-end.
- Deploying to a managed environment.

Those will appear here as each surface stabilizes. In the meantime, the
[Reference section](../reference/index.md) documents the current-state
facts about every package already shipping.
