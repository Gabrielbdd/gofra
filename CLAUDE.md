@AGENTS.md

@.claude/rules/documentation-contract.md
@.claude/rules/diataxis-classification.md
@.claude/rules/reference-scope.md

## Gospa Product

Gospa is the first product built with Gofra — an open-source PSA
(Professional Services Automation) for MSPs. Its product blueprint lives at
`docs/psa/index.md`. Consult it when making framework design decisions that
Gospa would exercise — it provides concrete use cases for ticketing, billing,
multi-tenancy, workflows, and AI.

Gospa lives in a separate repository at `github.com/Gabrielbdd/gospa`. It is
not a submodule of this repo. For simultaneous local development, the
external `gospa-workspace` repo wires both via submodules and `go.work`.
