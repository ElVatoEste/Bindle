# Bindle — Anatomía de un paquete

Un paquete Bindle tiene dos caras: el **repo fuente** (lo que editas y versionas en git) y el **artefacto distribuible** (lo que viaja al registry y se instala).

## 1. Repo fuente del módulo

Lo que un autor mantiene en git.

```text
modfact/                       # repo del módulo (1 módulo = 1 librería *LIB: MODFACT)
├── bindle.json                # manifiesto: identidad, exports, deps, build, migraciones
├── src/
│   ├── FACTSRV.rpgle          # service program = API pública
│   ├── FACTMOD.rpgle          # módulo = implementación (oculta)
│   └── FACTPR.rpgleinc        # /copy member = prototipos públicos ("header")
├── binder/
│   └── FACTSRV.bnd            # binder language: define exports + fija la signature
├── migrations/
│   ├── 0001_init.sql          # DDL versionado, orden lexicográfico
│   └── 0002_add_impuesto.sql
├── tests/                     # (opcional) pruebas del módulo
└── README.md
```

Reglas:
- **1 módulo = 1 librería.** Aísla como un módulo de Odoo aísla su carpeta.
- El consumidor **solo** ve `FACTSRV` (service program) + `FACTPR` (prototipos). El `*MODULE` interno nunca se expone.
- El **binder language** controla qué se exporta y, por ende, la **signature**. Agregar exports al final = signature compatible.

## 2. Artefacto distribuible (en el registry)

Lo que `bindle publish` empaqueta y `bindle install` consume. El código fuente **no** es la unidad de distribución — los **objetos ILE compilados** lo son.

```text
registry/modfact/2.3.0/
├── MODFACT.savf               # SAVF: *SRVPGM + *MODULE + *BNDDIR compilados
├── bindle.json                # manifiesto del release (con signature ya calculada)
├── migrations/                # copia del DDL versionado
│   ├── 0001_init.sql
│   └── 0002_add_impuesto.sql
├── copy/
│   └── FACTPR.rpgleinc        # prototipos (para compilar consumidores sin el fuente)
└── index.json                 # metadatos: version, signature, hashes, deps
```

### `index.json` (metadatos del release)

```jsonc
{
  "name": "modfact",
  "version": "2.3.0",
  "signature": "A1B2C3D4E5F6...",        // firma de la interfaz pública
  "artifact": "MODFACT.savf",
  "hash": "sha256:...",                   // integridad
  "copy": "copy/FACTPR.rpgleinc",
  "dependencies": { "modbase": ">=1.0.0 <2.0.0", "modimp": "^1.2.0" },
  "migrations": ["0001_init.sql", "0002_add_impuesto.sql"],
  "createdAt": "2026-06-27T00:00:00Z"
}
```

## 3. Qué pasa al instalar

`bindle install` toma el artefacto y lo materializa en el host:

| Pieza del paquete | Acción en IBM i |
|-------------------|-----------------|
| `MODFACT.savf` | `RSTLIB`/`RSTOBJ` → coloca `*SRVPGM`/`*MODULE`/`*BNDDIR` en la librería |
| `migrations/*.sql` | Ejecuta en orden contra el schema; registra las aplicadas |
| `copy/FACTPR` | Disponible para `/copy` al compilar el consumidor |
| `index.json.signature` | Validada contra `bindle.lock`; si difiere → aborta (`signature mismatch`) |
| `runtime.libraryList` | Se cablea en el `*LIBL` del proyecto consumidor |

## 4. Identidad y reproducibilidad

- **Versión** (semver) + **signature** (firma ILE) identifican un release de forma única.
- El **hash** del SAVF garantiza integridad en el fetch.
- El `bindle.lock` del consumidor fija los tres → instalación reproducible.

## 5. Dos modos de entrega (decisión de diseño)

1. **Binario (SAVF)** — rápido, no requiere compilar en el target. Necesita compatibilidad de nivel de OS/objeto.
2. **Desde fuente** — el artefacto incluye fuente y se recompila en el target. Más portable entre niveles de OS, más lento.

MVP: priorizar **SAVF**; soportar build-from-source como fallback. Ver [`ROADMAP.md`](ROADMAP.md).
