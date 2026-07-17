package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/ports"
	"github.com/Polo123456789/tasks/internal/tui/keymap"
	"github.com/Polo123456789/tasks/internal/tui/presenter"
	"github.com/Polo123456789/tasks/internal/tui/screens/calendar"
	"github.com/Polo123456789/tasks/internal/tui/screens/gantt"
	historyscreen "github.com/Polo123456789/tasks/internal/tui/screens/history"
	"github.com/Polo123456789/tasks/internal/tui/screens/kanban"
	"github.com/Polo123456789/tasks/internal/tui/screens/listutil"
	"github.com/Polo123456789/tasks/internal/tui/screens/settings"
	"github.com/Polo123456789/tasks/internal/tui/screens/table"
	"github.com/Polo123456789/tasks/internal/tui/screens/taskdetail"
	"github.com/Polo123456789/tasks/internal/tui/screens/trash"
	"github.com/Polo123456789/tasks/internal/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Backend interface {
	Mode() domain.Mode
	ContextLabel() string
	Today() domain.Date
	Maintain(context.Context) error
	Capabilities(string) domain.Capabilities
	List(context.Context, ports.TaskFilter) ([]domain.Task, error)
	Statuses(context.Context) ([]domain.Status, error)
	Create(context.Context, string) (domain.Task, error)
	SaveTask(context.Context, string, domain.Task) (domain.Task, error)
	FormStatuses(context.Context, string) ([]domain.Status, error)
	UpdateTitle(context.Context, string, int64, int64, string) (domain.Task, error)
	MoveStatus(context.Context, string, int64, int64, int) (domain.Task, error)
	SetLifecycle(context.Context, string, int64, int64, string) (domain.Task, error)
	CyclePriority(context.Context, string, int64, int64) (domain.Task, error)
	UpdateDate(context.Context, string, int64, int64, string, *domain.Date) (domain.Task, error)
	Detail(context.Context, string, int64) (domain.Task, error)
	History(context.Context, string, int64) ([]domain.HistoryEvent, error)
	AddSubtask(context.Context, string, int64, int64, string) (domain.Subtask, error)
	RenameSubtask(context.Context, string, int64, int64, int64, string) (domain.Subtask, error)
	ToggleSubtask(context.Context, string, int64, int64, int64) error
	MoveSubtaskStatus(context.Context, string, int64, int64, int64, int) error
	AddDependency(context.Context, string, int64, int64, int64) error
	RemoveDependency(context.Context, string, int64, int64, int64) error
	DependencyCandidates(context.Context, string, int64, bool) ([]domain.Task, error)
	UpdateRecurrence(context.Context, string, int64, int64, *domain.Recurrence) (domain.Task, error)
	CreateStatus(context.Context, string) (domain.Status, error)
	RenameStatus(context.Context, int64, string) error
	SetInitialStatus(context.Context, int64) error
	ReorderStatuses(context.Context, []int64) error
	DeleteStatus(context.Context, int64, *int64) error
	MarkdownEditor(context.Context, string, int64, int64) (tea.ExecCommand, func(error) error, error)
	DependencyImpact(context.Context, string, int64) ([]domain.Task, error)
	Trash(context.Context, string, int64, int64) ([]int64, error)
	Restore(context.Context, string, int64, int64) (domain.Task, error)
}
type loaded struct {
	tasks    []domain.Task
	statuses []domain.Status
	err      error
	trash    bool
}
type created struct {
	task domain.Task
	err  error
}
type mutated struct {
	action string
	err    error
}
type markdownFinished struct{ err error }
type detailLoaded struct {
	task       domain.Task
	history    []domain.HistoryEvent
	err        error
	historyErr error
}
type trashImpact struct {
	task     presenter.Task
	affected []presenter.Task
	err      error
}
type dayCheck struct{}
type maintenanceFinished struct {
	today domain.Date
	err   error
}
type historyLoaded struct {
	events []domain.HistoryEvent
	err    error
}
type pickerOption struct {
	ID    int64
	Label string
	Value string
}
type pickerLoaded struct {
	action  string
	task    presenter.Task
	options []pickerOption
	err     error
}

type panelFocus uint8

const (
	focusMain panelFocus = iota
	focusInspector
)

type inspectorMode uint8

const (
	inspectorNormal inspectorMode = iota
	inspectorExpanded
	inspectorHidden
)

type viewNavigation struct {
	initialized     bool
	selected        int
	selectedID      int64
	selectedSource  string
	inspectorCursor int
	focus           panelFocus
	inspectorMode   inspectorMode
	calendarMonth   time.Time
	ganttStartDay   int
}

type Model struct {
	backend                       Backend
	tasks, deleted                []presenter.Task
	statuses                      []domain.Status
	view, selected, width, height int
	selectedSubtask               int
	panelFocus                    panelFocus
	inspectorCursor               int
	inspectorMode                 inspectorMode
	inspectorPinned               bool
	viewNavigation                [6]viewNavigation
	detail                        *domain.Task
	confirmTrash                  *presenter.Task
	confirmAffected               []presenter.Task
	calendarMonth                 time.Time
	ganttStartDay                 int
	loading                       bool
	err                           error
	inputMode                     bool
	inputAction                   string
	input                         string
	notice                        string
	filter                        ports.TaskFilter
	priorityFilter                int
	statusNameFilter              string
	today                         domain.Date
	dayWatchStarted               bool
	dayWatchCancel                chan struct{}
	historyOpen                   bool
	helpOpen                      bool
	helpScroll                    int
	paletteOpen                   bool
	paletteQuery                  string
	paletteSelected               int
	paletteNotice                 string
	pickerOpen                    bool
	pickerAction                  string
	pickerOptions                 []pickerOption
	pickerSelected                int
	pickerTask                    presenter.Task
	pickerChosen                  map[int64]bool
	recurrenceOrdinal             string
	preserveSelectionID           int64
	preserveSelectionSource       string
	history                       []domain.HistoryEvent
	form                          taskForm
	nextFormRequestID             uint64
}

func New(b Backend) Model {
	view := "kanban"
	if b.Mode() == domain.ModeGlobal {
		view = "calendar"
	}
	return NewAt(b, view)
}
func NewAt(b Backend, view string) Model {
	views := map[string]int{"kanban": 0, "table": 1, "calendar": 2, "gantt": 3, "trash": 4, "settings": 5}
	v, ok := views[view]
	if !ok {
		v = 0
	}
	if b.Mode() == domain.ModeGlobal && (v == 0 || v == 5) {
		v = 2
	}
	filter := ports.TaskFilter{IncludeDone: true, IncludeCancelled: true, Sort: "updated"}
	if b.Mode() == domain.ModeGlobal {
		filter.IncludeDone = false
		filter.IncludeCancelled = false
	}
	today := b.Today()
	model := Model{backend: b, view: v, loading: true, calendarMonth: today.Time(), ganttStartDay: 1, filter: filter, priorityFilter: -1, today: today, dayWatchCancel: make(chan struct{})}
	model.viewNavigation[v] = viewNavigation{initialized: true, focus: focusMain, inspectorMode: inspectorNormal, calendarMonth: today.Time(), ganttStartDay: 1}
	return model
}
func (m Model) Init() tea.Cmd { return m.load(false) }
func (m Model) load(deleted bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		filter := m.filter
		filter.IncludeDeleted = deleted
		if deleted {
			filter.IncludeDone = true
			filter.IncludeCancelled = true
		}
		tasks, listErr := m.backend.List(ctx, filter)
		statuses, statusErr := m.backend.Statuses(ctx)
		return loaded{tasks, statuses, errors.Join(listErr, statusErr), deleted}
	}
}
func (m Model) loadDetail() tea.Cmd {
	if !m.hasSelectedTask() {
		return nil
	}
	task := m.tasks[m.selected]
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		detail, e := m.backend.Detail(ctx, task.Source, task.ID)
		if e != nil {
			return detailLoaded{task: detail, err: e}
		}
		history, historyErr := m.backend.History(ctx, task.Source, task.ID)
		return detailLoaded{task: detail, history: history, historyErr: historyErr}
	}
}
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
	case loaded:
		m.loading = false
		m.err = v.err
		if v.err != nil && (len(v.tasks) > 0 || m.backend.Mode() == domain.ModeGlobal) {
			m.notice = "Algunos orígenes no están disponibles: " + v.err.Error()
			m.err = nil
		}
		if v.trash {
			m.deleted = presenter.Tasks(v.tasks)
		} else {
			m.tasks = presenter.Tasks(v.tasks)
			m.statuses = v.statuses
		}
		if m.preserveSelectionID != 0 {
			items := m.tasks
			if v.trash {
				items = m.deleted
			}
			for index, item := range items {
				if item.ID == m.preserveSelectionID && (m.preserveSelectionSource == "" || item.Source == m.preserveSelectionSource) {
					m.selected = index
					break
				}
			}
			m.preserveSelectionID = 0
			m.preserveSelectionSource = ""
		}
		if m.view < 4 {
			m.ensureVisibleSelection()
		} else if count := m.visibleCount(); count == 0 {
			m.selected = 0
		} else if m.selected >= count {
			m.selected = count - 1
		}
		if !v.trash {
			if command := m.loadDetail(); command != nil {
				return m, command
			}
		}
		if !m.dayWatchStarted {
			m.dayWatchStarted = true
			return m, m.waitForDayCheck()
		}
	case dayCheck:
		current := m.backend.Today()
		if current.Equal(m.today) {
			return m, m.waitForDayCheck()
		}
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			return maintenanceFinished{today: current, err: m.backend.Maintain(ctx)}
		}
	case maintenanceFinished:
		m.err = v.err
		if v.err == nil {
			m.today = v.today
			m.calendarMonth = v.today.Time()
			m.notice = "Mantenimiento diario completado"
			return m, tea.Batch(m.load(m.view == 4), m.waitForDayCheck())
		}
		return m, m.waitForDayCheck()
	case historyLoaded:
		m.closePalette()
		m.err = v.err
		if v.err == nil {
			m.history = v.events
			m.historyOpen = true
		}
	case pickerLoaded:
		m.closePalette()
		m.loading = false
		m.err = v.err
		if v.err == nil {
			m.pickerAction = v.action
			m.pickerTask = v.task
			m.pickerOptions = v.options
			m.pickerSelected = 0
			m.pickerOpen = true
		}
	case formLoaded:
		if !m.form.open || v.requestID != m.form.requestID {
			break
		}
		if v.detailErr != nil || v.statusesErr != nil {
			m.form = newTaskForm(v)
			m.form.loadFailed = true
			m.form.errors["form"] = friendlyError(errors.Join(v.detailErr, v.statusesErr))
			break
		}
		m.form = newTaskForm(v)
	case formSaved:
		if !m.form.open {
			break
		}
		m.form.saving = false
		if v.err != nil {
			var validation domain.ValidationError
			if errors.As(v.err, &validation) {
				m.form.errors[validation.Field] = localizedValidation(validation.Field, validation.Message)
			} else {
				m.form.errors["form"] = friendlyError(v.err)
			}
			break
		}
		m.form = taskForm{}
		m.notice = "Tarea actualizada"
		if v.created {
			m.notice = "Tarea creada"
		}
		m.preserveSelectionID = v.task.ID
		m.preserveSelectionSource = v.task.Origin.Key
		return m, m.load(false)
	case detailLoaded:
		if v.err != nil {
			m.err = v.err
			break
		}
		if m.selected < len(m.tasks) && m.tasks[m.selected].ID == v.task.ID && m.tasks[m.selected].Source == v.task.Origin.Key {
			m.detail = &v.task
			m.history = v.history
			if v.historyErr != nil {
				m.notice = "No se pudo cargar el historial del inspector: " + v.historyErr.Error()
			}
			if len(v.task.Subtasks) == 0 {
				m.selectedSubtask = 0
			} else if m.selectedSubtask >= len(v.task.Subtasks) {
				m.selectedSubtask = len(v.task.Subtasks) - 1
			}
			m.clampInspectorCursor()
		}
		if !m.dayWatchStarted {
			m.dayWatchStarted = true
			return m, m.waitForDayCheck()
		}
	case trashImpact:
		m.closePalette()
		m.loading = false
		m.err = v.err
		if v.err == nil {
			if len(v.affected) == 0 {
				return m, m.trash(v.task)
			}
			m.confirmTrash = &v.task
			m.confirmAffected = v.affected
		}
	case created:
		m.err = v.err
		if v.err == nil {
			m.inputMode = false
			m.notice = "Tarea creada"
			m.preserveSelectionID = v.task.ID
			m.preserveSelectionSource = v.task.Origin.Key
			return m, m.load(false)
		}
	case mutated:
		m.inputMode = false
		m.loading = false
		m.err = v.err
		if v.err == nil {
			m.notice = v.action
			m.rememberSelection()
			return m, m.load(m.view == 4)
		}
	case markdownFinished:
		m.err = v.err
		if v.err == nil {
			m.notice = "Markdown actualizado"
			return m, m.load(false)
		}
	case tea.KeyMsg:
		if v.Type == tea.KeyCtrlC {
			m.cancelDayWatch()
			return m, tea.Quit
		}
		if m.form.open {
			return m.updateTaskForm(v)
		}
		if m.paletteOpen {
			return m.updatePalette(v)
		}
		if v.String() == "f1" {
			m.helpOpen = !m.helpOpen
			m.helpScroll = 0
			return m, nil
		}
		if m.helpOpen {
			switch v.String() {
			case "esc":
				m.helpOpen = false
			case "q":
				m.cancelDayWatch()
				return m, tea.Quit
			case "down", "j":
				m.helpScroll++
			case "up", "k":
				m.helpScroll = max(0, m.helpScroll-1)
			case "pgdown":
				m.helpScroll += max(1, m.height-8)
			case "pgup":
				m.helpScroll = max(0, m.helpScroll-max(1, m.height-8))
			}
			return m, nil
		}
		if m.pickerOpen {
			return m.updatePicker(v)
		}
		if m.historyOpen {
			if v.String() == "q" {
				m.cancelDayWatch()
				return m, tea.Quit
			}
			if v.String() == "H" || v.String() == "esc" {
				m.historyOpen = false
			}
			return m, nil
		}
		if m.confirmTrash != nil {
			switch v.String() {
			case "y", "Y", "enter":
				task := *m.confirmTrash
				m.confirmTrash = nil
				m.confirmAffected = nil
				m.loading = true
				return m, m.trash(task)
			case "n", "N", "esc":
				m.confirmTrash = nil
				m.confirmAffected = nil
				m.notice = "Eliminación cancelada"
			}
			return m, nil
		}
		if m.inputMode {
			return m.updateInput(v)
		}
		if v.Type == tea.KeyCtrlP {
			m.openPalette()
			return m, nil
		}
		refreshDetail := false
		switch v.String() {
		case "q":
			m.cancelDayWatch()
			return m, tea.Quit
		case "tab", "shift+tab":
			if m.hasSelectedTask() {
				if m.inspectorMode == inspectorHidden {
					m.inspectorMode = inspectorNormal
				}
				if m.panelFocus == focusMain {
					m.panelFocus = focusInspector
				} else {
					if m.inspectorMode == inspectorExpanded {
						m.inspectorMode = inspectorNormal
					}
					m.panelFocus = focusMain
				}
				m.saveViewNavigation()
			}
			return m, nil
		case "I":
			if m.hasSelectedTask() {
				switch m.inspectorMode {
				case inspectorNormal:
					m.inspectorMode = inspectorExpanded
					m.panelFocus = focusInspector
				case inspectorExpanded:
					m.inspectorMode = inspectorHidden
					m.panelFocus = focusMain
				default:
					m.inspectorMode = inspectorNormal
				}
				m.saveViewNavigation()
			}
			return m, nil
		case " ":
			if m.hasSelectedTask() && m.inspectorMode != inspectorHidden {
				m.inspectorPinned = !m.inspectorPinned
			}
			return m, nil
		case "right", "l":
			m.changeView(1)
			refreshDetail = true
		case "left", "h":
			m.changeView(-1)
			refreshDetail = true
		case "pgup":
			if m.view == 2 || m.view == 3 {
				m.calendarMonth = m.calendarMonth.AddDate(0, -1, 0)
				m.ganttStartDay = 1
				m.ensureVisibleSelection()
				m.selectedSubtask = 0
				m.detail = nil
				refreshDetail = true
			}
		case "pgdown":
			if m.view == 2 || m.view == 3 {
				m.calendarMonth = m.calendarMonth.AddDate(0, 1, 0)
				m.ganttStartDay = 1
				m.ensureVisibleSelection()
				m.selectedSubtask = 0
				m.detail = nil
				refreshDetail = true
			}
		case ",", ".":
			if m.view == 3 {
				direction := 7
				if v.String() == "," {
					direction = -7
				}
				m.ganttStartDay = max(1, min(31, m.ganttStartDay+direction))
			}
		case "down", "j":
			if m.panelFocus == focusInspector && m.inspectorMode != inspectorHidden {
				if m.detail != nil {
					m.moveInspectorCursor(1)
				}
				return m, nil
			}
			if m.moveSelection(1) {
				m.detail = nil
				m.history = nil
				m.saveViewNavigation()
				return m, m.loadDetail()
			}
		case "up", "k":
			if m.panelFocus == focusInspector && m.inspectorMode != inspectorHidden {
				if m.detail != nil {
					m.moveInspectorCursor(-1)
				}
				return m, nil
			}
			if m.moveSelection(-1) {
				m.detail = nil
				m.history = nil
				m.saveViewNavigation()
				return m, m.loadDetail()
			}
		case "J":
			m.focusAdjacentSubtask(1)
		case "K":
			m.focusAdjacentSubtask(-1)
		case "enter":
			if m.panelFocus == focusInspector && m.inspectorMode != inspectorHidden {
				return m.activateInspectorRow()
			}
		case "r":
			m.loading = true
			return m, m.load(m.view == 4)
		case "/", "?", "M", "P", "S", "D":
			if v.String() == "S" && m.backend.Mode() == domain.ModeLocal {
				options := []pickerOption{{Label: "Todos los estados"}}
				for _, status := range m.statuses {
					options = append(options, pickerOption{ID: status.ID, Label: fmt.Sprintf("#%d · %s", status.ID, status.Name)})
				}
				m.openPicker("filter-status", presenter.Task{}, options)
				break
			}
			m.inputMode = true
			m.input = ""
			switch v.String() {
			case "/":
				m.inputAction, m.input = "filter-title", m.filter.Query
			case "?", "M":
				m.inputAction, m.input = "filter-markdown", m.filter.Markdown
			case "P":
				if m.backend.Mode() == domain.ModeGlobal {
					m.inputAction, m.input = "filter-origin", m.filter.Origin
				} else {
					m.inputMode = false
				}
			case "S":
				m.inputAction = "filter-status-name"
				m.input = m.statusNameFilter
			case "D":
				m.inputAction = "filter-dates"
				from, to := "", ""
				if m.filter.From != nil {
					from = m.filter.From.String()
				}
				if m.filter.To != nil {
					to = m.filter.To.String()
				}
				m.input = from + ".." + to
			}
		case "B":
			m.filter.OnlyBlocked = !m.filter.OnlyBlocked
			return m.reloadFiltered("Filtro de bloqueo actualizado")
		case "R":
			m.filter.OnlyRecurring = !m.filter.OnlyRecurring
			return m.reloadFiltered("Filtro de recurrencia actualizado")
		case "f", "C", "z":
			if m.hasSelectedTask() {
				action, notice := "complete", "Tarea finalizada"
				if v.String() == "C" {
					action, notice = "cancel", "Tarea cancelada"
				} else if v.String() == "z" {
					action, notice = "reopen", "Tarea reabierta en el estado inicial"
				}
				task := m.tasks[m.selected]
				m.loading = true
				return m, func() tea.Msg {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_, e := m.backend.SetLifecycle(ctx, task.Source, task.ID, task.Version, action)
					return mutated{action: notice, err: e}
				}
			}
		case "F":
			m.filter.IncludeDone = !m.filter.IncludeDone
			return m.reloadFiltered("Visibilidad de finalizadas actualizada")
		case "X":
			m.filter.IncludeCancelled = !m.filter.IncludeCancelled
			return m.reloadFiltered("Visibilidad de canceladas actualizada")
		case "1":
			m.priorityFilter++
			if m.priorityFilter > int(domain.PriorityUrgent) {
				m.priorityFilter = -1
			}
			m.filter.Priorities = nil
			if m.priorityFilter >= 0 {
				m.filter.Priorities = []domain.Priority{domain.Priority(m.priorityFilter)}
			}
			return m.reloadFiltered("Filtro de prioridad actualizado")
		case "o":
			sorts := []string{"updated", "priority", "title", "status", "start", "due"}
			index := 0
			for i, sortName := range sorts {
				if m.filter.Sort == sortName {
					index = i
					break
				}
			}
			m.filter.Sort = sorts[(index+1)%len(sorts)]
			return m.reloadFiltered("Orden: " + m.filter.Sort)
		case "0":
			m.filter = ports.TaskFilter{IncludeDone: m.backend.Mode() == domain.ModeLocal, IncludeCancelled: m.backend.Mode() == domain.ModeLocal, Sort: "updated"}
			m.priorityFilter = -1
			m.statusNameFilter = ""
			return m.reloadFiltered("Filtros restablecidos")
		case "n":
			if m.backend.Capabilities("").CanCreateTask {
				return m.openTaskForm(false, false)
			} else {
				m.notice = "El origen global no está disponible para crear tareas"
			}
		case "N":
			if m.backend.Capabilities("").CanCreateTask {
				return m.openTaskForm(false, true)
			} else {
				m.notice = "El origen global no está disponible para crear tareas"
			}
		case "e":
			if m.view == 5 && m.selected < len(m.statuses) && m.statuses[m.selected].Kind == domain.StatusNormal {
				m.inputMode = true
				m.inputAction = "rename-status"
				m.input = m.statuses[m.selected].Name
			} else if m.hasSelectedTask() {
				return m.openTaskForm(true, false)
			}
		case "a":
			if m.view == 5 && m.backend.Capabilities("").CanCreateStatus {
				m.inputMode = true
				m.inputAction = "create-status"
				m.input = ""
			} else if m.hasSelectedTask() && m.backend.Capabilities(m.tasks[m.selected].Source).CanCreateSubtask {
				m.inputMode = true
				m.inputAction = "add-subtask"
				m.input = ""
			} else if m.backend.Mode() == domain.ModeGlobal {
				m.notice = "No se pueden añadir subtareas a una tarea de proyecto desde global"
			}
		case "E":
			if _, ok := m.focusedSubtask(); ok {
				m.inputMode = true
				m.inputAction = "rename-subtask"
				m.input = m.detail.Subtasks[m.selectedSubtask].Title
			}
		case "t":
			if _, ok := m.focusedSubtask(); ok && m.selected < len(m.tasks) {
				task := m.tasks[m.selected]
				subtaskID := m.detail.Subtasks[m.selectedSubtask].ID
				return m, func() tea.Msg {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					e := m.backend.ToggleSubtask(ctx, task.Source, task.ID, subtaskID, task.Version)
					return mutated{action: "Subtarea actualizada", err: e}
				}
			}
		case "{", "}":
			if _, ok := m.focusedSubtask(); ok && m.selected < len(m.tasks) {
				direction := 1
				if v.String() == "{" {
					direction = -1
				}
				task := m.tasks[m.selected]
				subtaskID := m.detail.Subtasks[m.selectedSubtask].ID
				return m, func() tea.Msg {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					e := m.backend.MoveSubtaskStatus(ctx, task.Source, task.ID, subtaskID, task.Version, direction)
					return mutated{action: "Estado de subtarea actualizado", err: e}
				}
			}
		case "g":
			if m.hasSelectedTask() && m.backend.Capabilities(m.tasks[m.selected].Source).CanCreateDependency {
				return m, m.loadDependencyPicker("add-dependency", false)
			} else if m.backend.Mode() == domain.ModeGlobal {
				m.notice = "No se pueden añadir dependencias a una tarea de proyecto desde global"
			}
		case "G":
			if m.hasSelectedTask() {
				if row, ok := m.focusedInspectorRow(); ok && row.Kind == taskdetail.RowDependency {
					task := m.tasks[m.selected]
					m.loading = true
					return m, func() tea.Msg {
						ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
						defer cancel()
						return mutated{action: "Dependencia eliminada", err: m.backend.RemoveDependency(ctx, task.Source, task.ID, row.ID, task.Version)}
					}
				}
				return m, m.loadDependencyPicker("remove-dependency", true)
			}
		case "c":
			if m.hasSelectedTask() {
				task := m.tasks[m.selected]
				if task.Recurrence == "" && !m.backend.Capabilities(task.Source).CanCreateRecurrence {
					m.notice = "No se puede añadir recurrencia a una tarea de proyecto desde global"
					break
				}
				m.openPicker("recurrence-kind", task, []pickerOption{
					{Label: "Sin recurrencia", Value: ""},
					{Label: "Diaria", Value: "daily"},
					{Label: "Semanal · elegir uno o varios días", Value: "weekly"},
					{Label: "Mensual · elegir día del mes", Value: "monthly"},
					{Label: "Último día de cada mes", Value: "month-end"},
					{Label: "Mensual · ordinal y día de semana", Value: "monthly-weekday"},
				})
			}
		case "s", "v":
			if m.hasSelectedTask() {
				m.inputMode = true
				m.inputAction = "start"
				m.input = m.tasks[m.selected].Start
				if v.String() == "v" {
					m.inputAction = "due"
					m.input = m.tasks[m.selected].Due
				}
			}
		case "p":
			if m.hasSelectedTask() {
				t := m.tasks[m.selected]
				m.loading = true
				return m, func() tea.Msg {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_, e := m.backend.CyclePriority(ctx, t.Source, t.ID, t.Version)
					return mutated{action: "Prioridad actualizada", err: e}
				}
			}
		case "[", "]":
			if m.view == 5 && m.selected < len(m.statuses) && m.statuses[m.selected].Kind == domain.StatusNormal {
				ids := make([]int64, 0)
				current := -1
				for _, status := range m.statuses {
					if status.Kind == domain.StatusNormal {
						if status.ID == m.statuses[m.selected].ID {
							current = len(ids)
						}
						ids = append(ids, status.ID)
					}
				}
				direction := 1
				if v.String() == "[" {
					direction = -1
				}
				target := current + direction
				if current >= 0 && target >= 0 && target < len(ids) {
					ids[current], ids[target] = ids[target], ids[current]
					return m, func() tea.Msg {
						ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
						defer cancel()
						e := m.backend.ReorderStatuses(ctx, ids)
						return mutated{action: "Estados reordenados", err: e}
					}
				}
			} else if m.hasSelectedTask() {
				direction := 1
				if v.String() == "[" {
					direction = -1
				}
				t := m.tasks[m.selected]
				m.loading = true
				return m, func() tea.Msg {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_, e := m.backend.MoveStatus(ctx, t.Source, t.ID, t.Version, direction)
					return mutated{action: "Estado actualizado", err: e}
				}
			}
		case "m":
			if m.hasSelectedTask() {
				t := m.tasks[m.selected]
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				command, finish, e := m.backend.MarkdownEditor(ctx, t.Source, t.ID, t.Version)
				cancel()
				if e != nil {
					m.err = e
					break
				}
				return m, tea.Exec(command, func(runErr error) tea.Msg {
					return markdownFinished{err: finish(runErr)}
				})
			}
		case "d":
			if m.view == 5 && m.selected < len(m.statuses) && m.statuses[m.selected].Kind == domain.StatusNormal {
				selectedStatus := m.statuses[m.selected]
				options := []pickerOption{{Label: "Sin destino (solo si está vacío)"}}
				for _, status := range m.statuses {
					if status.Kind == domain.StatusNormal && status.ID != selectedStatus.ID {
						options = append(options, pickerOption{ID: status.ID, Label: fmt.Sprintf("#%d · mover tareas a %s", status.ID, status.Name)})
					}
				}
				m.openPicker("delete-status", presenter.Task{ID: selectedStatus.ID, Title: selectedStatus.Name}, options)
			} else if m.hasSelectedTask() {
				t := m.tasks[m.selected]
				m.loading = true
				return m, func() tea.Msg {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					affected, e := m.backend.DependencyImpact(ctx, t.Source, t.ID)
					return trashImpact{task: t, affected: presenter.Tasks(affected), err: e}
				}
			}
		case "u":
			if m.view == 4 && m.selected < len(m.deleted) {
				t := m.deleted[m.selected]
				m.loading = true
				return m, func() tea.Msg {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_, e := m.backend.Restore(ctx, t.Source, t.ID, t.Version)
					return mutated{action: "Tarea restaurada", err: e}
				}
			}
		case "i":
			if m.view == 5 && m.selected < len(m.statuses) && m.statuses[m.selected].Kind == domain.StatusNormal {
				id := m.statuses[m.selected].ID
				return m, func() tea.Msg {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					e := m.backend.SetInitialStatus(ctx, id)
					return mutated{action: "Estado inicial actualizado", err: e}
				}
			}
		case "H":
			if m.hasSelectedTask() {
				task := m.tasks[m.selected]
				return m, func() tea.Msg {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					events, e := m.backend.History(ctx, task.Source, task.ID)
					return historyLoaded{events: events, err: e}
				}
			}
		}
		if refreshDetail && m.view != 4 {
			return m, m.loadDetail()
		}
	}
	if m.view == 4 && m.deleted == nil {
		return m, m.load(true)
	}
	return m, nil
}
func (m Model) waitForDayCheck() tea.Cmd {
	cancel := m.dayWatchCancel
	return func() tea.Msg {
		timer := time.NewTimer(time.Minute)
		defer timer.Stop()
		select {
		case <-timer.C:
			return dayCheck{}
		case <-cancel:
			return nil
		}
	}
}
func (m Model) cancelDayWatch() {
	if m.dayWatchCancel == nil {
		return
	}
	select {
	case <-m.dayWatchCancel:
	default:
		close(m.dayWatchCancel)
	}
}
func (m Model) trash(task presenter.Task) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, e := m.backend.Trash(ctx, task.Source, task.ID, task.Version)
		return mutated{action: "Tarea enviada a la papelera", err: e}
	}
}
func (m Model) reloadFiltered(notice string) (tea.Model, tea.Cmd) {
	m.loading = true
	m.selected = 0
	m.selectedSubtask = 0
	m.detail = nil
	m.notice = notice
	return m, m.load(m.view == 4)
}
func (m *Model) rememberSelection() {
	items := m.tasks
	if m.view == 4 {
		items = m.deleted
	}
	if m.selected >= 0 && m.selected < len(items) {
		m.preserveSelectionID = items[m.selected].ID
		m.preserveSelectionSource = items[m.selected].Source
	}
}
func (m *Model) openPicker(action string, task presenter.Task, options []pickerOption) {
	m.inputMode = false
	m.pickerOpen = true
	m.pickerAction = action
	m.pickerTask = task
	m.pickerOptions = options
	m.pickerSelected = 0
	m.pickerChosen = make(map[int64]bool)
	m.notice = ""
}

func (m Model) loadDependencyPicker(action string, existingOnly bool) tea.Cmd {
	task := m.tasks[m.selected]
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		candidates, err := m.backend.DependencyCandidates(ctx, task.Source, task.ID, existingOnly)
		options := make([]pickerOption, 0, len(candidates))
		for _, candidate := range candidates {
			options = append(options, pickerOption{
				ID:    candidate.ID,
				Label: fmt.Sprintf("#%d · %s [%s]", candidate.ID, candidate.Title, candidate.Status.Name),
			})
		}
		return pickerLoaded{action: action, task: task, options: options, err: err}
	}
}

func (m Model) updatePicker(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		m.pickerOpen = false
		m.pickerOptions = nil
		return m, nil
	case "up", "k":
		if m.pickerSelected > 0 {
			m.pickerSelected--
		}
		return m, nil
	case "down", "j":
		if m.pickerSelected < len(m.pickerOptions)-1 {
			m.pickerSelected++
		}
		return m, nil
	case " ":
		if m.pickerAction == "recurrence-weekly-days" && len(m.pickerOptions) > 0 {
			id := m.pickerOptions[m.pickerSelected].ID
			m.pickerChosen[id] = !m.pickerChosen[id]
			m.notice = ""
		}
		return m, nil
	case "enter":
		if len(m.pickerOptions) == 0 {
			return m, nil
		}
		option := m.pickerOptions[m.pickerSelected]
		action, task := m.pickerAction, m.pickerTask
		if action == "recurrence-weekly-days" {
			var days []string
			for _, candidate := range m.pickerOptions {
				if m.pickerChosen[candidate.ID] {
					days = append(days, candidate.Value)
				}
			}
			if len(days) == 0 {
				m.notice = "Selecciona al menos un día con Espacio"
				return m, nil
			}
			m.pickerOpen = false
			m.pickerOptions = nil
			m.loading = true
			return m, m.updateRecurrenceCommand(task, "weekly:"+strings.Join(days, ","))
		}
		if action == "recurrence-ordinal" {
			m.recurrenceOrdinal = option.Value
			m.openPicker("recurrence-weekday", task, weekdayPickerOptions())
			return m, nil
		}
		if action == "recurrence-weekday" {
			m.pickerOpen = false
			m.pickerOptions = nil
			m.loading = true
			return m, m.updateRecurrenceCommand(task, "monthly-weekday:"+m.recurrenceOrdinal+":"+option.Value)
		}
		m.pickerOpen = false
		m.pickerOptions = nil
		if action == "filter-status" {
			m.filter.StatusIDs = nil
			if option.ID != 0 {
				m.filter.StatusIDs = []int64{option.ID}
			}
			return m.reloadFiltered("Filtro de estado actualizado")
		}
		if action == "recurrence-kind" {
			if option.Value == "weekly" {
				m.openPicker("recurrence-weekly-days", task, weekdayPickerOptions())
				return m, nil
			}
			if option.Value == "monthly-weekday" {
				m.openPicker("recurrence-ordinal", task, []pickerOption{
					{ID: 1, Label: "Primer", Value: "first"},
					{ID: 2, Label: "Segundo", Value: "second"},
					{ID: 3, Label: "Tercer", Value: "third"},
					{ID: 4, Label: "Cuarto", Value: "fourth"},
					{ID: 5, Label: "Último", Value: "last"},
				})
				return m, nil
			}
			if option.Value == "monthly" {
				m.inputMode = true
				m.inputAction = "recurrence-monthly"
				m.input = ""
				return m, nil
			}
			m.loading = true
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				var recurrence *domain.Recurrence
				if option.Value != "" {
					parsed, err := domain.ParseRecurrence(option.Value)
					if err != nil {
						return mutated{err: err}
					}
					recurrence = &parsed
				}
				_, err := m.backend.UpdateRecurrence(ctx, task.Source, task.ID, task.Version, recurrence)
				return mutated{action: "Recurrencia actualizada", err: err}
			}
		}
		m.loading = true
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			switch action {
			case "add-dependency":
				return mutated{action: "Dependencia creada", err: m.backend.AddDependency(ctx, task.Source, task.ID, option.ID, task.Version)}
			case "remove-dependency":
				return mutated{action: "Dependencia eliminada", err: m.backend.RemoveDependency(ctx, task.Source, task.ID, option.ID, task.Version)}
			case "delete-status":
				var destination *int64
				if option.ID != 0 {
					id := option.ID
					destination = &id
				}
				return mutated{action: "Estado eliminado", err: m.backend.DeleteStatus(ctx, task.ID, destination)}
			default:
				return mutated{err: domain.ValidationError{Field: "selector", Message: "acción inválida"}}
			}
		}
	}
	return m, nil
}

func (m Model) pickerView(height int) string {
	title := "Seleccionar"
	switch m.pickerAction {
	case "add-dependency":
		title = "Agregar dependencia a " + m.pickerTask.Title
	case "remove-dependency":
		title = "Eliminar dependencia de " + m.pickerTask.Title
	case "filter-status":
		title = "Filtrar por estado"
	case "delete-status":
		title = "Eliminar estado " + m.pickerTask.Title + " · elegir destino"
	case "recurrence-kind":
		title = "Configurar recurrencia de " + m.pickerTask.Title
	case "recurrence-weekly-days":
		title = "Elegir días de la semana · Espacio marca o desmarca"
	case "recurrence-ordinal":
		title = "Elegir ordinal del mes"
	case "recurrence-weekday":
		title = "Elegir día de la semana"
	}
	lines := []string{theme.Title.Render(title)}
	if m.notice != "" {
		lines = append(lines, theme.Help.Foreground(theme.Danger).Render(m.notice))
	}
	if len(m.pickerOptions) == 0 {
		lines = append(lines, "No hay opciones disponibles.")
	} else {
		start, end := listutil.Bounds(len(m.pickerOptions), m.pickerSelected, max(1, height-8))
		if start > 0 {
			lines = append(lines, fmt.Sprintf("↑ %d opción(es) más", start))
		}
		for index := start; index < end; index++ {
			option := m.pickerOptions[index]
			label := option.Label
			if m.pickerAction == "recurrence-weekly-days" {
				mark := "[ ] "
				if m.pickerChosen[option.ID] {
					mark = "[x] "
				}
				label = mark + label
			}
			line := "  " + label
			if index == m.pickerSelected {
				line = theme.Selected.Render("› " + label)
			}
			lines = append(lines, line)
		}
		if end < len(m.pickerOptions) {
			lines = append(lines, fmt.Sprintf("↓ %d opción(es) más", len(m.pickerOptions)-end))
		}
	}
	return theme.Border.Render(strings.Join(lines, "\n"))
}

func weekdayPickerOptions() []pickerOption {
	return []pickerOption{
		{ID: 1, Label: "Lunes", Value: "mon"},
		{ID: 2, Label: "Martes", Value: "tue"},
		{ID: 3, Label: "Miércoles", Value: "wed"},
		{ID: 4, Label: "Jueves", Value: "thu"},
		{ID: 5, Label: "Viernes", Value: "fri"},
		{ID: 6, Label: "Sábado", Value: "sat"},
		{ID: 7, Label: "Domingo", Value: "sun"},
	}
}

func (m Model) updateRecurrenceCommand(task presenter.Task, raw string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		parsed, err := domain.ParseRecurrence(raw)
		if err != nil {
			return mutated{err: err}
		}
		_, err = m.backend.UpdateRecurrence(ctx, task.Source, task.ID, task.Version, &parsed)
		return mutated{action: "Recurrencia actualizada", err: err}
	}
}
func (m Model) visibleCount() int {
	if m.view == 4 {
		return len(m.deleted)
	}
	if m.view == 5 {
		return len(m.statuses)
	}
	return len(m.tasks)
}
func (m Model) hasSelectedTask() bool {
	if m.view >= 4 || m.selected < 0 || m.selected >= len(m.tasks) {
		return false
	}
	if m.view < 2 {
		return true
	}
	for _, index := range m.selectableIndices() {
		if index == m.selected {
			return true
		}
	}
	return false
}
func (m Model) selectableIndices() []int {
	indices := make([]int, 0, len(m.tasks))
	monthStart := time.Date(m.calendarMonth.Year(), m.calendarMonth.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0).AddDate(0, 0, -1)
	for index, task := range m.tasks {
		visible := true
		if m.view == 2 {
			visible = false
			if !task.Recurring {
				for _, raw := range []string{task.Start, task.Due} {
					if date, err := time.Parse("2006-01-02", raw); err == nil && !date.Before(monthStart) && !date.After(monthEnd) {
						visible = true
					}
				}
			}
		} else if m.view == 3 {
			visible = false
			if !task.Recurring && (task.Start != "" || task.Due != "") {
				start, startErr := time.Parse("2006-01-02", task.Start)
				due, dueErr := time.Parse("2006-01-02", task.Due)
				if startErr != nil {
					start = due
				}
				if dueErr != nil {
					due = start
				}
				visible = !start.After(monthEnd) && !due.Before(monthStart)
			}
		}
		if visible {
			indices = append(indices, index)
		}
	}
	return indices
}

func (m *Model) ensureVisibleSelection() {
	indices := m.selectableIndices()
	if len(indices) == 0 {
		m.selected = 0
		return
	}
	for _, index := range indices {
		if index == m.selected {
			return
		}
	}
	m.selected = indices[0]
}

func (m *Model) moveSelection(direction int) bool {
	if m.view == 4 || m.view == 5 {
		count := m.visibleCount()
		target := m.selected + direction
		if target >= 0 && target < count {
			m.selected = target
			return true
		}
		return false
	}
	indices := m.selectableIndices()
	for position, index := range indices {
		if index == m.selected {
			target := position + direction
			if target >= 0 && target < len(indices) {
				m.selected = indices[target]
				return true
			}
			return false
		}
	}
	if len(indices) > 0 {
		m.selected = indices[0]
		return true
	}
	return false
}
func (m Model) nextView(direction int) int {
	next := m.view
	for {
		next = (next + direction + 6) % 6
		if m.backend.Mode() != domain.ModeGlobal || (next != 0 && next != 5) {
			return next
		}
	}
}

func (m *Model) saveViewNavigation() {
	state := viewNavigation{
		initialized:     true,
		selected:        m.selected,
		inspectorCursor: m.inspectorCursor,
		focus:           m.panelFocus,
		inspectorMode:   m.inspectorMode,
		calendarMonth:   m.calendarMonth,
		ganttStartDay:   m.ganttStartDay,
	}
	if m.hasSelectedTask() {
		state.selectedID = m.tasks[m.selected].ID
		state.selectedSource = m.tasks[m.selected].Source
	}
	m.viewNavigation[m.view] = state
}

func (m *Model) restoreViewNavigation() {
	state := m.viewNavigation[m.view]
	if !state.initialized {
		state = viewNavigation{initialized: true, focus: focusMain, inspectorMode: inspectorNormal, calendarMonth: m.today.Time(), ganttStartDay: 1}
	}
	m.selected = state.selected
	if state.selectedID != 0 {
		for index, task := range m.tasks {
			if task.ID == state.selectedID && task.Source == state.selectedSource {
				m.selected = index
				break
			}
		}
	}
	m.ensureVisibleSelection()
	m.inspectorCursor = state.inspectorCursor
	m.panelFocus = state.focus
	if !m.inspectorPinned {
		m.inspectorMode = state.inspectorMode
	}
	if !state.calendarMonth.IsZero() {
		m.calendarMonth = state.calendarMonth
	} else {
		m.calendarMonth = m.today.Time()
	}
	m.ganttStartDay = state.ganttStartDay
	if m.ganttStartDay < 1 {
		m.ganttStartDay = 1
	}
	if m.inspectorMode == inspectorHidden {
		m.panelFocus = focusMain
	} else if m.inspectorMode == inspectorExpanded {
		m.panelFocus = focusInspector
	}
}

func (m *Model) changeView(direction int) {
	m.saveViewNavigation()
	m.view = m.nextView(direction)
	m.restoreViewNavigation()
	m.detail = nil
	m.history = nil
	m.selectedSubtask = 0
}

func (m Model) inspectorRows() []taskdetail.Row {
	if m.detail == nil {
		return nil
	}
	task := presenter.Tasks([]domain.Task{*m.detail})[0]
	return taskdetail.Rows(task, m.history)
}

func (m *Model) clampInspectorCursor() {
	rows := m.inspectorRows()
	if len(rows) == 0 {
		m.inspectorCursor = 0
		return
	}
	m.inspectorCursor = max(0, min(m.inspectorCursor, len(rows)-1))
	m.syncInspectorSelection()
}

func (m *Model) syncInspectorSelection() {
	rows := m.inspectorRows()
	if m.inspectorCursor >= 0 && m.inspectorCursor < len(rows) && rows[m.inspectorCursor].Kind == taskdetail.RowSubtask {
		m.selectedSubtask = rows[m.inspectorCursor].Index
	}
}

func (m *Model) moveInspectorCursor(direction int) bool {
	rows := m.inspectorRows()
	target := m.inspectorCursor + direction
	if target < 0 || target >= len(rows) {
		return false
	}
	m.inspectorCursor = target
	m.syncInspectorSelection()
	m.saveViewNavigation()
	return true
}

func (m *Model) focusAdjacentSubtask(direction int) {
	rows := m.inspectorRows()
	if len(rows) == 0 || m.inspectorMode == inspectorHidden {
		return
	}
	positions := make([]int, 0)
	current := -1
	for position, row := range rows {
		if row.Kind != taskdetail.RowSubtask {
			continue
		}
		if row.Index == m.selectedSubtask {
			current = len(positions)
		}
		positions = append(positions, position)
	}
	if len(positions) == 0 {
		return
	}
	if current < 0 {
		current = 0
		if direction < 0 {
			current = len(positions) - 1
		}
	} else {
		current = max(0, min(current+direction, len(positions)-1))
	}
	m.panelFocus = focusInspector
	m.inspectorCursor = positions[current]
	m.syncInspectorSelection()
	m.saveViewNavigation()
}

func (m Model) focusedInspectorRow() (taskdetail.Row, bool) {
	rows := m.inspectorRows()
	if m.panelFocus != focusInspector || m.inspectorMode == inspectorHidden || m.inspectorCursor < 0 || m.inspectorCursor >= len(rows) {
		return taskdetail.Row{}, false
	}
	return rows[m.inspectorCursor], true
}

func (m Model) focusedSubtask() (domain.Subtask, bool) {
	row, ok := m.focusedInspectorRow()
	if !ok || row.Kind != taskdetail.RowSubtask || m.detail == nil || row.Index < 0 || row.Index >= len(m.detail.Subtasks) {
		return domain.Subtask{}, false
	}
	return m.detail.Subtasks[row.Index], true
}

func (m Model) activateInspectorRow() (tea.Model, tea.Cmd) {
	row, ok := m.focusedInspectorRow()
	if !ok {
		return m, nil
	}
	key := ""
	switch row.Kind {
	case taskdetail.RowField:
		key = map[string]string{
			"title": "e", "status": "]", "priority": "p", "start": "s",
			"due": "v", "recurrence": "c", "markdown": "m",
		}[row.Field]
	case taskdetail.RowSubtask:
		key = "t"
	case taskdetail.RowDependency:
		key = "G"
	case taskdetail.RowHistory:
		m.historyOpen = true
		return m, nil
	}
	if key == "" {
		return m, nil
	}
	return m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
}

func (m Model) updateInput(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		m.inputMode = false
	case "enter":
		title := strings.TrimSpace(m.input)
		if strings.HasPrefix(m.inputAction, "filter-") {
			switch m.inputAction {
			case "filter-title":
				m.filter.Query = title
			case "filter-markdown":
				m.filter.Markdown = title
			case "filter-origin":
				m.filter.Origin = title
			case "filter-status-name":
				m.statusNameFilter = title
				m.filter.StatusNames = nil
				if title != "" {
					m.filter.StatusNames = []string{title}
				}
			case "filter-dates":
				from, to, ok := strings.Cut(title, "..")
				if !ok {
					m.err = domain.ValidationError{Field: "fechas", Message: "usa AAAA-MM-DD..AAAA-MM-DD"}
					return m, nil
				}
				m.filter.From, m.filter.To = nil, nil
				if strings.TrimSpace(from) != "" {
					date, e := domain.ParseDate(strings.TrimSpace(from))
					if e != nil {
						m.err = domain.ValidationError{Field: "fechas", Message: "usa AAAA-MM-DD..AAAA-MM-DD"}
						return m, nil
					}
					m.filter.From = &date
				}
				if strings.TrimSpace(to) != "" {
					date, e := domain.ParseDate(strings.TrimSpace(to))
					if e != nil {
						m.err = domain.ValidationError{Field: "fechas", Message: "usa AAAA-MM-DD..AAAA-MM-DD"}
						return m, nil
					}
					m.filter.To = &date
				}
				if m.filter.From != nil && m.filter.To != nil && m.filter.To.Before(*m.filter.From) {
					m.err = domain.ValidationError{Field: "fechas", Message: "el final no puede ser anterior al inicio"}
					return m, nil
				}
			}
			m.inputMode = false
			m.loading = true
			m.selected = 0
			m.detail = nil
			return m, m.load(m.view == 4)
		}
		if m.inputAction == "create-status" && title != "" {
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_, e := m.backend.CreateStatus(ctx, title)
				return mutated{action: "Estado creado", err: e}
			}
		}
		if m.inputAction == "rename-status" && title != "" && m.selected < len(m.statuses) {
			id := m.statuses[m.selected].ID
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				e := m.backend.RenameStatus(ctx, id, title)
				return mutated{action: "Estado renombrado", err: e}
			}
		}
		if m.inputAction == "add-subtask" && title != "" && m.selected < len(m.tasks) {
			task := m.tasks[m.selected]
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_, e := m.backend.AddSubtask(ctx, task.Source, task.ID, task.Version, title)
				return mutated{action: "Subtarea creada", err: e}
			}
		}
		if m.inputAction == "rename-subtask" && title != "" && m.detail != nil && m.selectedSubtask < len(m.detail.Subtasks) {
			task := m.tasks[m.selected]
			subtaskID := m.detail.Subtasks[m.selectedSubtask].ID
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_, e := m.backend.RenameSubtask(ctx, task.Source, task.ID, subtaskID, task.Version, title)
				return mutated{action: "Subtarea editada", err: e}
			}
		}
		if m.inputAction == "recurrence-monthly" && m.selected < len(m.tasks) {
			parsed, e := domain.ParseRecurrence("monthly:" + title)
			if e != nil {
				m.err = domain.ValidationError{Field: "recurrencia", Message: e.Error()}
				return m, nil
			}
			task := m.tasks[m.selected]
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_, e := m.backend.UpdateRecurrence(ctx, task.Source, task.ID, task.Version, &parsed)
				return mutated{action: "Recurrencia actualizada", err: e}
			}
		}
		if (m.inputAction == "start" || m.inputAction == "due") && m.selected < len(m.tasks) {
			var date *domain.Date
			if title != "" {
				parsed, e := domain.ParseDate(title)
				if e != nil {
					m.err = domain.ValidationError{Field: m.inputAction, Message: "usa AAAA-MM-DD"}
					return m, nil
				}
				date = &parsed
			}
			task := m.tasks[m.selected]
			field := m.inputAction
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_, e := m.backend.UpdateDate(ctx, task.Source, task.ID, task.Version, field, date)
				return mutated{action: "Fecha actualizada", err: e}
			}
		}
		if title != "" {
			if m.inputAction == "edit" && m.selected < len(m.tasks) {
				task := m.tasks[m.selected]
				return m, func() tea.Msg {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_, e := m.backend.UpdateTitle(ctx, task.Source, task.ID, task.Version, title)
					return mutated{action: "Tarea editada", err: e}
				}
			}
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				t, e := m.backend.Create(ctx, title)
				return created{t, e}
			}
		}
	case "backspace":
		r := []rune(m.input)
		if len(r) > 0 {
			m.input = string(r[:len(r)-1])
		}
	default:
		if len(k.Runes) > 0 {
			m.input += string(k.Runes)
		}
	}
	return m, nil
}
func (m Model) View() string {
	if m.width == 0 {
		return "Cargando…"
	}
	footerLines := wrapLines(strings.Split(m.footerContent(), "\n"), max(20, m.width))
	footer := strings.Join(footerLines, "\n")
	availableHeight := max(4, m.height-3-len(footerLines))
	header := theme.Title.Render("tasks") + "  " + theme.Help.Render(m.backend.ContextLabel()) + "  " + []string{"Kanban", "Tabla", "Calendario", "Gantt", "Papelera", "Estados"}[m.view]
	if summary := m.filterSummary(); summary != "" {
		header += "  " + theme.Help.Render(summary)
	}
	var body string
	if m.form.open {
		body = m.form.view(m.width, availableHeight)
	} else if m.loading {
		body = "Cargando…"
	} else if m.err != nil {
		message := "Error: " + friendlyError(m.err)
		if errors.Is(m.err, domain.ErrConflict) {
			message = "Conflicto: la tarea cambió en otra sesión. Pulsa r para refrescar antes de reintentar."
		}
		body = theme.Help.Foreground(theme.Danger).Render(message)
	} else {
		mainHeight := availableHeight
		detailHeight := 0
		showInspector := m.hasSelectedTask() && m.inspectorMode != inspectorHidden
		if showInspector && m.inspectorMode == inspectorExpanded {
			mainHeight = 0
			detailHeight = availableHeight
		} else if showInspector {
			detailHeight = min(10, max(8, availableHeight/4))
			mainHeight = max(3, availableHeight-detailHeight-1)
			detailHeight = max(3, availableHeight-mainHeight-1)
		}
		if mainHeight > 0 {
			contentWidth := max(20, m.width-4)
			contentHeight := max(1, mainHeight-3)
			switch m.view {
			case 0:
				body = kanban.View(m.tasks, m.statuses, m.selected, contentWidth, contentHeight)
			case 1:
				body = table.View(m.tasks, m.selected, contentWidth, contentHeight, m.backend.Mode() == domain.ModeGlobal)
			case 2:
				body = calendar.View(m.tasks, m.calendarMonth, m.selected, contentWidth, contentHeight)
			case 3:
				body = gantt.View(m.tasks, m.calendarMonth, m.selected, contentWidth, contentHeight, m.ganttStartDay)
			case 4:
				body = trash.View(m.deleted, m.selected, contentHeight)
			case 5:
				body = settings.View(m.statuses, m.selected, contentHeight)
			}
			body = panelView("Vista principal", body, m.panelFocus == focusMain || !showInspector, m.width)
		}
		if showInspector {
			detail := m.tasks[m.selected]
			if m.detail != nil && m.detail.ID == detail.ID && m.detail.Origin.Key == detail.Source {
				detail = presenter.Tasks([]domain.Task{*m.detail})[0]
			}
			for _, other := range m.tasks {
				if other.Origin == detail.Origin && other.Source != detail.Source && detail.SourceKind == domain.OriginProject {
					detail.Origin = detail.Source
					break
				}
			}
			inspector := taskdetail.InspectorView(detail, m.history, m.inspectorCursor, m.width, detailHeight, m.panelFocus == focusInspector, m.inspectorMode == inspectorExpanded, m.inspectorPinned)
			if mainHeight == 0 {
				body = inspector
			} else {
				body = lipgloss.JoinVertical(lipgloss.Left, body, "", inspector)
			}
		}
		if m.historyOpen {
			body = historyscreen.View(m.history, availableHeight)
		}
	}
	if m.paletteOpen {
		body = m.paletteView(availableHeight)
	} else if m.helpOpen {
		body = theme.Border.Render(m.helpView(availableHeight))
	} else if m.pickerOpen {
		body = m.pickerView(availableHeight)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, "", body, "", theme.Help.Render(footer))
}

func panelView(title, content string, active bool, width int) string {
	if active {
		title += " · ACTIVA"
	}
	style := theme.Border
	if active {
		style = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(theme.Primary).Padding(0, 1)
	}
	return style.Width(max(18, width-2)).Render(theme.Title.Render(title) + "\n" + content)
}

func (m Model) footerContent() string {
	if m.form.open {
		if m.form.loadFailed {
			return "FORMULARIO No se pudieron cargar datos seguros para guardar · Esc cerrar"
		}
		if m.form.confirmDiscard {
			return "CONFIRMAR  y/Enter descartar cambios · n/Esc continuar editando"
		}
		if m.form.compact {
			return "CAPTURA    Enter/Ctrl+S crear · Esc cancelar\nTEXTO     ←/→ cursor · Ctrl+←/→ palabra · Ctrl+W borrar palabra · Ctrl+U/K borrar línea · pegado admitido"
		}
		return "FORMULARIO Tab/Shift+Tab campo · ↑/↓ campo · ←/→ selector · Enter/Ctrl+S guardar · Esc cancelar\nTEXTO     ←/→ cursor · Ctrl+←/→ palabra · Ctrl+W borrar palabra · Ctrl+U/K borrar línea · pegado admitido"
	}
	if m.paletteOpen {
		return "PALETA    Escribir para buscar · ↑/↓ elegir · Enter ejecutar · Esc cancelar"
	}
	if m.helpOpen {
		return "AYUDA     ↑/↓ línea · PgUp/PgDn página · F1 o Esc cerrar · q salir"
	}
	if m.pickerOpen {
		keys := "SELECTOR  ↑/↓ elegir · Enter confirmar · Esc cancelar"
		if m.pickerAction == "recurrence-weekly-days" {
			keys = "SELECTOR  ↑/↓ elegir · Espacio marcar/desmarcar · Enter confirmar · Esc cancelar"
		}
		return keys + " · F1 ayuda general"
	}
	if m.historyOpen {
		return "HISTORIAL H o Esc cerrar · q salir · F1 ayuda general"
	}
	if m.confirmTrash != nil {
		labels := make([]string, 0, len(m.confirmAffected))
		for _, task := range m.confirmAffected {
			labels = append(labels, fmt.Sprintf("#%d %s", task.ID, task.Title))
		}
		return fmt.Sprintf("CONFIRMAR  Se eliminarán dependencias con %s\ny/Enter confirmar · n/Esc cancelar · F1 ayuda general", strings.Join(labels, ", "))
	}
	if m.inputMode {
		return fmt.Sprintf("FORMULARIO %s: %s█\nEnter guardar o aplicar · Esc cancelar · F1 ayuda general", m.inputLabel(), m.input)
	}

	contextCapabilities := m.backend.Capabilities("")
	context := keymap.Context{
		View:              m.view,
		Global:            m.backend.Mode() == domain.ModeGlobal,
		HasTask:           m.hasSelectedTask(),
		HasSubtask:        m.detail != nil && len(m.detail.Subtasks) > 0,
		CanCreateTask:     contextCapabilities.CanCreateTask,
		InspectorVisible:  m.hasSelectedTask() && m.inspectorMode != inspectorHidden,
		InspectorFocused:  m.panelFocus == focusInspector && m.inspectorMode != inspectorHidden,
		InspectorExpanded: m.inspectorMode == inspectorExpanded,
		InspectorPinned:   m.inspectorPinned,
	}
	if context.HasTask {
		taskCapabilities := m.backend.Capabilities(m.tasks[m.selected].Source)
		context.Recurring = m.tasks[m.selected].Recurring
		context.HasDependency = m.tasks[m.selected].Dependencies > 0
		context.CanCreateSubtask = taskCapabilities.CanCreateSubtask
		context.CanCreateDependency = taskCapabilities.CanCreateDependency
		context.CanCreateRecurrence = taskCapabilities.CanCreateRecurrence
	}
	if m.view == 4 {
		context.HasTask = m.selected >= 0 && m.selected < len(m.deleted)
	}
	if m.view == 5 && m.selected >= 0 && m.selected < len(m.statuses) {
		context.NormalStatus = m.statuses[m.selected].Kind == domain.StatusNormal
	}
	footer := keymap.Footer(context)
	if m.notice != "" {
		footer = "AVISO      " + m.notice + "\n" + footer
	}
	return footer
}

func (m Model) inputLabel() string {
	labels := map[string]string{
		"create":             "Nueva tarea",
		"edit":               "Editar título",
		"add-subtask":        "Nueva subtarea",
		"rename-subtask":     "Editar subtarea",
		"recurrence-monthly": "Día del mes (1–31; ejemplo: 15)",
		"start":              "Fecha de inicio (AAAA-MM-DD; vacío elimina)",
		"due":                "Vencimiento (AAAA-MM-DD; vacío elimina)",
		"filter-title":       "Buscar en título (vacío limpia)",
		"filter-markdown":    "Buscar en Markdown (vacío limpia)",
		"filter-origin":      "Origen por nombre o ruta (vacío limpia)",
		"filter-status-name": "Nombre exacto del estado (vacío limpia)",
		"filter-dates":       "Rango AAAA-MM-DD..AAAA-MM-DD (extremos opcionales)",
		"create-status":      "Nombre del nuevo estado",
		"rename-status":      "Nuevo nombre del estado",
	}
	if label, ok := labels[m.inputAction]; ok {
		return label
	}
	return "Valor"
}

func friendlyError(err error) string {
	switch {
	case errors.Is(err, domain.ErrForbidden):
		return "operación no disponible en este modo"
	case errors.Is(err, domain.ErrNotFound):
		return "elemento no encontrado; recarga la vista e inténtalo de nuevo"
	case errors.Is(err, domain.ErrDependencyCycle):
		return "esa dependencia produciría un ciclo"
	}
	message := err.Error()
	replacer := strings.NewReplacer(
		"required", "obligatorio",
		"invalid weekday", "día de semana no válido",
		"invalid month day", "día del mes no válido",
		"invalid ordinal", "ordinal no válido",
		"unsupported recurrence", "recurrencia no admitida",
		"weekly recurrence requires weekdays", "la recurrencia semanal requiere al menos un día",
		"must not precede start", "no puede ser anterior al inicio",
		"operation forbidden in this mode", "operación no disponible en este modo",
	)
	return replacer.Replace(message)
}

func (m Model) helpView(height int) string {
	lines := strings.Split(keymap.Full(m.backend.Mode() == domain.ModeGlobal), "\n")
	lines = wrapLines(lines, max(20, m.width-4))
	limit := max(1, height-2)
	maxStart := max(0, len(lines)-limit)
	start := min(m.helpScroll, maxStart)
	end := min(len(lines), start+limit)
	visible := append([]string(nil), lines[start:end]...)
	if start > 0 && len(visible) > 0 {
		visible[0] = fmt.Sprintf("↑ %d línea(s) anterior(es)", start)
	}
	if end < len(lines) && len(visible) > 0 {
		visible[len(visible)-1] = fmt.Sprintf("↓ %d línea(s) más", len(lines)-end)
	}
	return strings.Join(visible, "\n")
}

func wrapLines(lines []string, width int) []string {
	var out []string
	for _, line := range lines {
		if len([]rune(line)) <= width {
			out = append(out, line)
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
		words := strings.Fields(line)
		current := indent
		for _, word := range words {
			separator := ""
			if strings.TrimSpace(current) != "" {
				separator = " "
			}
			if len([]rune(current+separator+word)) > width && strings.TrimSpace(current) != "" {
				out = append(out, current)
				current = indent + word
			} else {
				current += separator + word
			}
		}
		out = append(out, current)
	}
	return out
}

func (m Model) filterSummary() string {
	var active []string
	if m.filter.Query != "" {
		active = append(active, "título="+m.filter.Query)
	}
	if m.filter.Markdown != "" {
		active = append(active, "markdown="+m.filter.Markdown)
	}
	if m.filter.Origin != "" {
		active = append(active, "origen="+m.filter.Origin)
	}
	if len(m.filter.StatusIDs) > 0 {
		active = append(active, fmt.Sprintf("estado=%d", m.filter.StatusIDs[0]))
	}
	if len(m.filter.StatusNames) > 0 {
		active = append(active, "estado="+m.filter.StatusNames[0])
	}
	if m.priorityFilter >= 0 {
		active = append(active, "prioridad="+domain.Priority(m.priorityFilter).String())
	}
	if m.filter.OnlyBlocked {
		active = append(active, "bloqueadas")
	}
	if m.filter.OnlyRecurring {
		active = append(active, "recurrentes")
	}
	if m.filter.From != nil || m.filter.To != nil {
		from, to := "", ""
		if m.filter.From != nil {
			from = m.filter.From.String()
		}
		if m.filter.To != nil {
			to = m.filter.To.String()
		}
		active = append(active, from+".."+to)
	}
	if m.filter.Sort != "updated" {
		active = append(active, "orden="+m.filter.Sort)
	}
	return strings.Join(active, " · ")
}
