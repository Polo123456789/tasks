package keymap

import "strings"

// Context describes the actions that are valid in the UI state currently on
// screen. Footer deliberately contains the complete contextual map: F1 remains
// useful as a reference, but is not required to discover an available action.
type Context struct {
	View                int
	Global              bool
	HasTask             bool
	HasSubtask          bool
	HasDependency       bool
	NormalStatus        bool
	Recurring           bool
	CanCreateTask       bool
	CanCreateSubtask    bool
	CanCreateDependency bool
	CanCreateRecurrence bool
}

// Footer returns one logical group per line. The app wraps these lines to the
// terminal width and reserves their rendered height before drawing the body.
func Footer(context Context) string {
	lines := []string{"ACCIONES"}
	switch context.View {
	case 4:
		lines = append(lines, "Navegar    ←/→ cambiar vista · ↑/↓ seleccionar · r recargar · q salir")
		if context.HasTask {
			lines = append(lines, "Papelera   u restaurar la tarea seleccionada")
		}
	case 5:
		lines = append(lines, "Navegar    ←/→ cambiar vista · ↑/↓ seleccionar · r recargar · q salir")
		lines = append(lines, "Estados    a crear estado")
		if context.NormalStatus {
			lines = append(lines, "Seleccionado e renombrar · i hacer inicial · [/] reordenar · d eliminar y elegir destino")
		}
	default:
		navigation := "Navegar    ←/→ cambiar vista · ↑/↓ seleccionar tarea"
		if context.View == 2 || context.View == 3 {
			navigation += " · PgUp/PgDn cambiar mes"
		}
		if context.View == 3 {
			navigation += " · ,/. desplazar 7 días"
		}
		lines = append(lines, navigation+" · r recargar · q salir")
		if context.CanCreateTask {
			lines = append(lines, "Crear      n nueva tarea")
		}
		if context.HasTask {
			lines = append(lines,
				"Tarea      e título · p prioridad · [/] estado · f finalizar · C cancelar · z reabrir",
				"Contenido  s inicio · v vencimiento · m Markdown · d papelera · H historial",
			)
			if context.CanCreateRecurrence || context.Recurring {
				lines[len(lines)-1] += " · c recurrencia"
			}
			relations := ""
			if context.CanCreateSubtask {
				relations = "Relaciones a añadir subtarea"
			}
			if context.CanCreateDependency {
				if relations == "" {
					relations = "Relaciones"
				}
				relations += " · g agregar dependencia"
			}
			if context.HasDependency {
				if relations == "" {
					relations = "Relaciones"
				}
				relations += " · G quitar dependencia"
			}
			if relations != "" {
				lines = append(lines, relations)
			}
			if context.HasSubtask {
				lines = append(lines, "Subtarea   J/K seleccionar · E renombrar · t completar/reabrir · {/} cambiar estado")
			}
		}
		filters := "Búsqueda   / título · ? Markdown"
		if context.Global {
			filters += " · P origen"
		}
		lines = append(lines,
			filters+" · S estado · D fechas",
			"Filtros    1 prioridad · B bloqueadas · R recurrentes · o ordenar · 0 limpiar",
			"Visibilidad F mostrar/ocultar finalizadas · X mostrar/ocultar canceladas",
		)
	}
	lines = append(lines, "Ayuda      F1 mapa general (opcional)")
	return strings.Join(lines, "\n")
}

func Full(global bool) string {
	mode := "Modo local: permite crear tareas, subtareas, dependencias, recurrencias y estados."
	if global {
		mode = "Modo global: crea tareas propias; los proyectos registrados no reciben elementos nuevos."
	}
	sections := []string{
		"AYUDA DE TASKS",
		mode,
		"",
		"Navegación",
		"  ←/→ o h/l  cambiar vista       ↑/↓ o j/k  seleccionar",
		"  PgUp/PgDn  cambiar mes          F1 o Esc  cerrar ayuda       q salir",
		"",
		"Tarea seleccionada",
		"  n crear      e título      p prioridad      s inicio      v vencimiento",
		"  m Markdown   [/] estado    f finalizar      C cancelar    z reabrir",
		"  d papelera   H historial",
		"",
		"Subtareas y dependencias",
		"  a crear      E renombrar      J/K seleccionar      t completar/reabrir",
		"  {/} mover estado      g agregar dependencia      G eliminar dependencia",
		"",
		"Búsqueda, filtros y orden",
		"  / título     ? Markdown     P origen       S estado     D fechas",
		"  1 prioridad    B bloqueadas    R recurrentes    F alternar finalizadas",
		"  X alternar canceladas      o ordenar      0 limpiar filtros",
		"",
		"Otras acciones",
		"  c recurrencia     u restaurar     r recargar     ,/. desplazar Gantt",
		"  Estados: a crear, e renombrar, i inicial, [/] ordenar, d eliminar",
	}
	return strings.Join(sections, "\n")
}
