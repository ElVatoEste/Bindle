# Bindle — Build subsystem (design)

Goal: turn a module's source into a distributable artifact (SAVF) **with a stable,
correctly-read ILE signature**, with no shortcuts that become technical debt.

## Backends (one seam, two implementations)

`build.engine` in the manifest selects the backend; both satisfy the same
`Builder` interface:

- **`native`** — compiles with CL (`CRTRPGMOD` / `CRTSRVPGM`) over the SSH
  transport. Zero host dependencies. The canonical, IBM-supported path.
- **`bob`** — wraps `ibmi-bob` (`makei`). Incremental, community standard. Added
  behind the same interface; not required for a complete native build.

## Signature: deterministic via explicit binder signature

`DSPSRVPGM` has **no OUTFILE** option (verified on 7.5: `CPD0043`), so reading the
signature back from a built object is not clean. Instead Bindle **controls** the
signature explicitly in the binder source — it knows the value without reading back:

```
STRPGMEXP PGMLVL(*CURRENT) SIGNATURE('<deterministic 1..16 chars>')
  EXPORT SYMBOL('BGREET')
ENDPGMEXP
```

- `CRTSRVPGM ... EXPORT(*SRCFILE) SRCSTMF('…/SRV.bnd')` uses it (binder from IFS
  works on 7.5 — verified).
- The signature value is **deterministic**, derived from the module name + **major**
  version (stable across minor/patch = compatible; changes on major = breaking).
  Bindle writes it, stores it in the lock, and publishes it — no fragile read-back.
- Backward compatibility when adding exports: keep the `*CURRENT` block and append
  prior signatures as `PGMLVL(*PRV)` blocks so old consumers still resolve.
- Export symbols come from the manifest (`exports.symbols`) or are scanned from the
  RPG source (`dcl-proc … export`); Bindle generates the binder source from them.

## Source delivery (CCSID-correct)

- Upload source to a remote IFS working dir over SFTP.
- Tag each stream file **CCSID 819** after upload (`setccsid 819`). Verified on 7.5:
  the RPG compiler **fails to open UTF-8/1208 source** (`RNS9339`) but reads 819
  cleanly. Mishandled CCSID = silently corrupt or unreadable source = classic debt.

## Compile pipeline (native)

```
1. ensure build library: CRTLIB if authorized; if not (e.g. pub400 CPD0032),
   require an existing library (profile defaultLibrary / manifest library)
2. generate binder source with explicit SIGNATURE from exports; upload src + binder
   to an IFS workdir; set CCSID 819
3. CRTRPGMOD MODULE(lib/MOD) SRCSTMF('.../MOD.rpgle')         # one per build object
4. CRTSRVPGM SRVPGM(lib/SRV) MODULE(lib/MOD...)
             EXPORT(*SRCFILE) SRCSTMF('.../SRV.bnd')          # binder => signature
5. signature is the value Bindle wrote (no read-back)
6. CRTSAVF + SAVOBJ OBJ(SRV + MODs) LIB(lib) DEV(*SAVF) TGTRLS(...)
7. CPYTOSTMF FROMMBR('/QSYS.LIB/lib.LIB/SAVF.FILE') TOSTMF('.../x.savf') CVTDTA(*NONE)
8. SFTP download the stream SAVF -> local artifact
9. cleanup IFS workdir (+ build objects if isolated)
```

All eight remote primitives above are **verified live on pub400 (IBM i 7.5)**.

## Error capture (the most-skipped completeness point)

CL compile failures do not print to stdout — diagnostics live in the **job log**
and the **compiler spooled listing**. On any failure Bindle retrieves and returns
them (`QSYS2.JOBLOG_INFO` via SQL, and/or the spooled file), so a failed build is
debuggable instead of blind.

## Isolation & reproducibility

- A unique IFS workdir per build; removed on success and failure.
- **`TGTRLS`** configurable (default `*CURRENT`) → cross-release portability and the
  basis for the v1 "build-from-source" fallback.
- Atomic: produce a valid SAVF or leave nothing behind.

## Builder interface (sketch)

```go
type Host interface {            // satisfied by transport.SSH (decoupled for tests)
    Run(cmd string) (transport.Result, error)
    RunCL(cl string) (transport.Result, error)
    Upload(local, remote string) error
    Download(remote, local string) error
}

type Builder interface {
    Build(h Host, m *manifest.Manifest, opts Options) (*Result, error)
}

type Result struct {
    Library, Srvpgm, Signature, TargetRls string
    Artifact string  // local SAVF path
    Log      string  // joblog/listing (always on failure)
}
```

## Testing strategy

- **Unit (no host):** binder-source generation, CL command construction, OUTFILE/CSV
  signature parsing, joblog parsing.
- **Integration (live):** one end-to-end build of the demo module against pub400
  (IBM i 7.5), asserting a non-empty signature and a downloadable SAVF.

## Verified host facts (pub400, IBM i 7.5)

Default shell `bsh` (no `uname`; `system "<CL>"` runs CL fine). yum present; Bob and
git not installed (native backend needs neither). Probe primitives live before
codifying each step.
