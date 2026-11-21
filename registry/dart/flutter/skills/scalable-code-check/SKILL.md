---
name: scalable-code-check
description: >-
  Run empirical code cleanliness, cohesion, coupling, and architectural boundary
  checks after completing a coding or refactoring task in a Flutter project, and
  orchestrate fixing any reported findings up to 3 iterations. Use automatically
  whenever finishing an implementation task.
---

# Scalable Code Check & Auto-Fix Loop

Usa esta skill **cada vez que termines de implementar o refactorizar código** en este proyecto Flutter.

## Paso 1: Verificar configuración previa

1. Comprueba si existe `.scalable-code/config.json`.
   - Si **no existe**, detente e informa al usuario que primero debe ejecutar `/scalable-code-setup` para analizar la arquitectura y generar la configuración.

## Paso 2: Ejecutar validación empírica

1. Ejecuta el script de validación en el directorio raíz del proyecto:
   - Modo normal post-tarea (incremental sobre archivos modificados + ciclos/fronteras donde participen):
     ```bash
     dart run tool/scalable_code_check.dart
     ```
   - Si el usuario pidió explícitamente revisar todo el repositorio:
     ```bash
     dart run tool/scalable_code_check.dart --all
     ```
2. Si el comando termina con **exit code `0`**:
   - Lee brevemente el resumen de `.scalable-code/report.md` (métricas de `lakos`, analyzer y DCL) y reporta al usuario que la validación empírica pasó sin findings bloqueantes.

## Paso 3: Ciclo de corrección de findings (máximo 3 iteraciones)

Si el comando termina con **exit code distinto de `0`**:

1. Lee `.scalable-code/report.md` (y `.scalable-code/report.json` si necesitas detalle exacto de líneas o aristas) junto con `.scalable-code/config.json`.
2. **Delegación a subagente corrector** (si tu entorno soporta subagentes, como Antigravity):
   - Lanza un subagente dedicado con el rol `Architecture & Code Quality Fixer` pasándole:
     - La ruta `.scalable-code/report.md` y `.scalable-code/config.json`.
     - La lista concreta de findings reportados.
     - Estas restricciones obligatorias:
       1. Corregir la causa raíz (extraer métodos pequeños para bajar complejidad/anidamiento, mover dependencias o introducir interfaces en `domain` para respetar fronteras, romper ciclos mediante inversión de dependencias o extracción a `core`/`domain`).
       2. **Prohibido** silenciar reglas con `// ignore:` o bajar umbrales en `analysis_options.yaml` / `.scalable-code/config.json` para ocultar problemas.
       3. Mantener o mejorar la cobertura de tests existente.
   - Si el entorno **no** soporta subagentes, aplica tú mismo las correcciones siguiendo exactamente las mismas reglas.
3. Tras aplicar las correcciones, vuelve a ejecutar:
   ```bash
   dart run tool/scalable_code_check.dart
   flutter test
   ```
4. Repite el ciclo hasta que `dart run tool/scalable_code_check.dart` devuelva exit code `0` y los tests pasen, con un límite máximo de **3 iteraciones**.
5. Si tras 3 iteraciones persisten findings, detente y reporta claramente al usuario qué findings quedaron sin resolver y qué decisión arquitectónica requieren.
