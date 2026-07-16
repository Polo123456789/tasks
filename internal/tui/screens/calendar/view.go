package calendar

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/tui/presenter"
	"github.com/Polo123456789/tasks/internal/tui/screens/listutil"
	"github.com/Polo123456789/tasks/internal/tui/theme"
	"github.com/charmbracelet/lipgloss"
)

type event struct {
	date       time.Time
	title      string
	origin     string
	status     string
	statusKind domain.StatusKind
	taskIndex  int
}

func View(tasks []presenter.Task, month time.Time, selected, width, height int) string {
	month = time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	cellWidth := max(2, (width-6)/7)
	if cellWidth > 16 {
		cellWidth = 16
	}
	counts := make(map[int]int)
	originSources := sourcesByOrigin(tasks)
	var events []event
	for taskIndex, task := range tasks {
		if task.Recurring {
			continue
		}
		seen := make(map[string]bool)
		for _, raw := range []string{task.Start, task.Due} {
			if raw == "" || seen[raw] {
				continue
			}
			seen[raw] = true
			date, err := time.Parse("2006-01-02", raw)
			if err != nil || date.Year() != month.Year() || date.Month() != month.Month() {
				continue
			}
			counts[date.Day()]++
			origin := ""
			if task.Origin != "" {
				origin = task.Origin
				if len(originSources[task.Origin]) > 1 && task.SourceKind != domain.OriginGlobal {
					origin = task.Source
				}
			}
			events = append(events, event{date: date, title: task.Title, origin: origin, status: task.Status, statusKind: task.StatusKind, taskIndex: taskIndex})
		}
	}
	lines := []string{theme.Title.Render(monthTitle(month))}
	if height >= 10 {
		headings := []string{"Lun", "Mar", "Mié", "Jue", "Vie", "Sáb", "Dom"}
		for i := range headings {
			headings[i] = pad(headings[i], cellWidth)
		}
		lines = append(lines, strings.Join(headings, " "))
		offset := (int(month.Weekday()) + 6) % 7
		days := time.Date(month.Year(), month.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
		for week := 0; week < 6; week++ {
			cells := make([]string, 7)
			hasDay := false
			for weekday := 0; weekday < 7; weekday++ {
				day := week*7 + weekday - offset + 1
				value := ""
				if day >= 1 && day <= days {
					hasDay = true
					value = fmt.Sprint(day)
					if counts[day] > 0 {
						value += fmt.Sprintf(" •%d", counts[day])
					}
				}
				cells[weekday] = pad(value, cellWidth)
			}
			if hasDay {
				lines = append(lines, strings.Join(cells, " "))
			}
		}
	} else {
		lines = append(lines, "Vista compacta: eventos del mes")
	}
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].date.Equal(events[j].date) {
			return events[i].title+events[i].origin < events[j].title+events[j].origin
		}
		return events[i].date.Before(events[j].date)
	})
	if len(events) == 0 {
		lines = append(lines, "", "No hay tareas con fechas en este mes")
	} else {
		lines = append(lines, "")
		selectedEvent := 0
		for index, item := range events {
			if item.taskIndex == selected {
				selectedEvent = index
				break
			}
		}
		available := max(1, height-len(lines)-3)
		start, end := listutil.Bounds(len(events), selectedEvent, available)
		layout := eventListLayout(events[start:end], width)
		if start > 0 {
			lines = append(lines, fmt.Sprintf("↑ %d evento(s) más", start))
		}
		for index := start; index < end; index++ {
			item := events[index]
			line := renderEventRow(item, layout, width, item.taskIndex != selected)
			if item.taskIndex == selected {
				line = theme.Selected.Render("› " + line)
			} else {
				line = "  " + line
			}
			lines = append(lines, line)
		}
		if end < len(events) {
			lines = append(lines, fmt.Sprintf("↓ %d evento(s) más", len(events)-end))
		}
	}
	if height > 0 && len(lines) > height {
		lines = append(lines[:height-1], "…")
	}
	return strings.Join(lines, "\n")
}

type calendarEventLayout struct {
	titleWidth, originWidth, statusWidth int
}

func eventListLayout(events []event, width int) calendarEventLayout {
	layout := calendarEventLayout{}
	maxTitleWidth := 0
	for _, item := range events {
		maxTitleWidth = max(maxTitleWidth, lipgloss.Width(item.title))
		if item.origin != "" {
			layout.originWidth = max(layout.originWidth, lipgloss.Width("["+item.origin+"]"))
		}
		layout.statusWidth = max(layout.statusWidth, lipgloss.Width(item.status))
	}
	layout.originWidth = min(layout.originWidth, min(24, max(0, width/4)))
	layout.statusWidth = min(layout.statusWidth, 18)
	fixedWidth := 7 // selection indent and "DD · "
	if layout.originWidth > 0 {
		fixedWidth += 2 + layout.originWidth
	}
	if layout.statusWidth > 0 {
		fixedWidth += 2 + layout.statusWidth
	}
	availableTitleWidth := width - fixedWidth
	if availableTitleWidth < 12 {
		return calendarEventLayout{}
	}
	layout.titleWidth = min(maxTitleWidth, availableTitleWidth)
	return layout
}

func renderEventRow(item event, layout calendarEventLayout, width int, styleStatus bool) string {
	prefix := fmt.Sprintf("%02d · ", item.date.Day())
	if layout.titleWidth == 0 {
		text := item.title
		if item.origin != "" {
			text += " [" + item.origin + "]"
		}
		suffix := ""
		if item.status != "" {
			suffix = " · " + item.status
		}
		title := listutil.Truncate(text, max(1, width-2-lipgloss.Width(prefix+suffix)))
		if styleStatus && item.status != "" {
			return prefix + title + " · " + theme.Status(item.statusKind, item.status).Render(item.status)
		}
		return prefix + title + suffix
	}

	line := prefix + fitCalendarCell(item.title, layout.titleWidth)
	if layout.originWidth > 0 {
		origin := ""
		if item.origin != "" {
			origin = "[" + item.origin + "]"
		}
		line += "  " + fitCalendarCell(origin, layout.originWidth)
	}
	if layout.statusWidth > 0 {
		status := listutil.Truncate(item.status, layout.statusWidth)
		if styleStatus && item.status != "" {
			status = theme.Status(item.statusKind, item.status).Render(status)
		}
		line += "  " + status
	}
	return strings.TrimRight(line, " ")
}

func fitCalendarCell(value string, width int) string {
	value = listutil.Truncate(value, width)
	return value + strings.Repeat(" ", max(0, width-lipgloss.Width(value)))
}

func monthTitle(value time.Time) string {
	months := [...]string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}
	return fmt.Sprintf("%s %d", months[value.Month()-1], value.Year())
}

func sourcesByOrigin(tasks []presenter.Task) map[string]map[string]struct{} {
	sources := make(map[string]map[string]struct{})
	for _, task := range tasks {
		if sources[task.Origin] == nil {
			sources[task.Origin] = make(map[string]struct{})
		}
		sources[task.Origin][task.Source] = struct{}{}
	}
	return sources
}

func pad(value string, width int) string {
	if utf8.RuneCountInString(value) > width {
		value = string([]rune(value)[:width])
	}
	return value + strings.Repeat(" ", width-utf8.RuneCountInString(value))
}
