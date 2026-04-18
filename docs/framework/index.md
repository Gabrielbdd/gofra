# Gofra

Gofra is a batteries-included Go framework inspired by Phoenix, Laravel,
and Rails. It is designed to be explicit, productive, and operationally
robust, with durable execution, typed APIs, integrated frontend support,
and strong local tooling.

!!! note "Alpha"
    Gofra is in active development. Public surfaces are stable enough to
    use in hobby and exploratory projects, but any API may change before
    a v1 release.

## Current scope of these docs

This site documents **only what ships today**. Pages appear only when they
describe real, current-state behavior. Empty placeholders are deliberately
not published — a missing page means that surface is not yet stable enough
to commit to.

What is covered:

- **[Tutorials](tutorials/index.md)** — a four-step onboarding track that
  takes you from install to a running app, with a clear explanation of
  everything the starter produces.
- **[Reference](reference/index.md)** — exact current-state facts about
  the `runtime/*` packages, the `gofra` CLI, and the generated app
  layout.

What is **not** covered yet:

- **How-to guides** — single-task recipes (adding an RPC service, a
  database migration, ZITADEL integration, production deployment).
  These will arrive as each surface stabilizes.
- **Explanation** — deep rationale for architectural choices. The
  numbered design documents in the repo (`docs/00-index.md` onward) cover
  this ground for maintainers today; user-facing explanation will be
  extracted as surfaces solidify.

## Where to start

If you have never used Gofra: start with
[Tutorial 1](tutorials/01-your-first-gofra-app.md). The four tutorials run
in order and take under an hour end-to-end.

If you know the framework shape and need facts: jump to
[Reference](reference/index.md).

## Source code

Gofra is developed openly at
[github.com/Gabrielbdd/gofra](https://github.com/Gabrielbdd/gofra).

Maintainer-facing design documents live in the `docs/` tree in the
repository. They are not published here because they describe both
current and planned behavior — exactly the material that would make a
user-facing reference unreliable.
