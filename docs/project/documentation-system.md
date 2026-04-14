# Documentation System

This document explains how Gofra's documentation is organized, who each part
is for, and how the docs-as-code contract is enforced.

## Two Documentation Trees

Gofra separates documentation into two distinct trees:

### `docs/framework/` — User-Facing Documentation

Organized using the [Diataxis](https://diataxis.fr/) framework:

| Section | Purpose | Audience |
|---------|---------|----------|
| `tutorials/` | Learning-oriented walkthroughs | New users |
| `how-to/` | Goal-oriented task recipes | Active users |
| `reference/` | Factual API/CLI/config documentation | All users |
| `explanation/` | Rationale and tradeoff discussion | Curious users |

**Reference is the source of truth** for current supported behavior. If the
code and the reference disagree, the reference must be updated.

Reference pages document only what exists today — no planned features, no
implementation gaps, no internal mechanics.

### `docs/project/` and Numbered Design Docs — Maintainer Documentation

- `docs/project/` contains contributor processes and meta-documentation (like
  this file).
- The numbered design documents (`docs/00-index.md` through
  `docs/19-implementation-gaps.md`) are the detailed architecture and design
  material. They describe both implemented and planned behavior and include
  decision rationale.

**The numbered design docs are not end-user reference.** They are maintainer-
facing design documents that predate the Diataxis split. They will remain as
the architecture record; user-facing content is extracted into
`docs/framework/` as surfaces are implemented.

## The Documentation Contract

### For Contributors

When you change a public framework surface, update the corresponding user
documentation in `docs/framework/` in the same pull request. Public surfaces
are:

- `runtime/**`
- `cmd/gofra/**`
- `proto/**`
- `internal/scaffold/starter/full/**`

For starter changes, document the generated-app behavior the user sees, not
the template mechanics.

If the change is internal-only (other `internal/` packages, CI, repo tooling),
no `docs/framework/` update is needed.

### For AI Agents (Claude Code)

The same contract applies. Additionally:

- `CLAUDE.md` imports rules from `.claude/rules/` that remind Claude to check
  for docs updates.
- A **Stop hook** in `.claude/settings.json` runs
  `scripts/claude/check-docs-stop.sh` before Claude finishes a task. If public
  surfaces changed but no `docs/framework/` file was updated, the hook blocks
  completion and tells Claude to fix it.
- A skill in `.claude/skills/diataxis-docs/` provides templates and
  classification guidance for creating new documentation.

### For CI

The same script supports a `--ci` mode:

```bash
scripts/claude/check-docs-stop.sh --ci
```

This prints human-readable errors to stderr and exits non-zero if the
contract is violated. It can be added to CI pipelines or run locally via:

```bash
mise run docs:guard
```

## Reference Scope

Reference pages under `docs/framework/reference/` may include:

- Public runtime APIs, types, functions, constants
- CLI commands and flags
- Config keys, env vars, defaults, precedence
- Generated app structure users work with
- Endpoints, routes, ports, and public behavior
- Error contracts users receive

Reference pages must not include:

- `internal/` package structure
- Generator/scaffold internals
- Repo contributor workflows
- Implementation gaps or readiness tracking
- ADR mechanics
- Future or unimplemented features

## Diataxis Classification

| If the content is... | Put it in... |
|----------------------|-------------|
| A guided first experience | `tutorials/` |
| Steps to solve a specific task | `how-to/` |
| Exact facts about an API, command, or config | `reference/` |
| Why something works this way | `explanation/` |
| Contributor process, internals, roadmap | `docs/project/` |
