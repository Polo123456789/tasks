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
	InspectorVisible    bool
	InspectorFocused    bool
	InspectorExpanded   bool
	InspectorPinned     bool
	HasUndo             bool
}

// Footer returns one logical group per line. The app wraps these lines to the
// terminal width and reserves their rendered height before drawing the body.
func Footer(context Context) string {
	lines := []string{"ACCIONES"}
	if context.HasUndo {
		lines = append(lines, "Deshacer   U revertir el último cambio compatible")
	}
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
		if context.InspectorVisible {
			navigation += " · Tab/Shift+Tab cambiar panel"
			if context.InspectorFocused {
				navigation = "Navegar    ↑/↓ recorrer inspector · Tab/Shift+Tab volver a la vista"
			}
		}
		if context.View == 2 || context.View == 3 {
			navigation += " · PgUp/PgDn cambiar mes"
		}
		if context.View == 3 {
			navigation += " · ,/. desplazar 7 días"
		}
		lines = append(lines, navigation, "Sistema    r recargar · q salir")
		if context.InspectorVisible {
			inspector := "Inspector  I normal/expandir/ocultar · Espacio fijar"
			if context.InspectorPinned {
				inspector += "/liberar"
			}
			if context.InspectorFocused {
				inspector += " · Enter actuar sobre la fila enfocada"
			} else {
				inspector += " · Tab enfocar"
			}
			lines = append(lines, inspector)
		}
		if context.CanCreateTask {
			lines = append(lines, "Crear      n nueva tarea (formulario) · N captura compacta")
		}
		if context.HasTask {
			lines = append(lines,
				"Tarea      e título+campos · p prioridad · [/] estado · f finalizar · C cancelar · z reabrir",
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
			if context.HasSubtask && context.InspectorVisible {
				subtask := "Subtarea   E renombrar · t completar/reabrir · {/} cambiar estado · Tab enfocar Inspector; ↑/↓ seleccionar"
				if context.InspectorFocused {
					subtask = "Subtarea   E renombrar · t completar/reabrir · {/} cambiar estado · ↑/↓ seleccionar"
				}
				lines = append(lines, subtask)
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
	lines = append(lines, "Paleta     Ctrl+P buscar y ejecutar comandos", "Ayuda      F1 mapa general (opcional)")
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
		"  ←/→ o h/l  cambiar vista       Tab/Shift+Tab  cambiar panel",
		"  En Vista: ↑/↓ o j/k seleccionar tarea    En Inspector: ↑/↓ o j/k recorrer",
		"  I normal/expandir/ocultar inspector      Espacio fijar/liberar inspector",
		"  PgUp/PgDn  cambiar mes          F1 o Esc  cerrar ayuda       q salir",
		"",
		"Tarea seleccionada",
		"  n formulario completo   N captura compacta   e editar formulario   p prioridad",
		"  s inicio      v vencimiento",
		"  m Markdown   [/] estado    f finalizar      C cancelar    z reabrir",
		"  d papelera   H historial",
		"",
		"Subtareas y dependencias",
		"  a crear      E renombrar      ↑/↓ con foco seleccionar      t completar/reabrir",
		"  {/} mover estado      g agregar dependencia      G eliminar dependencia",
		"",
		"Búsqueda, filtros y orden",
		"  / título     ? Markdown     P origen       S estado     D fechas",
		"  1 prioridad    B bloqueadas    R recurrentes    F alternar finalizadas",
		"  X alternar canceladas      o ordenar      0 limpiar filtros",
		"",
		"Otras acciones",
		"  U deshacer        c recurrencia     u restaurar     r recargar     ,/. desplazar Gantt",
		"  Estados: a crear, e renombrar, i inicial, [/] ordenar, d eliminar",
		"  Ctrl+P abrir paleta contextual de comandos",
	}
	return strings.Join(sections, "\n")
}
