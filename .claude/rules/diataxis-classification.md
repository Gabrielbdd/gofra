# Diataxis Classification

All user-facing documentation under `docs/framework/` follows the Diataxis
framework. Use this classification when deciding where content belongs.

## Categories

### Tutorial (`docs/framework/tutorials/`)
- Learning-oriented.
- Guides the reader through a first successful outcome.
- Aimed at beginners who have never used the framework.
- Always hands-on: the reader does something and sees a result.
- Do not explain why — just show the path to success.

### How-to (`docs/framework/how-to/`)
- Goal-oriented.
- Solves one specific real task the user already understands.
- Assumes the reader knows the basics and needs a recipe.
- Focused and practical — steps, not concepts.

### Reference (`docs/framework/reference/`)
- Information-oriented.
- Exact facts about the current public supported surfaces.
- Authoritative and current-state only — if the code changes, the reference
  must change in the same task.
- Factual, non-rhetorical, structured consistently.
- No design rationale, no opinions, no future plans.

### Explanation (`docs/framework/explanation/`)
- Understanding-oriented.
- Discusses why the framework works this way.
- Covers tradeoffs, alternatives considered, and conceptual models.
- May reference decisions from the design docs but does not duplicate them.
- Helps users build a mental model, not accomplish a task.

## Project docs (`docs/project/`)
- Contributor-only material: internals, plans, migration notes, readiness,
  gaps, architecture decisions.
- Not part of the Diataxis user-facing tree.

## Cross-linking

When creating or updating a page, add links between related pages across
Diataxis categories where useful. For example, a reference page may link to
a related how-to or explanation, and vice versa.
