# Repository Guidelines

## Project Scope

This repository defines a modern, batteries-included Go framework inspired by Phoenix, Laravel, and Rails. The framework aims to be explicit, productive, and operationally robust, with durable execution, typed APIs, integrated frontend support, and strong local tooling. The current phase is documentation-first: contributions mainly refine architecture, conventions, and workflow before full implementation lands.

This is no longer a docs-only repo. Early implementation slices now live beside
the design docs. Work may touch:

- architecture and product docs in `docs/`
- reusable framework packages under the public `runtime/` surface
- generator code under `internal/` and `cmd/`
- the canonical generated-app starter under `internal/scaffold/starter/`

## Project Structure & Module Organization

Start at `docs/00-index.md`, then follow the numbered design set by subsystem:
foundations, architecture, API layer, database, auth, tooling, testing, and
decision log. Keep new documents in the existing `NN-topic.md` format and
place them where they fit the design map.

The source of truth for repo layout is `docs/02-system-architecture.md`. It
documents two distinct structures that must not be conflated:

- the **framework repo layout**, where reusable packages such as
  `runtime/config/`, generator internals in `internal/`, codegen entrypoints in
  `cmd/`, and the canonical starter in `internal/scaffold/starter/` live
- the **generated app layout**, which is the target output of future
  `gofra new`

When adding implementation, prefer this sequence:

1. Add or refine the framework contract in the docs.
2. Implement reusable framework code under the public `runtime/` surface.
3. Wire the slice into the canonical starter under
   `internal/scaffold/starter/`.
4. Extract narrower post-create generators only after the base starter contract
   is stable.

Do not treat the framework repo itself as if it were a generated application.
The starter source in `internal/scaffold/starter/full/` is the canonical
generated user app shape for the current phase.

## Build, Test, and Development Commands

Implementation is still early. Some commands are real today, while others are
documented target workflow for the full framework.

Current runnable commands:

- `go test ./...` runs the current Go test suite.
- `go run ./cmd/gofra --help` shows the current CLI entrypoint shape.
- `mise run test` runs the same test suite through the repo task runner.
- `mise run new -- ../myapp` generates a starter-backed application for manual testing.
- `mise run smoke:new` generates a temporary app and runs `go test ./...` inside it.
- `go run ./cmd/gofra generate runtime-config -h` shows the current generator
  entrypoint shape.

Target workflow:

- `mise install` installs the pinned Go toolchain for this repo.
- `docker compose up -d` starts local infrastructure such as Postgres, Restate, and Zitadel.
- `mise run gen` regenerates code from protobuf and SQL definitions.
- `mise run dev` runs the Go app and Vite frontend locally.
- `mise run test` runs `go test ./...`.
- `mise run lint` runs `buf lint` and `golangci-lint run`.

When editing docs, keep these commands consistent with `docs/14-tooling.md` and `docs/15-docker-compose.md`.

## Documentation Style & Naming

Write docs as framework design, not feature notes. Be direct, opinionated, and specific about defaults, tradeoffs, and developer workflow. Prefer concrete examples such as `app/services/`, `gofra generate service`, and port numbers. Cross-link related docs and record durable architectural choices in `docs/17-decision-log.md`.

If a change advances or completes a tracked readiness item, update
`docs/18-readiness-checklist.md` in the same change so the checklist reflects
current progress.

If implementation changes the framework layout, generator shape, developer
workflow, or any kept v1 promise, update the relevant architecture docs in the
same change. In most cases this means reviewing:

- `docs/02-system-architecture.md`
- `docs/14-tooling.md`
- `docs/17-decision-log.md`
- `docs/18-readiness-checklist.md`

## Testing & Validation Expectations

Use `docs/16-testing.md` as the source of truth for the framework’s testing model: unit tests, Connect handler tests, and `integration`-tagged Restate tests. When proposing new behavior, document how it should be tested and whether it affects generators, runtime behavior, or local developer ergonomics.

For scaffold work, verify the narrowest realistic slice instead of pretending
the whole framework exists already. Example: a reusable package plus one
starter-backed runnable app slice plus focused tests is a valid first step if
the docs and commit message make that scope explicit.

## Commit & Pull Request Guidelines

Follow the existing Conventional Commit style, for example `docs: initial docs`. Use clear prefixes such as `docs:`, `feat:`, `fix:`, and `chore:`.

Always commit when you finish a task. Do not wait for the user to ask you to
commit. Commit after every discrete change set — do not batch multiple
finished changes into one later commit. If a task produces multiple logical
changes (e.g., a new package plus updated docs), prefer one commit per logical
unit rather than one giant commit at the end.

Before committing, run `go test ./...` (and `mise run smoke:new` when scaffold
files changed) to verify the build is green. Do not commit broken code.

Pull requests should explain the framework decision being changed, list affected docs, and call out any impact on generated code, developer workflow, or architecture. If a PR changes commands, ports, project layout, or framework defaults, update the relevant documentation in the same change.
