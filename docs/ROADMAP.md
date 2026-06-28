# Bindle — Roadmap

Tres horizontes: **MVP** (probar el flujo end-to-end), **v1.0** (usable en producción real), **v2.0** (ecosistema y adopción amplia).

## Decisiones abiertas (resolver primero)
- [x] **Lenguaje del CLI**: **Go 1.26+** (binario único, cross-platform, corre en PASE).
- [ ] **Backend del registry (MVP)**: dir IFS / SAVF en host IBM i / bucket S3-compatible.
- [ ] **Motor de build**: envolver Bob (ibmi-bob) vs orquestación propia. Preferencia: envolver Bob.
- [x] **Transporte a IBM i**: **SSH** (golang.org/x/crypto/ssh + SFTP). ODBC/mapepire queda pa SQL de migraciones.
- [ ] **IBM i de pruebas**: host propio / PUB400.com (gratis) / partición de la org.

---

## 🟡 MVP — "el flujo funciona"
Objetivo: instalar un módulo con dependencias, de extremo a extremo, en un IBM i real.

**Fundaciones**
- [x] Estructura del repo, docs de planeación, branding, `.gitignore`.
- [x] Esqueleto del CLI (`bindle --help`, subcomandos stub).
- [x] Sistema de perfiles de conexión host-agnóstico (`internal/config`, `bindle profile`, `docs/CONNECTION.md`).
- [ ] Confirmar decisiones abiertas + entorno de pruebas (host pub400.com:2222).

**Manifiesto + Resolver** (sin tocar IBM i)
- [x] Parser/validador de `bindle.json` (+ JSON Schema).
- [x] Grafo de dependencias + orden topológico.
- [x] Resolución de rangos semver (registry mock con fixtures).
- [x] Generación de `bindle.lock`.
- [x] Tests unitarios del resolver.

**Registry mínimo**
- [x] Layout + `versions.json` (registry backed por directorio: `internal/registry`).
- [x] `bindle publish` a registry local (artefacto + `versions.json` + hash; `--force`).
- [x] `bindle fetch` + verificación por hash (dentro de `bindle install`).
- [x] `bindle list` / `bindle list tree` (resuelve y muestra el grafo real).

**Build (CL directo, sin Bob)**
- [x] `bindle build`: sube fuente (SFTP) → `setccsid 1252` → `CRTRPGMOD` → `CRTSRVPGM` → SAVF → descarga. Validado en vivo en pub400 (SAVF real 211200 B).
- [x] Cálculo/registro de la **signature** del `*SRVPGM` (`DSPSRVPGM DETAIL(*SIGNATURE)`, parseado).
- [x] Empaquetado a SAVF + `CPYTOSTMF` a IFS + download.
- [x] **Binder-source con signature determinista**: Bindle genera el binder (`STRPGMEXP SIGNATURE(X'...')` + `EXPORT SYMBOL`), `CRTSRVPGM EXPORT(*SRCFILE)`. Signature derivada de name+major → estable en minor/patch, cambia en major. Símbolos escaneados del fuente (`dcl-proc … export`) o de `exports.symbols`. Verificado en vivo (pub400): signature controlada `99FB…` aplicada al `*SRVPGM`.
- [x] `bindle put` / `bindle get` (transferencia SFTP manual).
- [ ] Compilar en orden de dependencia (multi-módulo con deps).
- [ ] Captura de joblog en fallos de compilación (QSYS2.JOBLOG_INFO) para diagnóstico.
- [ ] Backend Bob como alternativa al `native` (CL directo).

**Install end-to-end**
- [x] `bindle install` (lado local): resolve → `bindle.lock` → fetch → verificación sha256 → cache.
- [x] Reuso del lock existente (reproducible) + flag `--update`.
- [x] Transport SSH: ejecutar PASE/QSH (`Run`), ejecutar CL (`RunCL` vía `system`), transferir archivos (SFTP `Upload`/`Download`). Validado en vivo contra pub400 (IBM i 7.5). `bindle ping` / `bindle exec`.
- [x] `bindle install --deploy` (lado IBM i): upload → CPYFRMSTMF → RSTOBJ → **validación de signature contra el lock** (aborta en mismatch) → wiring `*LIBL`. Metadata library/srvpgm propagada registry→lock.
- [x] Bindable: un programa `CRTBNDRPG` liga (BNDDIR) contra el `*SRVPGM` construido por Bindle y resuelve su export.
- [x] **Callable end-to-end** (verificado en pub400): `CALL` del programa que liga el `*SRVPGM` de Bindle ejecuta `bgreet('Bindle')` → `BINDLE-RESULT: Hello, Bindle! (from Bindle)`. Caller escribe a stdout vía `printf` (sin data area, sin MSGW). Ver `examples/modules/modgreet/test/`.
- [x] Canal SQL (`internal/sqlchan`): db2util-sobre-SSH (`Exec` + `Query` JSON). `bindle sql` + `bindle migrate`. Migraciones con control table, checksum (inmutabilidad), idempotencia. Verificado en vivo pub400.
- [x] Empaquetar `migrations/` en el paquete del registry: `publish` sube `migrations/` + schema → `versions.json`/lock; `install` las baja al cache; `install --deploy` corre `runDeployMigrations` tras RSTOBJ. Publish/fetch verificado en vivo (pub400); el run en deploy reusa `sqlchan.Migrate` ya probado.
- [ ] Nota: pub400 deniega `RSTOBJ` (host compartido); el deploy es correcto pero se ejerce en hosts con autoridad de restore.

**Definición de "MVP logrado":** módulo demo `modfact` (con 1 dependencia + 2 migraciones) instalable en una app `miapp` vía `bindle add` + `bindle install`, y `calcularFactura(...)` llamable sin cablear nada a mano.

---

## 🟢 v1.0 — "usable en producción"
Objetivo: un equipo IBM i real lo adopta para módulos productivos, conviviendo con su pipeline.

**Robustez de migraciones**
- [ ] Tabla de control de migraciones aplicadas (por schema) + checksum.
- [ ] Orden determinista; idempotencia; upgrade seguro.

**Confiabilidad y DX**
- [x] `bindle init` con plantillas (módulo / proyecto) — adelantado del v1.0.
- [x] `bindle add <module>[@version]` — resuelve la última versión del registry (rango `^`) o usa el constraint dado; escribe el manifiesto.
- [x] Diagnósticos IBM i legibles en fallos CL (`ParseDiagnostics`).
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
