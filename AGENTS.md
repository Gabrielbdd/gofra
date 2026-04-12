# Repository Guidelines

## Project Scope

This repository defines a modern, batteries-included Go framework inspired by Phoenix, Laravel, and Rails. The framework aims to be explicit, productive, and operationally robust, with durable execution, typed APIs, integrated frontend support, and strong local tooling. The current phase is documentation-first: contributions mainly refine architecture, conventions, and workflow before full implementation lands.

## Project Structure & Module Organization

All active work lives in `docs/`. Start at `docs/00-index.md`, then follow the numbered design set by subsystem: foundations, architecture, API layer, database, auth, tooling, testing, and decision log. Keep new documents in the existing `NN-topic.md` format and place them where they fit the design map.

The intended framework structure is documented in `docs/02-system-architecture.md`: `proto/` for contracts, `app/` for RPC handlers and Restate components, `db/` for migrations and queries, `web/` for the React SPA, `gen/` for generated code, and `cmd/app` for the executable entrypoint.

## Build, Test, and Development Commands

The implementation is still being designed, but the target workflow is already defined:

- `mise install` installs pinned Go, Node, buf, and generator tooling.
- `docker compose up -d` starts local infrastructure such as Postgres, Restate, and Zitadel.
- `mise run gen` regenerates code from protobuf and SQL definitions.
- `mise run dev` runs the Go app and Vite frontend locally.
- `mise run test` runs `go test ./...`.
- `mise run lint` runs `buf lint` and `golangci-lint run`.

When editing docs, keep these commands consistent with `docs/14-tooling.md` and `docs/15-docker-compose.md`.

## Documentation Style & Naming

Write docs as framework design, not feature notes. Be direct, opinionated, and specific about defaults, tradeoffs, and developer workflow. Prefer concrete examples such as `app/services/`, `forge generate service`, and port numbers. Cross-link related docs and record durable architectural choices in `docs/17-decision-log.md`.

If a change advances or completes a tracked readiness item, update
`docs/18-readiness-checklist.md` in the same change so the checklist reflects
current progress.

## Testing & Validation Expectations

Use `docs/16-testing.md` as the source of truth for the framework’s testing model: unit tests, Connect handler tests, and `integration`-tagged Restate tests. When proposing new behavior, document how it should be tested and whether it affects generators, runtime behavior, or local developer ergonomics.

## Commit & Pull Request Guidelines

Follow the existing Conventional Commit style, for example `docs: initial docs`. Use clear prefixes such as `docs:`, `feat:`, `fix:`, and `chore:`.

Commit after every discrete change set requested by the user. Do not batch
multiple finished doc changes into one later commit.

Pull requests should explain the framework decision being changed, list affected docs, and call out any impact on generated code, developer workflow, or architecture. If a PR changes commands, ports, project layout, or framework defaults, update the relevant documentation in the same change.
