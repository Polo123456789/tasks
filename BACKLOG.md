# Backlog de producto

Este documento reúne iniciativas posteriores al alcance inicial descrito en `SPEC.md`. El orden refleja la prioridad propuesta, no un compromiso de versión.

## B-001 — Captura rápida desde la CLI — Completada

Permitir crear una tarea sin preparar un documento JSON ni abrir la TUI.

Alcance inicial:

- Incorporar un comando dedicado, por ejemplo `tasks new "Título"`.
- Admitir prioridad, fecha de inicio y vencimiento mediante opciones explícitas.
- Dentro de un proyecto, usar por defecto el `.tasks` detectado; fuera de uno, usar el almacén global.
- Permitir elegir de forma explícita `--global` o `--project ruta.tasks`.
- Mantener sin cambios la semántica actual de `tasks add`, orientada a lotes JSON.

Criterios de aceptación:

- La ayuda explica de forma inequívoca el destino antes de crear.
- Fechas, prioridades y títulos reutilizan las validaciones del dominio.
- Un error no deja una tarea parcial.
- La salida identifica origen e ID de la tarea creada y puede consumirse desde scripts.

## B-002 — Paleta contextual de comandos en la TUI — Completada

Ofrecer una entrada buscable para descubrir y ejecutar acciones sin memorizar todos los atajos.

Alcance inicial:

- Abrir la paleta mediante una tecla global y cerrarla con `Esc`.
- Buscar por nombre, descripción y sinónimos de las acciones.
- Mostrar primero las acciones válidas para la vista, selección y origen actuales.
- Explicar por qué una acción está deshabilitada cuando resulte útil.
- Ejecutar los mismos comandos internos que los atajos existentes, sin duplicar reglas funcionales.
- Conservar los atajos como vía rápida y mostrar el atajo junto a cada resultado.

Criterios de aceptación:

- La paleta funciona en modo local y global y en todas las vistas.
- La selección, los filtros y el viewport se conservan al cancelar.
- Formularios, selectores y confirmaciones abiertos siguen teniendo precedencia clara.
- Es completamente operable con teclado y respeta el alto disponible.

## B-003 — Exportación, respaldo y diagnóstico — Completada

Hacer explícitas la portabilidad, recuperación y verificación de los datos.

Alcance inicial:

- `tasks export` con JSON portable y formatos legibles como Markdown o CSV.
- `tasks backup` para crear una copia consistente del almacén seleccionado.
- `tasks restore` con validación previa y sin sobrescrituras silenciosas.
- `tasks doctor` para revisar esquema, integridad SQLite, versión, permisos, registro global y proyectos no disponibles.
- Salidas humanas por defecto y una variante estructurada para automatización cuando corresponda.

Criterios de aceptación:

- Exportar y diagnosticar son operaciones de solo lectura.
- Los respaldos se generan desde un estado consistente y no dejan archivos parciales.
- Restaurar nunca reemplaza datos existentes sin una confirmación o una opción explícita.
- Los errores distinguen problemas reparables, incompatibilidades de versión y corrupción.
- Los comandos funcionan tanto con un proyecto local como con el almacén global seleccionado explícitamente.

## B-004 — Navegación con foco entre paneles — Completada

Convertir la vista principal, el inspector y sus secciones en áreas navegables coherentes, sin exigir grupos distintos de teclas para tareas y subtareas.

Alcance inicial:

- Tratar la vista principal y el inspector de tarea como paneles con foco explícito.
- Usar `Tab` y `Shift+Tab` para cambiar de panel y `↑`/`↓` para navegar dentro del panel activo.
- Permitir navegar por campos, subtareas, dependencias e historial desde el inspector.
- Distinguir claramente el panel activo mediante borde, título y selección visible.
- Permitir ocultar, fijar o expandir el inspector sin perder el contexto.
- Recordar por vista la tarea seleccionada, el viewport y la sección enfocada.

Criterios de aceptación:

- Cambiar de panel o cancelar una interacción conserva la tarea seleccionada.
- La navegación funciona en Kanban, Tabla, Calendario y Gantt.
- No es necesario usar teclas diferentes como `J`/`K` para recorrer subtareas.
- Las acciones continúan aplicándose únicamente al elemento que muestra foco visible.
- El modelo funciona completamente con teclado y después de redimensionar la terminal.

## B-005 — Formulario unificado de creación y edición — Completada

Permitir completar o modificar los principales atributos de una tarea en una sola interacción, conservando una captura rápida para el título.

Alcance inicial:

- Unificar título, estado, prioridad, inicio, vencimiento y recurrencia en un formulario estructurado.
- Reutilizar el mismo formulario para crear y editar tareas.
- Navegar entre campos con `Tab` y `Shift+Tab`.
- Incorporar un componente de texto con cursor, movimiento por palabras, pegado y borrado de palabras o línea.
- Ofrecer un modo compacto que cree inmediatamente una tarea solo con título.
- Preservar el borrador al abrir selectores, al validar o al recibir un error recuperable.
- Mostrar el destino y origen antes de guardar en modo global.

Criterios de aceptación:

- La validación aparece junto al campo correspondiente y no reemplaza la vista completa.
- Guardar todos los campos constituye una única operación coherente para el usuario.
- `Esc` solicita confirmación únicamente cuando existen cambios sin guardar.
- Fechas y recurrencia incompatibles se explican antes de enviar la mutación.
- Crear, editar y cancelar preservan correctamente selección y viewport.

## B-006 — Feedback no bloqueante, recuperación y deshacer — Completada

Mantener visible el contexto mientras la aplicación carga, valida o informa el resultado de una operación.

Alcance inicial:

- Conservar el contenido actual durante recargas y mostrar actividad en la cabecera o en el elemento afectado.
- Diferenciar mensajes de éxito, advertencia y error mediante texto, símbolo y color.
- Mostrar errores de formulario junto al campo que debe corregirse.
- Hacer que los avisos transitorios desaparezcan después de un tiempo o de la siguiente interacción relevante.
- Ofrecer deshacer para cambios de estado, finalización, cancelación y envío a papelera cuando la versión todavía lo permita.
- Ante conflictos concurrentes, ofrecer recargar, revisar el cambio o conservar el borrador local.

Criterios de aceptación:

- Una recarga ordinaria no sustituye toda la pantalla por `Cargando…`.
- Ningún mensaje depende únicamente del color.
- Deshacer respeta el control optimista de versiones y nunca sobrescribe cambios externos.
- Los borradores se conservan tras errores recuperables y conflictos.
- El usuario recibe confirmación inequívoca del resultado de cada mutación.

## B-007 — Selector de fechas mediante calendario — Completada

Permitir elegir fechas visualmente sin exigir que el usuario escriba siempre `AAAA-MM-DD`.

Alcance inicial:

- Abrir un calendario desde los campos de inicio y vencimiento del formulario unificado.
- Navegar por días con flechas, por semanas con `PgUp`/`PgDn` y por meses mediante una acción visible.
- Marcar claramente hoy, la fecha actualmente guardada y el día enfocado.
- Permitir elegir una sola fecha, limpiar el campo o cancelar sin cambios.
- Al seleccionar un rango, impedir o explicar un vencimiento anterior al inicio.
- Reutilizar el componente en edición, creación y filtros por rango de fechas.
- Mantener disponible la entrada manual en formato `AAAA-MM-DD`.

Criterios de aceptación:

- `Enter` confirma el día enfocado y `Esc` vuelve al formulario conservando el borrador.
- El calendario muestra mes y año y funciona correctamente en cambios de año y años bisiestos.
- La selección es visible sin depender exclusivamente del color.
- El componente respeta el viewport mínimo y se adapta al ancho disponible.
- La fecha confirmada utiliza el tipo `Date` del dominio sin introducir horas ni zonas horarias.
