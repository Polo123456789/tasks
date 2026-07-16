# Interacción y claridad de la TUI

Este documento define el contrato de usabilidad de `tasks`. Complementa `SPEC.md`: no cambia reglas de dominio, sino que establece cómo hacerlas visibles y operables desde la terminal.

## Principios

1. **Sin conocimiento interno obligatorio.** Ninguna operación normal exige consultar SQLite o adivinar un ID. Cuando una relación necesita una identidad estable, la TUI muestra un selector con ID y nombre.
2. **La acción y el filtro son conceptos distintos.** Finalizar, cancelar y reabrir modifican la tarea; mostrar u ocultar estados especiales solo cambia el filtro.
3. **Toda selección es visible.** Si `↑`/`↓` cambia el elemento sobre el que actuará una tecla, ese elemento debe quedar resaltado.
4. **El tamaño de terminal no oculta datos sin salida.** Las listas usan ventanas centradas en la selección y marcadores `↑`/`↓`. Kanban pagina columnas y Gantt permite desplazar días.
5. **El contexto siempre está presente.** La cabecera identifica modo local/global y proyecto. Las acciones prohibidas en global explican la restricción.
6. **Ayuda progresiva.** El pie contiene solo las acciones principales de la vista. `F1` abre el mapa completo, desplazable en terminales pequeñas. Cada formulario muestra formato, ejemplo y cómo confirmar o cancelar.
7. **Lenguaje de usuario.** Meses, recurrencias, eventos de historial y errores de interacción se presentan en español. La sintaxis compacta queda como detalle documentado, no como punto de entrada principal.

## Mapa de interacción

### Ciclo de vida

| Acción | Tecla | Resultado |
|---|---:|---|
| Finalizar | `f` | Salta directamente a `Finalizada` y aplica las reglas de subtareas. |
| Cancelar | `C` | Salta directamente a `Cancelada`. |
| Reabrir | `z` | Vuelve directamente al estado inicial del proyecto. |
| Alternar finalizadas | `F` | Cambia únicamente su visibilidad. |
| Alternar canceladas | `X` | Cambia únicamente su visibilidad. |

### Selectores

- `g`: lista todas las tareas elegibles del proyecto, aunque un filtro las oculte.
- `G`: lista únicamente las dependencias actuales de la tarea.
- `S` en local: lista los estados y permite volver a “Todos los estados”.
- `d` en Estados: muestra destinos normales y la opción “Sin destino” para estados vacíos.
- `c`: elige primero el tipo de recurrencia; semanal usa selección múltiple de días y mensual por día de semana usa selectores consecutivos de ordinal y día. Solo el día numérico del mes requiere escribir un valor.

Los selectores usan `↑`/`↓`, `Enter` y `Esc`. Una lista vacía lo indica explícitamente.

## Vistas y viewport

- **Kanban:** muestra tantas columnas como caben. `←` y `→` dentro del título de una columna indican columnas fuera de pantalla; al mover una tarea, su columna permanece en la ventana. Cada columna pagina sus tareas alrededor de la selección.
- **Tabla:** cada tarea ocupa una línea adaptable. Los campos se expresan como segmentos y se truncan con elipsis al ancho disponible.
- **Calendario:** `↑`/`↓` solo recorre tareas con eventos en el mes y resalta sus entradas.
- **Gantt:** `↑`/`↓` solo recorre tareas visibles en el mes. `,`/`.` desplaza la ventana siete días; la cabecera muestra el rango actual.
- **Papelera y Estados:** la fila que recibirá `u`, `e`, `i` o `d` siempre está resaltada.
- **Detalle y subtareas:** el ID de tarea es visible y la lista de subtareas mantiene la selección dentro de su ventana.

Después de una mutación que cambie el orden —por ejemplo, prioridad, título o estado— la selección se conserva por identidad `(proyecto, ID)`, no por posición. Así el foco no salta silenciosamente a otra tarea durante la recarga.

## Terminales pequeñas

El modelo reserva espacio para cabecera, cuerpo, detalle y pie. Cada pantalla recibe el alto disponible y limita su contenido. Los indicadores `↑ N más` y `↓ N más` hacen explícito que existe contenido fuera de la ventana. La ayuda y los selectores también tienen viewport propio.

El objetivo mínimo de revisión es `80x24`; también se prueba degradación segura por debajo de ese tamaño. El pie debe conservar al menos `F1 ayuda` y `q` dentro de 80 columnas.

## Mensajes y restricciones

- Una acción de creación en modo global no abre un formulario ni falla silenciosamente: explica que el modo global no permite crear ese tipo de elemento.
- Los conflictos de versión indican que se debe recargar con `r`.
- Las advertencias de papelera nombran las tareas afectadas (`#ID título`), no solo IDs aislados.
- Los formularios de fecha usan `AAAA-MM-DD`; los errores se traducen antes de mostrarse.

## Checklist de revisión visual

Para cada cambio de UI se revisan, como mínimo:

- local y global;
- vacío, normal, saturado, error y conflicto;
- `80x24`, `120x40` y un tamaño estrecho;
- selección al inicio, medio y final de listas largas;
- `F1`, selectores, confirmaciones y formularios;
- que ninguna acción se aplique a una selección invisible.
