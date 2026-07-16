package keymap

import "strings"

// Short keeps the persistent footer intentionally compact. The complete map
// lives behind F1 so it remains reachable in an 80-column terminal.
func Short(view int, global bool) string {
	switch view {
	case 0:
		return "n nueva · e título · [/] estado · f fin · C cancelar · z reabrir · F1 ayuda · q"
	case 1:
		return "e título · f fin · C cancelar · z reabrir · / buscar · F1 ayuda · q"
	case 2, 3:
		period := "PgUp/PgDn mes · "
		if view == 3 {
			period += ",/. días · "
		}
		return period + "↑/↓ tarea · e editar · F1 ayuda · q salir"
	case 4:
		return "↑/↓ seleccionar · u restaurar · F1 ayuda · q salir"
	case 5:
		return "a crear · e renombrar · i inicial · [/] ordenar · d eliminar · F1 ayuda · q salir"
	default:
		return "F1 ayuda · ←/→ vista · ↑/↓ seleccionar · q salir"
	}
}

func Full(global bool) string {
	mode := "Modo local: permite crear tareas, subtareas, dependencias, recurrencias y estados."
	if global {
		mode = "Modo global: permite editar, pero no crear elementos."
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
		"  / título     ? Markdown     P proyecto     S estado     D fechas",
		"  1 prioridad    B bloqueadas    R recurrentes    F alternar finalizadas",
		"  X alternar canceladas      o ordenar      0 limpiar filtros",
		"",
		"Otras acciones",
		"  c recurrencia     u restaurar     r recargar     ,/. desplazar Gantt",
		"  Estados: a crear, e renombrar, i inicial, [/] ordenar, d eliminar",
	}
	return strings.Join(sections, "\n")
}
