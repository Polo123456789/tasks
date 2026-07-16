package gantt

import (
	"fmt"
	"strings"
	"time"

	"github.com/Polo123456789/tasks/internal/tui/presenter"
	"github.com/Polo123456789/tasks/internal/tui/screens/listutil"
	"github.com/Polo123456789/tasks/internal/tui/theme"
)

func View(tasks []presenter.Task, month time.Time, selected, width, height, startDay int) string {
	month = time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	daysInMonth := time.Date(month.Year(), month.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	labelWidth := min(24, max(10, width/3))
	if startDay < 1 {
		startDay = 1
	}
	if startDay > daysInMonth {
		startDay = daysInMonth
	}
	visibleDays := min(daysInMonth-startDay+1, max(1, width-labelWidth-4))
	endDay := startDay + visibleDays - 1
	lines := []string{theme.Title.Render(fmt.Sprintf("Gantt · %s · días %d–%d/%d", monthTitle(month), startDay, endDay, daysInMonth))}
	projectSources := sourcesByProject(tasks)
	header := make([]rune, visibleDays)
	for offset := 0; offset < visibleDays; offset++ {
		header[offset] = rune('0' + (startDay+offset)%10)
	}
	lines = append(lines, fmt.Sprintf("  %-*s %s", labelWidth, "TAREA", string(header)))
	type row struct {
		taskIndex int
		lines     []string
	}
	var rows []row
	for taskIndex, task := range tasks {
		if task.Recurring || (task.Start == "" && task.Due == "") {
			continue
		}
		start, startOK := parse(task.Start)
		due, dueOK := parse(task.Due)
		if !startOK {
			start = due
		}
		if !dueOK {
			due = start
		}
		monthEnd := time.Date(month.Year(), month.Month(), daysInMonth, 0, 0, 0, 0, time.UTC)
		if due.Before(month) || start.After(monthEnd) {
			continue
		}
		windowStart := time.Date(month.Year(), month.Month(), startDay, 0, 0, 0, 0, time.UTC)
		windowEnd := time.Date(month.Year(), month.Month(), endDay, 0, 0, 0, 0, time.UTC)
		bar := make([]rune, visibleDays)
		for i := range bar {
			bar[i] = ' '
		}
		outsideBefore := due.Before(windowStart)
		outsideAfter := start.After(windowEnd)
		from := max(startDay, start.Day())
		to := min(endDay, due.Day())
		if start.Before(month) {
			from = startDay
		}
		if due.After(monthEnd) {
			to = endDay
		}
		if outsideBefore {
			bar[0] = '←'
		} else if outsideAfter {
			bar[len(bar)-1] = '→'
		} else if from <= to {
			if start.Equal(due) || !startOK || !dueOK {
				bar[from-startDay] = '◆'
			} else {
				for day := from; day <= to; day++ {
					bar[day-startDay] = '━'
				}
				bar[from-startDay] = '●'
				bar[to-startDay] = '●'
			}
		}
		label := task.Title
		if task.Project != "" {
			project := task.Project
			if len(projectSources[task.Project]) > 1 {
				project = task.Source
			}
			label += " [" + project + "]"
		}
		label = truncate(label, labelWidth)
		line := fmt.Sprintf("%-*s %s", labelWidth, label, string(bar))
		if taskIndex == selected {
			line = theme.Selected.Render("› " + line)
		} else {
			line = "  " + line
		}
		rowLines := []string{line}
		if len(task.DependencyIDs) > 0 {
			ids := make([]string, 0, len(task.DependencyIDs))
			for _, id := range task.DependencyIDs {
				ids = append(ids, fmt.Sprintf("#%d", id))
			}
			rowLines = append(rowLines, fmt.Sprintf("%-*s ↳ depende de %s", labelWidth, "", strings.Join(ids, ", ")))
		}
		rows = append(rows, row{taskIndex: taskIndex, lines: rowLines})
	}
	if len(rows) == 0 {
		lines = append(lines, "No hay tareas planificadas en este mes")
	} else {
		selectedRow := 0
		for index, item := range rows {
			if item.taskIndex == selected {
				selectedRow = index
				break
			}
		}
		start, end := listutil.Bounds(len(rows), selectedRow, max(1, height-3))
		if start > 0 {
			lines = append(lines, fmt.Sprintf("↑ %d tarea(s) más", start))
		}
		for _, item := range rows[start:end] {
			lines = append(lines, item.lines...)
		}
		if end < len(rows) {
			lines = append(lines, fmt.Sprintf("↓ %d tarea(s) más", len(rows)-end))
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

func parse(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	date, err := time.Parse("2006-01-02", value)
	return date, err == nil
}

func truncate(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}
