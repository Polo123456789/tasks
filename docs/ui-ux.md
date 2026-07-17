# Interacción y claridad de la TUI

Este documento define el contrato de usabilidad de `tasks`. Complementa `SPEC.md`: no cambia reglas de dominio, sino que establece cómo hacerlas visibles y operables desde la terminal.

## Principios

1. **Sin conocimiento interno obligatorio.** Ninguna operación normal exige consultar SQLite o adivinar un ID. Cuando una relación necesita una identidad estable, la TUI muestra un selector con ID y nombre.
2. **La acción y el filtro son conceptos distintos.** Finalizar, cancelar y reabrir modifican la tarea; mostrar u ocultar estados especiales solo cambia el filtro.
3. **Toda selección es visible.** Si `↑`/`↓` cambia el elemento sobre el que actuará una tecla, ese elemento debe quedar resaltado.
4. **El tamaño de terminal no oculta datos sin salida.** Las listas usan ventanas centradas en la selección y marcadores `↑`/`↓`. Kanban pagina columnas y Gantt permite desplazar días.
5. **El contexto siempre está presente.** La cabecera identifica modo local/global y, en las vistas agregadas, cada tarea identifica su origen. Las acciones prohibidas explican la restricción concreta.
6. **Ayuda contextual completa.** El pie enumera todas las acciones disponibles en el contexto actual, agrupadas por navegación, tarea, relaciones, subtarea y filtros. Cambia al entrar en Papelera, Estados, historial, formularios, confirmaciones o selectores. `F1` conserva un mapa general opcional, pero ninguna operación debe exigir consultarlo. Cada formulario muestra formato, ejemplo y cómo confirmar o cancelar.
7. **Lenguaje de usuario.** Meses, recurrencias, eventos de historial y errores de interacción se presentan en español. La sintaxis compacta queda como detalle documentado, no como punto de entrada principal.
8. **Color como refuerzo.** Tabla, Calendario y Gantt comparten una paleta estable por estado: finalizadas en verde, canceladas en tono apagado y estados normales personalizados en colores diferenciados por nombre. El nombre, símbolo o leyenda permanece visible; ninguna distinción depende únicamente del color y el resaltado de selección tiene prioridad de contraste.
9. **Paleta contextual.** `Ctrl+P` busca comandos por nombre, descripción y sinónimos. Ordena primero los válidos para la vista, selección y origen, explica los deshabilitados y ejecuta el mismo comando interno que el atajo mostrado.
10. **Foco de panel explícito.** Vista e Inspector tienen borde y título de foco. `Tab`/`Shift+Tab` cambia de panel y `↑`/`↓` recorre únicamente el panel activo, de modo que una acción anidada nunca usa una selección invisible.
11. **Formulario único y transaccional.** `n` crea y `e` edita mediante el mismo formulario de título, estado, prioridad, fechas y recurrencia. `N` conserva una captura compacta solo con título. La validación permanece junto al campo y el backend recibe una sola mutación optimista con todos los valores.
12. **Feedback no bloqueante.** Una carga posterior a la inicial no sustituye el cuerpo. La cabecera indica `⟳ Actualizando…` y cada resultado combina símbolo, palabra y color: `✓ ÉXITO`, `⚠ ADVERTENCIA` o `✗ ERROR`.

## Mapa de interacción

### Ciclo de vida

| Acción | Tecla | Resultado |
|---|---:|---|
| Finalizar | `f` | Salta directamente a `Finalizada` y aplica las reglas de subtareas. |
| Cancelar | `C` | Salta directamente a `Cancelada`. |
| Reabrir | `z` | Vuelve directamente al estado inicial del origen. |
| Alternar finalizadas | `F` | Cambia únicamente su visibilidad. |
| Alternar canceladas | `X` | Cambia únicamente su visibilidad. |

### Selectores

- `g`: lista todas las tareas elegibles del mismo origen, aunque un filtro las oculte.
- `G`: lista únicamente las dependencias actuales de la tarea.
- `S` en local: lista los estados y permite volver a “Todos los estados”.
- `d` en Estados: muestra destinos normales y la opción “Sin destino” para estados vacíos.
- `c`: elige primero el tipo de recurrencia; semanal usa selección múltiple de días y mensual por día de semana usa selectores consecutivos de ordinal y día. Solo el día numérico del mes requiere escribir un valor.

Los selectores usan `↑`/`↓`, `Enter` y `Esc`. Una lista vacía lo indica explícitamente.

## Formulario de tarea

- `Tab`/`Shift+Tab` o `↑`/`↓` recorre los campos; `←`/`→` cambia estado y prioridad o mueve el cursor en texto.
- El editor admite saltos por palabras (`Ctrl+←`/`Ctrl+→`), pegado, `Ctrl+W`, `Ctrl+U` y `Ctrl+K`. El cursor se muestra dentro del valor activo.
- `Enter`/`Ctrl+S` valida y guarda el conjunto completo. Errores de título, fecha o recurrencia aparecen debajo del campo sin cerrar ni reemplazar el formulario.
- El borrador sobrevive a validaciones y errores recuperables. `Esc` cierra directamente un formulario intacto y solicita confirmación solo cuando su firma difiere de los valores iniciales.
- Crear, editar o cancelar no modifica la selección, el viewport ni el foco previo. El formulario identifica el proyecto u origen global que recibirá la escritura antes de confirmarla.
- En conflicto, `v` carga una comparación remota sin reemplazar el borrador; `k` conserva el texto local y actualiza su versión base con una lectura remota, y `r` solicita confirmación explícita antes de descartarlo para recargar. La elección se presenta antes de aceptar más edición.

## Feedback, conflictos y deshacer

- El primer arranque puede mostrar `Cargando…`; recargas, filtros y mutaciones posteriores conservan el último contenido válido. Un fallo de recarga tampoco vacía esa vista.
- Éxitos, advertencias y errores ocupan una línea sobre el mapa contextual. Desaparecen con la siguiente interacción relevante, mientras la acción de deshacer permanece disponible hasta que otra mutación la reemplace o su versión deje de coincidir.
- `U` deshace cambios de columna, finalización, cancelación, reapertura y envío a papelera. La entrada guarda origen, ID, estado anterior y la versión resultante devuelta por el backend; este exige esa versión al revertir. El undo de papelera restaura la tarea, no las relaciones eliminadas, y se confirma como advertencia explícita.
- Un conflicto optimista nunca se fuerza. `r` recarga y `v` inspecciona la versión remota; dentro del formulario también se puede conservar el borrador local. Texto y símbolos hacen que ningún estado dependa solo del color.

## Selector de fechas

- `Ctrl+O` abre el mismo calendario desde Inicio y Vencimiento, tanto en el formulario completo como en la edición rápida, o desde el filtro de rango. La entrada manual `AAAA-MM-DD` permanece disponible y `Esc` vuelve exactamente al borrador anterior.
- `←`/`→`/`↑`/`↓` cambia un día, `PgUp`/`PgDn` siete días, `[`/`]` un mes con el día ajustado al final del mes y `Home` enfoca hoy. La cabecera siempre nombra mes y año.
- Cada celda combina símbolos textuales: `>` foco, `=` fecha guardada o filtro aplicado y `*` hoy. Los símbolos se acumulan cuando coinciden, de modo que la selección no depende del color.
- `Enter` confirma el `Date` enfocado, `x` limpia el valor y `Esc` cancela. Un vencimiento anterior al inicio —en formulario, edición rápida o filtro— mantiene el calendario abierto y explica el conflicto sin mutar el borrador.
- El filtro selecciona primero el extremo inicial y después el final; solo actualiza su texto al completar ambos y no aplica el filtro hasta la confirmación normal del formulario.

### Paleta de comandos

- `Ctrl+P` la abre desde cualquier vista local o global cuando no hay otra interacción transitoria activa.
- Escribir filtra por nombre, descripción, sinónimos o atajo; `↑`/`↓` recorre resultados, `Enter` ejecuta y `Esc` cancela.
- Los comandos válidos aparecen antes que los deshabilitados. Estos últimos permanecen buscables y muestran la restricción concreta.
- Cada resultado muestra el atajo equivalente. La ejecución reinyecta ese atajo en el manejador raíz, por lo que no existe una segunda implementación de las reglas funcionales.
- Cancelar solo cierra la paleta: conserva vista, tarea y subtarea seleccionadas, filtros, mes y desplazamiento del Gantt.
- Formularios, selectores, confirmaciones, historial y ayuda tienen precedencia. Mientras uno esté abierto, `Ctrl+P` no lo reemplaza ni descarta su estado.

## Vistas y viewport

- **Foco:** la vista principal y el Inspector son paneles navegables. El Inspector presenta filas para campos, subtareas, dependencias e historial; `Enter` ejecuta la acción natural de la fila. `I` alterna normal, expandido y oculto, y `Espacio` fija la disposición entre vistas. Cada vista recuerda por separado la identidad de tarea, la fila enfocada y la ventana derivada de esa selección.

- **Kanban:** muestra tantas columnas como caben. `←` y `→` dentro del título de una columna indican columnas fuera de pantalla; al mover una tarea, su columna permanece en la ventana. Cada columna pagina sus tareas alrededor de la selección.
- **Tabla:** cada tarea ocupa una fila con columnas alineadas para ID, título, estado, prioridad, planificación, progreso de subtareas y bloqueo. La celda de estado usa la paleta compartida y el indicador de bloqueo conserva el color de peligro. En modo global, el origen usa una columna propia cuando cabe y acompaña al título en terminales estrechas. La columna del título absorbe el ancho restante y trunca con elipsis.
- **Calendario:** `↑`/`↓` solo recorre tareas con eventos en el mes y resalta sus entradas. Cada evento conserva el nombre textual del estado y lo colorea cuando no es la selección activa.
- **Gantt:** `↑`/`↓` solo recorre tareas visibles en el mes. `,`/`.` desplaza la ventana siete días; la cabecera muestra el rango actual. Las celdas de día crecen para aprovechar el ancho disponible y, cuando el espacio es estrecho, la escala etiqueta intervalos de cinco días sin perder una celda por fecha. Las dependencias se resumen en la misma fila para no reducir innecesariamente el número de tareas visibles. Las barras usan el color del estado y una leyenda textual coloreada explica la correspondencia.
- **Papelera y Estados:** la fila que recibirá `u`, `e`, `i` o `d` siempre está resaltada.
- **Inspector:** mantiene visibles el ID y el título de tarea y pagina sus filas alrededor del foco. Campos, subtareas, dependencias e historial usan una sola navegación; las acciones de subtarea solo se habilitan cuando una subtarea tiene el foco. En modo expandido ocupa el cuerpo completo sin descartar la selección de la vista principal y nunca excede el alto asignado.

Después de una mutación que cambie el orden —por ejemplo, prioridad, título o estado— la selección se conserva por identidad `(origen, ID)`, no por posición. Así el foco no salta silenciosamente a otra tarea durante la recarga.

## Terminales pequeñas

El modelo reserva espacio para cabecera, cuerpo, detalle y el pie contextual multilínea. El pie se compone primero y su altura renderizada se descuenta del cuerpo, incluso cuando una línea debe ajustarse al ancho. Cada pantalla recibe el alto restante y limita su contenido. Los indicadores `↑ N más` y `↓ N más` hacen explícito que existe contenido fuera de la ventana. La ayuda y los selectores también tienen viewport propio.

La terminal mínima soportada es de **90 columnas por 40 filas** (`90x40`). En ese tamaño el pie debe mostrar el mapa contextual completo sin ocultar el cuerpo ni depender de `F1`. Los tamaños inferiores quedan fuera del contrato de soporte.

Un aviso no reemplaza el mapa de acciones: ocupa una línea propia sobre él. El pie normal omite acciones que no aplican y habilita `a`, `g` y `c` solo cuando la tarea seleccionada admite creación en su origen. Los modos transitorios sustituyen temporalmente el mapa normal por todas las teclas válidas para completar, cancelar o cerrar esa interacción.

## Mensajes y restricciones

- `n` en global abre el formulario de una tarea propia y muestra ese destino; `N` ofrece la variante compacta. Sobre tareas globales también se permiten subtareas, dependencias y recurrencias; sobre tareas de proyecto, esas acciones explican que no se pueden añadir elementos desde global.
- Los conflictos de versión indican que se debe recargar con `r`.
- Las advertencias de papelera nombran las tareas afectadas (`#ID título`), no solo IDs aislados.
- Los formularios de fecha usan `AAAA-MM-DD`; los errores se traducen antes de mostrarse.

## Checklist de revisión visual

Para cada cambio de UI se revisan, como mínimo:

- local y global;
- vacío, normal, saturado, error y conflicto;
- `90x40`, `120x40` y una pantalla ancha;
- selección al inicio, medio y final de listas largas;
- pie normal completo, `F1`, avisos, historial, selectores, confirmaciones y formularios;
- que ninguna acción se aplique a una selección invisible.
