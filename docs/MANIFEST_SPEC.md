# Bindle — Especificación del manifiesto (`bindle.json`)

> Borrador v0. Sujeto a cambio hasta el primer release.

Cada módulo (y cada proyecto consumidor) tiene un `bindle.json` en su raíz.

## Ejemplo — módulo que publica lógica

```jsonc
{
  "schema": "bindle/v0",
  "name": "modfact",                 // nombre lógico del paquete (kebab/lowercase)
  "version": "2.3.0",                // semver
  "description": "Lógica de facturación",
  "author": "Escalia Technologies",
  "license": "MIT",

  "library": "MODFACT",              // *LIB destino (1 módulo = 1 librería)

  "exports": {
    "srvpgm": "FACTSRV",             // service program público
    "binder": "binder/FACTSRV.bnd",  // binder language (define signature)
    "signature": "A1B2C3D4E5F6...",  // firma de la interfaz pública (calculada al build)
    "copy": "FACTPR"                  // /copy member con los prototipos (API/header)
  },

  "dependencies": {
    "modbase": ">=1.0.0 <2.0.0",
    "modimp":  "^1.2.0"
  },

  "build": {
    "engine": "bob",                 // "bob" | "native"
    "src": "src/",
    "objects": ["FACTMOD", "FACTSRV"]
  },

  "migrations": {
    "dir": "migrations/",            // DDL versionado, aplicado en orden lexicográfico
    "schema": "MODFACT"              // schema/colección SQL (= la librería)
  },

  "runtime": {
    "libraryList": ["MODFACT", "MODBASE", "MODIMP"]  // *LIBL sugerida para consumidores
  }
}
```

## Ejemplo — proyecto consumidor

```jsonc
{
  "schema": "bindle/v0",
  "name": "miapp",
  "version": "0.1.0",
  "private": true,
  "library": "MIAPP",
  "dependencies": {
    "modfact": "^2.3.0",
    "modclientes": "^1.0.0"
  },
  "registries": {
    "default": "ifs:///bindle/registry"   // o ssh://host/path, o s3://bucket
  }
}
```

## Campos

| Campo | Req. | Descripción |
|-------|------|-------------|
| `schema` | sí | Versión del formato del manifiesto. |
| `name` | sí | Identificador lógico del paquete. |
| `version` | sí | Semver del paquete. |
| `library` | sí | `*LIB` destino. Convención: 1 módulo = 1 librería. |
| `exports.srvpgm` | módulos | Service program público. |
| `exports.binder` | módulos | Binder language que fija los exports y la signature. |
| `exports.signature` | auto | Firma de la interfaz; la calcula/actualiza el build. |
| `exports.copy` | módulos | `/copy` member de prototipos (header público). |
| `dependencies` | no | Mapa nombre → rango semver. |
| `build.engine` | no | `bob` (default recomendado) o `native`. |
| `migrations.dir` | no | Carpeta de DDL versionado. |
| `migrations.schema` | no | Schema SQL destino (normalmente = `library`). |
| `runtime.libraryList` | no | `*LIBL` sugerida al consumir. |
| `registries` | proyecto | Mapa de registries (uri). |
| `private` | no | Si `true`, no publicable. |

## Lock file (`bindle.lock`)
Generado por `bindle install`. Fija resolución exacta — **no editar a mano**.

```jsonc
{
  "schema": "bindle-lock/v0",
  "resolved": {
    "modfact": {
      "version": "2.3.0",
      "signature": "A1B2C3D4E5F6...",
      "artifact": "ifs:///bindle/registry/modfact/2.3.0/MODFACT.savf",
      "hash": "sha256:...",
      "dependencies": ["modbase", "modimp"]
    },
    "modbase": { "version": "1.4.2", "signature": "...", "hash": "sha256:..." }
  }
}
```

## Reglas de versionado
- **patch**: fix interno, signature intacta.
- **minor**: nuevos exports **al final** del binder → signature compatible hacia atrás.
- **major**: cambio incompatible de API o de signature → consumidores deben recompilar.
- El install **aborta** si la signature resuelta ≠ la del lock (`signature mismatch`).
