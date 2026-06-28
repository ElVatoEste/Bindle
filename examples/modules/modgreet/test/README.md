# modgreet — callable proof (manual, needs a reachable IBM i)

This proves end-to-end that the service program **Bindle built** is callable on the
host: a program binds it and invokes its exported procedure.

```bash
# from the repo root, with the host reachable:
PROFILE=pub400 LIB=VATODEV1 SRVPGM=GRTBNDL \
  bash examples/modules/modgreet/test/run-callable.sh
```

Expected final value:

```
Hello, Bindle! (from Bindle)
```

What it does:
1. `bindle put` uploads `GRTCALL.rpgle` (the caller).
2. Sets up `*DTAARA GRTRES`, a binding directory over the built `*SRVPGM`, and
   compiles the caller (`GRTCALL`) plus a CL runner (`GRTRUN`).
3. `GRTRUN` sets `INQMSGRPY(*DFT)` then `CALL GRTCALL`, so a runtime escape
   auto-replies instead of leaving the job in **MSGW** (which hangs SSH).
4. Reads `GRTRES` — the greeting returned by the bound procedure.

> Status: written and ready; not yet run end-to-end because pub400 was
> unreachable. Run it when the host is back to close the "installed & callable"
> proof.
