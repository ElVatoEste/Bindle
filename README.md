<div align="center">

<img src="assets/banner.svg" alt="Bindle — package manager for IBM i" width="100%">

<br>

**The package &amp; dependency manager for IBM i.**
Reusable RPG/ILE business-logic modules — declared, resolved, built, and deployed from one CLI.

<br>

[![Status](https://img.shields.io/badge/status-working%20alpha-3b82f6)](docs/ROADMAP.md)
[![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Platform](https://img.shields.io/badge/platform-IBM%20i%20(ILE)-052FAD)](https://www.ibm.com/products/ibm-i)
[![License](https://img.shields.io/badge/license-GPLv3-10b981)](LICENSE)
[![PRs](https://img.shields.io/badge/PRs-welcome-10b981)](#contributing)

<sub>by <strong>Escalia Technologies</strong></sub>

</div>

---

## Why

IBM i has no unified, open package manager for native **ILE objects** (service programs, modules, RPG business logic). Teams reinvent the same plumbing: copy service programs by hand, juggle binding directories, hand-edit library lists, and re-run DDL from memory.

`yum`/`RPM` covers the PASE/open-source side. **Bindle covers the native ILE side** — the gap.

Bindle lets you package a unit of business logic as a **module** (think: an Odoo module for IBM i), publish it to a registry, and install it into any project with its dependencies resolved automatically.

```text
1 module  =  1 library (*LIB)
public API =  *SRVPGM  +  /copy prototype member
state      =  versioned DB migrations
manifest   =  bindle.json
```

## Quickstart

```bash
# 1. scaffold a module or project (creates bindle.json)
bindle init --module --name modgreet

# 2. build the module on the IBM i host (compile → signature → SAVF)
bindle build --profile prod

# 3. publish the artifact + metadata to the registry
bindle publish --artifact .bindle/build/GREETSRV.savf

# --- in a consumer project ---
bindle add modgreet                  # pin ^latest from the registry
bindle install                       # resolve → lock → fetch → verify (local)
bindle install --deploy --profile prod   # + restore onto IBM i, verify signature, wire *LIBL
```

That's the loop: **build → publish → add → install (→ deploy).** No hand-written
binder source, no manual `RSTOBJ`, no guessing which DDL to run.

> **Try it now** (no IBM i needed) — these already resolve a real dependency graph,
> write a reproducible lock, and fetch + verify artifacts:
> ```bash
> go run ./cmd/bindle list      -f examples/miapp/bindle.json --registry examples/registry
> go run ./cmd/bindle list tree -f examples/miapp/bindle.json --registry examples/registry
> go run ./cmd/bindle install   -f examples/miapp/bindle.json --registry examples/registry
> ```
> See [`examples/`](examples/).

## A module in one file — `bindle.json`

```jsonc
{
  "schema": "bindle/v0",
  "name": "modfact",
  "version": "2.3.0",
  "library": "MODFACT",                 // 1 module = 1 library (*LIB)

  "exports": {
    "srvpgm": "FACTSRV",                // public service program
    "copy":   "FACTPR"                  // /copy member = the public API "header"
    // Bindle generates the binder source + a deterministic signature.
    // Optionally pin the export symbols: "symbols": ["CALCFACT", "APPLYTAX"]
  },

  "dependencies": {
    "modbase": ">=1.0.0 <2.0.0",
    "modimp":  "^1.2.0"
  },

  "build":      { "engine": "native", "src": "src/", "objects": ["FACTMOD"] },
  "migrations": { "dir": "migrations/", "schema": "MODFACT" },
  "runtime":    { "libraryList": ["MODFACT", "MODBASE", "MODIMP"] }
}
```

Full spec: [`docs/MANIFEST_SPEC.md`](docs/MANIFEST_SPEC.md) · Package layout: [`docs/PACKAGE_ANATOMY.md`](docs/PACKAGE_ANATOMY.md)

## How `install` works

```text
bindle install                       bindle install --deploy
  │                                    │  (everything on the left, then:)
  ├─ read   bindle.json + lock         ├─ upload SAVF over SFTP
  ├─ resolve graph (vers + sigs)       ├─ CPYFRMSTMF → RSTOBJ into target lib
  ├─ fetch  artifacts from registry    ├─ verify *SRVPGM signature == lock
  ├─ verify sha256 against the lock    └─ wire the library list (*LIBL)
  └─ cache locally                           │
        │                                     ▼
        ▼                              ✓ module installed & bindable on IBM i
  ✓ reproducible, verified cache
```

> Internal component view (manifest · resolver · registry · builder · installer · transport) lives in [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

## Registry

Modules are published to a **registry** — an index of versioned artifacts (SAVF + metadata) that `install` reads from.

```text
publish:  bindle build → package SAVF → push artifact + metadata → registry
store:    registry/<name>/<version>/{ MODFACT.savf, bindle.json, index.json }
consume:  bindle add → resolve → fetch from registry → install
```

MVP backend is pluggable (IFS directory / SAVF host / S3-compatible bucket). Details: [`docs/REGISTRY.md`](docs/REGISTRY.md).

## Where Bindle fits

Each tool solves a different slice. Bindle fills the **native-ILE package management** gap.

| Tool | Scope | Native ILE objects | Dep resolution | Registry / publish | DB migrations |
|------|-------|:------------------:|:--------------:|:------------------:|:-------------:|
| **yum / RPM** | PASE / open-source pkgs | ❌ | ✅ (PASE only) | ✅ | ❌ |
| **Git** | Source version control | ❌ (source, not objects) | ❌ | ❌ | ❌ |
| **ACS** | GUI client (5250, DB, transfer) | ❌ | ❌ | ❌ | ❌ |
| **Bob (ibmi-bob)** | Build / compile | ✅ (build) | ⚠️ compile-time only | ❌ | ❌ |
| **ARCAD / Merlin** | Full ALM (commercial, heavy) | ✅ | ✅ | ✅ | ⚠️ partial |
| **Bindle** | **ILE package management** | ✅ | ✅ (version + signature) | ✅ | ✅ |

Bindle is **complementary**: it can wrap Bob for builds and lives alongside Git for source.

## Built to coexist (adoption-first)

The hard part of a tool like this is not the code — it's **adoption in conservative IBM i shops**. Bindle is designed to slot into existing processes, not replace them:

- **No big-bang rewrite.** Adopt one module at a time; the rest of the system keeps building the traditional way.
- **Coexists with CL / existing change management.** Bindle generates standard objects (`*SRVPGM`, `*BNDDIR`, SAVF) and CL you can inspect and run yourself.
- **Wraps, doesn't fight, the toolchain.** Uses Bob for builds, Git for source, standard `RSTLIB`/library lists for deploy.
- **Reversible.** Everything Bindle produces is a normal IBM i object — you are never locked in.

## Project layout

```text
cmd/bindle/            entrypoint
internal/
  manifest/            read & validate bindle.json / bindle.lock
  resolver/            dependency graph, version + signature resolution
  registry/            fetch / publish artifacts (SAVF + metadata)
  builder/             compile RPG, generate binder + deterministic signature, SAVF
  installer/           local install (lock/fetch/verify) + deploy (RSTOBJ/sig-check/wire)
  config/              host-agnostic connection profiles (~/.bindle/config.json)
  transport/           SSH: run CL/PASE, SFTP upload/download
docs/                  VISION · ARCHITECTURE · MANIFEST_SPEC · PACKAGE_ANATOMY · REGISTRY · CONNECTION · BUILD · ROADMAP
examples/              runnable demo registry, consumer, and a buildable module
assets/                logo & banner
```

## Status

🛠️ **Working alpha.** All CLI commands are implemented (no stubs). The local
flow and on-host build run end-to-end; full deploy is implemented but not yet
proven on an unrestricted host.

**Verified working** (the last two live against a real IBM i, 7.5):

| Area | State |
|------|-------|
| `init` · `add` · `list` · `list tree` | ✅ |
| `install` (resolve → lock → fetch → **sha256 verify** → cache) | ✅ |
| `publish` (artifact + metadata to registry) | ✅ |
| `profile` · `ping` · `exec` · `put` · `get` | ✅ |
| `build` (compile RPG → `*SRVPGM` → SAVF) | ✅ live |
| **deterministic, signature-controlled** binder builds | ✅ live |
| **callable** — a program binds the built `*SRVPGM` and runs its export | ✅ live |
| `sql` · `migrate` (SQL channel: db2util, control table, idempotent) | ✅ live |

> Live end-to-end on pub400 (IBM i 7.5): `build` → a caller binds the result →
> `CALL` prints `BINDLE-RESULT: Hello, Bindle! (from Bindle)`. See
> [`examples/modules/modgreet/test`](examples/modules/modgreet/test).

**Implemented, not yet fully proven end-to-end:**

- `install --deploy` (RSTOBJ + signature check + wire `*LIBL`) — code + unit tests;
  the test host (pub400, shared) denies `RSTOBJ`, so the real restore awaits a host
  with restore authority.
- `install --deploy`'s RSTOBJ + auto-migrations path — migrations are packaged,
  fetched, and wired to run after restore; the restore itself awaits a host with
  `RSTOBJ` authority (pub400 denies it). `bindle migrate` and the packaging are
  verified live.
- mapepire SQL backend + job-log diagnostics — designed ([`docs/SQL_CHANNEL.md`](docs/SQL_CHANNEL.md)).

Docs: [`VISION`](docs/VISION.md) · [`ARCHITECTURE`](docs/ARCHITECTURE.md) · [`MANIFEST_SPEC`](docs/MANIFEST_SPEC.md) · [`PACKAGE_ANATOMY`](docs/PACKAGE_ANATOMY.md) · [`REGISTRY`](docs/REGISTRY.md) · [`CONNECTION`](docs/CONNECTION.md) · [`BUILD`](docs/BUILD.md) · [`SQL_CHANNEL`](docs/SQL_CHANNEL.md) · [`ROADMAP`](docs/ROADMAP.md)

## Build from source

```bash
git clone https://github.com/ElVatoEste/Bindle.git
cd Bindle
go build ./...
go run ./cmd/bindle --help
```

## Contributing

Early days — issues and discussion welcome. The MVP "definition of done" is in [`docs/ROADMAP.md`](docs/ROADMAP.md).

## License

Bindle is licensed under the **[GNU General Public License v3.0](LICENSE)**.

You may use, study, modify, and redistribute it freely. Any distributed derivative
must remain open under the GPLv3 — it **cannot** be taken closed-source.

© Escalia Technologies. Copyright is retained by the author.
**Commercial licensing** (for use without GPLv3 obligations) is available — contact Escalia Technologies.
