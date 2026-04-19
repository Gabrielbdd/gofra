# Agent Workflow — Long-Running Work

Some tasks are too large for a single session: multi-phase refactors,
architectural migrations, investigations that span multiple files and
decisions. This document defines how agents (and humans) hand work off
between sessions without losing context.

## When to use a progress file

Create a progress file when any of the following is true:

- The task is expected to span more than one working session.
- The task is an investigation whose outcome is not yet a single clear
  implementation step.
- The task is a refactor staged in phases, where each phase ships
  independently.
- You are blocked and need to resume later with the same context.

Isolated bug fixes, small doc edits, and single-session feature work do
not need a progress file. The commit message is enough.

## Location and naming

Progress files live under:

```
docs/project/progress/<kebab-slug>.md
```

The slug is short, descriptive, and kebab-cased. Examples:

- `docs/project/progress/restate-runtime-slice.md`
- `docs/project/progress/auth-mvp.md`
- `docs/project/progress/scaffold-starter-vite-wiring.md`

Archived (completed) progress files move to:

```
docs/project/progress/done/<kebab-slug>.md
```

## Required content

Every progress file must include the following sections, in this order:

1. **Objective** — what this work achieves and why it matters.
2. **Context consulted** — specific docs, files, and external sources
   that informed the current state of the plan. Include
   `docs/17-decision-log.md` entries by number when relevant.
3. **Decisions already taken** — concrete choices that are locked in.
   Link to new entries in `docs/17-decision-log.md` when the decision is
   architecturally durable.
4. **Next steps** — the immediate upcoming actions, in order.
5. **Blockers** — anything preventing forward progress, and what would
   unblock it.
6. **Pending validations** — tests, builds, or smoke checks that must
   pass before the task is considered done.
7. **Current state** — a single dated line summarizing where things
   stand. Example:
   ```
   2026-04-18 — scaffold generator refactor complete; starter tests
   green; auth slice not started.
   ```

## When to update

Update the file at the end of each working session or the moment you
become blocked. The goal is that a new agent starting from scratch
tomorrow can pick up your work by reading only this file and the
referenced sources of truth.

Do not batch updates to "when I'm done" — that defeats the purpose. A
stale progress file is worse than no progress file.

## When to archive

When the task is fully resolved — code merged, docs updated, validations
green — move the file from `docs/project/progress/` to
`docs/project/progress/done/` and remove its entry from any index. Keep
the file; the history is useful for future work that revisits the same
area.
