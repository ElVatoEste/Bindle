# modgreet — callable proof

Proves end-to-end that the service program **Bindle built** is callable on the
host: a program binds it and invokes its exported procedure.

```bash
# from the repo root, with the host reachable:
PROFILE=pub400 LIB=VATODEV1 SRVPGM=GRTBNDL \
  bash examples/modules/modgreet/test/run-callable.sh
```

Output (verified live on pub400, IBM i 7.5):

```
BINDLE-RESULT: Hello, Bindle! (from Bindle)
```

What it does:
1. `bindle put` uploads `GRTOUT.rpgle` (the caller).
2. Creates a binding directory over the built `*SRVPGM` and compiles the caller.
3. `CALL GRTOUT` — binds `BGREET` from the Bindle-built service program, calls it,
   and prints the result to stdout via C `printf` (captured over SSH; no data area
   and no job that can hang in MSGW).

✅ **Status: verified.** The Bindle-built service program (deterministic signature
`99FB4E0FACC21955936359A366D9FBD3`) resolves and runs its export end to end.
