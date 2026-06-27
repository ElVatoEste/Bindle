# Bindle — Roadmap

## Decisiones abiertas (resolver primero)
- [x] **Lenguaje del CLI**: **Go 1.26+** (binario único, cross-platform, corre en PASE).
- [ ] **Backend del registry (MVP)**: dir IFS / SAVF en host IBM i / bucket S3-compatible.
- [ ] **Motor de build**: envolver Bob (ibmi-bob) vs orquestación propia.
- [ ] **Transporte a IBM i**: SSH (ssh/scp + CL) vs ODBC (mapepire) vs itoolkit.
- [ ] **Acceso a un IBM i de pruebas**: ¿host propio, PUB400.com (gratis), o partición de la org?

## Fase 0 — Fundaciones (este setup)
- [x] Estructura del repo, docs de planeación, .gitignore, CLAUDE.md.
- [ ] Confirmar decisiones abiertas.
- [ ] Definir entorno de pruebas IBM i.
- [ ] Esqueleto del CLI (`bindle --help`, subcomandos vacíos).

## Fase 1 — Manifiesto + Resolver (sin tocar IBM i todavía)
- [ ] Parser/validador de `bindle.json` (+ JSON Schema).
- [ ] Construcción del grafo de dependencias + orden topológico.
- [ ] Resolución de versiones (rangos semver) en memoria/registry mock.
- [ ] Generación de `bindle.lock`.
- [ ] Tests unitarios del resolver con un registry simulado (fixtures).

## Fase 2 — Registry mínimo
- [ ] Formato de índice (`index.json`) y layout de artefactos.
- [ ] `bindle publish` a un registry local (dir/IFS).
- [ ] `bindle fetch` + verificación por hash.
- [ ] `bindle list` / `bindle tree`.

## Fase 3 — Build (envolver Bob)
- [ ] `bindle build`: compila en orden de dependencia.
- [ ] Generación de `*BNDDIR` desde el manifiesto.
- [ ] Cálculo/registro de la **signature** del `*SRVPGM`.
- [ ] Empaquetado a SAVF.

## Fase 4 — Install end-to-end en IBM i
- [ ] Transport elegido funcionando (ejecutar CL + transferir SAVF + correr SQL).
- [ ] `bindle install`: resolve → fetch → RSTLIB/RSTOBJ → migraciones → wiring *LIBL/*BNDDIR.
- [ ] Validación de signature contra el lock (abortar en mismatch).
- [ ] Idempotencia + upgrade.

## Fase 5 — Migraciones robustas
- [ ] Tracking de migraciones aplicadas (tabla de control por schema).
- [ ] Up only (MVP) → rollback (después).
- [ ] Orden determinista + checksum por migración.

## Fase 6 — DX y publicación
- [ ] `bindle init` con plantillas (módulo / proyecto).
- [ ] Mensajes de error claros (signature mismatch, conflicto de deps, lib faltante).
- [ ] Docs de uso + ejemplo real (módulo demo `modfact` + app consumidora).
- [ ] CI (lint, tests). Versionado del propio Bindle.

## Más allá (backlog)
- Firma criptográfica de paquetes en el registry.
- Registry público/compartido (si hay tracción de comunidad).
- Integración con Code4i (VS Code).
- Análisis de impacto (qué consumidores rompe un cambio de signature).
- Soporte COBOL/CL además de RPG.
- Rollback de migraciones.

## Caso de prueba guía (definición de "funciona")
Módulo demo **`modfact`** (facturación) con:
- `*SRVPGM` público + `/copy` de prototipos,
- 1 dependencia (`modbase`),
- 2 migraciones DDL.

App **`miapp`** que hace `bindle add modfact` → `bindle install` y puede llamar `calcularFactura(...)` sin cablear nada a mano. Ese flujo end-to-end = MVP logrado.
