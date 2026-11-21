---
name: scalable-code-setup
description: >-
  Analyze a Flutter project's dependencies, folder structure, and import graph
  to identify its architecture, confirm layers/boundaries and quality thresholds
  with the user, and configure analysis_options.yaml and .scalable-code/config.json.
  Invoke with /scalable-code-setup after installing scalable_code_ai.
---

# Scalable Code Setup (Flutter / Dart)

Usa esta skill para inspeccionar el proyecto Flutter actual, acordar con el usuario su arquitectura y nivel de exigencia empírica, y generar la configuración de `custom_lint`, `dart_code_linter` y `lakos`.

## Paso 1: Inspección del proyecto

1. Lee `pubspec.yaml`:
   - Identifica paquetes de manejo de estado o DI (ej. `flutter_riverpod`, `riverpod`, `flutter_bloc`, `bloc`, `get_it`, `injectable`) para sugerir plugins compatibles de `custom_lint` si el usuario desea activarlos (ej. `riverpod_lint`).
   - Identifica paquetes de generación de código (`freezed`, `json_serializable`, `build_runner`, `auto_route`, `retrofit`) para excluir `**/*.g.dart`, `**/*.freezed.dart`, `**/*.gr.dart` en análisis y métricas.
2. Lee `analysis_options.yaml` (si existe) para preservar reglas previas del equipo.
3. Explora el árbol de directorios e imports dentro de `lib/` (puedes ejecutar `dart run lakos -f json -m lib/` en solo lectura si ya está instalado para ver el grafo actual de módulos y ciclos existentes).

## Paso 2: Confirmar arquitectura y nivel de exigencia con el usuario

Presenta un resumen conciso de la estructura detectada en `lib/` y pregunta al usuario (usando preguntas claras y opciones concretas) para confirmar o corregir:

1. **Arquitectura del proyecto** (elige una o personaliza):
   - **Clean Architecture (por capas o por feature)**:
     - Capas típicas: `domain` (núcleo puro), `application`/`usecases`, `data`/`infrastructure`, `presentation`/`ui`.
     - Fronteras: `domain` no importa `data` ni `presentation`; `application` no importa `presentation`; `presentation` no importa `data` directamente.
   - **Feature-First (Modular por dominio)**:
     - Carpetas `lib/features/<feature>/...` + `lib/core/` o `lib/shared/`.
     - Fronteras: aislamiento entre features (`featureIsolation: true`, salvo contratos públicos/barrel explícitos si el usuario lo indica) y `core`/`shared` no importa `features`.
   - **Layered MVC / MVVM**:
     - Capas: `models`/`services`/`repositories` -> `controllers`/`viewmodels` -> `views`/`widgets`.
     - Fronteras: `models`/`services` no importan `views`/`controllers`; `views` no importan `repositories` directamente si hay `viewmodels`.
   - **Custom**:
     - El usuario define sus nombres de carpetas/capas y reglas de imports permitidos/prohibidos.

2. **Nivel de exigencia de métricas empíricas**:
   - **Equilibrado (Recomendado)**:
     - Complejidad ciclomática (`cyclomatic-complexity`): `15`
     - Anidamiento máximo (`maximum-nesting-level`): `5`
     - Líneas ejecutables por función (`source-lines-of-code`): `60`
     - Parámetros por función (`number-of-parameters`): `5`
     - Fan-out máximo por archivo en `lakos` (`maxOutDegree`): `12`
     - ACD máximo global (`maxAcd`): `10.0` (ajustable en repos grandes)
     - Dependencias circulares permitidas: `0`
   - **Estricto**:
     - Complejidad ciclomática (`cyclomatic-complexity`): `10`
     - Anidamiento máximo (`maximum-nesting-level`): `4`
     - Líneas ejecutables por función (`source-lines-of-code`): `45`
     - Parámetros por función (`number-of-parameters`): `4`
     - Fan-out máximo por archivo en `lakos` (`maxOutDegree`): `8`
     - ACD máximo global (`maxAcd`): `6.0`
     - Dependencias circulares permitidas: `0`

## Paso 3: Generar o actualizar `analysis_options.yaml`

Actualiza `analysis_options.yaml` preservando los `include:` y `linter.rules` existentes del proyecto:

1. Revisa si `custom_lint` quedó instalado en `pubspec.yaml`:
   - Si `custom_lint` **sí** está en `pubspec.yaml`: actívalo dentro de `analyzer.plugins` (`- custom_lint`).
   - Si `custom_lint` **no** está en `pubspec.yaml` (proyectos con `analyzer >= 9`, ej. `riverpod_generator >= 4`): **no** agregues `custom_lint` en `analyzer.plugins`. Si el proyecto usa Riverpod 3+, ofrece agregar `riverpod_lint` en `dev_dependencies` y registrarlo en el bloque nativo top-level `plugins:` de `analysis_options.yaml`:
     ```yaml
     plugins:
       riverpod_lint: ^3.2.0
     ```
2. **No** agregues `dart_code_linter` en `analyzer.plugins`, ya que se ejecutará principalmente por CLI (`dart run dart_code_linter:metrics analyze lib`).
3. Agrega exclusiones para archivos generados (`**/*.g.dart`, `**/*.freezed.dart`, `**/*.gr.dart`, `tool/**`).
4. Configura la sección `dart_code_linter:` con las métricas del nivel elegido y reglas enfocadas en cohesión, limpieza y mantenibilidad:

```yaml
analyzer:
  # Incluir "plugins: [custom_lint]" solo si custom_lint esta en pubspec.yaml
  exclude:
    - "**/*.g.dart"
    - "**/*.freezed.dart"
    - "**/*.gr.dart"

dart_code_linter:
  metrics:
    cyclomatic-complexity: 15
    maximum-nesting-level: 5
    number-of-parameters: 5
    source-lines-of-code: 60
  metrics-exclude:
    - test/**
    - "**/*.g.dart"
    - "**/*.freezed.dart"
    - "**/*.gr.dart"
  rules:
    - avoid-dynamic
    - avoid-global-state
    - avoid-late-keyword
    - avoid-nested-conditional-expressions
    - avoid-returning-widgets
    - avoid-unnecessary-setstate
    - avoid-unused-parameters
    - no-boolean-literal-compare
    - no-empty-block
    - no-magic-number:
        allowed: [-1, 0, 1, 2, 100]
    - prefer-conditional-expressions
    - prefer-extracting-callbacks
    - prefer-first
    - prefer-last
    - prefer-single-widget-per-file
```

*(Ajusta los valores numéricos de `metrics:` según el nivel Equilibrado o Estricto confirmado por el usuario).*

## Paso 4: Generar `.scalable-code/config.json`

Crea `.scalable-code/config.json` reflejando la arquitectura confirmada por el usuario. Ejemplo de esquema:

```json
{
  "version": "1.0.0",
  "architecture": "clean-architecture",
  "strictness": "balanced",
  "targetDir": "lib",
  "excludeGlobs": [
    "**/*.g.dart",
    "**/*.freezed.dart",
    "**/*.gr.dart"
  ],
  "lakos": {
    "allowCycles": false,
    "maxOutDegree": 12,
    "maxAcd": 10.0
  },
  "boundaries": {
    "featureIsolation": {
      "enabled": true,
      "featuresPath": "features",
      "allowedShared": ["core", "shared", "common"]
    },
    "forbiddenDependencies": [
      {
        "from": "domain",
        "to": "data",
        "reason": "La capa domain no debe depender de data."
      },
      {
        "from": "domain",
        "to": "presentation",
        "reason": "La capa domain no debe depender de presentation/UI."
      },
      {
        "from": "application",
        "to": "presentation",
        "reason": "La capa application no debe depender de presentation/UI."
      },
      {
        "from": "presentation",
        "to": "data",
        "reason": "Presentation debe comunicarse a traves de domain/application, no directamente con data."
      }
    ]
  }
}
```

Adapta `featuresPath`, `allowedShared` y `forbiddenDependencies` exactamente a los nombres de carpetas reales confirmados en el Paso 2. Si `features/` no existe en la arquitectura elegida, pon `"featureIsolation": { "enabled": false }`.

## Paso 5: Verificación inicial

1. Ejecuta `dart run tool/scalable_code_check.dart --all`.
2. Si el proyecto es existente y ya tenía deuda previa, explícale al usuario:
   - El estado inicial detectado en `.scalable-code/report.md`.
   - Que al terminar tareas futuras, `scalable-code-check` correrá por defecto en modo incremental (`--changed`), evitando bloquearlo por código antiguo que no modificó.
   - Cómo puede ajustar `.scalable-code/config.json` o `analysis_options.yaml` si necesita refinar algún umbral.
