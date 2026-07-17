package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type datePickerTarget uint8

const (
	dateFormStart datePickerTarget = iota
	dateFormDue
	dateInputStart
	dateInputDue
	dateFilterFrom
	dateFilterTo
)

type datePickerState struct {
	open       bool
	target     datePickerTarget
	focused    domain.Date
	current    *domain.Date
	today      domain.Date
	error      string
	filterFrom *domain.Date
	filterTo   *domain.Date
}

func copyDate(date *domain.Date) *domain.Date {
	if date == nil {
		return nil
	}
	copy := *date
	return &copy
}

func parsedOptionalDate(value string) *domain.Date {
	date, err := domain.ParseDate(strings.TrimSpace(value))
	if err != nil {
		return nil
	}
	return &date
}

func pickerFocus(current *domain.Date, fallback domain.Date) domain.Date {
	if current != nil {
		return *current
	}
	return fallback
}

func (m *Model) openFormDatePicker() bool {
	if !m.form.open || (m.form.field != formStart && m.form.field != formDue) {
		return false
	}
	target := dateFormStart
	if m.form.field == formDue {
		target = dateFormDue
	}
	draft := parsedOptionalDate(m.form.text[m.form.field].String())
	current := copyDate(m.form.task.Start)
	if target == dateFormDue {
		current = copyDate(m.form.task.Due)
	}
	fallback := pickerFocus(current, m.today)
	m.datePicker = datePickerState{open: true, target: target, focused: pickerFocus(draft, fallback), current: current, today: m.today}
	return true
}

func (m *Model) openInputDatePicker() bool {
	if !m.inputMode || (m.inputAction != "start" && m.inputAction != "due") || !m.hasSelectedTask() {
		return false
	}
	target := dateInputStart
	current := parsedOptionalDate(m.tasks[m.selected].Start)
	if m.inputAction == "due" {
		target = dateInputDue
		current = parsedOptionalDate(m.tasks[m.selected].Due)
	}
	draft := parsedOptionalDate(m.input)
	m.datePicker = datePickerState{
		open: true, target: target, focused: pickerFocus(draft, pickerFocus(current, m.today)), current: copyDate(current), today: m.today,
	}
	return true
}

func parseFilterDraft(value string, fallbackFrom, fallbackTo *domain.Date) (*domain.Date, *domain.Date) {
	fromText, toText, ok := strings.Cut(value, "..")
	if !ok {
		return copyDate(fallbackFrom), copyDate(fallbackTo)
	}
	var from, to *domain.Date
	if strings.TrimSpace(fromText) != "" {
		from = parsedOptionalDate(fromText)
		if from == nil {
			return copyDate(fallbackFrom), copyDate(fallbackTo)
		}
	}
	if strings.TrimSpace(toText) != "" {
		to = parsedOptionalDate(toText)
		if to == nil {
			return copyDate(fallbackFrom), copyDate(fallbackTo)
		}
	}
	return from, to
}

func (m *Model) openFilterDatePicker() bool {
	if !m.inputMode || m.inputAction != "filter-dates" {
		return false
	}
	from, to := parseFilterDraft(m.input, m.filter.From, m.filter.To)
	m.datePicker = datePickerState{
		open: true, target: dateFilterFrom, focused: pickerFocus(from, m.today), current: copyDate(m.filter.From), today: m.today,
		filterFrom: copyDate(from), filterTo: copyDate(to),
	}
	return true
}

func shiftPickerMonth(date domain.Date, amount int) domain.Date {
	t := date.Time()
	first := time.Date(t.Year(), t.Month()+time.Month(amount), 1, 0, 0, 0, 0, time.UTC)
	day := t.Day()
	days := time.Date(first.Year(), first.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > days {
		day = days
	}
	shifted, _ := domain.NewDate(first.Year(), first.Month(), day)
	return shifted
}

func (m Model) updateDatePicker(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	picker := &m.datePicker
	picker.error = ""
	switch key.String() {
	case "esc":
		picker.open = false
		return m, nil
	case "left", "up":
		picker.focused = picker.focused.AddDays(-1)
	case "right", "down":
		picker.focused = picker.focused.AddDays(1)
	case "pgup":
		picker.focused = picker.focused.AddDays(-7)
	case "pgdown":
		picker.focused = picker.focused.AddDays(7)
	case "[":
		picker.focused = shiftPickerMonth(picker.focused, -1)
	case "]":
		picker.focused = shiftPickerMonth(picker.focused, 1)
	case "home":
		picker.focused = picker.today
	case "x", "delete", "backspace":
		return m.confirmPickerDate(nil)
	case "enter":
		date := picker.focused
		return m.confirmPickerDate(&date)
	}
	return m, nil
}

func (m Model) confirmPickerDate(date *domain.Date) (tea.Model, tea.Cmd) {
	picker := &m.datePicker
	switch picker.target {
	case dateFormStart, dateFormDue:
		if date != nil {
			otherField := formStart
			if picker.target == dateFormStart {
				otherField = formDue
			}
			other := parsedOptionalDate(m.form.text[otherField].String())
			if picker.target == dateFormStart && other != nil && other.Before(*date) {
				picker.error = "El inicio no puede ser posterior al vencimiento guardado."
				return m, nil
			}
			if picker.target == dateFormDue && other != nil && date.Before(*other) {
				picker.error = "El vencimiento no puede ser anterior al inicio."
				return m, nil
			}
		}
		field := formStart
		name := "start"
		if picker.target == dateFormDue {
			field, name = formDue, "due"
		}
		value := ""
		if date != nil {
			value = date.String()
		}
		m.form.text[field] = newTextField(value)
		delete(m.form.errors, name)
		picker.open = false
	case dateInputStart, dateInputDue:
		if date != nil && m.hasSelectedTask() {
			other := parsedOptionalDate(m.tasks[m.selected].Start)
			if picker.target == dateInputStart {
				other = parsedOptionalDate(m.tasks[m.selected].Due)
			}
			if picker.target == dateInputStart && other != nil && other.Before(*date) {
				picker.error = "El inicio no puede ser posterior al vencimiento guardado."
				return m, nil
			}
			if picker.target == dateInputDue && other != nil && date.Before(*other) {
				picker.error = "El vencimiento no puede ser anterior al inicio."
				return m, nil
			}
		}
		m.input = ""
		if date != nil {
			m.input = date.String()
		}
		picker.open = false
	case dateFilterFrom:
		picker.filterFrom = copyDate(date)
		picker.target = dateFilterTo
		picker.current = copyDate(m.filter.To)
		picker.focused = pickerFocus(picker.filterTo, pickerFocus(picker.filterFrom, picker.today))
	case dateFilterTo:
		if date != nil && picker.filterFrom != nil && date.Before(*picker.filterFrom) {
			picker.error = "El final del rango no puede ser anterior al inicio."
			return m, nil
		}
		picker.filterTo = copyDate(date)
		from, to := "", ""
		if picker.filterFrom != nil {
			from = picker.filterFrom.String()
		}
		if picker.filterTo != nil {
			to = picker.filterTo.String()
		}
		m.input = from + ".." + to
		picker.open = false
	}
	return m, nil
}

var spanishMonths = [...]string{"", "enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}

func (p datePickerState) title() string {
	field := map[datePickerTarget]string{
		dateFormStart: "inicio", dateFormDue: "vencimiento", dateInputStart: "inicio", dateInputDue: "vencimiento",
		dateFilterFrom: "inicio del rango", dateFilterTo: "final del rango",
	}[p.target]
	t := p.focused.Time()
	return fmt.Sprintf("Calendario · %s · %s %d", field, spanishMonths[int(t.Month())], t.Year())
}

func calendarCell(date domain.Date, focused domain.Date, current *domain.Date, today domain.Date, width int) string {
	markers := ""
	if date.Equal(focused) {
		markers += ">"
	}
	if current != nil && date.Equal(*current) {
		markers += "="
	}
	if date.Equal(today) {
		markers += "*"
	}
	value := markers + fmt.Sprintf("%02d", date.Time().Day())
	if len([]rune(value)) > width {
		value = string([]rune(value)[:width])
	}
	return fmt.Sprintf("%-*s", width, value)
}

func padVisual(value string, width int) string {
	if padding := width - lipgloss.Width(value); padding > 0 {
		return value + strings.Repeat(" ", padding)
	}
	return value
}

func (p datePickerState) view(width, height int) string {
	contentWidth := max(35, width-6)
	cellWidth := max(5, min(10, contentWidth/7))
	weekWidth := cellWidth * 7
	lines := []string{theme.Title.Render(p.title()), theme.Help.Render("> foco · = guardada/aplicada · * hoy"), ""}
	days := []string{"Lun", "Mar", "Mié", "Jue", "Vie", "Sáb", "Dom"}
	for index := range days {
		days[index] = padVisual(days[index], cellWidth)
	}
	lines = append(lines, strings.Join(days, ""))
	focusTime := p.focused.Time()
	first, _ := domain.NewDate(focusTime.Year(), focusTime.Month(), 1)
	offset := (int(first.Weekday()) + 6) % 7
	cursor := first.AddDays(-offset)
	for week := 0; week < 6; week++ {
		row := strings.Builder{}
		for day := 0; day < 7; day++ {
			date := cursor.AddDays(week*7 + day)
			if date.Time().Month() != focusTime.Month() {
				row.WriteString(strings.Repeat(" ", cellWidth))
			} else {
				row.WriteString(calendarCell(date, p.focused, p.current, p.today, cellWidth))
			}
		}
		lines = append(lines, strings.TrimRight(row.String(), " "))
	}
	if p.error != "" {
		lines = append(lines, "", theme.Help.Foreground(theme.Danger).Render("✗ ERROR "+p.error))
	}
	lines = append(lines, "", theme.Help.Render("←/→/↑/↓ día · PgUp/PgDn semana · [/] mes · Home hoy"))
	lines = append(lines, theme.Help.Render("Enter elegir · x limpiar · Esc cancelar sin cambios"))
	if len(lines) > max(10, height-2) {
		lines = lines[:max(10, height-2)]
	}
	return theme.Border.Width(max(35, min(contentWidth, weekWidth+2))).Render(strings.Join(lines, "\n"))
}
