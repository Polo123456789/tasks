# tasks

Gestor de tareas completamente local para terminal. Cada proyecto es una base SQLite autocontenida y portable con extensión `.tasks`.

## Instalación

Requiere Go 1.26 o posterior. El binario no necesita CGO.

```sh
go install github.com/Polo123456789/tasks/cmd/tasks@latest
```

Linux y macOS son las plataformas soportadas inicialmente.

## Inicio rápido

```sh
mkdir mi-proyecto
cd mi-proyecto
tasks init mi-proyecto.tasks
```

`tasks` busca el archivo `.tasks` desde el directorio actual hacia sus padres. Fuera de un proyecto abre el modo global con todos los proyectos registrados. El índice global solo guarda rutas; cada mutación se escribe en el archivo de origen.

## Ayuda de línea de comandos

La ayuda global está disponible mediante cualquiera de estas formas:

```sh
tasks help
tasks -h
tasks --help
```

Ejecutar `tasks` sin argumentos continúa abriendo la TUI. Los comandos y opciones desconocidos terminan con error y sugieren ejecutar `tasks help`; nunca se interpretan como una solicitud para abrir la interfaz. La ayuda es global y no admite un nombre de subcomando.

## Importar un proyecto desde una conversación con IA

`tasks` no se conecta a ningún servicio externo. En su lugar, genera un prompt autocontenido para usarlo con el agente con el que se discutió el proyecto:

```sh
tasks ai-prompt
```

El agente debe responder únicamente con JSON puro. Guarde esa respuesta en un archivo o pásela por la entrada estándar:

```sh
tasks import mi-proyecto.tasks resultado.json
tasks import mi-proyecto.tasks - < resultado.json
cat resultado.json | tasks import mi-proyecto.tasks
```

La importación crea un proyecto nuevo, lo registra, imprime un resumen y termina sin abrir la TUI. Nunca sobrescribe un archivo ni mezcla contenido con un proyecto existente; si cualquier validación falla, no deja un `.tasks` parcial.

El formato actual es `tasks-project` versión 1:

```json
{
  "format": "tasks-project",
  "version": 1,
  "statuses": [
    {"key": "pending", "name": "Pendiente", "initial": true},
    {"key": "in_progress", "name": "En progreso"}
  ],
  "tasks": [
    {
      "key": "scope",
      "title": "Definir alcance",
      "status": "done",
      "priority": "high",
      "markdown": "Decisiones relevantes del proyecto."
    },
    {
      "key": "implementation",
      "title": "Implementar primera versión",
      "status": "pending",
      "start": "2026-07-20",
      "due": "2026-07-25",
      "subtasks": [{"title": "Implementar parser"}],
      "depends_on": ["scope"]
    }
  ]
}
```

- `statuses` declara estados normales en el orden del Kanban y exactamente uno debe ser inicial. `done` y `cancelled` son claves reservadas para los estados especiales.
- Las claves de tareas y estados solo existen en el intercambio; SQLite asigna sus IDs internos.
- La prioridad admite `none`, `low`, `medium`, `high` y `urgent`.
- `start` y `due` usan `YYYY-MM-DD`. `recurrence` usa la sintaxis compacta documentada abajo y no puede combinarse con fechas.
- Las subtareas admiten solo título y estado. `depends_on` referencia claves de otras tareas y no admite ciclos.
- Los campos omitidos usan el estado inicial, prioridad `none`, Markdown vacío o listas vacías según corresponda. Se rechazan campos desconocidos y texto fuera del objeto JSON.

## Navegación

- La cabecera indica siempre si se está en modo local o global y, en local, el proyecto abierto.
- El pie muestra siempre todas las teclas válidas en el contexto actual y cambia con la vista, selección, formulario, selector o confirmación. `F1` abre además un mapa general opcional; `↑`/`↓` desplazan esa ayuda.
- `←`/`→` o `h`/`l`: cambiar vista; `↑`/`↓` o `j`/`k`: seleccionar un elemento visible.
- `PgUp` / `PgDn`: periodo anterior/siguiente en Calendario y Gantt.
- `,` / `.`: desplazar la ventana de días del Gantt cuando el mes no cabe completo.
- `n`, `e`, `p`, `s`, `v`: crear, editar título, prioridad, inicio y vencimiento.
- `[` / `]`: mover entre estados; `m`: editar Markdown externamente.
- `f`, `C`, `z`: finalizar, cancelar o reabrir directamente en el estado inicial.
- `a`, `E`, `t`, `J`, `K`: crear, renombrar, alternar y seleccionar subtareas.
- `{` / `}`: mover la subtarea seleccionada entre estados.
- `g` / `G`: crear/eliminar una dependencia mediante un selector de tareas; `c`: configurar recurrencia mediante un formulario guiado.
- `d`: papelera; `u`: restaurar; `H`: historial.
- `/`, `?`, `P`, `S`, `D`: buscar título/Markdown y filtrar proyecto/estado/fechas.
- `1`, `B`, `R`, `F`, `X`, `o`, `0`: prioridad, bloqueo, recurrencia, visibilidad de finalizadas/canceladas, orden y limpiar filtros.
- En la vista Estados: `a`, `e`, `i`, `[`/`]`, `d` administran estados normales.
- `r`: refrescar; `q` o `Ctrl+C`: salir.

Las operaciones que relacionan elementos no requieren memorizar IDs: dependencias, filtro local por estado y destino al eliminar un estado presentan selectores con ID, título y estado. Los IDs permanecen visibles en Tabla, detalle y Estados para diagnóstico.

En modo global, una acción de creación muestra una explicación en lugar de fallar silenciosamente. La selección solo recorre elementos que realmente aparecen en Calendario o Gantt, y todas las listas muestran la fila activa y marcadores `↑`/`↓` cuando existe contenido fuera del viewport.

La terminal mínima soportada es de 90 columnas por 40 filas (`90x40`). El cuerpo reserva dinámicamente el espacio ocupado por el pie contextual multilínea.

El editor Markdown se resuelve primero mediante `$VISUAL` y después `$EDITOR`.

### Recurrencia

La TUI primero permite elegir el tipo de recurrencia. Solo solicita datos adicionales cuando son necesarios, por ejemplo los días semanales o el día del mes. Internamente acepta las formas compactas:

```text
daily
weekly:mon,thu
monthly:15
month-end
monthly-weekday:first:mon
monthly-weekday:last:fri
```

Una entrada vacía elimina la recurrencia. No se aceptan expresiones cron.

El diseño de interacción, comportamiento adaptable y decisiones de claridad se documentan en [`docs/ui-ux.md`](docs/ui-ux.md).

## Preview de interfaz

`ui-preview` no abre bases reales y usa fechas/fixtures deterministas:

```sh
go run ./cmd/ui-preview --screen kanban --fixture crowded
go run ./cmd/ui-preview --screen calendar --fixture crowded --mode global
go run ./cmd/ui-preview --screen gantt --fixture dependencies
go run ./cmd/ui-preview --screen settings --fixture default
```

Pantallas: `kanban`, `table`, `calendar`, `gantt`, `trash`, `settings`.

Fixtures: `default`, `empty`, `crowded`, `dependencies`, `loading`, `error`, `conflict`.

## Datos y diagnóstico

- El proyecto vive únicamente en su archivo `.tasks`.
- El registro y `tasks.log` se guardan bajo el directorio de configuración del usuario devuelto por el sistema operativo, dentro de `tasks/`.
- SQLite usa `foreign_keys=ON`, journal `DELETE`, sincronización `FULL`, timeout limitado y control optimista de versiones.
- La papelera conserva tareas durante 30 días.

## Desarrollo

```sh
go test ./...
go test -race ./...
go vet ./...
CGO_ENABLED=0 go build ./cmd/tasks ./cmd/ui-preview
go test ./cmd/tasks -run TestE2EInitCreateCloseAndReopen -v
go test ./internal/adapters/sqlite ./internal/application -bench . -run '^$'
```

La [matriz de trazabilidad](docs/traceability.md) enlaza el spec con implementación y pruebas. Las decisiones de arquitectura están en [docs/adr](docs/adr).
