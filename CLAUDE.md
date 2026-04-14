@AGENTS.md

@.claude/rules/documentation-contract.md
@.claude/rules/diataxis-classification.md
@.claude/rules/reference-scope.md

## Documentation

- User-facing docs live in `docs/framework/`, organized by Diataxis
  (tutorials, how-to, reference, explanation).
- Project/maintainer docs live in `docs/project/` and the numbered design docs
  in `docs/`.
- Reference (`docs/framework/reference/`) is end-user only: current-state
  facts about public surfaces. No internals, no rationale, no future plans.
- If a change touches `runtime/**`, `cmd/gofra/**`, `proto/**`, or
  `internal/scaffold/starter/full/**`, update the corresponding docs under
  `docs/framework/` in the same task.
- If a new public surface is introduced, create a new reference page for it.
- Before finishing any task, confirm which `docs/framework/` pages were
  created or updated. A Stop hook enforces this.
