# Bindle — Registry

> Diseño previsto. El backend concreto del MVP es una **decisión abierta** (ver [`ROADMAP.md`](ROADMAP.md)).

El **registry** es el índice de artefactos versionados (SAVF + metadatos) que `bindle install` lee y `bindle publish` alimenta. No hay un registry público central de IBM i hoy — Bindle define el suyo, empezando por **privado**.

## Ciclo de vida

```text
PUBLISH                         STORE                          CONSUME
─────────                       ──────                         ────────
bindle build                    registry/<name>/<version>/     bindle add <name>@<range>
  → package SAVF                  ├─ <LIB>.savf                   → resolve (versions + sig)
  → compute signature            ├─ bindle.json                  → fetch artifact + index
  → push artifact + index  ──►   ├─ copy/<member>           ──►  → verify hash
                                 ├─ migrations/                  → install (restore + migrate + wire)
                                 └─ index.json
```

## Layout del registry

```text
registry/
├── registry.json                  # índice raíz: lista de paquetes
└── modfact/
    ├── versions.json              # versiones disponibles de modfact
    ├── 2.3.0/
    │   ├── MODFACT.savf
    │   ├── bindle.json
    │   ├── copy/FACTPR.rpgleinc
    │   ├── migrations/*.sql
    │   └── index.json             # version, signature, hash, deps
    └── 2.2.1/ ...
```

`versions.json` permite al resolver listar versiones sin descargar artefactos:

```jsonc
{
  "name": "modfact",
  "versions": [
    { "version": "2.3.0", "signature": "A1B2...", "hash": "sha256:...", "yanked": false },
    { "version": "2.2.1", "signature": "9F8E...", "hash": "sha256:...", "yanked": false }
  ]
}
```

## Backends (pluggable)

El cliente de registry abstrae el transporte. Candidatos MVP:

| Backend | URI | Pro | Contra |
|---------|-----|-----|--------|
| **Directorio IFS** | `ifs:///bindle/registry` | Simple, vive en el propio IBM i | Compartir entre sitios requiere montaje/replicación |
| **SAVF en host** | `ssh://host/QGPL` | Nativo, fácil con SSH | Menos estructurado para metadatos |
| **Bucket S3-compatible** | `s3://bucket/bindle` | Compartible, versionable, off-host | Necesita credenciales + red |

`bindle.json` del proyecto declara el registry:

```jsonc
{ "registries": { "default": "ifs:///bindle/registry" } }
```

## Resolución y selección de versión

1. `bindle add modfact@^2.3.0` agrega el rango al manifiesto.
2. El resolver lee `versions.json`, elige la versión más alta que satisface el rango.
3. Valida la **signature** contra lo esperado por los consumidores de ese `*SRVPGM`.
4. Fija versión + signature + hash en `bindle.lock`.

## Integridad y seguridad

- Cada artefacto lleva **hash** (`sha256`) en su `index.json`; `fetch` lo verifica.
- Credenciales de backend fuera del repo (`.env` / gestor de secretos).
- **Yank** (no borrar): marcar una versión como retirada sin romper locks existentes.
- (v2.0) **Firma criptográfica** de paquetes para autenticidad del publicador.

## No-objetivos (por ahora)

- No es un registry público federado (eso depende de tracción de comunidad → v2.0).
- No hace mirroring/proxy de otros registries.
- No gestiona paquetes PASE (eso es yum/RPM).
