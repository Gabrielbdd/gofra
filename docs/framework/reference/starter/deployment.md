# Deployment

> Files the starter ships for building a container image and running CI.

## Status

Alpha — the shape of these files may change before v1.

## Files

| File | Purpose |
| --- | --- |
| `Dockerfile` | Multi-stage build that produces a static binary in a distroless runtime image. |
| `.dockerignore` | Excludes local state (git, IDE, compose, env files) from the build context. |
| `.github/workflows/ci.yml` | GitHub Actions workflow that tests, builds, and locally builds the image. |

## `Dockerfile`

The Dockerfile uses three stages:

1. **Frontend** — `node:22-alpine`. Runs `npm install` and `npm run build`
   in `web/`, producing `web/dist/`.
2. **Builder** — `golang:1.25-alpine`. Downloads modules, copies the
   project, copies `web/dist/` from the frontend stage, then compiles
   `./cmd/app` with `CGO_ENABLED=0`, `-trimpath` and `-ldflags="-s -w"`.
   `//go:embed all:dist` in `web/embed.go` bundles the built frontend
   into the binary.
3. **Runtime** — `gcr.io/distroless/static-debian12:nonroot`. Copies the
   compiled binary, switches to the `nonroot` user, exposes port `3000`,
   and sets the binary as the entrypoint.

The result is a reproducible, static image with the SPA embedded that
runs as a non-root user by default.

Build it with:

```bash
docker build -t <app>:dev .
```

## `.dockerignore`

Excludes `.git`, `.github`, `.gitignore`, `.dockerignore`, `bin/`, `dist/`,
`tmp/`, `*.md`, `.env*`, `compose.yaml`, `.vscode`, and `.idea`. The build
context is limited to source code plus whatever the build actually needs.

## `.github/workflows/ci.yml`

Triggers:

- Every pull request.
- Every push to `main`.

Steps, in order:

1. `actions/checkout@v4`.
2. `jdx/mise-action@v2` — installs the toolchain declared in `mise.toml`
   (Go and Node).
3. `mise run test` — runs `go test ./...` after `mise run generate`.
4. `mise run build` — runs `generate`, `web:build` (npm install + vite
   build), then `go build` to `bin/<app>` with `web/dist` embedded.
5. `docker/setup-buildx-action@v3` + `docker/build-push-action@v6` — builds
   the Docker image locally (`push: false`) tagged `<app>:ci` using the
   three-stage Dockerfile (Node → Go → distroless).

The workflow does not publish the image. Registry publishing is added per
project when deployment needs it.

## Related

- [Generated App Layout](generated-app-layout.md)
