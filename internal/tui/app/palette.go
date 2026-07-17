package app

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/tui/screens/listutil"
	"github.com/Polo123456789/tasks/internal/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

type paletteRequirement int

const (
	paletteAlways paletteRequirement = iota
	paletteSelectNext
	paletteSelectPrevious
	paletteTaskView
	paletteMonthView
	paletteGanttView
	paletteInspector
	paletteTask
	paletteCreateTask
	paletteSubtask
	paletteSubtaskNext
	paletteSubtaskPrevious
	paletteCreateSubtask
	paletteCreateDependency
	paletteRemoveDependency
	paletteRecurrence
	paletteGlobal
	paletteTrashTask
	paletteSettings
	paletteNormalStatus
	paletteNormalStatusLeft
	paletteNormalStatusRight
)

type paletteAction struct {
	Name        string
	Description string
	Synonyms    string
	Shortcut    string
	Key         string
	Requirement paletteRequirement
}

type paletteEntry struct {
	Action  paletteAction
	Enabled bool
	Reason  string
}

var paletteCatalog = []paletteAction{
	{Name: "Vista siguiente", Description: "Cambiar al siguiente panel principal", Synonyms: "navegar pantalla derecha", Shortcut: "→ / l", Key: "l", Requirement: paletteAlways},
	{Name: "Vista anterior", Description: "Cambiar al panel principal anterior", Synonyms: "navegar pantalla izquierda", Shortcut: "← / h", Key: "h", Requirement: paletteAlways},
	{Name: "Seleccionar siguiente", Description: "Mover la selección una fila hacia abajo", Synonyms: "navegar tarea estado elemento", Shortcut: "↓ / j", Key: "j", Requirement: paletteSelectNext},
	{Name: "Seleccionar anterior", Description: "Mover la selección una fila hacia arriba", Synonyms: "navegar tarea estado elemento", Shortcut: "↑ / k", Key: "k", Requirement: paletteSelectPrevious},
	{Name: "Cambiar foco de panel", Description: "Alternar entre la vista principal y el inspector", Synonyms: "tabulador panel detalle", Shortcut: "Tab", Key: "tab", Requirement: paletteInspector},
	{Name: "Cambiar tamaño del inspector", Description: "Alternar normal, expandido y oculto", Synonyms: "detalle mostrar esconder ampliar", Shortcut: "I", Key: "I", Requirement: paletteTask},
	{Name: "Fijar o liberar inspector", Description: "Conservar su disposición al cambiar de vista", Synonyms: "detalle pin anclar", Shortcut: "Espacio", Key: "space", Requirement: paletteInspector},
	{Name: "Mes anterior", Description: "Mostrar el periodo mensual anterior", Synonyms: "calendario gantt fecha", Shortcut: "PgUp", Key: "pgup", Requirement: paletteMonthView},
	{Name: "Mes siguiente", Description: "Mostrar el periodo mensual siguiente", Synonyms: "calendario gantt fecha", Shortcut: "PgDn", Key: "pgdown", Requirement: paletteMonthView},
	{Name: "Gantt siete días atrás", Description: "Desplazar la ventana temporal hacia atrás", Synonyms: "cronograma fecha izquierda", Shortcut: ",", Key: ",", Requirement: paletteGanttView},
	{Name: "Gantt siete días adelante", Description: "Desplazar la ventana temporal hacia adelante", Synonyms: "cronograma fecha derecha", Shortcut: ".", Key: ".", Requirement: paletteGanttView},
	{Name: "Recargar", Description: "Volver a leer las tareas del almacenamiento", Synonyms: "refrescar actualizar sincronizar", Shortcut: "r", Key: "r", Requirement: paletteAlways},
	{Name: "Nueva tarea", Description: "Crear una tarea en el origen escribible", Synonyms: "añadir agregar capturar", Shortcut: "n", Key: "n", Requirement: paletteCreateTask},
	{Name: "Editar título", Description: "Modificar el título de la tarea seleccionada", Synonyms: "renombrar cambiar nombre", Shortcut: "e", Key: "e", Requirement: paletteTask},
	{Name: "Cambiar prioridad", Description: "Avanzar la prioridad de la tarea seleccionada", Synonyms: "urgencia importancia", Shortcut: "p", Key: "p", Requirement: paletteTask},
	{Name: "Estado anterior", Description: "Mover la tarea al estado previo", Synonyms: "columna flujo izquierda", Shortcut: "[", Key: "[", Requirement: paletteTask},
	{Name: "Estado siguiente", Description: "Mover la tarea al estado posterior", Synonyms: "columna flujo derecha", Shortcut: "]", Key: "]", Requirement: paletteTask},
	{Name: "Finalizar tarea", Description: "Marcar la tarea seleccionada como finalizada", Synonyms: "completar terminar cerrar", Shortcut: "f", Key: "f", Requirement: paletteTask},
	{Name: "Cancelar tarea", Description: "Marcar la tarea seleccionada como cancelada", Synonyms: "descartar anular cerrar", Shortcut: "C", Key: "C", Requirement: paletteTask},
	{Name: "Reabrir tarea", Description: "Devolver la tarea al estado inicial", Synonyms: "restablecer pendiente", Shortcut: "z", Key: "z", Requirement: paletteTask},
	{Name: "Editar inicio", Description: "Cambiar o limpiar la fecha de inicio", Synonyms: "fecha planificación comienzo", Shortcut: "s", Key: "s", Requirement: paletteTask},
	{Name: "Editar vencimiento", Description: "Cambiar o limpiar la fecha límite", Synonyms: "fecha planificación entrega due", Shortcut: "v", Key: "v", Requirement: paletteTask},
	{Name: "Editar Markdown", Description: "Abrir la documentación de la tarea en el editor", Synonyms: "notas descripción contenido", Shortcut: "m", Key: "m", Requirement: paletteTask},
	{Name: "Configurar recurrencia", Description: "Crear, cambiar o quitar una repetición", Synonyms: "repetir periodicidad ciclo", Shortcut: "c", Key: "c", Requirement: paletteRecurrence},
	{Name: "Enviar a papelera", Description: "Eliminar temporalmente la tarea seleccionada", Synonyms: "borrar descartar trash", Shortcut: "d", Key: "d", Requirement: paletteTask},
	{Name: "Ver historial", Description: "Mostrar los eventos de la tarea seleccionada", Synonyms: "auditoría cambios eventos", Shortcut: "H", Key: "H", Requirement: paletteTask},
	{Name: "Añadir subtarea", Description: "Crear una subtarea bajo la tarea seleccionada", Synonyms: "agregar hija checklist", Shortcut: "a", Key: "a", Requirement: paletteCreateSubtask},
	{Name: "Seleccionar subtarea siguiente", Description: "Mover la selección del inspector hacia abajo", Synonyms: "hija detalle navegar", Shortcut: "J", Key: "J", Requirement: paletteSubtaskNext},
	{Name: "Seleccionar subtarea anterior", Description: "Mover la selección del inspector hacia arriba", Synonyms: "hija detalle navegar", Shortcut: "K", Key: "K", Requirement: paletteSubtaskPrevious},
	{Name: "Renombrar subtarea", Description: "Modificar el título de la subtarea seleccionada", Synonyms: "editar hija nombre", Shortcut: "E", Key: "E", Requirement: paletteSubtask},
	{Name: "Alternar subtarea", Description: "Completar o reabrir la subtarea seleccionada", Synonyms: "marcar checklist terminar", Shortcut: "t", Key: "t", Requirement: paletteSubtask},
	{Name: "Estado anterior de subtarea", Description: "Mover la subtarea al estado previo", Synonyms: "hija columna flujo", Shortcut: "{", Key: "{", Requirement: paletteSubtask},
	{Name: "Estado siguiente de subtarea", Description: "Mover la subtarea al estado posterior", Synonyms: "hija columna flujo", Shortcut: "}", Key: "}", Requirement: paletteSubtask},
	{Name: "Añadir dependencia", Description: "Elegir una tarea que debe completarse antes", Synonyms: "agregar bloqueo requisito", Shortcut: "g", Key: "g", Requirement: paletteCreateDependency},
	{Name: "Quitar dependencia", Description: "Eliminar una dependencia existente", Synonyms: "desbloquear requisito", Shortcut: "G", Key: "G", Requirement: paletteRemoveDependency},
	{Name: "Buscar por título", Description: "Filtrar tareas por texto del título", Synonyms: "encontrar nombre", Shortcut: "/", Key: "/", Requirement: paletteTaskView},
	{Name: "Buscar en Markdown", Description: "Filtrar tareas por texto de su documentación", Synonyms: "encontrar notas contenido descripción", Shortcut: "?", Key: "?", Requirement: paletteTaskView},
	{Name: "Filtrar por origen", Description: "Limitar tareas por proyecto o almacén global", Synonyms: "fuente proyecto ruta", Shortcut: "P", Key: "P", Requirement: paletteGlobal},
	{Name: "Filtrar por estado", Description: "Limitar tareas por su estado actual", Synonyms: "columna flujo", Shortcut: "S", Key: "S", Requirement: paletteTaskView},
	{Name: "Filtrar por fechas", Description: "Limitar tareas por un rango de planificación", Synonyms: "inicio vencimiento periodo", Shortcut: "D", Key: "D", Requirement: paletteTaskView},
	{Name: "Filtrar por prioridad", Description: "Recorrer los niveles de prioridad visibles", Synonyms: "urgencia importancia", Shortcut: "1", Key: "1", Requirement: paletteTaskView},
	{Name: "Alternar bloqueadas", Description: "Mostrar solo tareas bloqueadas o quitar el filtro", Synonyms: "dependencias impedidas", Shortcut: "B", Key: "B", Requirement: paletteTaskView},
	{Name: "Alternar recurrentes", Description: "Mostrar solo tareas recurrentes o quitar el filtro", Synonyms: "repetición ciclo", Shortcut: "R", Key: "R", Requirement: paletteTaskView},
	{Name: "Alternar finalizadas", Description: "Mostrar u ocultar tareas finalizadas", Synonyms: "completadas visibilidad", Shortcut: "F", Key: "F", Requirement: paletteTaskView},
	{Name: "Alternar canceladas", Description: "Mostrar u ocultar tareas canceladas", Synonyms: "anuladas visibilidad", Shortcut: "X", Key: "X", Requirement: paletteTaskView},
	{Name: "Cambiar orden", Description: "Usar el siguiente criterio de ordenamiento", Synonyms: "clasificar ordenar sort", Shortcut: "o", Key: "o", Requirement: paletteTaskView},
	{Name: "Limpiar filtros", Description: "Restablecer búsqueda, filtros y orden", Synonyms: "borrar reset mostrar todo", Shortcut: "0", Key: "0", Requirement: paletteTaskView},
	{Name: "Restaurar tarea", Description: "Sacar la tarea seleccionada de la papelera", Synonyms: "recuperar deshacer eliminación", Shortcut: "u", Key: "u", Requirement: paletteTrashTask},
	{Name: "Crear estado", Description: "Añadir una columna normal al flujo local", Synonyms: "nuevo columna configuración", Shortcut: "a", Key: "a", Requirement: paletteSettings},
	{Name: "Renombrar estado", Description: "Modificar el nombre del estado seleccionado", Synonyms: "editar columna", Shortcut: "e", Key: "e", Requirement: paletteNormalStatus},
	{Name: "Hacer estado inicial", Description: "Usar el estado seleccionado para tareas nuevas", Synonyms: "predeterminado default columna", Shortcut: "i", Key: "i", Requirement: paletteNormalStatus},
	{Name: "Mover estado a la izquierda", Description: "Reordenar el estado seleccionado una posición", Synonyms: "columna anterior", Shortcut: "[", Key: "[", Requirement: paletteNormalStatusLeft},
	{Name: "Mover estado a la derecha", Description: "Reordenar el estado seleccionado una posición", Synonyms: "columna siguiente", Shortcut: "]", Key: "]", Requirement: paletteNormalStatusRight},
	{Name: "Eliminar estado", Description: "Borrar el estado y elegir destino para sus tareas", Synonyms: "quitar columna", Shortcut: "d", Key: "d", Requirement: paletteNormalStatus},
	{Name: "Salir", Description: "Cerrar tasks", Synonyms: "terminar cerrar quit", Shortcut: "q", Key: "q", Requirement: paletteAlways},
}

func (m *Model) openPalette() {
	m.paletteOpen = true
	m.paletteQuery = ""
	m.paletteSelected = 0
	m.paletteNotice = ""
}

func (m *Model) closePalette() {
	m.paletteOpen = false
	m.paletteQuery = ""
	m.paletteSelected = 0
	m.paletteNotice = ""
}

func (m Model) paletteEntries() []paletteEntry {
	queryTerms := strings.Fields(paletteSearchText(m.paletteQuery))
	entries := make([]paletteEntry, 0, len(paletteCatalog))
	for _, action := range paletteCatalog {
		haystack := paletteSearchText(strings.Join([]string{action.Name, action.Description, action.Synonyms, action.Shortcut}, " "))
		matches := true
		for _, term := range queryTerms {
			if !strings.Contains(haystack, term) {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}
		enabled, reason := m.paletteAvailability(action.Requirement)
		entries = append(entries, paletteEntry{Action: action, Enabled: enabled, Reason: reason})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Enabled && !entries[j].Enabled
	})
	return entries
}

func paletteSearchText(value string) string {
	value = strings.ToLower(value)
	value = strings.Map(func(r rune) rune {
		switch r {
		case 'á', 'à', 'ä', 'â':
			return 'a'
		case 'é', 'è', 'ë', 'ê':
			return 'e'
		case 'í', 'ì', 'ï', 'î':
			return 'i'
		case 'ó', 'ò', 'ö', 'ô':
			return 'o'
		case 'ú', 'ù', 'ü', 'û':
			return 'u'
		default:
			return r
		}
	}, value)
	return strings.Join(strings.Fields(value), " ")
}

func (m Model) paletteAvailability(requirement paletteRequirement) (bool, string) {
	hasTask := m.hasSelectedTask()
	hasSubtask := hasTask && m.detail != nil && len(m.detail.Subtasks) > 0
	normalStatus := m.view == 5 && m.selected >= 0 && m.selected < len(m.statuses) && m.statuses[m.selected].Kind == domain.StatusNormal
	switch requirement {
	case paletteAlways:
		return true, ""
	case paletteSelectNext:
		if m.canMoveSelection(1) {
			return true, ""
		}
		return false, "no hay otro elemento visible después de la selección"
	case paletteSelectPrevious:
		if m.canMoveSelection(-1) {
			return true, ""
		}
		return false, "no hay otro elemento visible antes de la selección"
	case paletteTaskView:
		if m.view < 4 {
			return true, ""
		}
		return false, "solo está disponible en vistas de tareas"
	case paletteMonthView:
		if m.view == 2 || m.view == 3 {
			return true, ""
		}
		return false, "solo está disponible en Calendario o Gantt"
	case paletteGanttView:
		if m.view == 3 {
			return true, ""
		}
		return false, "solo está disponible en Gantt"
	case paletteInspector:
		if hasTask && m.inspectorMode != inspectorHidden {
			return true, ""
		}
		return false, "muestra el inspector de una tarea visible"
	case paletteTask:
		if hasTask {
			return true, ""
		}
		return false, "selecciona una tarea visible"
	case paletteCreateTask:
		if m.backend.Capabilities("").CanCreateTask {
			return true, ""
		}
		return false, "el origen escribible no está disponible"
	case paletteSubtask:
		if _, ok := m.focusedSubtask(); ok {
			return true, ""
		}
		return false, "enfoca una subtarea visible en el inspector"
	case paletteSubtaskNext:
		if hasSubtask && m.inspectorMode != inspectorHidden && m.selectedSubtask < len(m.detail.Subtasks)-1 {
			return true, ""
		}
		if m.inspectorMode == inspectorHidden {
			return false, "muestra el inspector para navegar sus subtareas"
		}
		return false, "no hay otra subtarea después de la selección"
	case paletteSubtaskPrevious:
		if hasSubtask && m.inspectorMode != inspectorHidden && m.selectedSubtask > 0 {
			return true, ""
		}
		if m.inspectorMode == inspectorHidden {
			return false, "muestra el inspector para navegar sus subtareas"
		}
		return false, "no hay otra subtarea antes de la selección"
	case paletteCreateSubtask:
		if !hasTask {
			return false, "selecciona una tarea visible"
		}
		if m.backend.Capabilities(m.tasks[m.selected].Source).CanCreateSubtask {
			return true, ""
		}
		return false, "el origen de la tarea no permite añadir subtareas"
	case paletteCreateDependency:
		if !hasTask {
			return false, "selecciona una tarea visible"
		}
		if m.backend.Capabilities(m.tasks[m.selected].Source).CanCreateDependency {
			return true, ""
		}
		return false, "el origen de la tarea no permite añadir dependencias"
	case paletteRemoveDependency:
		if !hasTask {
			return false, "selecciona una tarea visible"
		}
		if m.tasks[m.selected].Dependencies > 0 {
			return true, ""
		}
		return false, "la tarea no tiene dependencias"
	case paletteRecurrence:
		if !hasTask {
			return false, "selecciona una tarea visible"
		}
		if m.tasks[m.selected].Recurring || m.backend.Capabilities(m.tasks[m.selected].Source).CanCreateRecurrence {
			return true, ""
		}
		return false, "el origen de la tarea no permite añadir recurrencia"
	case paletteGlobal:
		if m.view >= 4 {
			return false, "solo está disponible en vistas de tareas"
		}
		if m.backend.Mode() == domain.ModeGlobal {
			return true, ""
		}
		return false, "solo es útil al agregar varios orígenes en modo global"
	case paletteTrashTask:
		if m.view == 4 && m.selected >= 0 && m.selected < len(m.deleted) {
			return true, ""
		}
		return false, "selecciona una tarea en Papelera"
	case paletteSettings:
		if m.view != 5 {
			return false, "solo está disponible en Estados"
		}
		if m.backend.Capabilities("").CanCreateStatus {
			return true, ""
		}
		return false, "los estados solo se administran en modo local"
	case paletteNormalStatus:
		if normalStatus {
			return true, ""
		}
		return false, "selecciona un estado normal en Estados"
	case paletteNormalStatusLeft:
		if normalStatus && m.canReorderSelectedStatus(-1) {
			return true, ""
		}
		return false, "el estado ya es el primero o no es un estado normal"
	case paletteNormalStatusRight:
		if normalStatus && m.canReorderSelectedStatus(1) {
			return true, ""
		}
		return false, "el estado ya es el último o no es un estado normal"
	default:
		return false, "acción no disponible"
	}
}

func (m Model) canMoveSelection(direction int) bool {
	if m.panelFocus == focusInspector && m.inspectorMode != inspectorHidden {
		target := m.inspectorCursor + direction
		return target >= 0 && target < len(m.inspectorRows())
	}
	if m.view == 4 {
		target := m.selected + direction
		return target >= 0 && target < len(m.deleted)
	}
	if m.view == 5 {
		target := m.selected + direction
		return target >= 0 && target < len(m.statuses)
	}
	indices := m.selectableIndices()
	for position, index := range indices {
		if index == m.selected {
			target := position + direction
			return target >= 0 && target < len(indices)
		}
	}
	return false
}

func (m Model) canReorderSelectedStatus(direction int) bool {
	position := -1
	normalCount := 0
	for index, status := range m.statuses {
		if status.Kind != domain.StatusNormal {
			continue
		}
		if index == m.selected {
			position = normalCount
		}
		normalCount++
	}
	target := position + direction
	return position >= 0 && target >= 0 && target < normalCount
}

func (m Model) updatePalette(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "ctrl+p":
		m.closePalette()
		return m, nil
	case "up":
		if m.paletteSelected > 0 {
			m.paletteSelected--
		}
		return m, nil
	case "down":
		if m.paletteSelected+1 < len(m.paletteEntries()) {
			m.paletteSelected++
		}
		return m, nil
	case "backspace":
		runes := []rune(m.paletteQuery)
		if len(runes) > 0 {
			m.paletteQuery = string(runes[:len(runes)-1])
			m.paletteSelected = 0
			m.paletteNotice = ""
		}
		return m, nil
	case "enter":
		entries := m.paletteEntries()
		if len(entries) == 0 {
			return m, nil
		}
		m.paletteSelected = min(m.paletteSelected, len(entries)-1)
		entry := entries[m.paletteSelected]
		if !entry.Enabled {
			m.paletteNotice = "No disponible: " + entry.Reason
			return m, nil
		}
		actionKey := entry.Action.Key
		m.closePalette()
		return m.Update(paletteKeyMessage(actionKey))
	default:
		if len(key.Runes) > 0 {
			m.paletteQuery += string(key.Runes)
			m.paletteSelected = 0
			m.paletteNotice = ""
		}
		return m, nil
	}
}

func paletteKeyMessage(key string) tea.KeyMsg {
	switch key {
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

func (m Model) paletteView(height int) string {
	entries := m.paletteEntries()
	contentWidth := max(1, m.width-4)
	contentHeight := max(1, height-2)
	truncate := func(value string) string { return ansi.Truncate(value, contentWidth, "…") }
	lines := []string{theme.Title.Render(truncate("Paleta de comandos"))}
	if len(lines) < contentHeight {
		lines = append(lines, truncate("Buscar: "+m.paletteQuery+"█"))
	}
	if len(lines) < contentHeight {
		lines = append(lines, theme.Help.Render(truncate("Nombre, descripción o sinónimo · disponibles primero")))
	}
	if m.paletteNotice != "" && len(lines) < contentHeight {
		lines = append(lines, theme.Help.Foreground(theme.Danger).Render(truncate(m.paletteNotice)))
	}
	if len(entries) == 0 {
		if len(lines) < contentHeight {
			lines = append(lines, truncate("Sin comandos coincidentes."))
		}
		return theme.Border.Render(strings.Join(lines, "\n"))
	}
	remaining := contentHeight - len(lines)
	if remaining <= 0 {
		return theme.Border.Render(strings.Join(lines, "\n"))
	}
	selected := min(m.paletteSelected, len(entries)-1)
	entryLimit := remaining
	if len(entries) > remaining && remaining > 1 {
		entryLimit--
	}
	start, end := listutil.Bounds(len(entries), selected, max(1, entryLimit))
	for index := start; index < end; index++ {
		entry := entries[index]
		label := fmt.Sprintf("[%s] %s — %s", entry.Action.Shortcut, entry.Action.Name, entry.Action.Description)
		if !entry.Enabled {
			label = fmt.Sprintf("[%s] %s — No disponible: %s", entry.Action.Shortcut, entry.Action.Name, entry.Reason)
		}
		prefix := "  "
		if index == selected {
			prefix = "› "
			label = theme.Selected.Render(truncate(prefix + label))
		} else if !entry.Enabled {
			label = theme.Help.Render(truncate("× " + label))
		} else {
			label = truncate(prefix + label)
		}
		lines = append(lines, label)
	}
	if outside := start + len(entries) - end; outside > 0 && len(lines) < contentHeight {
		lines = append(lines, truncate(fmt.Sprintf("↕ %d comando(s) fuera de la ventana", outside)))
	}
	return theme.Border.Render(strings.Join(lines, "\n"))
}

func init() {
	// Keep catalog typos from silently making a command impossible to execute.
	for _, action := range paletteCatalog {
		if strings.TrimSpace(action.Name) == "" || strings.TrimSpace(action.Key) == "" {
			panic("invalid command palette action")
		}
		for _, r := range action.Key {
			if unicode.IsControl(r) {
				panic("invalid command palette key")
			}
		}
	}
}
