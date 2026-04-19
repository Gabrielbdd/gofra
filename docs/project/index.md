# Project Documentation

This directory contains maintainer and contributor documentation for the Gofra
framework. It covers internals, architecture decisions, roadmap, and processes
that are not relevant to developers using Gofra in their applications.

## Contents

- [Documentation System](documentation-system.md) — How user-facing and
  project docs are organized and enforced.
- [Agent Workflow](agent-workflow.md) — Progress file convention for
  long-running work that spans more than one session.
- [Deferred Auth/Authz MVP Proposal](auth-authz-mvp-proposal.md) — Captured
  implementation shape for the postponed auth/authz runtime slice.

## Numbered Design Documents

The numbered design documents in `docs/` (e.g., `docs/01-foundations.md`
through `docs/19-implementation-gaps.md`) are the detailed architecture and
design material for the framework. They are maintainer-facing and describe
both implemented and planned behavior.

These are **not** end-user reference. For current supported behavior, see
[docs/framework/reference/](../framework/reference/index.md).

## What Belongs Here

- Contributor workflows and processes
- Internal architecture and implementation details
- Roadmap, readiness tracking, and implementation gaps
- ADRs and decision log context
- Migration notes and upgrade plans
- Anything that is not relevant to a developer using Gofra in their app
