# Bindle — Visión

## Problema
IBM i no tiene un gestor de paquetes abierto y ligero para **objetos ILE nativos** (service programs, módulos, lógica de negocio RPG/COBOL).

- `yum`/`RPM` cubre solo el lado PASE/open-source, no objetos ILE.
- Las suites que cubren ALM completo (ARCAD, IBM Merlin) son corporativas y caras.
- Bob (ibmi-bob) resuelve build/dependencias de compilación, pero no registry, ni deploy, ni migraciones, ni distribución de paquetes.
- Resultado: cada tienda reinventa el plumbing — copiar `*SRVPGM` a mano, editar binding directories, ajustar library lists, correr DDL de memoria.

## Propuesta
**Bindle**: un CLI único que trata la lógica de negocio reutilizable como **módulos instalables** (estilo módulo de Odoo), con:

```
manifiesto (bindle.json)
  → resolución de dependencias (por versión + signature ILE)
  → fetch desde un registry
  → restore de objetos / build desde fuente
  → migraciones de DB versionadas
  → wiring de *BNDDIR (compile) y *LIBL (runtime)
  → lock file reproducible
```

## Modelo de módulo
| Concepto | En Bindle |
|----------|-----------|
| Unidad de paquete | **1 módulo = 1 librería (*LIB)** |
| API pública | **`*SRVPGM`** + `/copy` member de prototipos (el "header") |
| Implementación | `*MODULE` (oculto, no exportado) |
| Estado / esquema | Migraciones DDL versionadas dentro del módulo |
| Metadatos | `bindle.json` |
| Artefacto distribuible | SAVF (u objetos restaurables) en el registry |

Regla de oro: el consumidor **solo ve la API pública** (service program + prototipos). El interior puede cambiar sin romper a nadie mientras la signature se respete.

## Principios de diseño
1. **Declarar, no cablear a mano.** El manifiesto es la fuente de verdad.
2. **Consciente de signatures.** El versionado ILE es más estricto que semver: reordenar exports rompe consumidores. Bindle rastrea signatures, no solo strings de versión.
3. **Instalaciones reproducibles.** Lock file fija versión + signature exactas.
4. **Estado incluido.** Migraciones de DB se aplican en orden al instalar/actualizar.
5. **Wiring automático.** Bindle arma `*BNDDIR` (compile-time) y `*LIBL` (runtime).
6. **Abierto y ligero.** CLI simple, sin plataforma pesada. Se apoya en lo que ya existe (Bob, git, SSH).
7. **Coexistencia primero (adoption-first).** Bindle se mete en los procesos existentes, no los reemplaza. Genera objetos IBM i estándar (`*SRVPGM`, `*BNDDIR`, SAVF, CL inspeccionable) → reversible, sin lock-in. Se adopta **un módulo a la vez**, sin big-bang.

## El reto real: adopción, no tecnología
El obstáculo principal de Bindle no es construir el CLI — es que **las tiendas IBM i son conservadoras** y desconfían de herramientas que tocan su build/deploy crítico. Por eso el diseño se subordina a la adopción gradual:

- **Sin reescritura masiva.** Adoptas Bindle para un módulo nuevo o uno aislado; el resto sigue compilando como siempre.
- **Convive con change management existente.** Lo que Bindle produce son objetos y CL normales que el equipo puede revisar, versionar y correr a mano si quiere.
- **Envuelve, no pelea, el toolchain.** Bob para build, git para fuente, `RSTLIB`/library list para deploy. Nada propietario obligatorio.
- **Reversible.** Todo artefacto es un objeto IBM i estándar. Si abandonas Bindle, no quedas atrapado.
- **Migración gradual medible.** Un equipo puede empezar con `bindle build` (solo build), luego sumar registry, luego install/migraciones — escalando confianza por fases.

Métrica de éxito de adopción: que un equipo pueda meter **su primer módulo Bindle en producción sin cambiar su pipeline existente**.

## Quién lo usa
- **Equipos / organizaciones**: estandarizan lógica común (facturación, impuestos, clientes); cada proyecto nuevo arranca instalando módulos.
- **Desarrolladores / casas de software**: empaquetan y distribuyen sus frameworks como producto.

## No-objetivos (al menos al inicio)
- No reemplaza el IDE (convive con Code4i / RDi).
- No es una plataforma ALM completa (no compite frontalmente con ARCAD/Merlin al inicio).
- No gestiona paquetes PASE (eso es yum/RPM).
- No es un build system desde cero — preferimos envolver Bob donde aporte.

## Diferenciador
El "cargo/npm de IBM i" **abierto, ligero y nativo de ILE** no existe. Bindle ocupa ese hueco: registry + resolución por signature + deploy + migraciones en una sola herramienta.
