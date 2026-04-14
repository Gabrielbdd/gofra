# Diataxis Documentation Skill

Use this skill when creating or updating user-facing documentation for Gofra.

## Workflow

1. **Classify the change.** Determine which Diataxis category the
   documentation belongs in:
   - Tutorial — learning-oriented, first success
   - How-to — goal-oriented, solve one task
   - Reference — exact facts about public surfaces
   - Explanation — why it works this way, tradeoffs

2. **Identify required updates.** Check whether the change affects:
   - An existing reference page that needs updating
   - A new public surface that needs a new reference page
   - A how-to or tutorial that references changed behavior

3. **Create or update the page.** Use the appropriate template from
   `templates/` in this skill directory. Place the file in the correct
   subdirectory of `docs/framework/`.

4. **Follow the style rules:**
   - Reference: factual, non-rhetorical, structured. No opinions or rationale.
   - Explanation: conceptual, discusses tradeoffs. No step-by-step instructions.
   - How-to: practical steps for a specific goal. Assumes baseline knowledge.
   - Tutorial: guided path to first success. No assumptions about prior knowledge.

5. **Cross-link.** Add links between related pages in different Diataxis
   categories where they help the reader.

6. **Verify scope.** Reference pages must not describe framework internals,
   generator mechanics, or unimplemented features. If content is
   internal-only, it belongs in `docs/project/`.

## Templates

- `templates/reference.md` — for API, CLI, and config reference pages
- `templates/how-to.md` — for goal-oriented guides
- `templates/tutorial.md` — for learning-oriented walkthroughs
- `templates/explanation.md` — for conceptual and rationale content
