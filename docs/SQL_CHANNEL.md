# Bindle — SQL channel (design)

A second channel to the IBM i host, alongside SSH, that runs **SQL** and reads
**structured system data**. It unlocks two things SSH-only cannot do cleanly:

1. **DB migrations** — run a module's versioned DDL/DML in order, transactionally.
2. **Full diagnostics** — read the job log (`QSYS2.JOBLOG_INFO`) and object
   metadata as result sets, instead of scraping CL spooled listings.

## Why a separate channel

CL over SSH (`system "<CL>"`) starts a **new job per call**, so:
- `QTEMP` and job state don't carry across calls.
- A command's job log is gone by the time the next call runs — binder `CPDxxxx`
  detail (which goes to the spooled listing, not stdout) is unreachable.
- SQL via `RUNSQLSTM` works but is still one-job-per-call and stringly-typed.

An SQL connection holds **one job for many statements**: migrations share a
transaction, and after a failed compile the *same* job's log is queryable.

## Backend options (one interface)

```go
// internal/sqlchan
type Conn interface {
    Exec(stmt string) error              // DDL/DML, no result set
    Query(stmt string) ([]Row, error)    // result set as []map[string]any
    Begin() (Tx, error)                  // commitment control
    Close() error
}
```

| Backend | Transport | Pros | Cons |
|---------|-----------|------|------|
| **mapepire** | TCP/WebSocket to the mapepire daemon (port 8076, present on pub400) | purpose-built for IBM i SQL, JSON results, pooled | needs the daemon running |
| **JDBC via SSH tunnel** | jt400 over a tunnel | mature | drags in the JVM |
| **`RUNSQLSTM` over SSH** | existing SSH transport | zero new deps; good *fallback* | one job per call, no shared tx |

Plan: define the `Conn` interface now; implement **mapepire** first (it's already
available on the test host), keep **`RUNSQLSTM`-over-SSH** as a no-daemon fallback.

## Migrations

```
module/
  migrations/
    0001_init.sql
    0002_add_impuesto.sql
```

Algorithm (`bindle install --deploy`, after RSTOBJ):

```
1. ensure control table:  <schema>.BINDLE_MIGRATIONS (id, checksum, applied_ts)
2. list migrations/, sort lexicographically
3. for each not in the control table:
     BEGIN
       run statements (split on ; , honoring CL/SQL block boundaries)
       INSERT into BINDLE_MIGRATIONS (id, checksum=sha256(file))
     COMMIT            -- one tx per migration; stop on first failure
4. if an applied migration's checksum changed -> error (immutability)
```

Properties: **idempotent** (re-running applies nothing), **ordered**,
**checksum-guarded** (a published migration must never change), **atomic per file**.

## Diagnostics

On a build/compile failure, query the same job's log:

```sql
SELECT MESSAGE_ID, MESSAGE_TEXT, MESSAGE_SECOND_LEVEL_TEXT
FROM TABLE(QSYS2.JOBLOG_INFO('*')) 
WHERE SEVERITY >= 30
ORDER BY ORDINAL_POSITION DESC
```

This returns the `CPDxxxx` binder detail and second-level help that the current
`ParseDiagnostics` (which scrapes stdout/stderr) cannot see. The builder keeps the
cheap stdout scrape and *adds* the job-log query when an SQL channel is configured.

## Security & config

- Reuses the connection profile (`~/.bindle/config.json`); add optional
  `sqlTransport` (`mapepire` | `runsqlstm`) and `mapepirePort` (default 8076).
- Credentials come from the same profile/env — never the repo.
- TLS for mapepire where the daemon offers it.

## Status

**Implemented** (`internal/sqlchan`) and verified live on pub400 (IBM i 7.5):

- `Conn` interface with a **db2util-over-SSH** backend (`Exec` + JSON `Query`).
  db2util is the open-source PASE CLI (present on pub400; `yum install db2util`
  elsewhere). `RUNSQL` was rejected as the query backend — it runs DDL/DML but
  does not return result sets; db2util returns `{"records":[...]}`.
- **Migrations** (`Migrate`): control table `<schema>.BINDLE_MIGRATIONS`, ordered
  apply, sha256 checksum guard (immutability), idempotent re-runs, one statement
  at a time with `;`-splitting that respects quoted literals.
- CLI: `bindle sql -- <stmt>` (query → JSON, else exec) and `bindle migrate`.

Verified: `bindle migrate` applied `0001_init` then skipped on re-run; the control
table recorded the checksum and the migration-created table accepted insert/select.

Not yet done: mapepire backend (TCP/WebSocket, no db2util dependency);
job-log diagnostics via `QSYS2.JOBLOG_INFO`; packaging a module's `migrations/`
into the registry artifact so `install --deploy` can run them automatically
(today `bindle migrate` runs them from the module source tree).
