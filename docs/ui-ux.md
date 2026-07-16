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

## Vistas y viewport

- **Kanban:** muestra tantas columnas como caben. `←` y `→` dentro del título de una columna indican columnas fuera de pantalla; al mover una tarea, su columna permanece en la ventana. Cada columna pagina sus tareas alrededor de la selección.
- **Tabla:** cada tarea ocupa una fila con columnas alineadas para ID, título, estado, prioridad, planificación, progreso de subtareas y bloqueo. La celda de estado usa la paleta compartida y el indicador de bloqueo conserva el color de peligro. En modo global, el origen usa una columna propia cuando cabe y acompaña al título en terminales estrechas. La columna del título absorbe el ancho restante y trunca con elipsis.
- **Calendario:** `↑`/`↓` solo recorre tareas con eventos en el mes y resalta sus entradas. Cada evento conserva el nombre textual del estado y lo colorea cuando no es la selección activa.
- **Gantt:** `↑`/`↓` solo recorre tareas visibles en el mes. `,`/`.` desplaza la ventana siete días; la cabecera muestra el rango actual. Las celdas de día crecen para aprovechar el ancho disponible y, cuando el espacio es estrecho, la escala etiqueta intervalos de cinco días sin perder una celda por fecha. Las dependencias se resumen en la misma fila para no reducir innecesariamente el número de tareas visibles. Las barras usan el color del estado y una leyenda textual coloreada explica la correspondencia.
- **Papelera y Estados:** la fila que recibirá `u`, `e`, `i` o `d` siempre está resaltada.
- **Detalle y subtareas:** el ID de tarea es visible y la lista de subtareas mantiene la selección dentro de su ventana. El panel ocupa todo el ancho; distribuye subtareas y un avance breve de Markdown en columnas y nunca excede el alto que le asigna el modelo.

Después de una mutación que cambie el orden —por ejemplo, prioridad, título o estado— la selección se conserva por identidad `(origen, ID)`, no por posición. Así el foco no salta silenciosamente a otra tarea durante la recarga.

## Terminales pequeñas

El modelo reserva espacio para cabecera, cuerpo, detalle y el pie contextual multilínea. El pie se compone primero y su altura renderizada se descuenta del cuerpo, incluso cuando una línea debe ajustarse al ancho. Cada pantalla recibe el alto restante y limita su contenido. Los indicadores `↑ N más` y `↓ N más` hacen explícito que existe contenido fuera de la ventana. La ayuda y los selectores también tienen viewport propio.

La terminal mínima soportada es de **90 columnas por 40 filas** (`90x40`). En ese tamaño el pie debe mostrar el mapa contextual completo sin ocultar el cuerpo ni depender de `F1`. Los tamaños inferiores quedan fuera del contrato de soporte.

Un aviso no reemplaza el mapa de acciones: ocupa una línea propia sobre él. El pie normal omite acciones que no aplican y habilita `a`, `g` y `c` solo cuando la tarea seleccionada admite creación en su origen. Los modos transitorios sustituyen temporalmente el mapa normal por todas las teclas válidas para completar, cancelar o cerrar esa interacción.

## Mensajes y restricciones

- `n` en global abre el formulario de una tarea propia. Sobre tareas globales también se permiten subtareas, dependencias y recurrencias; sobre tareas de proyecto, esas acciones explican que no se pueden añadir elementos desde global.
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
