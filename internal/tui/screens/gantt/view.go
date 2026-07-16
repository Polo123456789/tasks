package gantt

import (
	"fmt"
	"strings"
	"time"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/tui/presenter"
	"github.com/Polo123456789/tasks/internal/tui/screens/listutil"
	"github.com/Polo123456789/tasks/internal/tui/theme"
	"github.com/charmbracelet/lipgloss"
)

type legendItem struct {
	name string
	kind domain.StatusKind
}

func View(tasks []presenter.Task, month time.Time, selected, width, height, startDay int) string {
	month = time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	daysInMonth := time.Date(month.Year(), month.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if startDay < 1 {
		startDay = 1
	}
	if startDay > daysInMonth {
		startDay = daysInMonth
	}
	remainingDays := daysInMonth - startDay + 1
	minimumLabelWidth := min(24, max(10, width/4))
	timelineWidth := max(1, width-minimumLabelWidth-3)
	cellWidth := min(4, max(1, timelineWidth/remainingDays))
	visibleDays := min(remainingDays, max(1, timelineWidth/cellWidth))
	labelWidth := min(40, max(minimumLabelWidth, width-visibleDays*cellWidth-3))
	endDay := startDay + visibleDays - 1
	titleLine := theme.Title.Render(fmt.Sprintf("Gantt · %s · días %d–%d/%d", monthTitle(month), startDay, endDay, daysInMonth))
	projectSources := sourcesByProject(tasks)
	header := make([]string, 0, visibleDays)
	for offset := 0; offset < visibleDays; offset++ {
		day := startDay + offset
		label := fmt.Sprint(day)
		if cellWidth <= 2 && day != 1 && day%5 != 0 {
			label = ""
		}
		if cellWidth == 1 && label != "" {
			label = fmt.Sprint(day % 10)
		}
		header = append(header, center(label, cellWidth))
	}
	headerLine := fmt.Sprintf("  %-*s %s", labelWidth, "TAREA", strings.Join(header, ""))
	type row struct {
		taskIndex int
		line      string
	}
	var rows []row
	var statuses []legendItem
	seenStatuses := make(map[string]bool)
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
		if task.Status != "" {
			key := string(task.StatusKind) + "\x00" + task.Status
			if !seenStatuses[key] {
				seenStatuses[key] = true
				statuses = append(statuses, legendItem{name: task.Status, kind: task.StatusKind})
			}
		}
		windowStart := time.Date(month.Year(), month.Month(), startDay, 0, 0, 0, 0, time.UTC)
		windowEnd := time.Date(month.Year(), month.Month(), endDay, 0, 0, 0, 0, time.UTC)
		bar := make([]string, visibleDays)
		for i := range bar {
			bar[i] = strings.Repeat(" ", cellWidth)
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
			bar[0] = center("←", cellWidth)
		} else if outsideAfter {
			bar[len(bar)-1] = center("→", cellWidth)
		} else if from <= to {
			if start.Equal(due) || !startOK || !dueOK {
				bar[from-startDay] = center("◆", cellWidth)
			} else {
				for day := from; day <= to; day++ {
					bar[day-startDay] = strings.Repeat("━", cellWidth)
				}
				bar[from-startDay] = "●" + strings.Repeat("━", cellWidth-1)
				bar[to-startDay] = strings.Repeat("━", cellWidth-1) + "●"
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
		if len(task.DependencyIDs) > 0 {
			ids := make([]string, 0, len(task.DependencyIDs))
			for _, id := range task.DependencyIDs {
				ids = append(ids, fmt.Sprintf("#%d", id))
			}
			dependency := " ↳" + strings.Join(ids, ",")
			label = truncate(label, max(1, labelWidth-len([]rune(dependency)))) + dependency
		}
		label = truncate(label, labelWidth)
		barText := strings.Join(bar, "")
		line := fmt.Sprintf("%-*s %s", labelWidth, label, barText)
		if taskIndex == selected {
			line = theme.Selected.Render("› " + line)
		} else {
			line = "  " + fmt.Sprintf("%-*s %s", labelWidth, label, theme.Status(task.StatusKind, task.Status).Render(barText))
		}
		rows = append(rows, row{taskIndex: taskIndex, line: line})
	}
	lines := []string{titleLine}
	if legend := statusLegend(statuses, width); legend != "" {
		lines = append(lines, legend)
	}
	lines = append(lines, headerLine)
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
		start, end := listutil.Bounds(len(rows), selectedRow, max(1, height-len(lines)-1))
		if start > 0 {
			lines = append(lines, fmt.Sprintf("↑ %d tarea(s) más", start))
		}
		for _, item := range rows[start:end] {
			lines = append(lines, item.line)
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

func statusLegend(items []legendItem, width int) string {
	if len(items) == 0 {
		return ""
	}
	line := theme.Help.Render("Estados  ")
	for index, item := range items {
		separator := ""
		if index > 0 {
			separator = theme.Help.Render(" · ")
		}
		segment := separator + theme.Status(item.kind, item.name).Render(item.name)
		if lipgloss.Width(line+segment) > width {
			if lipgloss.Width(line+theme.Help.Render(" …")) <= width {
				line += theme.Help.Render(" …")
			}
			break
		}
		line += segment
	}
	return line
}

func center(value string, width int) string {
	padding := max(0, width-len([]rune(value)))
	left := padding / 2
	return strings.Repeat(" ", left) + value + strings.Repeat(" ", padding-left)
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
