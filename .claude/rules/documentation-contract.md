# Documentation Contract

Every change to a public framework surface must include a corresponding
documentation update in `docs/framework/`. This is enforced by a Stop hook.

## Public surfaces

A change is public-facing if it touches any of these paths:

- `runtime/**`
- `cmd/gofra/**`
- `proto/**`
- `internal/scaffold/starter/full/**`

## Rules

1. If a change modifies a public surface, update or create the appropriate
   page under `docs/framework/` in the same task.
2. If a new public surface is introduced, create a new reference page for it.
3. If a change is internal-only (e.g., `internal/` packages other than the
   starter, CI config, repo tooling), no user-facing doc update is required.
4. For `internal/scaffold/starter/full/**` changes, document the generated-app
   behavior or layout the user sees — not the scaffold template mechanics.
5. Before finishing any task, confirm which docs changed. If public surfaces
   changed but no `docs/framework/` file was added or modified, stop and fix
   that before completing.

## Doc locations

- **User-facing docs:** `docs/framework/` — organized by Diataxis.
- **Project/maintainer docs:** `docs/project/` and the numbered design docs
  in `docs/`.
- **Reference:** `docs/framework/reference/` — strictly current-state facts
  about public surfaces. No design rationale, no internals.
