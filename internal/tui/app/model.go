package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/ports"
	"github.com/Polo123456789/tasks/internal/tui/keymap"
	"github.com/Polo123456789/tasks/internal/tui/presenter"
	"github.com/Polo123456789/tasks/internal/tui/screens/calendar"
	"github.com/Polo123456789/tasks/internal/tui/screens/gantt"
	"github.com/Polo123456789/tasks/internal/tui/screens/kanban"
	"github.com/Polo123456789/tasks/internal/tui/screens/table"
	"github.com/Polo123456789/tasks/internal/tui/screens/trash"
	"github.com/Polo123456789/tasks/internal/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Backend interface {
	Mode() domain.Mode
	Capabilities() domain.Capabilities
	List(context.Context, ports.TaskFilter) ([]domain.Task, error)
	Statuses(context.Context) ([]domain.Status, error)
	Create(context.Context, string) (domain.Task, error)
	Trash(context.Context, string, int64, int64) ([]int64, error)
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
type Model struct {
	backend                       Backend
	tasks, deleted                []presenter.Task
	statuses                      []domain.Status
	view, selected, width, height int
	loading                       bool
	err                           error
	inputMode                     bool
	input                         string
	notice                        string
}

func New(b Backend) Model {
	view := "kanban"
	if b.Mode() == domain.ModeGlobal {
		view = "calendar"
	}
	return NewAt(b, view)
}
func NewAt(b Backend, view string) Model {
	views := map[string]int{"kanban": 0, "table": 1, "calendar": 2, "gantt": 3, "trash": 4}
	v, ok := views[view]
	if !ok {
		v = 0
	}
	return Model{backend: b, view: v, loading: true}
}
func (m Model) Init() tea.Cmd { return m.load(false) }
func (m Model) load(deleted bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		tasks, e := m.backend.List(ctx, ports.TaskFilter{IncludeDone: true, IncludeCancelled: true, IncludeDeleted: deleted})
		if e != nil {
			return loaded{err: e, trash: deleted}
		}
		statuses, e := m.backend.Statuses(ctx)
		return loaded{tasks, statuses, e, deleted}
	}
}
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
	case loaded:
		m.loading = false
		m.err = v.err
		if v.trash {
			m.deleted = presenter.Tasks(v.tasks)
		} else {
			m.tasks = presenter.Tasks(v.tasks)
			m.statuses = v.statuses
		}
	case created:
		m.inputMode = false
		m.err = v.err
		if v.err == nil {
			m.notice = "Tarea creada"
			return m, m.load(false)
		}
	case tea.KeyMsg:
		if m.inputMode {
			return m.updateInput(v)
		}
		switch v.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "right", "l":
			m.view = (m.view + 1) % 5
		case "left", "h":
			m.view = (m.view + 4) % 5
		case "down", "j":
			if m.selected < len(m.tasks)-1 {
				m.selected++
			}
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "r":
			m.loading = true
			return m, m.load(m.view == 4)
		case "n":
			if m.backend.Capabilities().CanCreateTask {
				m.inputMode = true
				m.input = ""
			}
		case "d":
			if m.selected < len(m.tasks) {
				t := m.tasks[m.selected]
				return m, func() tea.Msg {
					_, e := m.backend.Trash(context.Background(), t.Source, t.ID, t.Version)
					return loaded{err: e, tasks: nil, trash: false}
				}
			}
		}
	}
	if m.view == 4 && m.deleted == nil {
		return m, m.load(true)
	}
	return m, nil
}
func (m Model) updateInput(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		m.inputMode = false
	case "enter":
		title := strings.TrimSpace(m.input)
		if title != "" {
			return m, func() tea.Msg { t, e := m.backend.Create(context.Background(), title); return created{t, e} }
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
	header := theme.Title.Render("tasks") + "  " + []string{"Kanban", "Tabla", "Calendario", "Gantt", "Papelera"}[m.view]
	var body string
	if m.loading {
		body = "Cargando…"
	} else if m.err != nil {
		body = theme.Help.Foreground(theme.Danger).Render("Error: " + m.err.Error())
	} else {
		switch m.view {
		case 0:
			body = kanban.View(m.tasks, m.statuses, m.selected, m.width)
		case 1:
			body = table.View(m.tasks, m.selected)
		case 2:
			body = calendar.View(m.tasks, time.Now())
		case 3:
			body = gantt.View(m.tasks)
		case 4:
			body = trash.View(m.deleted)
		}
	}
	footer := keymap.Help
	if m.inputMode {
		footer = fmt.Sprintf("Nueva tarea: %s█  (enter guardar, esc cancelar)", m.input)
	} else if m.notice != "" {
		footer = m.notice + " · " + footer
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, "", body, "", theme.Help.Render(footer))
}
