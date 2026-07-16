# Matriz de trazabilidad del release

Esta matriz enlaza cada sección de `SPEC.md` con evidencia ejecutable o inspeccionable. Los comandos de validación se ejecutan en Linux y macOS mediante `.github/workflows/ci.yml`.

| Spec | Implementación principal | Evidencia automática |
|---|---|---|
| 1. Objetivo y archivo único | `cmd/tasks`, adaptador SQLite con journal `DELETE` | `TestClosedProjectIsSinglePortableFileWithoutSidecars`, E2E PTY |
| 2. Detección de modo | `internal/adapters/filesystem`, bootstrap del comando | `discovery_test.go`: actual, padres, raíz, conflictos, espacios, Unicode y symlinks |
| 2.3 Ayuda y despacho CLI | parser de invocación y ayuda global en `cmd/tasks` | alias equivalentes, rechazo de comandos/opciones, ausencia de efectos y E2E de códigos de salida |
| 3. `tasks init` | `createProject`, esquema y registro | `TestCreateProjectIsExclusiveAndPortable`, E2E `init` |
| 3.1 Importación asistida | contrato `projectimport`, importación SQLite y despacho CLI | decoder/normalización, rollback integral, publicación/registro y E2E `import` |
| 4. Modo local | fachada de aplicación y vistas TUI | pruebas de modelo TUI y E2E |
| 5. Modo global | agregación por proyecto, capacidades y vistas sin Kanban/Estados | `TestGlobalNavigationNeverEntersKanban`, restricciones globales, resultados parciales |
| 6. Registro global | `internal/adapters/registry` | canonicalización, unicidad y poda en `registry_test.go` |
| 7. Modelo de tarea | dominio, esquema y CRUD SQLite | validación de título/prioridad/fechas, ciclo de vida y reapertura |
| 8. Estados | administración transaccional y vista Estados | `TestStatusAdministrationInvariants`, traslado de tareas/subtareas y tests TUI |
| 9. Subtareas | operaciones SQLite/aplicación y detalle TUI | reglas para cero/una/varias, propagación y E2E |
| 10. Dependencias | grafo recursivo SQLite y bloqueo calculado | ciclos 1/2/N, finalizada/cancelada/reabierta, papelera y Gantt |
| 11. Fechas | `domain.Date`, restricciones SQL y formularios | bisiestos, orden inicio/vencimiento, filtros, Calendario y Gantt |
| 12. Recurrencia | módulo puro, mantenimiento diario y sintaxis TUI | casos 28–31 días, ordinales, cruces, omitidas, idempotencia y fuzzing |
| 13. Markdown | adaptador de editor, sesión Bubble Tea y Glamour | precedencia de editor, temporal, fallo, render y E2E con editor falso |
| 14. Papelera | eliminación/restauración transaccional y mantenimiento | límites de 30 días, dependencias no restauradas, confirmación TUI |
| 15. Vistas | Kanban, Tabla, Calendario y Gantt independientes | tests de cada presentador/vista, Unicode, terminal pequeña y E2E resize |
| 16. Búsqueda/filtros/orden | `TaskFilter`, SQL y controles TUI compartidos | filtros combinados, estado global por nombre y tests de interacción |
| 17. Historial | tabla append-only y eventos en las mismas transacciones | rollback conjunto, tipos de evento e historial TUI |
| 18. Requisitos técnicos | arquitectura por capas, migraciones v1→v2, locks/versiones | futuro/corrupto, dos conexiones, race, vet, build sin CGO, CI Linux/macOS |
| 19. Fuera de alcance | no existen red, etiquetas, adjuntos, horas, cron ni relaciones entre proyectos | revisión de dependencias y pruebas de rechazo de cron/ciclos entre IDs locales |

## Claridad de interacción

El contrato adicional de usabilidad está en [`ui-ux.md`](ui-ux.md). La evidencia automática correspondiente vive en las pruebas de `internal/tui/app` y de cada pantalla: acciones directas de ciclo de vida, selectores sin IDs manuales, selección visible, viewports, ayuda contextual, contexto local/global y desplazamiento de Gantt.

## Puertas de release

```sh
test -z "$(gofmt -l .)"
go vet ./...
go test ./...
go test -race ./...
CGO_ENABLED=0 go build ./cmd/tasks ./cmd/ui-preview
```

La prueba E2E ejecuta el binario en un pseudo-terminal, redimensiona la terminal, usa un editor falso, persiste datos y vuelve a abrir el proyecto. Los benchmarks cubren 1,000 tareas locales, veinte proyectos globales y renderizado de las cuatro vistas principales.
