# Plan de implementación y pruebas

## 1. Objetivo del plan

Implementar la aplicación descrita en `SPEC.md` manteniendo una separación estricta entre:

1. **Dominio y funcionamiento**: reglas, casos de uso, persistencia y acceso a proyectos.
2. **Interfaz TUI**: navegación, composición visual, formularios y adaptación de las acciones del usuario.
3. **Adaptadores del sistema**: SQLite, registro global, reloj, sistema de archivos y editor externo.

La TUI será un consumidor del núcleo de la aplicación, no el lugar donde se implementen las reglas de negocio. Esto permitirá cambiar pantallas, navegación, estilos o componentes sin reescribir la lógica funcional.

## 2. Stack

- **Go** estable.
- **Bubble Tea** para el bucle y estado de la TUI.
- **Bubbles** para controles reutilizables.
- **Lip Gloss** para estilos y layout.
- **Glamour** para renderizar Markdown.
- **SQLite** mediante `database/sql` y `modernc.org/sqlite`, sin CGO.
- **SQL explícito o sqlc** para el acceso tipado a datos; no se usará un ORM.
- **Migraciones SQL embebidas** en el binario.
- **`log/slog`** para logs enviados a archivo, nunca a la pantalla activa de la TUI.

## 3. Arquitectura

```text
cmd/
  tasks/                    # Ejecutable real
  ui-preview/               # Catálogo/preview de pantallas con datos falsos
internal/
  domain/                   # Entidades, valores y reglas puras
  application/              # Casos de uso y transacciones
  ports/                    # Contratos con infraestructura
  adapters/
    sqlite/                 # Bases .tasks y migraciones
    registry/               # Índice global de rutas
    filesystem/             # Detección local/global
    editor/                 # $VISUAL, $EDITOR y temporales
    clock/                  # Fecha actual del sistema
  tui/
    app/                    # Modelo raíz y navegación
    screens/
      kanban/
      table/
      calendar/
      gantt/
      trash/
      taskdetail/
      settings/
    components/             # Diálogos, formularios, filtros, etc.
    theme/                  # Tokens visuales
    keymap/                 # Atajos centralizados
    presenter/              # DTO de aplicación -> modelos de vista
migrations/                 # Migraciones del índice global si fueran necesarias
testdata/
  projects/                 # Bases y fixtures versionados para pruebas
```

Las migraciones propias de los archivos `.tasks` podrán vivir dentro del adaptador SQLite para poder embeberlas en el binario.

### 3.1 Dependencias permitidas

```text
TUI ───────> Application ───────> Domain
  │                │
  │                └────────────> Ports
  │                                  ▲
  └── integración de terminal        │
                              Adapters SQLite/FS/Clock
```

Reglas:

- `domain` no importará Bubble Tea, SQLite ni paquetes de terminal.
- `application` no conocerá colores, tamaños de pantalla, teclas ni widgets.
- `tui` no ejecutará SQL ni decidirá reglas de estado, recurrencia, bloqueo o subtareas.
- Los adaptadores no decidirán comportamiento visual.
- Los tipos de Bubble Tea no saldrán de `internal/tui`.

### 3.2 Contrato entre aplicación y TUI

La TUI trabajará con una fachada de aplicación formada por interfaces pequeñas definidas desde el lado consumidor. Conceptualmente incluirá:

- Obtener sesión, modo y capacidades.
- Consultar resúmenes y detalle de tareas.
- Crear y editar tareas.
- Cambiar estado y prioridad.
- Administrar subtareas y dependencias.
- Administrar recurrencia.
- Consultar y restaurar papelera.
- Consultar historial y estados.

Los resultados serán DTO independientes de la UI. La sesión expondrá capacidades como `CanCreateTask` o `CanCreateStatus`, pero las restricciones también se comprobarán dentro de los casos de uso. Ocultar una acción en la TUI no será el mecanismo de seguridad funcional.

Las operaciones desde Bubble Tea se ejecutarán como comandos asíncronos y devolverán mensajes de éxito o errores tipados. Se evitarán llamadas a SQLite dentro de `View()` o trabajo bloqueante dentro de `Update()`.

### 3.3 Preview independiente de UI

Se creará `cmd/ui-preview`, que usará un backend falso y fixtures deterministas. Permitirá abrir una pantalla concreta sin crear ni modificar bases reales:

```bash
go run ./cmd/ui-preview --screen kanban --fixture default
go run ./cmd/ui-preview --screen calendar --fixture crowded
go run ./cmd/ui-preview --screen gantt --fixture dependencies
```

Objetivos:

- Revisar la UI antes de completar toda la persistencia.
- Reproducir estados vacíos, cargando, con errores y con muchos datos.
- Facilitar cambios visuales sin depender de la lógica funcional.
- Generar capturas o grabaciones reproducibles para cada revisión.

## 4. Decisiones técnicas principales

### 4.1 Archivos `.tasks`

Cada archivo será una base SQLite completa. Se configurará, como mínimo:

- `foreign_keys=ON`.
- `busy_timeout` limitado.
- Transacciones cortas.
- Modo de journal `DELETE`, evitando los archivos persistentes `-wal` y `-shm`.
- Nivel de sincronización seguro.
- Rechazo explícito si la base tiene una versión de esquema más nueva que la aplicación.

Las pruebas usarán archivos SQLite temporales reales, no solo `:memory:`, para reproducir reaperturas, bloqueos y sidecars.

### 4.2 Modelo de fechas

Se implementará un tipo de dominio `Date` sin hora ni zona horaria, persistido como `YYYY-MM-DD`. Toda comparación, recurrencia y validación de inicio/vencimiento utilizará ese tipo.

El acceso a la fecha actual se hará mediante un puerto `Clock`, sustituible por un reloj falso en pruebas.

### 4.3 Índice global

El índice será una pequeña base SQLite en el directorio de datos de usuario. Solo almacenará una tabla con rutas absolutas únicas. Al entrar en modo global se eliminarán silenciosamente las rutas inexistentes.

No se usarán copias de tareas ni relaciones entre proyectos.

### 4.4 Consultas globales

No se adjuntarán todos los proyectos mediante `ATTACH DATABASE`, porque existe un límite en el número de bases adjuntas. La aplicación:

1. Abrirá cada archivo registrado de forma independiente.
2. Consultará y agregará resultados en memoria.
3. Mantendrá en cada resultado la ruta/proyecto de origen.
4. Enviará cada mutación únicamente a la base de origen.

La identidad de una tarea global será la combinación de proyecto y ID interno.

### 4.5 Concurrencia

Además del bloqueo de SQLite:

- Las mutaciones incluirán control optimista mediante una versión de fila.
- Una edición obsoleta devolverá un error de conflicto y obligará a refrescar.
- Las escrituras de un mismo proyecto se serializarán desde la capa de aplicación.
- No se mantendrán transacciones abiertas mientras el usuario completa un formulario.

### 4.6 Historial

Cada mutación y su evento de historial se guardarán dentro de la misma transacción. El historial será append-only desde los casos de uso y no tendrá API de edición.

### 4.7 Recurrencia y mantenimiento

La recurrencia será un módulo propio, limitado a las reglas de `SPEC.md`. No se introducirá un motor cron o RRULE general.

El mantenimiento se ejecutará:

- Al abrir cada base.
- Cuando cambie la fecha mientras la aplicación está abierta.
- Antes de devolver datos si se detecta un cambio de día pendiente.

En una única transacción se procesarán reinicios recurrentes y depuración de la papelera.

## 5. Fases de implementación

### Fase 0 — Contratos y diseño verificable

Entregables:

- Inicialización del módulo Go.
- Estructura de paquetes.
- Interfaces entre TUI y aplicación.
- Tipos de dominio iniciales.
- Convenciones de errores tipados.
- Fixtures iniciales para `ui-preview`.
- Decisiones técnicas breves en `docs/adr/`.

Pruebas:

- Compilación de todos los paquetes.
- Comprobaciones de dependencias entre capas.
- Pruebas de los tipos `Date`, prioridad, modo y capacidades.

Criterio de salida: la TUI falsa y el núcleo vacío pueden compilar de forma independiente.

### Fase 1 — Descubrimiento, inicialización y almacenamiento

Implementar:

- Búsqueda ascendente de `*.tasks`.
- Detección de conflictos en el mismo directorio.
- `tasks init nombre.tasks`.
- Validación de extensión y no sobrescritura.
- Creación del esquema inicial.
- Registro automático de proyectos.
- Selección de modo local/global.
- Apertura, validación y migración de bases.

Pruebas:

- Directorio actual, padres y raíz.
- Cero, uno y varios archivos `.tasks`.
- Archivos con espacios y Unicode.
- Rutas relativas, absolutas y enlaces simbólicos.
- `init` sobre un nombre inválido, archivo existente o directorio conflictivo.
- Registro al crear y al abrir.
- Limpieza silenciosa de rutas inexistentes.
- Migración desde cada versión histórica disponible.
- Fallo ante una versión futura o una base corrupta.

Criterio de salida: el ejecutable puede decidir el modo y crear/reabrir un proyecto portable.

### Fase 2 — Núcleo local de tareas

Implementar:

- Estados normales y especiales.
- Estado inicial único.
- CRUD de tareas.
- Prioridad, fechas y documentación.
- Finalizar, cancelar y reabrir.
- Historial transaccional.
- Búsqueda, filtros y ordenamientos básicos.
- Papelera y restauración, inicialmente sin dependencias.

Pruebas unitarias:

- Invariantes de estados.
- Estado inicial obligatorio y único.
- Estado inicial no eliminable sin reemplazo.
- Estado con tareas no eliminable sin destino.
- Fecha de vencimiento no anterior a inicio.
- Estados especiales inmutables.
- Restricciones de creación según modo.
- Cada comando produce exactamente los eventos esperados.

Pruebas de integración:

- Commit y rollback completos.
- Reapertura del archivo conservando datos.
- Ediciones concurrentes y conflicto de versión.
- Consultas combinadas de filtros y ordenamiento.

Criterio de salida: todas las operaciones locales pueden usarse sin TUI mediante tests de casos de uso.

### Fase 3 — Primera revisión de UI: shell, Kanban y detalle

Implementar primero contra el backend falso:

- Modelo raíz y navegación.
- Barra de contexto, ayuda y mensajes.
- Gestión de foco y overlays.
- Kanban adaptable al tamaño de terminal.
- Panel de detalle y fragmento Markdown.
- Estados vacío, cargando, error y conflicto.
- Indicadores visuales de prioridad, recurrencia y bloqueo.
- Tema y mapa de teclas centralizados.

Después conectar con la aplicación real:

- Carga del proyecto.
- Movimiento entre estados.
- Creación/edición mediante formularios.
- Confirmaciones y errores.
- Editor Markdown externo.

Revisión de UI obligatoria antes de avanzar a las demás vistas. Los comentarios visuales se resolverán dentro de `internal/tui` y `ui-preview`, sin cambiar reglas de dominio salvo que el comportamiento solicitado lo requiera.

Criterio de salida: flujo local usable de principio a fin desde Kanban.

### Fase 4 — Subtareas, dependencias y bloqueo automático

Implementar:

- Subtareas de un solo nivel.
- Reglas de finalización, reapertura y cancelación.
- Dependencias entre tareas del mismo proyecto.
- Detección de ciclos directos e indirectos.
- Bloqueo automático independiente del estado manual.
- Advertencia y eliminación de relaciones al enviar a papelera.
- Registro completo en historial.

Pruebas:

- Cero, una y múltiples subtareas.
- Finalización automática solo con dos o más subtareas.
- Propagación al finalizar, cancelar y reabrir.
- Ciclos de longitud 1, 2 y varios nodos.
- Dependencias completadas, canceladas, reabiertas y eliminadas.
- Restauración sin recuperar relaciones borradas.
- Rollback si falla cualquier parte de la operación.

Revisión de UI:

- Editor de subtareas.
- Selector de dependencias.
- Representación de bloqueo y tareas afectadas.
- Diálogos de confirmación.

### Fase 5 — Recurrencia y cambio de día

Implementar:

- Reglas diarias.
- Semanales con varios días.
- Mensuales por día.
- Último día del mes.
- Mensuales por ordinal y día de semana.
- Reinicio de tarea y subtareas.
- Registro de ciclos completados/no completados y ciclos omitidos.
- Procesamiento al abrir y al cambiar el día.

Pruebas unitarias exhaustivas:

- Meses de 28, 29, 30 y 31 días.
- Años bisiestos.
- Primer, segundo, tercer, cuarto y último día de semana.
- Cruces de mes y año.
- Varias recurrencias omitidas.
- Tarea ya situada en el ciclo vigente.
- Conservación de documentación, prioridad y dependencias.
- Exclusión de fechas en tareas recurrentes.

Se añadirán fuzz tests para el cálculo de próximas ocurrencias, comprobando que siempre avanza y produce fechas válidas.

### Fase 6 — Tabla, calendario y Gantt locales

Orden recomendado:

1. Tabla.
2. Calendario.
3. Gantt.

Cada vista se desarrollará primero en `ui-preview` y tendrá su propio punto de revisión visual.

Implementar:

- Presentadores independientes por vista.
- Búsqueda, filtros y ordenamiento compartidos.
- Calendario adaptable y navegación por periodo.
- Gantt con intervalos, hitos y dependencias.
- Exclusión correcta de recurrentes y tareas sin fechas.
- Persistencia de preferencias solo si se acuerda explícitamente.

Pruebas:

- Transformación de tareas a modelos de cada vista.
- Límites de periodos y rangos.
- Intervalos de un día e hitos.
- Dependencias fuera de la ventana visible.
- Ausencia de tareas recurrentes donde corresponda.
- Renderizado con títulos largos, Unicode y grandes cantidades de datos.

### Fase 7 — Modo global

Implementar:

- Agregación desde proyectos registrados.
- Limpieza de rutas desaparecidas.
- Identificación visual del proyecto.
- Tabla, calendario y Gantt globales.
- Escritura dirigida al proyecto de origen.
- Restricciones de creación en todos los niveles.
- Filtros por proyecto.

Pruebas:

- Proyectos con nombres iguales y rutas distintas.
- IDs de tarea iguales en bases distintas.
- Proyectos eliminados durante la ejecución.
- Un proyecto bloqueado, corrupto o con versión incompatible.
- Modificación de una tarea en la base correcta.
- Intentos de creación rechazados incluso sin pasar por la TUI.
- Agregación y ordenamiento deterministas.

Revisión de UI:

- Diferenciación visual de proyectos.
- Selector/filtro de proyecto.
- Tratamiento de proyectos temporalmente no disponibles.
- Ausencia de Kanban y acciones de creación.

### Fase 8 — Papelera completa, búsqueda Markdown y administración

Implementar:

- Vista de papelera.
- Depuración automática a los 30 días.
- Búsqueda explícita dentro de Markdown.
- Administración de estados.
- Historial completo de tarea.
- Mensajes de conflicto y recuperación.

Pruebas:

- Día anterior, exacto y posterior al límite de 30 días.
- Depuración idempotente.
- Restauración antes del vencimiento.
- Búsqueda de Markdown separada de búsqueda por título.
- Renombrado y reordenamiento de estados.
- Traslado obligatorio al eliminar un estado con tareas.

### Fase 9 — Endurecimiento y lanzamiento

Implementar/verificar:

- Manejo de señales y restauración de la terminal.
- Redimensionamiento continuo.
- Errores de permisos y disco lleno.
- Bases bloqueadas por otro proceso.
- Logs de diagnóstico.
- Empaquetado para Linux y macOS.
- Documentación de instalación y uso.

Criterio de salida: todos los requisitos de `SPEC.md` están enlazados con al menos una prueba automática o un caso de validación manual.

## 6. Estrategia de testing

### 6.1 Pirámide de pruebas

#### Pruebas unitarias de dominio

Serán la mayor parte y no abrirán SQLite ni terminales. Usarán tablas de casos para:

- Estados y transiciones.
- Subtareas.
- Dependencias y ciclos.
- Fechas.
- Recurrencias.
- Papelera.
- Capacidades local/global.

Se inyectarán reloj e identificadores deterministas.

#### Pruebas de casos de uso

Ejecutarán la capa `application` con puertos falsos para comprobar:

- Secuencia de operaciones.
- Errores tipados.
- Autorización según modo.
- Eventos de historial.
- Rollback solicitado ante errores.

#### Pruebas de integración SQLite

Usarán un directorio temporal y archivos `.tasks` reales. Cubrirán:

- Esquema y restricciones.
- Migraciones.
- Transacciones.
- Bloqueos.
- Control de concurrencia.
- Reapertura y portabilidad.
- Ausencia de datos fuera del `.tasks` una vez cerrada la base.

#### Pruebas de UI

Se dividirán en:

1. **Update tests**: secuencias de mensajes y teclas sobre modelos Bubble Tea.
2. **Presenter tests**: DTO de aplicación convertido a view model.
3. **Golden tests**: salida renderizada para tamaños fijos, normalizando ANSI cuando sea necesario.
4. **Component tests**: foco, validación de formularios, diálogos y listas.
5. **Preview manual**: revisión visual con fixtures documentados.

Tamaños mínimos de prueba:

- `90x40`, terminal mínima soportada.
- `120x40`.
- Pantalla ancha.
- Redimensionamientos sucesivos durante una interacción, sin bajar de `90x40`.

Los golden tests no sustituirán las aserciones de estado. Su actualización requerirá una acción explícita para evitar aceptar regresiones visuales accidentalmente.

#### Pruebas end-to-end

Se ejecutará el binario en un pseudo-terminal y con `HOME`/directorios de datos temporales. Flujos principales:

- `init` -> crear tarea -> editar -> finalizar -> reabrir.
- Crear subtareas y completar la principal.
- Crear dependencias y observar bloqueo.
- Editar Markdown con un editor falso.
- Cerrar, reabrir y comprobar persistencia.
- Entrar en modo global y editar la tarea correcta.
- Eliminar, restaurar y depurar papelera.
- Redimensionar mientras hay un diálogo abierto.

El editor falso será un script controlado por la prueba que modifica el temporal y termina con códigos de éxito o error.

### 6.2 Pruebas no funcionales

- `go test -race` al menos en Linux.
- Benchmark de carga local con miles de tareas.
- Benchmark global con decenas de proyectos.
- Medición de renderizado de Kanban, tabla, calendario y Gantt.
- Pruebas con títulos y Markdown Unicode.
- Pruebas de apertura concurrente desde dos procesos.
- Verificación en Linux y macOS mediante CI.

Los presupuestos exactos de rendimiento se fijarán después del primer prototipo, usando datos representativos en lugar de optimizaciones prematuras.

### 6.3 Matriz de CI

En cada cambio:

- Formato.
- Compilación.
- Análisis estático.
- Unit tests.
- Integration tests rápidos.

En la rama principal o antes de publicar:

- Suite completa.
- Detector de carreras.
- End-to-end en pseudo-terminal.
- Migraciones desde todas las versiones conservadas.
- Linux y macOS.
- Construcción de binarios sin CGO.

## 7. Proceso específico para cambios de UI

Cada cambio deberá conservar el contrato de interacción descrito en `docs/ui-ux.md`: contexto visible, pie contextual completo sin depender de `F1`, selección perceptible, operaciones sin IDs manuales y viewport seguro desde `90x40`.

Cada pantalla seguirá este flujo:

1. Definir fixtures: vacío, normal, saturado, error y loading.
2. Implementar en `ui-preview` usando el backend falso.
3. Añadir tests de modelo, presentador y snapshots.
4. Realizar revisión visual y recoger comentarios.
5. Ajustar layout, tema, textos y navegación únicamente en la capa TUI.
6. Conectar la pantalla al caso de uso real.
7. Añadir una prueba end-to-end del flujo principal.

Los elementos de diseño que probablemente cambien estarán centralizados:

- Colores y atributos en `theme`.
- Atajos y ayuda en `keymap`.
- Formato de prioridad/estado/proyecto en `presenter`.
- Barras, diálogos, formularios y selectores en `components`.
- Reglas de layout dentro de cada pantalla.

No se persistirá directamente estado visual en las entidades de dominio. Si más adelante se desean preferencias de UI, se guardarán en una configuración separada del modelo de tareas.

## 8. Puntos de revisión

Se proponen revisiones explícitas tras estos hitos:

1. **Arquitectura y contratos**: antes de crear el esquema definitivo.
2. **Shell y Kanban en `ui-preview`**: primera revisión visual.
3. **Flujo local integrado**: primera versión utilizable.
4. **Subtareas y dependencias**: revisión de interacciones complejas.
5. **Tabla**.
6. **Calendario**.
7. **Gantt**.
8. **Modo global**.
9. **Release candidate**: recorrido completo de `SPEC.md`.

## 9. Riesgos y mitigaciones

### Gantt y calendario en terminal

Son los widgets visualmente más costosos. Se desarrollarán después de estabilizar componentes, tema y navegación, siempre con fixtures densos y terminales pequeñas.

### Recurrencias mensuales

Concentran muchos casos límite. El cálculo se mantendrá como función pura con pruebas exhaustivas, fuzzing y reloj inyectado.

### Múltiples bases en modo global

Puede afectar tiempo de inicio y manejo de errores parciales. Se evitará `ATTACH`, se limitará la concurrencia de lecturas y cada resultado conservará explícitamente su origen.

### Concurrencia y datos obsoletos

SQLite evita corrupción, pero no por sí solo las ediciones perdidas. Se combinarán transacciones cortas, versión de fila y mensajes de conflicto recuperables desde la TUI.

### Acoplamiento accidental de la UI

El ejecutable `ui-preview`, los backends falsos y la prohibición de importar paquetes TUI desde el núcleo harán visible cualquier acoplamiento temprano.

## 10. Definición de terminado

Una funcionalidad se considerará terminada cuando:

- Su regla esté implementada fuera de la TUI.
- Tenga pruebas unitarias o de integración según corresponda.
- Sus escrituras y eventos de historial sean atómicos.
- La TUI contemple éxito, vacío, carga, error y conflicto aplicables.
- Funcione tras redimensionar la terminal.
- No rompa modo local ni global.
- Esté cubierta por un flujo end-to-end si es una operación principal.
- Su requisito de `SPEC.md` esté marcado en la matriz de trazabilidad del release.
