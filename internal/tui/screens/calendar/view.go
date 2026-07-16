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
)

type event struct {
	date       time.Time
	text       string
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
	projectSources := sourcesByProject(tasks)
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
			label := task.Title
			if task.Project != "" {
				project := task.Project
				if len(projectSources[task.Project]) > 1 {
					project = task.Source
				}
				label += " [" + project + "]"
			}
			events = append(events, event{date: date, text: label, status: task.Status, statusKind: task.StatusKind, taskIndex: taskIndex})
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
			return events[i].text < events[j].text
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
		if start > 0 {
			lines = append(lines, fmt.Sprintf("↑ %d evento(s) más", start))
		}
		for index := start; index < end; index++ {
			item := events[index]
			prefix := fmt.Sprintf("%02d · ", item.date.Day())
			suffix := ""
			if item.status != "" {
				suffix = " · " + item.status
			}
			title := listutil.Truncate(item.text, max(1, width-2-len([]rune(prefix+suffix))))
			line := prefix + title + suffix
			if item.taskIndex == selected {
				line = theme.Selected.Render("› " + line)
			} else {
				if item.status != "" {
					line = prefix + title + " · " + theme.Status(item.statusKind, item.status).Render(item.status)
				}
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

func monthTitle(value time.Time) string {
	months := [...]string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}
	return fmt.Sprintf("%s %d", months[value.Month()-1], value.Year())
}

func sourcesByProject(tasks []presenter.Task) map[string]map[string]struct{} {
	sources := make(map[string]map[string]struct{})
	for _, task := range tasks {
		if sources[task.Project] == nil {
			sources[task.Project] = make(map[string]struct{})
		}
		sources[task.Project][task.Source] = struct{}{}
	}
	return sources
}

func pad(value string, width int) string {
	if utf8.RuneCountInString(value) > width {
		value = string([]rune(value)[:width])
	}
	return value + strings.Repeat(" ", width-utf8.RuneCountInString(value))
}
