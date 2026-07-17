package taskdetail

import (
	"fmt"
	"strings"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/tui/presenter"
	"github.com/Polo123456789/tasks/internal/tui/screens/listutil"
	"github.com/Polo123456789/tasks/internal/tui/theme"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type RowKind string

const (
	RowField      RowKind = "field"
	RowSubtask    RowKind = "subtask"
	RowDependency RowKind = "dependency"
	RowHistory    RowKind = "history"
)

type Row struct {
	Kind  RowKind
	Field string
	Index int
	ID    int64
	Label string
	Value string
}

func Rows(task presenter.Task, history []domain.HistoryEvent) []Row {
	field := func(key, label, value string) Row {
		if strings.TrimSpace(value) == "" {
			value = "—"
		}
		return Row{Kind: RowField, Field: key, Label: label, Value: value}
	}
	rows := []Row{
		field("title", "Título", task.Title),
		field("status", "Estado", task.Status),
		field("priority", "Prioridad", task.Priority),
		field("start", "Inicio", task.Start),
		field("due", "Vencimiento", task.Due),
		field("recurrence", "Recurrencia", task.Recurrence),
		field("markdown", "Markdown", firstContentLine(task.Markdown)),
	}
	for index, subtask := range task.Subtasks {
		mark := "○"
		if subtask.Done {
			mark = "✓"
		}
		rows = append(rows, Row{Kind: RowSubtask, Index: index, ID: subtask.ID, Label: "Subtarea", Value: fmt.Sprintf("%s %s [%s]", mark, subtask.Title, subtask.Status)})
	}
	for index, id := range task.DependencyIDs {
		rows = append(rows, Row{Kind: RowDependency, Index: index, ID: id, Label: "Dependencia", Value: fmt.Sprintf("#%d", id)})
	}
	for index := len(history) - 1; index >= 0; index-- {
		event := history[index]
		value := historyEventName(event.Kind)
		if event.Detail != "" {
			value += " · " + event.Detail
		}
		rows = append(rows, Row{Kind: RowHistory, Index: index, ID: event.ID, Label: "Historial", Value: event.CreatedAt.Local().Format("2006-01-02 15:04") + " · " + value})
	}
	return rows
}

func historyEventName(kind string) string {
	names := map[string]string{
		"created": "creada", "updated": "editada", "title_changed": "título cambiado",
		"status_changed": "estado cambiado", "priority_changed": "prioridad cambiada",
		"completed": "finalizada", "cancelled": "cancelada", "reopened": "reabierta",
		"subtask_added": "subtarea creada", "subtask_updated": "subtarea actualizada",
		"dependency_added": "dependencia creada", "dependency_removed": "dependencia eliminada",
		"trashed": "enviada a papelera", "restored": "restaurada", "recurrence_reset": "recurrencia reiniciada",
	}
	if name, ok := names[kind]; ok {
		return name
	}
	return kind
}

func firstContentLine(value string) string {
	for _, line := range strings.Split(value, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

func InspectorView(task presenter.Task, history []domain.HistoryEvent, selected, width, height int, active, expanded, pinned bool) string {
	width = max(20, width)
	height = max(3, height)
	rows := Rows(task, history)
	if len(rows) == 0 {
		selected = 0
	} else {
		selected = max(0, min(selected, len(rows)-1))
	}
	title := "Inspector"
	if active {
		title += " · ACTIVO"
	}
	if expanded {
		title += " · EXPANDIDO"
	}
	if pinned {
		title += " · FIJADO"
	}
	titleLine := title + " · " + truncate(task.Title, max(1, width-len(title)-10))
	if task.SubtasksTotal > 0 {
		titleLine += fmt.Sprintf(" · subtareas %d/%d", task.SubtasksDone, task.SubtasksTotal)
	}
	lines := []string{theme.Title.Render(truncate(titleLine, max(1, width-6)))}
	capacity := max(1, height-3)
	start, end := listutil.Bounds(len(rows), selected, capacity)
	if start > 0 && capacity > 1 {
		lines = append(lines, theme.Help.Render(fmt.Sprintf("↑ %d elemento(s) anterior(es)", start)))
		capacity--
		start, end = listutil.Bounds(len(rows), selected, capacity)
	}
	for index := start; index < end && len(lines) < height-2; index++ {
		row := rows[index]
		label := fmt.Sprintf("%-11s %s", row.Label, row.Value)
		label = truncate(label, max(1, width-6))
		prefix := "  "
		if index == selected {
			prefix = "› "
			if active {
				lines = append(lines, theme.Selected.Render(prefix+label))
				continue
			}
		}
		lines = append(lines, prefix+label)
	}
	if end < len(rows) && len(lines) < height-2 {
		lines = append(lines, theme.Help.Render(fmt.Sprintf("↓ %d elemento(s) más", len(rows)-end)))
	}
	style := theme.Border
	if active {
		style = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(theme.Primary).Padding(0, 1)
	}
	return style.Width(width - 2).Render(strings.Join(lines, "\n"))
}

func View(task presenter.Task, selectedSubtask, width, height int) string {
	width = max(20, width)
	height = max(3, height)
	contentWidth := max(1, width-4)
	contentHeight := max(1, height-2)

	metadata := []string{fmt.Sprintf("tarea #%d", task.ID), task.Status, task.Priority}
	if task.Origin != "" {
		metadata = append(metadata, "origen "+task.Origin)
	}
	if task.Dates != "" {
		metadata = append(metadata, task.Dates)
	}
	if task.Recurring {
		metadata = append(metadata, "recurrente")
	}
	if task.Blocked {
		metadata = append(metadata, "bloqueada automáticamente")
	}
	if task.SubtasksTotal > 0 {
		metadata = append(metadata, fmt.Sprintf("subtareas %d/%d", task.SubtasksDone, task.SubtasksTotal))
	}
	if task.Dependencies > 0 {
		metadata = append(metadata, fmt.Sprintf("dependencias %d", task.Dependencies))
	}

	header := []string{theme.Title.Render(truncate(task.Title, contentWidth))}
	header = append(header, wrap(strings.Join(metadata, " · "), contentWidth, 3)...)
	remainingHeight := max(0, contentHeight-len(header)-1)
	if remainingHeight > 0 {
		leftWidth, rightWidth := columnWidths(contentWidth)
		left := detailLines(task, selectedSubtask, leftWidth, remainingHeight)
		right := markdownLines(task.Markdown, rightWidth, remainingHeight)
		columns := lipgloss.JoinHorizontal(
			lipgloss.Top,
			lipgloss.NewStyle().Width(leftWidth).Render(strings.Join(left, "\n")),
			theme.Help.Render(" │ "),
			lipgloss.NewStyle().Width(rightWidth).Render(strings.Join(right, "\n")),
		)
		header = append(header, "", columns)
	}

	physicalLines := strings.Split(strings.Join(header, "\n"), "\n")
	if len(physicalLines) > contentHeight {
		physicalLines = physicalLines[:contentHeight]
		physicalLines[len(physicalLines)-1] = truncateANSI(physicalLines[len(physicalLines)-1], contentWidth-2) + theme.Help.Render(" …")
	}
	return theme.Border.Width(width - 2).Render(strings.Join(physicalLines, "\n"))
}

func columnWidths(width int) (int, int) {
	right := min(50, max(24, width/3))
	left := max(1, width-right-3)
	if left < 24 {
		left = max(1, width/2-2)
		right = max(1, width-left-3)
	}
	return left, right
}

func detailLines(task presenter.Task, selectedSubtask, width, height int) []string {
	lines := []string{"Subtareas"}
	if len(task.Subtasks) == 0 {
		lines = append(lines, theme.Help.Render("Sin subtareas"))
	} else if height > 1 {
		capacity := max(1, height-1)
		statusWidth := subtaskStatusWidth(task.Subtasks, width)
		taskLimit := capacity
		start, end := listutil.Bounds(len(task.Subtasks), selectedSubtask, taskLimit)
		for taskLimit > 1 {
			markers := 0
			if start > 0 {
				markers++
			}
			if end < len(task.Subtasks) {
				markers++
			}
			if taskLimit+markers <= capacity {
				break
			}
			taskLimit--
			start, end = listutil.Bounds(len(task.Subtasks), selectedSubtask, taskLimit)
		}
		markerSlots := capacity - taskLimit
		showAbove := start > 0 && markerSlots > 0
		if showAbove {
			markerSlots--
		}
		showBelow := end < len(task.Subtasks) && markerSlots > 0
		if showAbove {
			lines = append(lines, fmt.Sprintf("↑ %d más", start))
		}
		for index := start; index < end; index++ {
			subtask := task.Subtasks[index]
			mark := "○"
			if subtask.Done {
				mark = "✓"
			}
			prefix := "  "
			if index == selectedSubtask {
				prefix = "› "
			}
			lead := prefix + mark + " "
			status := "[" + subtask.Status + "]"
			gap := 0
			if statusWidth > 0 {
				gap = 2
			}
			titleWidth := max(1, width-lipgloss.Width(lead)-gap-statusWidth)
			line := lead + fitCell(subtask.Title, titleWidth)
			if statusWidth > 0 {
				line += "  " + fitCell(status, statusWidth)
			}
			lines = append(lines, strings.TrimRight(line, " "))
		}
		if showBelow {
			lines = append(lines, fmt.Sprintf("↓ %d más", len(task.Subtasks)-end))
		}
	}
	if len(task.DependencyIDs) > 0 && len(lines) < height {
		ids := make([]string, 0, len(task.DependencyIDs))
		for _, id := range task.DependencyIDs {
			ids = append(ids, fmt.Sprintf("#%d", id))
		}
		lines = append(lines, truncate("Depende de "+strings.Join(ids, ", "), width))
	}
	return lines[:min(len(lines), height)]
}

func subtaskStatusWidth(subtasks []presenter.Subtask, width int) int {
	statusWidth := 0
	for _, subtask := range subtasks {
		statusWidth = max(statusWidth, lipgloss.Width("["+subtask.Status+"]"))
	}
	return min(statusWidth, max(0, min(width/3, width-8)))
}

func fitCell(value string, width int) string {
	value = truncateANSI(value, width)
	return value + strings.Repeat(" ", max(0, width-lipgloss.Width(value)))
}

func markdownLines(markdown string, width, height int) []string {
	lines := []string{"Markdown"}
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return append(lines, theme.Help.Render("Sin contenido"))
	}
	if height <= 1 {
		return lines
	}
	previewHeight := min(3, height-1)
	rendered := markdown
	renderer, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(max(12, width-2)))
	if err == nil {
		if value, renderErr := renderer.Render(markdown); renderErr == nil {
			rendered = value
		}
	}
	all := compactLines(rendered)
	if len(all) == 0 {
		return append(lines, theme.Help.Render("Sin contenido"))
	}
	truncated := len(all) > previewHeight || len(compactLines(markdown)) > previewHeight
	all = all[:min(len(all), previewHeight)]
	if truncated {
		all[len(all)-1] += theme.Help.Render(" …")
	}
	return append(lines, all...)
}

func compactLines(value string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(value), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func wrap(value string, width, limit int) []string {
	if width <= 0 || value == "" {
		return nil
	}
	words := strings.Fields(value)
	lines := make([]string, 0, limit)
	current := ""
	for _, word := range words {
		candidate := word
		if current != "" {
			candidate = current + " " + word
		}
		if len([]rune(candidate)) <= width {
			current = candidate
			continue
		}
		if current != "" {
			lines = append(lines, current)
			if len(lines) == limit {
				lines[len(lines)-1] = truncate(lines[len(lines)-1], max(1, width-1)) + "…"
				return lines
			}
		}
		current = word
	}
	if current != "" && len(lines) < limit {
		lines = append(lines, truncate(current, width))
	}
	return lines
}

func truncate(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 1 {
		return string(runes[:max(0, width)])
	}
	return string(runes[:width-1]) + "…"
}

// truncateANSI is only used as a final safety net for a line already styled by
// Lip Gloss or Glamour. Lip Gloss measures the printable width without counting
// escape sequences, so styling remains intact.
func truncateANSI(value string, width int) string {
	return lipgloss.NewStyle().MaxWidth(max(1, width)).Render(value)
}
