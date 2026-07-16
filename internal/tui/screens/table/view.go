package table

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

type column struct {
	title string
	width int
	right bool
}

func View(tasks []presenter.Task, selected, width, height int, global bool) string {
	if len(tasks) == 0 {
		return "No hay tareas"
	}

	columns := tableColumns(width, global)
	lines := []string{
		theme.Title.Render("  " + renderRow(columns, headerValues(columns))),
		theme.Help.Render("  " + renderDivider(columns)),
	}
	origins := disambiguatedOrigins(tasks)
	available := max(1, height-len(lines))
	start, end, showAbove, showBelow := visibleWindow(len(tasks), selected, available)
	if showAbove {
		lines = append(lines, fmt.Sprintf("  ↑ %d tarea(s) más", start))
	}
	for index := start; index < end; index++ {
		task := tasks[index]
		origin := origins[index]
		originColumn := hasColumn(columns, "ORIGEN")
		values := rowValues(task, origin, columnWidth(columns, "PLANIFICACIÓN"))
		if global && !originColumn && origin != "" {
			values["TÍTULO"] = titleWithOrigin(task.Title, origin, columns[1].width)
		}
		styles := map[string]lipgloss.Style(nil)
		if index != selected {
			styles = map[string]lipgloss.Style{"ESTADO": theme.Status(task.StatusKind, task.Status)}
			if task.Blocked {
				styles["BLOQ."] = theme.Blocked()
			}
		}
		row := renderRowStyled(columns, values, styles)
		if index == selected {
			row = theme.Selected.Render("› " + row)
		} else {
			row = "  " + row
		}
		lines = append(lines, row)
	}
	if showBelow {
		lines = append(lines, fmt.Sprintf("  ↓ %d tarea(s) más", len(tasks)-end))
	}
	return strings.Join(lines, "\n")
}

func tableColumns(width int, global bool) []column {
	statusWidth, priorityWidth, planWidth := 12, 9, 23
	if width < 110 {
		statusWidth, priorityWidth, planWidth = 11, 7, 17
	}
	columns := []column{
		{title: "ID", width: 4, right: true},
		{title: "TÍTULO"},
		{title: "ESTADO", width: statusWidth},
		{title: "PRIOR.", width: priorityWidth},
		{title: "PLANIFICACIÓN", width: planWidth},
		{title: "SUB", width: 7},
		{title: "BLOQ.", width: 5},
	}
	if global && width >= 130 {
		columns = append(columns, column{title: "ORIGEN", width: min(24, max(18, width/7))})
	}
	fixed := 0
	for _, item := range columns {
		fixed += item.width
	}
	separators := (len(columns) - 1) * 3
	titleWidth := max(8, width-2-fixed-separators)
	columns[1].width = titleWidth
	return columns
}

func headerValues(columns []column) map[string]string {
	values := make(map[string]string, len(columns))
	for _, item := range columns {
		values[item.title] = item.title
	}
	return values
}

func rowValues(task presenter.Task, origin string, planWidth int) map[string]string {
	plan := planning(task, planWidth)
	if task.Recurrence != "" {
		plan = "↻ " + task.Recurrence
	}
	if plan == "" {
		plan = "—"
	}
	subtasks := "—"
	if task.SubtasksTotal > 0 {
		subtasks = fmt.Sprintf("%d/%d", task.SubtasksDone, task.SubtasksTotal)
	}
	blocked := "—"
	if task.Blocked {
		blocked = "sí"
	}
	return map[string]string{
		"ID":            fmt.Sprintf("#%d", task.ID),
		"TÍTULO":        task.Title,
		"ESTADO":        task.Status,
		"PRIOR.":        task.Priority,
		"PLANIFICACIÓN": plan,
		"SUB":           subtasks,
		"BLOQ.":         blocked,
		"ORIGEN":        origin,
	}
}

func planning(task presenter.Task, width int) string {
	if task.Dates == "" || width >= 20 {
		return task.Dates
	}
	start, startOK := parseDate(task.Start)
	due, dueOK := parseDate(task.Due)
	switch {
	case startOK && dueOK && start.Equal(due):
		return fmt.Sprintf("%d %s %d", start.Day(), shortMonth(start.Month()), start.Year())
	case startOK && dueOK && start.Year() == due.Year() && start.Month() == due.Month():
		return fmt.Sprintf("%d–%d %s %d", start.Day(), due.Day(), shortMonth(start.Month()), start.Year())
	case startOK && dueOK && start.Year() == due.Year():
		return fmt.Sprintf("%d %s–%d %s %02d", start.Day(), shortMonth(start.Month()), due.Day(), shortMonth(due.Month()), due.Year()%100)
	case startOK && dueOK:
		return fmt.Sprintf("%d %s %02d–%d %s %02d", start.Day(), shortMonth(start.Month()), start.Year()%100, due.Day(), shortMonth(due.Month()), due.Year()%100)
	case startOK:
		return fmt.Sprintf("%d %s %d", start.Day(), shortMonth(start.Month()), start.Year())
	case dueOK:
		return fmt.Sprintf("%d %s %d", due.Day(), shortMonth(due.Month()), due.Year())
	default:
		return task.Dates
	}
}

func parseDate(value string) (time.Time, bool) {
	parsed, err := time.Parse("2006-01-02", value)
	return parsed, err == nil
}

func shortMonth(value time.Month) string {
	return [...]string{"ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"}[value-1]
}

func titleWithOrigin(title, origin string, width int) string {
	suffix := " [" + origin + "]"
	if lipgloss.Width(suffix) >= width {
		return truncateWidth(suffix, width)
	}
	return truncateWidth(title, width-lipgloss.Width(suffix)) + suffix
}

func renderRow(columns []column, values map[string]string) string {
	return renderRowStyled(columns, values, nil)
}

func renderRowStyled(columns []column, values map[string]string, styles map[string]lipgloss.Style) string {
	cells := make([]string, 0, len(columns))
	for _, item := range columns {
		cell := fit(values[item.title], item.width, item.right)
		if style, ok := styles[item.title]; ok {
			cell = style.Render(cell)
		}
		cells = append(cells, cell)
	}
	return strings.Join(cells, " │ ")
}

func renderDivider(columns []column) string {
	parts := make([]string, 0, len(columns))
	for _, item := range columns {
		parts = append(parts, strings.Repeat("─", item.width))
	}
	return strings.Join(parts, "─┼─")
}

func fit(value string, width int, right bool) string {
	value = truncateWidth(value, width)
	padding := max(0, width-lipgloss.Width(value))
	if right {
		return strings.Repeat(" ", padding) + value
	}
	return value + strings.Repeat(" ", padding)
}

func truncateWidth(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	runes := []rune(value)
	for len(runes) > 0 && lipgloss.Width(string(runes)+"…") > width {
		runes = runes[:len(runes)-1]
	}
	if len(runes) == 0 {
		return listutil.Truncate(value, width)
	}
	return string(runes) + "…"
}

func disambiguatedOrigins(tasks []presenter.Task) []string {
	sources := make(map[string]map[string]struct{})
	for _, task := range tasks {
		if sources[task.Origin] == nil {
			sources[task.Origin] = make(map[string]struct{})
		}
		sources[task.Origin][task.Source] = struct{}{}
	}
	origins := make([]string, len(tasks))
	for index, task := range tasks {
		origins[index] = task.Origin
		if task.Origin != "" && len(sources[task.Origin]) > 1 && task.SourceKind != domain.OriginGlobal {
			origins[index] = task.Source
		}
	}
	return origins
}

func visibleWindow(total, selected, capacity int) (start, end int, showAbove, showBelow bool) {
	limit := max(1, capacity)
	start, end = listutil.Bounds(total, selected, limit)
	for limit > 1 {
		markers := 0
		if start > 0 {
			markers++
		}
		if end < total {
			markers++
		}
		if limit+markers <= capacity {
			break
		}
		limit--
		start, end = listutil.Bounds(total, selected, limit)
	}
	markerSlots := max(0, capacity-limit)
	showAbove = start > 0 && markerSlots > 0
	if showAbove {
		markerSlots--
	}
	showBelow = end < total && markerSlots > 0
	return start, end, showAbove, showBelow
}

func hasColumn(columns []column, title string) bool {
	for _, item := range columns {
		if item.title == title {
			return true
		}
	}
	return false
}

func columnWidth(columns []column, title string) int {
	for _, item := range columns {
		if item.title == title {
			return item.width
		}
	}
	return 0
}
