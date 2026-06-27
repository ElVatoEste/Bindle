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

This exercises the manifest parser, the dependency resolver (version selection +
topological order), and the file-backed registry — end to end, all in Go.
