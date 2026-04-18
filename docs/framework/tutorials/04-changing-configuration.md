# 4. Changing configuration

Gofra apps read configuration from four layers, in strict precedence order:

```
struct defaults  →  gofra.yaml  →  environment variables  →  CLI flags
     lowest                                                    highest
```

In this tutorial you will change the HTTP port three different ways and
watch each layer override the previous one. By the end you will understand
not just *how* to configure a Gofra app but *why* the four layers exist.

Keep your `hello/` project from Tutorial 1 open. Postgres should be running
(`mise run infra`).

## 1. The starting point

Stop any running `mise run dev`. Then start it again:

```bash
mise run dev
```

You should see:

```
level=INFO msg="starting app" app=hello addr=:3000
```

The port `:3000` comes from the proto default for `app.port`. Open
`proto/<package>/config/v1/config.proto`:

```proto
int32 port = 2 [(gofra.config.v1.field).default_value.int32_value = 3000];
```

The annotation is the source of truth. `mise run generate` reads it and
emits a `DefaultConfig()` Go function that sets `3000`. You can confirm
this in `config/config_gen.go`.

Stop the app with `Ctrl+C`.

## 2. Layer 2: override with `gofra.yaml`

Open `gofra.yaml`. It looks like this:

```yaml
app:
  name: hello
  port: 3000
```

Change the port to `4000`:

```yaml
app:
  name: hello
  port: 4000
```

Restart:

```bash
mise run dev
```

The log now shows `addr=:4000`. Point your browser at
<http://localhost:4000/livez> and you should get `{"status":"alive"}`.

What just happened: `runtimeconfig.Load` applied `gofra.yaml` on top of the
defaults. `gofra.yaml` is the right place for values that every
environment shares (every developer, every CI run, every deployment) but
that differ from the proto defaults.

It is not the place for secrets, because this file is in version control.

Stop the app.

## 3. Layer 3: override with an environment variable

Revert `gofra.yaml` back to port `3000` so you can see the env layer win:

```yaml
app:
  name: hello
  port: 3000
```

Now start the app with an env var:

```bash
GOFRA_APP__PORT=5000 mise run dev
```

The log shows `addr=:5000`. The browser answers on port 5000.

Two things to internalize about the env layer:

- **Prefix is `GOFRA_`.** You can change it for your own app with
  `runtimeconfig.WithEnvPrefix("MYAPP_")`, but the starter uses the
  default.
- **Nesting separator is `__` (double underscore).** `GOFRA_APP__PORT`
  maps to `app.port`. `GOFRA_PUBLIC__AUTH__CLIENT_ID` maps to
  `public.auth.client_id`. Single underscore is a valid character inside
  a key name (e.g., `auto_migrate`), so nesting can't use it without
  ambiguity. This is the same convention Docker Compose and .NET use for
  the same reason.

Env vars are the right layer for **environment-specific** values:
deployment URLs, database DSNs, credentials, feature flags. They set them
without editing tracked files and without requiring a code change.

Stop the app.

## 4. Layer 4: override with a CLI flag

`mise run dev` does not forward extra arguments to the Go binary, so invoke
the app directly for this step. First regenerate config once (if you have
not already run `dev`):

```bash
mise run generate
```

Now start the app with a flag:

```bash
go run ./cmd/app --app.port=6000
```

The log shows `addr=:6000`.

Flag names are dotted paths matching the YAML structure: `--app.port`,
`--public.app_name`, `--database.auto_migrate`. You can see the full list
in `config/load_gen.go` under `NewFlagSet()`.

Flags are the highest-precedence layer. They are right when you want a
one-off override that doesn't deserve a YAML or env change — usually
during debugging.

## 5. Watch all four layers interact

Now try the flag with a conflicting env var:

```bash
GOFRA_APP__PORT=5000 go run ./cmd/app --app.port=7000
```

The result is `addr=:7000`. The flag wins over the env var.

Try the opposite — only the env var, with `gofra.yaml` set to 4000:

```yaml
app:
  port: 4000
```

```bash
GOFRA_APP__PORT=5000 mise run dev
```

Result: `addr=:5000`. The env var wins over YAML.

Revert `gofra.yaml` back to `port: 3000` when you are done experimenting.

## 6. Secret handling

One field behaves differently: `database.dsn`.

Try to set it with a flag:

```bash
go run ./cmd/app --database.dsn=postgres://...
```

You will get:

```
unknown flag: --database.dsn
```

Look back at the proto:

```proto
string dsn = 1 [(gofra.config.v1.field).secret = true, ...];
```

The `secret = true` annotation has two consequences:

1. **No CLI flag is registered.** `config/load_gen.go` does not include
   `database.dsn` in `NewFlagSet()`. Command-line invocations can't leak
   it into shell history.
2. **It is not served to the browser.** The `PublicConfigHandler` only
   serves fields under the `public` message. `database.*` is invisible
   from `/_gofra/config.js` regardless of the `secret` flag — but the
   `secret` flag is the general rule for any field that must never leave
   the server.

You can still override a secret with an env var or with `gofra.yaml`:

```bash
GOFRA_DATABASE__DSN="postgres://..." mise run dev
```

That is the intended path: secrets flow through env vars provisioned by
your deployment platform, never through CLI flags or code.

## What you learned

- **Four layers, strict precedence.** Defaults → YAML → env → flags, and
  each layer overrides anything lower.
- **Each layer has a purpose.** Proto defaults for framework-wide
  sensibility. `gofra.yaml` for project-shared overrides. Env vars for
  environment-specific values including secrets. Flags for temporary
  overrides during development.
- **Double-underscore separates nesting in env vars.** Not a stylistic
  choice — single underscore is ambiguous for keys that contain
  underscores.
- **`secret` is a proto annotation, not a separate config.** One boolean
  on the field removes CLI flag support and keeps the value out of the
  browser.

## Where to go from here

- Read [runtime/config](../reference/runtime/config.md) for the full
  public API of the loader, including `WithEnvPrefix`, `WithYAMLFile`, and
  programmatic overrides.
- Read [Generated App Layout](../reference/starter/generated-app-layout.md)
  to see how the proto schema, `gofra.yaml`, and the generated flag set
  fit together.
- The numbered design document
  [06-configuration.md](https://github.com/Gabrielbdd/gofra/blob/main/docs/06-configuration.md)
  covers the architectural rationale behind these choices.
