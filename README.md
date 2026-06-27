<div align="center">

<img src="assets/banner.svg" alt="Bindle — package manager for IBM i" width="100%">

<br>

**The package &amp; dependency manager for IBM i.**
Reusable RPG/ILE business-logic modules — declared, resolved, built, and deployed from one CLI.

<br>

[![Status](https://img.shields.io/badge/status-early%20planning-f59e0b)](docs/ROADMAP.md)
[![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Platform](https://img.shields.io/badge/platform-IBM%20i%20(ILE)-052FAD)](https://www.ibm.com/products/ibm-i)
[![License](https://img.shields.io/badge/license-TBD-64748b)](#license)
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
# 1. scaffold a project (creates bindle.json)
bindle init

# 2. add a dependency from the registry
bindle add modfact@^2.3.0

# 3. resolve + fetch + restore objects + run migrations + wire *LIBL
bindle install

# 4. build your own module's objects (in dependency order)
bindle build

# 5. publish your module so others can install it
bindle publish
```

That's the whole loop: **init → add → install → build → publish.** No hand-edited binding directories, no manual `RSTLIB`, no remembering which DDL to run.

## A module in one file — `bindle.json`

```jsonc
{
  "schema": "bindle/v0",
  "name": "modfact",
  "version": "2.3.0",
  "library": "MODFACT",                 // 1 module = 1 library (*LIB)

  "exports": {
    "srvpgm": "FACTSRV",                // public service program
    "binder": "binder/FACTSRV.bnd",     // binder language = defines the signature
    "copy":   "FACTPR"                  // /copy member = the public API "header"
  },

  "dependencies": {
    "modbase": ">=1.0.0 <2.0.0",
    "modimp":  "^1.2.0"
  },

  "build":      { "engine": "bob", "src": "src/", "objects": ["FACTMOD", "FACTSRV"] },
  "migrations": { "dir": "migrations/", "schema": "MODFACT" },
  "runtime":    { "libraryList": ["MODFACT", "MODBASE", "MODIMP"] }
}
```

Full spec: [`docs/MANIFEST_SPEC.md`](docs/MANIFEST_SPEC.md) · Package layout: [`docs/PACKAGE_ANATOMY.md`](docs/PACKAGE_ANATOMY.md)

## How `install` works

```text
bindle install
  │
  ├─ read   bindle.json + bindle.lock
  ├─ resolve dependency graph        (versions + ILE signatures)
  ├─ fetch   artifacts from registry (SAVF + metadata, verified by hash)
  ├─ restore objects to IBM i        (SAVF → RSTLIB / RSTOBJ)
  ├─ migrate DB schema               (run module migrations, in order)
  └─ wire    *BNDDIR + library list  (compile-time + runtime resolution)
        │
        ▼
  ✓ module API callable from your program
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
  builder/             compile in dep order, *BNDDIR, signature, SAVF
  installer/           resolve→fetch→restore→migrate→wire *LIBL/*BNDDIR
  transport/           SSH (CL/SAVF) + ODBC (SQL) to the host
docs/                  VISION · ARCHITECTURE · MANIFEST_SPEC · PACKAGE_ANATOMY · REGISTRY · ROADMAP
assets/                logo & banner
```

## Status

🚧 **Early planning.** The CLI scaffold builds and runs (`bindle --help`); commands are stubs. See:

- [`docs/VISION.md`](docs/VISION.md) — the problem and the bet
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — components & data model
- [`docs/MANIFEST_SPEC.md`](docs/MANIFEST_SPEC.md) — the `bindle.json` format
- [`docs/PACKAGE_ANATOMY.md`](docs/PACKAGE_ANATOMY.md) — what's inside a package
- [`docs/REGISTRY.md`](docs/REGISTRY.md) — publish · store · consume
- [`docs/ROADMAP.md`](docs/ROADMAP.md) — MVP → v1.0 → v2.0

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

TBD.
