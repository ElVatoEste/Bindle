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

## Core ideas

| Idea | What it means |
|------|---------------|
| 🧩 **Declare, don't wire** | A `bindle.json` lists what a module exports and depends on. No hand-edited binding dirs. |
| 🔏 **Signature-aware** | ILE bindings break when a `*SRVPGM` signature changes. Bindle tracks signatures, not just version strings. |
| 📌 **Reproducible** | A lock file pins exact versions **and** signatures. Same inputs, same objects. |
| 🗄️ **State included** | Modules ship versioned DB migrations; install/upgrade applies them in order. |
| 🔗 **Runtime wiring** | Bindle assembles the binding directory at build time and the library list at runtime. |

## Planned CLI

```bash
bindle init                 # scaffold a new module / project (bindle.json)
bindle add <module>[@ver]   # add a dependency
bindle install              # resolve + fetch + restore objects + run migrations + wire *LIBL
bindle build                # compile module objects in dependency order
bindle publish              # package (SAVF) + push to registry
bindle list | list tree     # inspect the resolved dependency graph
```

## Architecture (at a glance)

```text
        ┌──────────────── bindle CLI (Go, single binary) ────────────────┐
        │   manifest → resolver → registry → builder → installer → transport │
        └───────────────────────────────┬─────────────────────────────────┘
                              SSH (CL/SAVF) + ODBC (SQL)
                                          ▼
        ┌──────────────────────────── IBM i host ───────────────────────────┐
        │   *LIB  *SRVPGM  *MODULE  *BNDDIR  Db2 tables  *LIBL  journals       │
        └────────────────────────────────────────────────────────────────────┘
```

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
docs/                  VISION · ARCHITECTURE · MANIFEST_SPEC · ROADMAP
assets/                logo & banner
```

## Status

🚧 **Early planning.** The CLI scaffold builds and runs (`bindle --help`); commands are stubs. See:

- [`docs/VISION.md`](docs/VISION.md) — the problem and the bet
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — components & data model
- [`docs/MANIFEST_SPEC.md`](docs/MANIFEST_SPEC.md) — the `bindle.json` format
- [`docs/ROADMAP.md`](docs/ROADMAP.md) — phases & open decisions

## Build from source

```bash
git clone https://github.com/ElVatoEste/Bindle.git
cd Bindle
go build ./...
go run ./cmd/bindle --help
```

## Contributing

Early days — issues and discussion welcome. The guiding "definition of done" for the MVP is in [`docs/ROADMAP.md`](docs/ROADMAP.md).

## License

TBD.
