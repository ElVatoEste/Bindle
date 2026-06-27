# Bindle examples

A self-contained demo you can run today — no IBM i host needed.

```text
examples/
├── registry/            a local file-backed registry
│   ├── modfact/versions.json   (depends on modbase, modimp)
│   ├── modbase/versions.json
│   └── modimp/versions.json    (depends on modbase)
└── miapp/
    └── bindle.json      a consumer project depending on modfact
```

## Run it

From the repo root:

```bash
# flat list (install order: dependencies first)
go run ./cmd/bindle list -f examples/miapp/bindle.json --registry examples/registry

# dependency tree
go run ./cmd/bindle list tree -f examples/miapp/bindle.json --registry examples/registry

# resolve, write bindle.lock, fetch + verify artifacts into the cache
go run ./cmd/bindle install -f examples/miapp/bindle.json --registry examples/registry
```

Expected output:

```text
miapp 0.1.0
resolved 3 package(s):
  modbase  1.4.2
  modimp   1.2.5
  modfact  2.3.0

miapp 0.1.0
└── modfact 2.3.0
    ├── modbase 1.4.2
    └── modimp 1.2.5
        └── modbase 1.4.2
```

`install` resolves the graph, writes a reproducible [`miapp/bindle.lock`](miapp/bindle.lock),
then fetches each artifact and verifies its `sha256` against the lock before caching it
under `.bindle/cache/`. Run it twice: the first run writes the lock, the second reuses it.

This exercises the manifest parser, the dependency resolver (version selection +
topological order), the file-backed registry, and the installer (lock + fetch + hash
verification) — end to end, all in Go. Deploy to an IBM i host is the next step.
