# Reference

Authoritative, factual documentation for Gofra's public surfaces. Reference
pages describe current supported behavior — what exists today, not planned
features.

## Runtime Packages

- [runtime/config](runtime/config.md) — Configuration loading with
  four-layer precedence and frontend config serving.
- [runtime/health](runtime/health.md) — Kubernetes-aligned health check
  probes (startup, liveness, readiness).
- [runtime/serve](runtime/serve.md) — Graceful HTTP server lifecycle with
  signal handling and shutdown sequencing.
- [runtime/errors](runtime/errors.md) — Connect RPC error helpers with
  structured error details.

## CLI

- [gofra CLI](cli/gofra.md) — Project bootstrapping and code generation.

## Generated App

- [Generated App Layout](starter/generated-app-layout.md) — Structure and
  conventions of a `gofra new` application.
