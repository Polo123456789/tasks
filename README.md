# tasks

Gestor de tareas completamente local para terminal. Cada proyecto es una base SQLite autocontenida y portable con extensión `.tasks`; fuera de proyectos también puedes mantener tareas propias del modo global.

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

`tasks` busca el archivo `.tasks` desde el directorio actual hacia sus padres. Fuera de un proyecto abre el modo global con sus tareas propias y todos los proyectos registrados. El índice global solo guarda rutas; cada mutación se escribe en el almacén de origen.

En global, `n` crea una tarea sin proyecto. Estas tareas admiten subtareas, recurrencias y dependencias entre ellas. Al seleccionar una tarea de un proyecto registrado se puede editar lo existente, pero no añadir tareas, subtareas, dependencias ni recurrencias nuevas a ese proyecto.

## Ayuda de línea de comandos

La ayuda global está disponible mediante cualquiera de estas formas:

```sh
tasks help
tasks -h
tasks --help
```

Ejecutar `tasks` sin argumentos continúa abriendo la TUI. Los comandos y opciones desconocidos terminan con error y sugieren ejecutar `tasks help`; nunca se interpretan como una solicitud para abrir la interfaz. La ayuda es global y no admite un nombre de subcomando.

`tasks is-project` permite comprobar el contexto desde scripts de shell. No imprime nada: termina con código `0` si el directorio actual está dentro del árbol de un proyecto y con código `1` si no lo está.

```sh
if tasks is-project; then
  echo "Proyecto activo"
fi
```

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

La importación crea un proyecto nuevo, lo registra, imprime un resumen y termina sin abrir la TUI. Nunca sobrescribe un archivo ni mezcla contenido con un proyecto existente; si cualquier validación falla, no deja un `.tasks` parcial. Si el archivo completo ya fue publicado y solo falla el registro global, se conserva y el error indica su ruta para no perder una importación recibida por `stdin`.

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

## Agregar tareas desde scripts o agentes

`tasks add` agrega un lote usando el mismo formato `tasks-project` versión 1. Sin opciones, el destino siempre es el almacén global, incluso si el comando se ejecuta dentro de un proyecto:

La referencia del formato y un ejemplo completo están disponibles con `tasks add --help`.

```sh
tasks add resultado.json
cat resultado.json | tasks add -
```

Para agregar el lote a un proyecto existente, indique su archivo mediante una ruta absoluta o relativa:

```sh
tasks add --project mi-proyecto.tasks resultado.json
cat resultado.json | tasks add --project=../otro/plan.tasks
```

En `add`, la sección `statuses` no modifica la configuración del destino: sus nombres deben coincidir exactamente con estados normales existentes y el marcado con `initial` debe ser el estado inicial real. El destino puede tener otros estados. Las claves `done` y `cancelled` siguen identificando los estados especiales.

El lote completo se guarda de forma atómica. Las claves de tareas solo relacionan elementos del mismo JSON; no referencian tareas preexistentes ni evitan duplicados entre ejecuciones. Una entrada correcta imprime JSON con el destino, los conteos y el ID asignado a cada clave:

```json
{
  "destination": {"kind": "project", "path": "/ruta/mi-proyecto.tasks"},
  "created": {"tasks": 2, "subtasks": 1, "dependencies": 1},
  "tasks": [{"key": "scope", "id": 17}, {"key": "implementation", "id": 18}]
}
```

## Captura rápida desde la CLI

`tasks new` crea una sola tarea sin abrir la TUI ni preparar JSON:

```sh
tasks new "Revisar el despliegue"
tasks new --priority high --start 2026-07-20 --due 2026-07-22 "Preparar entrega"
```

Dentro del árbol de un proyecto, el destino predeterminado es el `.tasks` detectado; fuera de uno, es el almacén global. Se puede elegir de forma explícita con `--global` o `--project ruta.tasks`. La ayuda `tasks new --help` describe el destino antes de crear. La salida es JSON e incluye el origen y el ID local asignado:

```json
{"destination":{"kind":"project","path":"/ruta/proyecto.tasks"},"task":{"id":17}}
```

Prioridades válidas: `none`, `low`, `medium`, `high` y `urgent`. Las fechas usan `AAAA-MM-DD` y el vencimiento no puede preceder al inicio. `tasks add` conserva su semántica separada para lotes JSON.

Si un título comienza por guion, separe las opciones con `--`, por ejemplo `tasks new -- "-Seguimiento"`. En la salida, `destination.path` aparece únicamente para destinos de proyecto; el origen global se identifica con `{"kind":"global"}`.

## Exportación, respaldo y diagnóstico

Los comandos de datos usan el proyecto detectado por defecto. Fuera de un proyecto, el almacén global debe seleccionarse explícitamente con `--global`; también se puede indicar cualquier proyecto con `--project ruta.tasks`.

```sh
tasks export --format json > proyecto.json
tasks export --format markdown
tasks export --format csv
tasks backup respaldo.tasks.bak
tasks doctor
tasks doctor --global --json
```

`export` abre el almacén en modo de solo lectura. JSON usa el formato portable `tasks-project` versión 1; Markdown y CSV ofrecen representaciones legibles. `backup` crea una instantánea SQLite consistente, privada y publicada atómicamente; el destino no puede existir. Se recomienda `.tasks.bak` para que el descubrimiento no confunda un respaldo con un proyecto activo. Para garantizar que estas operaciones y `doctor` no creen ni modifiquen sidecars, rechazan bases en modo WAL o con archivos `-wal`, `-shm` o `-journal`: cierre los procesos que las usan, complete el checkpoint y vuelva al modo `DELETE` antes de reintentar.

`restore` valida integridad, claves foráneas, tablas y versión antes de publicar. Puede crear un proyecto nuevo o restaurar el almacén global explícito. Nunca reemplaza un destino existente salvo con `--force`:

```sh
tasks restore respaldo.tasks.bak --project recuperado.tasks
tasks restore respaldo.tasks.bak --global --force
```

Los proyectos restaurados se registran. Una versión antigua compatible se migra en staging; una versión futura incompatible o una base corrupta se rechaza sin tocar el destino.

`doctor` no crea, migra, repara ni poda. Revisa versión del esquema, tablas, `integrity_check`, claves foráneas y permisos. Con `--global` también inspecciona el registro y cada proyecto registrado, distinguiendo problemas `repairable`, `incompatible` y `corruption`. La salida humana es predeterminada y `--json` ofrece un contrato estructurado para automatización; cualquier advertencia o error produce estado no saludable.

### Resumen para el inicio de la terminal

`tasks summary` imprime un panel no interactivo con las tareas atrasadas, las que corresponden al ciclo o intervalo vigente hoy y las que están en un estado activo. Dentro de un proyecto muestra primero el Gantt del mes actual y después resume solo ese archivo; fuera de uno agrega el origen `Global` y los proyectos registrados e identifica cada tarea por origen.

La salida usa el ancho disponible, nunca supera 20 filas y activa colores automáticamente solo al escribir en una terminal. Se puede controlar con `--color=always`, `--color=never` o `--no-color`.

Para mostrarlo al iniciar Bash:

```sh
if command -v tasks >/dev/null 2>&1; then
  tasks summary
fi
```

El panel de tareas relevantes omite las finalizadas, canceladas, eliminadas y pendientes sin fecha. Una tarea atrasada tiene vencimiento anterior a hoy. Una tarea es para hoy mientras su intervalo iniciado siga vigente, vence hoy o tenga un ciclo recurrente pendiente. Las demás tareas en un estado normal distinto del inicial se muestran como activas; cada tarea aparece una sola vez.

## Navegación

- La cabecera indica siempre si se está en modo local o global y, en local, el proyecto abierto.
- `Ctrl+P`: abrir la paleta contextual; escribir busca por nombre, descripción o sinónimos, `↑`/`↓` selecciona, `Enter` ejecuta y `Esc` cancela.
- El pie muestra siempre todas las teclas válidas en el contexto actual y cambia con la vista, selección, formulario, selector o confirmación. `F1` abre además un mapa general opcional; `↑`/`↓` desplazan esa ayuda.
- `←`/`→` o `h`/`l`: cambiar vista. Cada vista recuerda su tarea, ventana visible, panel y fila del inspector.
- `Tab` / `Shift+Tab`: cambiar el foco entre la vista principal y el inspector. `↑`/`↓` o `j`/`k` recorren el panel activo; el borde, el título `ACTIVA` y la fila resaltada muestran dónde se aplicará la acción.
- El inspector permite recorrer campos, subtareas, dependencias e historial. `Enter` ejecuta la acción natural de la fila; `I` alterna disposición normal, expandida y oculta, y `Espacio` fija o libera la disposición al cambiar de vista.
- `n`: abrir el formulario completo de una tarea; `e`: reutilizarlo con la tarea seleccionada; `N`: captura compacta inmediata de solo título. El formulario recorre título, estado, prioridad, inicio, vencimiento y recurrencia con `Tab`/`Shift+Tab`, y muestra el destino/origen antes de guardar.
- En campos de texto, `←`/`→` mueve el cursor, `Ctrl+←`/`Ctrl+→` salta por palabras, `Ctrl+W` borra la palabra anterior y `Ctrl+U`/`Ctrl+K` borra hasta el inicio/final. El pegado preserva el borrador. `Enter` o `Ctrl+S` guarda todos los campos como una sola operación; `Esc` solo pide confirmación si hubo cambios.
- Las recargas y mutaciones conservan el contenido actual y muestran `⟳ Actualizando…` en la cabecera. Los resultados se distinguen también por `✓ ÉXITO`, `⚠ ADVERTENCIA` o `✗ ERROR`, nunca solo por color, y desaparecen en la siguiente interacción relevante.
- `U` deshace el último cambio compatible de estado, finalización, cancelación o papelera. El deshacer usa exactamente la versión producida por el cambio; si otra sesión ya modificó el elemento, se rechaza y ofrece recargar o revisar en vez de sobrescribir. En papelera restaura la tarea, pero las relaciones que la eliminación quitó explícitamente no se reconstruyen y la advertencia lo indica.
- `PgUp` / `PgDn`: periodo anterior/siguiente en Calendario y Gantt.
- `,` / `.`: desplazar la ventana de días del Gantt cuando el mes no cabe completo.
- `n`, `e`, `p`, `s`, `v`: crear, editar título, prioridad, inicio y vencimiento.
- `[` / `]`: mover entre estados; `m`: editar Markdown externamente.
- `f`, `C`, `z`: finalizar, cancelar o reabrir directamente en el estado inicial.
- `a`, `E`, `t`: crear, renombrar y alternar subtareas; enfóquelas con `Tab` y recórralas con las mismas flechas usadas para las tareas (`J`/`K` se conservan como alias).
- `{` / `}`: mover la subtarea seleccionada entre estados.
- `g` / `G`: crear/eliminar una dependencia mediante un selector de tareas; `c`: configurar recurrencia mediante un formulario guiado.
- `d`: papelera; `u`: restaurar; `H`: historial.
- `/`, `?`, `P`, `S`, `D`: buscar título/Markdown y filtrar origen/estado/fechas.
- `1`, `B`, `R`, `F`, `X`, `o`, `0`: prioridad, bloqueo, recurrencia, visibilidad de finalizadas/canceladas, orden y limpiar filtros.
- En la vista Estados: `a`, `e`, `i`, `[`/`]`, `d` administran estados normales.
- `r`: refrescar; `q` o `Ctrl+C`: salir.

Las operaciones que relacionan elementos no requieren memorizar IDs: dependencias, filtro local por estado y destino al eliminar un estado presentan selectores con ID, título y estado. Los IDs permanecen visibles en Tabla, detalle y Estados para diagnóstico.

En modo global, `n` y `N` crean siempre en el origen propio, que aparece explícitamente en el formulario completo. Las acciones de creación anidada aparecen para tareas globales y muestran una explicación cuando la tarea pertenece a un proyecto registrado. La selección solo recorre elementos que realmente aparecen en Calendario o Gantt, y todas las listas muestran la fila activa y marcadores `↑`/`↓` cuando existe contenido fuera del viewport.

La paleta muestra primero los comandos disponibles para la vista, selección y origen actuales. Los demás resultados explican por qué no están disponibles y conservan visible su atajo. Ejecutar desde la paleta recorre exactamente el mismo manejador que pulsar ese atajo; al cancelar no cambian la selección, los filtros ni el periodo visible. Los formularios, selectores, confirmaciones, historial y ayuda ya abiertos tienen precedencia sobre `Ctrl+P`.

Ante un conflicto del formulario, `r` pide confirmación antes de descartar el borrador y recargar, `v` muestra la versión remota para compararla y `k` conserva el texto local rebasándolo sobre la versión remota recién leída. Los conflictos fuera del formulario mantienen la vista visible y ofrecen recargar o revisar la versión remota en el Inspector.

La terminal mínima soportada es de 90 columnas por 40 filas (`90x40`). El cuerpo reserva dinámicamente el espacio ocupado por el pie contextual multilínea.

El editor Markdown se resuelve primero mediante `$VISUAL` y después `$EDITOR`. Si un cambio concurrente impide guardar, las ediciones se conservan en el archivo temporal y el error muestra su ruta.

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
- El registro, `tasks.log` y el almacén privado `global.sqlite` se guardan bajo el directorio de configuración del usuario devuelto por el sistema operativo, dentro de `tasks/`.
- `global.sqlite` no se registra, no se descubre como proyecto y solo aparece en modo global.
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
