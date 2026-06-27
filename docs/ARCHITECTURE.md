# Bindle — Arquitectura

> Borrador inicial. Las decisiones marcadas `[DECISIÓN]` están abiertas (ver ROADMAP).

## Vista de alto nivel

```
        ┌─────────────────────────────────────────────┐
        │                 bindle CLI                    │
        │  (corre en máquina dev y/o en IBM i PASE)     │
        └───────┬───────────────┬───────────────┬──────┘
                │               │               │
        ┌───────▼──────┐ ┌──────▼──────┐ ┌──────▼───────┐
        │  Resolver    │ │  Registry    │ │  Transport    │
        │ (grafo deps, │ │  client      │ │  (a IBM i)    │
        │  signatures) │ │ (fetch/push) │ │ SSH / ODBC    │
        └───────┬──────┘ └──────┬──────┘ └──────┬───────┘
                │               │               │
                ▼               ▼               ▼
        ┌──────────────────────────────────────────────┐
        │                   IBM i host                   │
        │   *LIB  *SRVPGM  *MODULE  *BNDDIR  Db2 tables   │
        │   library list (*LIBL)        journals          │
        └──────────────────────────────────────────────┘
```

## Componentes del CLI

### 1. Manifest engine
- Lee/valida `bindle.json` (ver `MANIFEST_SPEC.md`).
- Calcula el closure de dependencias declaradas.

### 2. Resolver
- Construye el grafo de dependencias (orden topológico para build/install).
- Resuelve versiones (rango → versión concreta) **y valida signatures**.
- Detecta conflictos: misma lib pedida con signatures incompatibles → error claro.
- Produce/lee `bindle.lock` (pin exacto: versión + signature + hash del artefacto).

### 3. Registry client
- `fetch`: descarga artefactos (SAVF u objetos) + metadatos.
- `publish`: empaqueta y sube.
- **[DECISIÓN]** backend MVP: directorio IFS / SAVF en un host IBM i / bucket S3-compatible.
- Índice del registry: `index.json` por paquete con versiones, signatures, hashes, deps.

### 4. Builder
- Compila fuente → `*MODULE` → `*SRVPGM` en orden de dependencia.
- **[DECISIÓN]** envolver **Bob (ibmi-bob)** vs orquestación propia. Preferencia: envolver Bob al inicio.
- Genera/actualiza el `*BNDDIR` del módulo a partir del manifiesto.

### 5. Installer
- Resuelve (Resolver) → fetch (Registry) → coloca objetos en el host:
  - **RSTOBJ/RSTLIB** desde SAVF, o **recompila** desde fuente en el target.
- Corre **migraciones DDL** del módulo en orden (crea/altera tablas, triggers, procs).
- Wiring: agrega el `*BNDDIR` (compile) y ajusta el **library list** (runtime) del proyecto/job.
- Idempotente: re-instalar no duplica; respeta el lock.

### 6. Transport
- **[DECISIÓN]** SSH (ssh/scp a PASE + comandos CL vía `system`) vs ODBC (mapepire) vs itoolkit.
- Necesita: ejecutar CL, transferir SAVF/IFS, correr SQL (migraciones).

## Modelo de datos del paquete

```
módulo MODFACT @ 2.3.0
├── bindle.json                # manifiesto
├── src/                       # fuente RPG (QRPGLESRC equivalente)
│   ├── FACTSRV.rpgle          # service program (API pública)
│   ├── FACTMOD.rpgle          # implementación
│   └── FACTPR.rpgleinc        # /copy prototipos (header público)
├── binder/                    # binder language (controla exports + signature)
│   └── FACTSRV.bnd
├── migrations/                # DDL versionado
│   ├── 0001_init.sql
│   └── 0002_add_impuesto.sql
└── bindle.lock                # (en proyectos consumidores)
```

## Versionado y signatures (el corazón)
- Cada release de un `*SRVPGM` lleva: `version` (semver) **+** `signature` (firma binaria de la interfaz pública).
- Regla: **agregar exports al final** del binder language → signature estable hacia atrás.
- El lock fija ambos. En install, Bindle valida que la signature del objeto resuelto coincide con la esperada; si no → `signature mismatch`, aborta antes de romper runtime.
- Cambio incompatible de API = bump de **major** + nueva signature documentada.

## Resolución runtime
- En ejecución, IBM i busca objetos por **library list**. El installer escribe la `*LIBL` correcta (o un setup CL/`ADDLIBLE`) para el proyecto consumidor.
- Bindle puede generar un programa/CL de arranque que fija el library list según el lock (DEV/TEST/PROD por entorno).

## Stack del CLI
- **Go 1.26+** (decidido). Binario único estático, cross-platform, port `aix/ppc64` (corre en PASE si hace falta). Sin runtime que instalar.
- CLI con **cobra**. Output con un wrapper propio (tablas/árbol/errores).
- Conectividad: **SSH** (`golang.org/x/crypto/ssh`) pa CL + transferencia SAVF; **ODBC/mapepire** pa SQL (migraciones).
- Layout: `cmd/bindle` (entrypoint), `internal/{manifest,resolver,registry,builder,installer,transport}`.
- Tests con `go test`. Lint `golangci-lint`.

## Seguridad
- Credenciales de host/registry fuera del repo (`.env`, gestor de secretos).
- Verificación de integridad de artefactos por hash en el lock.
- (Futuro) firma de paquetes en el registry.
