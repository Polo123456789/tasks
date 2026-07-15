# Especificación funcional — Gestor de tareas para terminal

## 1. Objetivo

Aplicación TUI para gestionar tareas desde la terminal. Cada proyecto almacena sus datos en un único archivo con extensión `.tasks`.

La aplicación opera automáticamente en modo **local** o **global** según el directorio desde el que se ejecute.

## 2. Detección del modo

### 2.1 Búsqueda de proyecto

Al ejecutar `tasks`, la aplicación busca archivos `*.tasks`:

1. En el directorio actual.
2. Sucesivamente en cada directorio padre.
3. Se detiene en el primer directorio que contenga uno.

Resultados:

- **Un archivo:** inicia en modo local.
- **Más de uno en el mismo directorio:** termina con error y muestra los archivos conflictivos.
- **Ninguno:** inicia en modo global.

### 2.2 Identidad del proyecto

El nombre visible del proyecto es el nombre del archivo sin la extensión.

Ejemplo:

```text
/home/user/proyectos/backend/backend.tasks
```

Nombre visible: `backend`.

La ruta también se mostrará cuando sea necesario distinguir proyectos con el mismo nombre.

## 3. Creación de proyectos

```bash
tasks init nombre.tasks
```

El comando:

- Exige la extensión `.tasks`.
- Crea el archivo en el directorio actual.
- No sobrescribe archivos existentes.
- Falla si el directorio ya contiene otro archivo `.tasks`.
- Inicializa la configuración y los estados predeterminados.
- Registra el proyecto en el índice global.
- Abre la aplicación en modo local.

El archivo será una base de datos SQLite autocontenida y portable.

## 4. Modo local

Muestra únicamente los datos del proyecto detectado.

### Vistas

- **Kanban**, vista predeterminada.
- Gantt.
- Calendario.
- Tabla.

### Operaciones

Permite:

- Crear, editar y eliminar tareas.
- Crear y editar subtareas.
- Administrar dependencias.
- Administrar estados normales.
- Configurar recurrencias.
- Editar documentación Markdown.
- Consultar y restaurar la papelera.

## 5. Modo global

Se activa cuando no se encuentra un archivo `.tasks`.

Agrega los datos de todos los proyectos registrados.

### Vistas

- **Calendario**, vista predeterminada.
- Gantt.
- Tabla.
- No incluye Kanban.

### Restricciones

No permite ningún tipo de creación:

- Tareas.
- Subtareas.
- Proyectos.
- Estados.
- Dependencias.
- Reglas de recurrencia.

Sí permite operar sobre elementos existentes:

- Consultar y editar tareas.
- Cambiar estado y prioridad.
- Finalizar, cancelar o reabrir.
- Modificar fechas.
- Modificar o eliminar recurrencias y dependencias.
- Editar documentación.
- Enviar a la papelera y restaurar.

Cada modificación se escribe directamente en la base del proyecto correspondiente.

### Visibilidad predeterminada

Las tareas `Finalizadas` y `Canceladas` estarán ocultas. Podrán mostrarse mediante filtros.

## 6. Registro global

Cada archivo `.tasks` se registra cuando se crea o abre por primera vez.

El registro global almacena su ruta absoluta.

Al entrar en modo global:

- Se comprueba cada ruta registrada.
- Si la ruta o el archivo ya no existe, la entrada se elimina inmediatamente.
- La eliminación es silenciosa.
- Si el proyecto reaparece, deberá abrirse nuevamente para registrarlo.

Los proyectos son unidades independientes. No existen relaciones entre proyectos.

## 7. Modelo de tarea

Una tarea contiene:

- Identificador interno.
- Título obligatorio.
- Estado.
- Prioridad.
- Documentación Markdown opcional.
- Fecha de inicio opcional.
- Fecha de vencimiento opcional.
- Regla de recurrencia opcional.
- Subtareas.
- Dependencias.
- Historial.
- Fechas de creación y última modificación.

No existirá un campo de descripción separado: la descripción formará parte del Markdown.

No habrá etiquetas.

### Prioridades

- Sin prioridad, valor predeterminado.
- Baja.
- Media.
- Alta.
- Urgente.

## 8. Estados

### 8.1 Estados normales

Cada proyecto puede crear, renombrar, ordenar y eliminar sus propios estados.

Estados iniciales:

1. Pendiente.
2. En progreso.
3. Bloqueada.

Cada proyecto debe tener exactamente un estado normal designado como **estado inicial**. `Pendiente` será el inicial por defecto.

No se puede eliminar el estado inicial sin designar antes otro.

No se puede eliminar un estado que contenga tareas sin indicar a qué estado deben trasladarse.

### 8.2 Estados especiales

Existirán dos estados fijos:

- `Cancelada`.
- `Finalizada`.

No pueden renombrarse ni eliminarse. `Finalizada` aparece siempre como la última columna del Kanban.

Una tarea cancelada no satisface sus dependencias. Las tareas que dependan de ella continuarán bloqueadas hasta que la dependencia sea eliminada o la tarea sea reabierta y finalizada.

## 9. Subtareas

Solo se permite un nivel de subtareas.

Una subtarea contiene únicamente:

- Título.
- Estado.

No puede tener fechas, prioridad, documentación, dependencias, recurrencia ni otras subtareas.

### Finalización

- Finalizar una tarea principal finaliza todas sus subtareas.
- Si hay dos o más subtareas, finalizar todas finaliza automáticamente la tarea principal.
- Si hay una sola subtarea, la tarea principal debe finalizarse manualmente.
- Reabrir una tarea principal reabre todas sus subtareas en el estado inicial.
- Cancelar una tarea principal cancela sus subtareas.

## 10. Dependencias

Una tarea puede depender de una o varias tareas del mismo proyecto.

- No se permiten ciclos directos ni indirectos.
- Una tarea queda bloqueada automáticamente mientras alguna dependencia no esté `Finalizada`.
- El bloqueo automático es independiente de su estado manual.
- Al finalizar todas sus dependencias, el bloqueo automático desaparece.
- Las dependencias se muestran en el detalle y en Gantt.
- Las subtareas no admiten dependencias.

## 11. Fechas

La aplicación trabaja exclusivamente con fechas, sin horas.

Una tarea normal puede tener:

- Inicio y vencimiento.
- Solo inicio.
- Solo vencimiento.
- Ninguna fecha.

La fecha de vencimiento no puede ser anterior a la fecha de inicio.

En Gantt:

- Inicio y vencimiento forman un intervalo.
- Una sola fecha se representa como un hito.
- Las tareas sin fechas no aparecen.

## 12. Recurrencia

Las tareas recurrentes reutilizan la misma tarea; no generan copias.

Una tarea recurrente:

- No tiene fecha de inicio.
- No tiene fecha de vencimiento.
- No aparece en Calendario ni Gantt.
- Sí aparece en Kanban local y Tabla.

### Reglas admitidas

- Diaria.
- Semanal, en uno o varios días.
- Mensual, en un día concreto.
- Último día del mes.
- Mensual por ordinal y día de semana:
  - Primer, segundo, tercer, cuarto o último.
  - Lunes a domingo.

Ejemplos:

- Cada lunes y jueves.
- El día 15 de cada mes.
- El último día del mes.
- El primer lunes del mes.
- El último viernes del mes.

No se admiten expresiones cron.

### Reinicio del ciclo

Al comenzar un nuevo ciclo:

- La tarea vuelve al estado inicial.
- Todas sus subtareas vuelven al estado inicial.
- El ciclo anterior se registra como completado o no completado.
- La documentación, prioridad y dependencias se conservan.

Si transcurrieron varios ciclos mientras la aplicación estaba cerrada:

- Se realiza un único reinicio hasta el ciclo vigente.
- No se crean ciclos intermedios.
- El historial registra cuántas recurrencias se omitieron.

El cálculo se realiza al abrir la base y cuando cambia el día mientras la aplicación está abierta.

## 13. Documentación Markdown

Cada tarea puede contener documentación Markdown almacenada dentro de la base.

La TUI:

- Renderiza un fragmento al seleccionar la tarea.
- Permite abrir el contenido completo en un editor externo.

Resolución del editor:

1. `$VISUAL`.
2. `$EDITOR`.
3. Error explicativo si ninguna variable está configurada.

Para editar:

1. Se genera un archivo temporal.
2. Se abre el editor.
3. Al cerrarse correctamente, el contenido se guarda en la base.
4. El archivo temporal se elimina.

## 14. Papelera

Eliminar una tarea la mueve a la papelera durante 30 días.

- Conserva subtareas, documentación e historial.
- No aparece en las vistas normales.
- Puede restaurarse antes del vencimiento.
- Después de 30 días se elimina definitivamente.
- La depuración ocurre automáticamente al abrir la base.

### Dependencias al eliminar

Si la tarea participa en dependencias:

- Se muestra una advertencia con las tareas afectadas.
- Se solicita confirmación.
- Se eliminan todas sus relaciones de dependencia.
- La operación se registra en el historial.
- Las dependencias no se recuperan al restaurar la tarea.

## 15. Vistas

### Kanban local

- Una columna por estado normal, según el orden configurado.
- Columnas fijas para `Cancelada` y `Finalizada`.
- Permite mover tareas entre estados.
- Indica prioridad, recurrencia y bloqueo automático.

### Gantt

- Muestra tareas normales con fechas.
- Representa intervalos, hitos y dependencias.
- Excluye tareas recurrentes y tareas sin fechas.
- En global, agrupa o identifica cada tarea por proyecto.

### Calendario

- Muestra tareas normales con fecha de inicio o vencimiento.
- Excluye tareas recurrentes.
- En global, identifica visualmente el proyecto.

### Tabla

Puede mostrar:

- Proyecto, en modo global.
- Título.
- Estado.
- Prioridad.
- Inicio.
- Vencimiento.
- Recurrencia.
- Bloqueo.
- Cantidad/progreso de subtareas.

## 16. Búsqueda, filtros y ordenamiento

Todas las vistas admitirán, cuando resulte aplicable:

### Búsqueda

- Búsqueda textual por título.
- Búsqueda dentro del Markdown bajo una acción explícita.

### Filtros

- Proyecto, en modo global.
- Estado.
- Prioridad.
- Rango de fechas.
- Tareas recurrentes.
- Tareas bloqueadas.
- Finalizadas y canceladas.

### Ordenamiento

- Prioridad.
- Título.
- Estado.
- Fecha de inicio.
- Fecha de vencimiento.
- Última modificación.

## 17. Historial

Se registrarán eventos relevantes:

- Creación y edición.
- Cambios de estado y prioridad.
- Finalización, cancelación y reapertura.
- Reinicios recurrentes.
- Ciclos completados o no completados.
- Cambios en subtareas.
- Creación o eliminación de dependencias.
- Entrada y restauración desde la papelera.

El historial pertenece a la tarea y no podrá editarse manualmente.

## 18. Requisitos técnicos

- Funcionamiento completamente local y sin servicios externos.
- Persistencia transaccional mediante SQLite.
- Escrituras atómicas.
- Migraciones versionadas del esquema.
- Bloqueo o detección segura de acceso concurrente.
- Manejo de terminal redimensionable.
- Navegación completa mediante teclado.
- Compatibilidad inicial con Linux y macOS.
- Archivo `.tasks` portable y autocontenido.
- El índice global no contiene copias de tareas, solamente rutas.

## 19. Fuera del alcance inicial

- Sincronización remota.
- Colaboración multiusuario.
- Dependencias entre proyectos.
- Etiquetas.
- Archivos adjuntos.
- Subtareas anidadas.
- Horas, zonas horarias o recordatorios por hora.
- Expresiones cron.
- Creación de elementos desde el modo global.
- Notificaciones del sistema.
- Kanban global.
