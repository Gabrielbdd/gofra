# Reference Scope

Reference pages under `docs/framework/reference/` describe the public surfaces
that developers using Gofra interact with in their applications. This file
defines what belongs in reference and what does not.

## Reference may include

- Public runtime APIs under `runtime/` — types, functions, constants, defaults.
- CLI commands and flags in `cmd/gofra`.
- Config keys, environment variables, defaults, precedence, and validation
  visible to users.
- Generated app structure that users edit and work with daily.
- Endpoints, routes, health paths, ports, and public behavior users rely on.
- Public error contracts users receive (Connect error codes, detail types).
- Stability/status of each surface.

## Reference must not include

- `internal/` package structure or implementation details.
- Scaffold/generator template mechanics (how the generator works internally).
- Repo-only contributor workflows (how to contribute, release process).
- Implementation gaps or readiness tracking.
- ADR mechanics or decision log entries.
- Future or unimplemented features — reference is current-state only.
- Design rationale — that belongs in `docs/framework/explanation/`.

## Template

Every reference page should follow a consistent structure:

1. What it is (one-line description)
2. Status/stability
3. Import path or command
4. Public API surface (types, functions, constants, flags)
5. Defaults
6. Behavior
7. Errors or edge cases
8. Short examples
9. Related pages
