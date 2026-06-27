# Bindle — Connection profiles

Bindle is **host-agnostic**: it is never tied to one IBM i. The host is
configuration, supplied per user / per environment. Credentials never live in a
repository.

## Where config lives

```text
~/.bindle/config.json      # local, never committed
```

```jsonc
{
  "defaultProfile": "pub400",
  "profiles": {
    "pub400": {
      "host": "pub400.com",
      "port": 2222,
      "user": "VATODEV",
      "transport": "ssh",
      "auth": "password"
    },
    "prod": {
      "host": "ibmi-prod.empresa.com",
      "user": "DEPLOY",
      "auth": "key",
      "keyFile": "~/.ssh/id_ed25519",
      "defaultLibrary": "DEPLOYLIB"
    }
  }
}
```

Same module, different host — just switch the profile (`--profile prod`). This is
how Bindle slots into an existing shop's environments instead of imposing one.

## Resolution precedence

A profile is assembled from, lowest to highest precedence:

```text
built-in defaults  <  config file profile  <  environment  <  command-line flags
```

| Field | Env var | Flag | Default |
|-------|---------|------|---------|
| profile | `BINDLE_PROFILE` | `--profile` | `defaultProfile` |
| host | `BINDLE_HOST` | `--host` | — (required) |
| user | `BINDLE_USER` | `--user` | — (required) |
| port | `BINDLE_PORT` | `--port` | `22` |
| transport | `BINDLE_TRANSPORT` | — | `ssh` |
| keyFile | `BINDLE_KEYFILE` | — | — |
| password | `BINDLE_PASSWORD` | — | — |

## Credentials

- **Preferred:** key or SSH agent (`auth: "key"` / `"agent"`).
- **Password:** supported, but provide it at runtime via `BINDLE_PASSWORD` rather
  than storing it. If stored in `~/.bindle/config.json`, it stays in your home —
  never in a repo.
- `BINDLE_PASSWORD` automatically selects password auth.

## Inspecting

```bash
bindle profile list          # configured profiles
bindle profile show          # the resolved profile (password masked)
bindle profile show --profile prod --host override.example.com
```

## Host requirements

Any IBM i a profile points at must provide:

- IBM i **7.3+** (7.4/7.5 recommended).
- **SSH** enabled (or ODBC for the SQL path).
- For `build`: ILE compilers (and ideally Bob).
- **Journaling** for migrations under commitment control.
- A user profile with authority to create libraries/objects and run CL.
