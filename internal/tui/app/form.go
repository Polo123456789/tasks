package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	formTitle = iota
	formStatus
	formPriority
	formStart
	formDue
	formRecurrence
	formFieldCount
)

type textField struct {
	value  []rune
	cursor int
}

func newTextField(value string) textField {
	runes := []rune(value)
	return textField{value: runes, cursor: len(runes)}
}

func (f textField) String() string { return string(f.value) }

func (f *textField) insert(runes []rune) {
	clean := make([]rune, 0, len(runes))
	for _, r := range runes {
		if r == '\r' || r == '\n' || r == '\t' {
			r = ' '
		}
		clean = append(clean, r)
	}
	f.value = append(f.value, make([]rune, len(clean))...)
	copy(f.value[f.cursor+len(clean):], f.value[f.cursor:len(f.value)-len(clean)])
	copy(f.value[f.cursor:], clean)
	f.cursor += len(clean)
}

func wordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

func (f *textField) wordLeft() {
	for f.cursor > 0 && !wordRune(f.value[f.cursor-1]) {
		f.cursor--
	}
	for f.cursor > 0 && wordRune(f.value[f.cursor-1]) {
		f.cursor--
	}
}

func (f *textField) wordRight() {
	for f.cursor < len(f.value) && !wordRune(f.value[f.cursor]) {
		f.cursor++
	}
	for f.cursor < len(f.value) && wordRune(f.value[f.cursor]) {
		f.cursor++
	}
}

func (f *textField) update(key tea.KeyMsg) {
	if key.Paste {
		f.insert(key.Runes)
		return
	}
	switch key.String() {
	case "left", "ctrl+b":
		f.cursor = max(0, f.cursor-1)
	case "right", "ctrl+f":
		f.cursor = min(len(f.value), f.cursor+1)
	case "home", "ctrl+a":
		f.cursor = 0
	case "end", "ctrl+e":
		f.cursor = len(f.value)
	case "alt+b", "ctrl+left":
		f.wordLeft()
	case "alt+f", "ctrl+right":
		f.wordRight()
	case "backspace", "ctrl+h":
		if f.cursor > 0 {
			f.value = append(f.value[:f.cursor-1], f.value[f.cursor:]...)
			f.cursor--
		}
	case "delete", "ctrl+d":
		if f.cursor < len(f.value) {
			f.value = append(f.value[:f.cursor], f.value[f.cursor+1:]...)
		}
	case "ctrl+w", "alt+backspace":
		end := f.cursor
		f.wordLeft()
		f.value = append(f.value[:f.cursor], f.value[end:]...)
	case "ctrl+u":
		f.value = append([]rune(nil), f.value[f.cursor:]...)
		f.cursor = 0
	case "ctrl+k":
		f.value = f.value[:f.cursor]
	case " ":
		f.insert([]rune{' '})
	default:
		if key.Type == tea.KeyRunes && !key.Alt {
			f.insert(key.Runes)
		}
	}
}

func (f textField) view(active bool) string {
	if !active {
		return string(f.value)
	}
	before := string(f.value[:f.cursor])
	after := " "
	rest := ""
	if f.cursor < len(f.value) {
		after = string(f.value[f.cursor])
		rest = string(f.value[f.cursor+1:])
	}
	return before + lipgloss.NewStyle().Reverse(true).Render(after) + rest
}

type taskForm struct {
	open, loading, loadFailed, editing, compact, saving, confirmDiscard, conflict, conflictLoading bool
	confirmRemoteReload, keepAfterReview                                                           bool
	requestID                                                                                      uint64
	field                                                                                          int
	source, destination                                                                            string
	task                                                                                           domain.Task
	statuses                                                                                       []domain.Status
	statusIndex                                                                                    int
	priority                                                                                       domain.Priority
	text                                                                                           map[int]textField
	errors                                                                                         map[string]string
	original                                                                                       string
	remote                                                                                         *domain.Task
}

type formLoaded struct {
	requestID   uint64
	editing     bool
	compact     bool
	source      string
	destination string
	task        domain.Task
	statuses    []domain.Status
	detailErr   error
	statusesErr error
}

type formSaved struct {
	task    domain.Task
	created bool
	err     error
}

type formConflictReviewed struct {
	requestID uint64
	task      domain.Task
	err       error
}

func newTaskForm(message formLoaded) taskForm {
	form := taskForm{
		open: true, editing: message.editing, compact: message.compact, requestID: message.requestID, source: message.source,
		destination: message.destination, task: message.task,
		statuses:    append([]domain.Status(nil), message.statuses...),
		statusIndex: -1, priority: message.task.Priority, text: map[int]textField{},
		errors: map[string]string{},
	}
	form.text[formTitle] = newTextField(message.task.Title)
	if message.task.Start != nil {
		form.text[formStart] = newTextField(message.task.Start.String())
	} else {
		form.text[formStart] = newTextField("")
	}
	if message.task.Due != nil {
		form.text[formDue] = newTextField(message.task.Due.String())
	} else {
		form.text[formDue] = newTextField("")
	}
	recurrence := ""
	if message.task.Recurrence != nil {
		recurrence = message.task.Recurrence.Text()
	}
	form.text[formRecurrence] = newTextField(recurrence)
	for index, status := range form.statuses {
		if status.ID == message.task.StatusID || (message.task.StatusID == 0 && status.Initial) {
			form.statusIndex = index
			form.task.StatusID = status.ID
			break
		}
	}
	form.original = form.signature()
	return form
}

func (f taskForm) signature() string {
	return strings.Join([]string{
		f.text[formTitle].String(), fmt.Sprint(f.selectedStatusID()), f.priority.Key(),
		f.text[formStart].String(), f.text[formDue].String(), f.text[formRecurrence].String(),
	}, "\x00")
}

func (f taskForm) dirty() bool { return f.signature() != f.original }

func (f taskForm) selectedStatusID() int64 {
	if f.statusIndex >= 0 && f.statusIndex < len(f.statuses) {
		return f.statuses[f.statusIndex].ID
	}
	return f.task.StatusID
}

func (f taskForm) selectedStatus() string {
	if f.statusIndex >= 0 && f.statusIndex < len(f.statuses) {
		return f.statuses[f.statusIndex].Name
	}
	if f.task.Status.Name != "" {
		return f.task.Status.Name
	}
	return "Automático"
}

func (f *taskForm) cycleSelector(direction int) {
	switch f.field {
	case formStatus:
		if len(f.statuses) > 0 {
			if f.statusIndex < 0 {
				f.statusIndex = 0
				if direction < 0 {
					f.statusIndex = len(f.statuses) - 1
				}
			} else {
				f.statusIndex = (f.statusIndex + direction + len(f.statuses)) % len(f.statuses)
			}
		}
	case formPriority:
		count := int(domain.PriorityUrgent) + 1
		f.priority = domain.Priority((int(f.priority) + direction + count) % count)
	}
}

func (f *taskForm) clearFieldError() {
	name := []string{"title", "status", "priority", "start", "due", "recurrence"}[f.field]
	delete(f.errors, name)
}

func (f *taskForm) build(today domain.Date) (domain.Task, error) {
	f.errors = map[string]string{}
	task := f.task
	task.Title = strings.TrimSpace(f.text[formTitle].String())
	task.StatusID = f.selectedStatusID()
	task.Priority = f.priority
	parseDate := func(field int, name string) *domain.Date {
		value := strings.TrimSpace(f.text[field].String())
		if value == "" {
			return nil
		}
		date, err := domain.ParseDate(value)
		if err != nil {
			f.errors[name] = "usa AAAA-MM-DD"
			return nil
		}
		return &date
	}
	task.Start = parseDate(formStart, "start")
	task.Due = parseDate(formDue, "due")
	recurrenceText := strings.TrimSpace(f.text[formRecurrence].String())
	if recurrenceText == "" {
		task.Recurrence = nil
		task.RecurrenceAnchor = nil
	} else if recurrence, err := domain.ParseRecurrence(recurrenceText); err != nil {
		f.errors["recurrence"] = "usa daily, weekly:mon,thu, monthly:15, month-end o monthly-weekday:first:mon"
	} else {
		task.Recurrence = &recurrence
		if task.RecurrenceAnchor == nil {
			anchor := today
			task.RecurrenceAnchor = &anchor
		}
	}
	if len(f.errors) > 0 {
		return task, domain.ErrValidation
	}
	if err := domain.ValidateTask(task); err != nil {
		var validation domain.ValidationError
		if errors.As(err, &validation) {
			f.errors[validation.Field] = localizedValidation(validation.Field, validation.Message)
		}
		return task, err
	}
	return task, nil
}

func localizedValidation(field, message string) string {
	switch message {
	case "required":
		return "es obligatorio"
	case "must not precede start":
		return "no puede ser anterior al inicio"
	case "recurring tasks cannot have dates":
		return "la recurrencia no admite fechas de inicio o vencimiento"
	case "invalid":
		return "valor no válido"
	}
	return friendlyError(domain.ValidationError{Field: field, Message: message})
}

func (m Model) openTaskForm(editing, compact bool) (tea.Model, tea.Cmd) {
	if editing && !m.hasSelectedTask() {
		return m, nil
	}
	source := ""
	destination := m.backend.ContextLabel()
	var selectedID int64
	if editing {
		selected := m.tasks[m.selected]
		source, selectedID = selected.Source, selected.ID
		destination = selected.Origin
		if destination == "" {
			destination = selected.Source
		}
	} else if m.backend.Mode() == domain.ModeGlobal {
		destination = "Global · origen propio"
	}
	m.err = nil
	m.nextFormRequestID++
	requestID := m.nextFormRequestID
	if compact {
		m.form = newTaskForm(formLoaded{requestID: requestID, compact: true, destination: destination})
		return m, nil
	}
	m.form = taskForm{open: true, loading: true, editing: editing, requestID: requestID, source: source, destination: destination}
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var task domain.Task
		var detailErr error
		if editing {
			task, detailErr = m.backend.Detail(ctx, source, selectedID)
		}
		statuses, statusErr := m.backend.FormStatuses(ctx, source)
		return formLoaded{requestID: requestID, editing: editing, source: source, destination: destination, task: task, statuses: statuses, detailErr: detailErr, statusesErr: statusErr}
	}
}

func (m Model) updateTaskForm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.form.loading || m.form.saving {
		if key.String() == "esc" && !m.form.saving {
			m.form = taskForm{}
		}
		return m, nil
	}
	if m.form.loadFailed {
		if key.String() == "esc" {
			m.form = taskForm{}
		}
		return m, nil
	}
	if m.form.conflict {
		if m.form.confirmRemoteReload {
			switch key.String() {
			case "y", "Y", "enter":
				m.form = taskForm{}
				m.loading = true
				return m, m.load(m.view == 4)
			case "n", "N", "esc":
				m.form.confirmRemoteReload = false
			}
			return m, nil
		}
		switch key.String() {
		case "r":
			m.form.confirmRemoteReload = true
			return m, nil
		case "v":
			if m.form.task.ID == 0 || m.form.conflictLoading {
				return m, nil
			}
			requestID, source, id := m.form.requestID, m.form.source, m.form.task.ID
			m.form.conflictLoading = true
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				task, err := m.backend.Detail(ctx, source, id)
				return formConflictReviewed{requestID: requestID, task: task, err: err}
			}
		case "k", "K", "esc":
			if key.String() == "esc" {
				m.form.conflict = false
				m.form.remote = nil
				delete(m.form.errors, "form")
				return m, nil
			}
			if m.form.remote != nil {
				m.form.rebaseOntoRemote()
				return m, nil
			}
			if !m.form.conflictLoading {
				requestID, source, id := m.form.requestID, m.form.source, m.form.task.ID
				m.form.conflictLoading = true
				m.form.keepAfterReview = true
				return m, func() tea.Msg {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					task, err := m.backend.Detail(ctx, source, id)
					return formConflictReviewed{requestID: requestID, task: task, err: err}
				}
			}
			return m, nil
		}
		return m, nil
	}
	if m.form.confirmDiscard {
		switch key.String() {
		case "y", "Y", "enter":
			m.form = taskForm{}
		case "n", "N", "esc":
			m.form.confirmDiscard = false
		}
		return m, nil
	}
	switch key.String() {
	case "ctrl+o":
		if m.openFormDatePicker() {
			return m, nil
		}
	case "esc":
		if m.form.dirty() {
			m.form.confirmDiscard = true
		} else {
			m.form = taskForm{}
		}
		return m, nil
	case "tab", "down":
		limit := formFieldCount
		if m.form.compact {
			limit = 1
		}
		m.form.field = (m.form.field + 1) % limit
		return m, nil
	case "shift+tab", "up":
		limit := formFieldCount
		if m.form.compact {
			limit = 1
		}
		m.form.field = (m.form.field + limit - 1) % limit
		return m, nil
	case "enter", "ctrl+s":
		task, err := m.form.build(m.today)
		if err != nil {
			return m, nil
		}
		m.form.saving = true
		created := task.ID == 0
		source := m.form.source
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			saved, saveErr := m.backend.SaveTask(ctx, source, task)
			return formSaved{task: saved, created: created, err: saveErr}
		}
	case "left":
		if m.form.field == formStatus || m.form.field == formPriority {
			m.form.cycleSelector(-1)
			m.form.clearFieldError()
			return m, nil
		}
	case "right", " ":
		if m.form.field == formStatus || m.form.field == formPriority {
			m.form.cycleSelector(1)
			m.form.clearFieldError()
			return m, nil
		}
	}
	if field, ok := m.form.text[m.form.field]; ok {
		field.update(key)
		m.form.text[m.form.field] = field
		m.form.clearFieldError()
	}
	return m, nil
}

func (f *taskForm) rebaseOntoRemote() {
	if f.remote == nil {
		return
	}
	f.task = *f.remote
	f.remote = nil
	f.conflict = false
	f.conflictLoading = false
	f.keepAfterReview = false
	delete(f.errors, "form")
}

func (f taskForm) view(width, height int) string {
	if f.loading {
		return theme.Border.Width(max(30, width-4)).Render(theme.Title.Render("Formulario de tarea") + "\n\nCargando datos y estados…")
	}
	title := "Nueva tarea"
	if f.editing {
		title = "Editar tarea"
	} else if f.compact {
		title = "Captura compacta"
	}
	lines := []string{theme.Title.Render(title), theme.Help.Render("Destino/origen: " + f.destination), ""}
	if message := f.errors["form"]; message != "" {
		lines = append(lines, theme.Help.Foreground(theme.Danger).Render("⚠ "+message), "")
	}
	labels := []string{"Título", "Estado", "Prioridad", "Inicio", "Vencimiento", "Recurrencia"}
	names := []string{"title", "status", "priority", "start", "due", "recurrence"}
	if f.compact {
		labels = labels[:1]
		names = names[:1]
	}
	for field, label := range labels {
		value := ""
		switch field {
		case formStatus:
			value = "‹ " + f.selectedStatus() + " ›"
		case formPriority:
			value = "‹ " + f.priority.String() + " ›"
		default:
			value = f.text[field].view(f.field == field)
		}
		prefix := "  "
		if f.field == field {
			prefix = "▸ "
		}
		line := fmt.Sprintf("%s%-13s %s", prefix, label+":", value)
		if f.field == field {
			line = theme.Selected.Render(line)
		}
		lines = append(lines, line)
		if message := f.errors[names[field]]; message != "" {
			lines = append(lines, theme.Help.Foreground(theme.Danger).Render("                 ⚠ "+message))
		}
	}
	if f.compact {
		lines = append(lines, "", theme.Help.Render("Solo se guardará el título; el resto usará los valores iniciales."))
	} else {
		lines = append(lines, "", theme.Help.Render("Recurrencia: daily · weekly:mon,thu · monthly:15 · month-end"))
	}
	if f.saving {
		lines = append(lines, "", "Guardando todos los campos…")
	}
	if f.confirmDiscard {
		lines = append(lines, "", theme.Help.Foreground(theme.Danger).Render("¿Descartar cambios sin guardar? y/Enter sí · n/Esc no"))
	}
	if f.remote != nil {
		lines = append(lines, "", theme.Help.Foreground(theme.Warning).Render(fmt.Sprintf("⚠ REMOTO v%d · %s · estado #%d · prioridad %s", f.remote.Version, f.remote.Title, f.remote.StatusID, f.remote.Priority.String())))
		lines = append(lines, theme.Help.Render("El borrador local permanece arriba sin modificar."))
	}
	content := strings.Join(lines, "\n")
	return theme.Border.Width(max(30, width-4)).Height(max(10, min(height-2, len(lines)+1))).Render(content)
}
