package taskdetail

import (
	"fmt"
	"strings"

	"github.com/Polo123456789/tasks/internal/tui/presenter"
	"github.com/Polo123456789/tasks/internal/tui/screens/listutil"
	"github.com/Polo123456789/tasks/internal/tui/theme"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

func View(task presenter.Task, selectedSubtask, width, height int) string {
	width = max(20, width)
	height = max(3, height)
	contentWidth := max(1, width-4)
	contentHeight := max(1, height-2)

	metadata := []string{fmt.Sprintf("tarea #%d", task.ID), task.Status, task.Priority}
	if task.Project != "" {
		metadata = append(metadata, "proyecto "+task.Project)
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
			status := " [" + subtask.Status + "]"
			available := max(1, width-len([]rune(prefix+mark+" "+status)))
			lines = append(lines, prefix+mark+" "+truncate(subtask.Title, available)+status)
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
