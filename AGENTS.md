# Repository Guidelines

## Project Scope

This repository defines **Gofra**, a modern, batteries-included Go framework
inspired by Phoenix, Laravel, and Rails. The framework aims to be explicit,
productive, and operationally robust, with durable execution, typed APIs,
integrated frontend support, and strong local tooling.

Gofra is co-developed alongside its first real-world product, **Gospa** — an
open-source PSA (Professional Services Automation) for MSPs. Gospa validates
the framework in production while informing its design. See
[Gofra + Gospa](#gofra--gospa) below.

Work in this repository may touch:

- Architecture and product docs in `docs/`
- Reusable framework packages under the public `runtime/` surface
- Generator code under `internal/` and `cmd/`
- The canonical generated-app starter under `internal/scaffold/starter/`

## Gofra + Gospa

Gospa lives in a **separate repository** at `github.com/Gabrielbdd/gospa`. It
is not a submodule of this repo. The two projects are wired together only
through the normal Go module graph: Gospa has `require github.com/Gabrielbdd/gofra vX.Y.Z`
in its `go.mod`; Gofra has no dependency on Gospa.

For simultaneous local development, an external workspace repo
(`gospa-workspace`) includes both as git submodules and uses a `go.work` file
so edits in one resolve immediately in the other — without any `replace`
directive committed to `go.mod`.

### Rules of direction

- Gospa depends on Gofra (via `go.mod`). Gofra does NOT depend on Gospa.
- If Gospa needs something new from the framework → build it in Gofra first,
  then consume it in Gospa.
- If Gofra evolves a contract → update Gospa to validate it works in a real
  app.
- Bugs found via Gospa are fixed in Gofra if they are framework problems, in
  Gospa if they are product logic.

### When working on Gofra

Focus on generic, reusable framework code. Gospa is context for understanding
real use cases, not a target for edits from this repo.

### When working on Gospa

Focus on product. Gofra provides the building blocks. If something is missing
in the framework, build it in Gofra first.

### Gospa blueprint

`docs/psa/index.md` is the product blueprint — market analysis, features,
architecture, and roadmap. It lives in the Gofra repo because it informs
framework design decisions.

## Sources of Truth

Before any implementation work, consult these documents. They are the
authoritative references for architecture, scope, decisions, and progress.

| Document | What it tells you |
|----------|-------------------|
| `docs/00-index.md` | Entry point — document map, tech stack summary, how to find anything |
| `docs/02-system-architecture.md` | Runtime diagram, framework-repo layout, generated-app structure — architectural guardrails |
| `docs/17-decision-log.md` | All numbered architectural decisions with rationale — search before proposing something that may already be decided |
| `docs/18-readiness-checklist.md` | V1 promises, release blockers, ship gates — the suggested order of work |
| `docs/19-implementation-gaps.md` | What is documented but not yet implemented — the concrete backlog |
| `docs/psa/index.md` | PSA product blueprint — market, features, architecture, roadmap |
| `docs/framework/reference/` | Current-state facts about public surfaces — what exists today |

If a numbered decision in `17-decision-log.md` already covers a topic, follow
it. If you disagree with a decision, surface that disagreement to the user
with reasoning — do not silently override it.

## Work Protocol — Plan Before You Build

### Agent posture: consultative partner, not passive executor

You are a technical partner, not a command runner. This means:

- **Question premises.** If the user asks for something that conflicts with
  the architecture, documented decisions, or established patterns, point it
  out and propose an alternative. Do not silently comply.
- **Propose improvements.** If during investigation you find a better, simpler,
  or more aligned approach, suggest it proactively — even if the user did not
  ask.
- **Clarify ambiguity.** If a request is vague or has multiple interpretations,
  ask before assuming. A wrong assumption wastes more time than a question.
- **Protect the project.** If a request would introduce technical debt, break
  conventions, or deviate from the architecture, you are obligated to flag it.
  Flagging is not blocking — it is informing so the decision is conscious.
- **Verify, don't trust blindly.** Do not treat user assertions as facts
  without checking. If the user says "this file does X", read the file and
  confirm. If the user says "we don't have Y", search before agreeing.
- **Be proactive.** Anticipate follow-up questions, related impacts, and
  missing context. Surface them before the user has to ask.

### Protocol steps

Every task follows this sequence. Skipping steps is not acceptable.

#### 1. Investigate

- Read the code, docs, and tests related to the task.
- Consult the [Sources of Truth](#sources-of-truth) above.
- Search for prior decisions in `docs/17-decision-log.md`.
- Check `docs/19-implementation-gaps.md` for known gaps.
- When needed, research official upstream documentation (Restate, Connect RPC,
  Zitadel, sqlc, TanStack, etc.) to ensure correctness.
- Understand what exists before proposing anything new.

#### 2. Clarify

- Confirm that the problem, the real objective, and the scope are clear.
- Identify which surfaces, packages, and docs are affected.
- Verify that the proposed direction is consistent with the architecture and
  documented decisions.
- If anything is unclear, ask. If something the user said seems wrong, say so
  with evidence.

#### 3. Plan and present

Present a structured plan before touching any files. Use this format:

```
Objective: what will be achieved and why
Context consulted: which docs, files, and sources were read
Areas affected: paths, surfaces, packages
Proposed plan: step-by-step description of what will be done
Risks and open questions: anything uncertain or potentially problematic
Confidence: problem X/10 · context Y/10 · solution Z/10
Awaiting user approval.
```

The confidence scores are critical. Be honest:
- **Problem** — how well you understand what needs to be solved.
- **Context** — how well you understand the surrounding architecture, patterns,
  and constraints.
- **Solution** — how confident you are that the proposed plan is correct and
  complete.

A score below 7 on any axis means you should explicitly state what you are
unsure about and what would raise your confidence.

#### 4. Gate — wait for approval

**Do not edit files, generate code, or commit before the user explicitly
approves the plan.** If the user requests changes to the plan, revise and
re-present. Only proceed after clear approval.

Exception: pure investigation (reading files, running tests, searching code)
does not require prior approval — only implementation does.

#### 5. Implement

- Follow the approved plan.
- If you need to deviate significantly from the plan, stop and re-present the
  updated plan before continuing.
- Small tactical adjustments (e.g., a slightly different function signature)
  are fine without re-approval, but mention them.

#### 6. Track progress

- If the work advances a V1 promise, update `docs/18-readiness-checklist.md`.
- If the work resolves an implementation gap, update
  `docs/19-implementation-gaps.md`.
- Update or create `docs/framework/` pages as required by the documentation
  contract.
- Confirm which docs were created or updated before finishing.

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
- `go run ./cmd/gofra generate config -h` shows the current generator
  entrypoint shape.

Target workflow:

- `mise install` installs the pinned Go toolchain for this repo.
- `docker compose up -d` starts local infrastructure such as Postgres, Restate, and Zitadel.
- `mise run gen` regenerates code from protobuf and SQL definitions.
- `mise run dev` runs the Go app and Vite frontend locally.
- `mise run test` runs `go test ./...`.
- `mise run lint` runs `buf lint` and `golangci-lint run`.

When editing docs, keep these commands consistent with `docs/14-tooling.md` and `docs/15-docker-compose.md`.

## Documentation

### Organization

The repository has two documentation trees:

- **`docs/framework/`** — user-facing documentation organized by
  [Diataxis](https://diataxis.fr/) (tutorials, how-to, reference,
  explanation). Reference is the source of truth for current supported
  behavior.
- **`docs/project/`** and the numbered design docs in `docs/` — maintainer
  and contributor documentation. Not end-user reference.

See `docs/project/documentation-system.md` for the full documentation system
design and enforcement rules.

### Style and naming

Write docs as framework design, not feature notes. Be direct, opinionated, and
specific about defaults, tradeoffs, and developer workflow. Prefer concrete
examples such as `app/services/`, `gofra generate service`, and port numbers.
Cross-link related docs and record durable architectural choices in
`docs/17-decision-log.md`.

### Documentation contract

If a change touches `runtime/**`, `cmd/gofra/**`, `proto/**`, or
`internal/scaffold/starter/full/**`, update the corresponding docs under
`docs/framework/` in the same task. If a new public surface is introduced,
create a new reference page. For `internal/scaffold/starter/full/**` changes,
document the generated-app behavior the user sees — not the template
mechanics.

Before finishing any task, confirm which `docs/framework/` pages were created
or updated. A Stop hook enforces this.

### Progress tracking

If a change advances or completes a tracked readiness item, update
`docs/18-readiness-checklist.md` in the same change. If implementation changes
the framework layout, generator shape, developer workflow, or any V1 promise,
review and update:

- `docs/02-system-architecture.md`
- `docs/14-tooling.md`
- `docs/17-decision-log.md`
- `docs/18-readiness-checklist.md`

## Testing & Validation Expectations

Use `docs/16-testing.md` as the source of truth for the framework's testing
model: unit tests, Connect handler tests, and `integration`-tagged Restate
tests. When proposing new behavior, document how it should be tested and
whether it affects generators, runtime behavior, or local developer ergonomics.

For scaffold work, verify the narrowest realistic slice instead of pretending
the whole framework exists already. Example: a reusable package plus one
starter-backed runnable app slice plus focused tests is a valid first step if
the docs and commit message make that scope explicit.

## Commit & Pull Request Guidelines

Follow the existing Conventional Commit style, for example `docs: initial
docs`. Use clear prefixes such as `docs:`, `feat:`, `fix:`, and `chore:`.

Always commit when you finish a task. Do not wait for the user to ask you to
commit. Commit after every discrete change set — do not batch multiple
finished changes into one later commit. If a task produces multiple logical
changes (e.g., a new package plus updated docs), prefer one commit per logical
unit rather than one giant commit at the end.

Before committing, run `go test ./...` (and `mise run smoke:new` when scaffold
files changed) to verify the build is green. Do not commit broken code.

Pull requests should explain the framework decision being changed, list
affected docs, and call out any impact on generated code, developer workflow,
or architecture. If a PR changes commands, ports, project layout, or framework
defaults, update the relevant documentation in the same change.
