# Bindle — Roadmap

Tres horizontes: **MVP** (probar el flujo end-to-end), **v1.0** (usable en producción real), **v2.0** (ecosistema y adopción amplia).

## Decisiones abiertas (resolver primero)
- [x] **Lenguaje del CLI**: **Go 1.26+** (binario único, cross-platform, corre en PASE).
- [ ] **Backend del registry (MVP)**: dir IFS / SAVF en host IBM i / bucket S3-compatible.
- [ ] **Motor de build**: envolver Bob (ibmi-bob) vs orquestación propia. Preferencia: envolver Bob.
- [ ] **Transporte a IBM i**: SSH (ssh/scp + CL) vs ODBC (mapepire) vs itoolkit.
- [ ] **IBM i de pruebas**: host propio / PUB400.com (gratis) / partición de la org.

---

## 🟡 MVP — "el flujo funciona"
Objetivo: instalar un módulo con dependencias, de extremo a extremo, en un IBM i real.

**Fundaciones**
- [x] Estructura del repo, docs de planeación, branding, `.gitignore`.
- [x] Esqueleto del CLI (`bindle --help`, subcomandos stub).
- [ ] Confirmar decisiones abiertas + entorno de pruebas.

**Manifiesto + Resolver** (sin tocar IBM i)
- [x] Parser/validador de `bindle.json` (+ JSON Schema).
- [x] Grafo de dependencias + orden topológico.
- [x] Resolución de rangos semver (registry mock con fixtures).
- [x] Generación de `bindle.lock`.
- [x] Tests unitarios del resolver.

**Registry mínimo**
- [x] Layout + `versions.json` (registry backed por directorio: `internal/registry`).
- [ ] `bindle publish` a registry local (dir/IFS).
- [x] `bindle fetch` + verificación por hash (dentro de `bindle install`).
- [x] `bindle list` / `bindle list tree` (resuelve y muestra el grafo real).

**Build (envolver Bob)**
- [ ] `bindle build`: compila en orden de dependencia.
- [ ] Generación de `*BNDDIR` desde el manifiesto.
- [ ] Cálculo/registro de la **signature** del `*SRVPGM`.
- [ ] Empaquetado a SAVF.

**Install end-to-end**
- [x] `bindle install` (lado local): resolve → `bindle.lock` → fetch → verificación sha256 → cache.
- [x] Reuso del lock existente (reproducible) + flag `--update`.
- [ ] Transport elegido (ejecutar CL + transferir SAVF + correr SQL).
- [ ] `bindle install` (lado IBM i): RSTLIB → migraciones → wiring `*LIBL`/`*BNDDIR`.
- [ ] Validación de signature contra el lock (abortar en mismatch).

**Definición de "MVP logrado":** módulo demo `modfact` (con 1 dependencia + 2 migraciones) instalable en una app `miapp` vía `bindle add` + `bindle install`, y `calcularFactura(...)` llamable sin cablear nada a mano.

---

## 🟢 v1.0 — "usable en producción"
Objetivo: un equipo IBM i real lo adopta para módulos productivos, conviviendo con su pipeline.

**Robustez de migraciones**
- [ ] Tabla de control de migraciones aplicadas (por schema) + checksum.
- [ ] Orden determinista; idempotencia; upgrade seguro.

**Confiabilidad y DX**
- [ ] `bindle init` con plantillas (módulo / proyecto).
- [ ] Mensajes de error claros (signature mismatch, conflicto de deps, lib faltante).
- [ ] Idempotencia total de install + `bindle update`.
- [ ] Build-from-source como fallback al SAVF (portabilidad entre niveles de OS).

**Coexistencia (adoption-first)**
- [ ] `bindle plan` / dry-run: muestra qué objetos/CL/SQL ejecutaría, sin aplicar.
- [ ] Export de CL inspeccionable para integrarse a change management existente.
- [ ] Adopción incremental: un módulo bajo Bindle sin tocar el resto del build.

**Calidad del proyecto**
- [ ] CI (lint, tests), releases versionados del propio Bindle.
- [ ] Docs de uso + ejemplo real completo.
- [ ] Registry privado estable (backend MVP endurecido).

---

## 🔵 v2.0 — "ecosistema y adopción amplia"
Objetivo: pasar de herramienta a ecosistema.

- [ ] **Firma criptográfica** de paquetes (autenticidad del publicador).
- [ ] **Registry compartido/federado** (si hay tracción de comunidad).
- [ ] **Análisis de impacto**: qué consumidores rompe un cambio de signature.
- [ ] Integración con **Code4i (VS Code)** y/o RDi.
- [ ] Soporte **COBOL/CL** además de RPG.
- [ ] **Rollback** de migraciones.
- [ ] Backends de registry adicionales + mirroring.

---

## Riesgo guía
El mayor riesgo es **adopción**, no tecnología (ver [`VISION.md`](VISION.md)). Toda fase se valida contra una pregunta: *¿puede un equipo conservador meter esto en su pipeline sin romper lo que ya tiene?* Si la respuesta es no, se rediseña antes de avanzar.
