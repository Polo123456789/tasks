package taskdetail

import (
	"fmt"
	"strings"

	"github.com/Polo123456789/tasks/internal/tui/presenter"
	"github.com/Polo123456789/tasks/internal/tui/screens/listutil"
	"github.com/Polo123456789/tasks/internal/tui/theme"
	"github.com/charmbracelet/glamour"
)

func View(task presenter.Task, selectedSubtask, width, height int) string {
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
	lines := []string{theme.Title.Render(task.Title), strings.Join(metadata, " · ")}
	if len(task.Subtasks) > 0 {
		lines = append(lines, "", "Subtareas")
		start, end := listutil.Bounds(len(task.Subtasks), selectedSubtask, max(1, height-7))
		if start > 0 {
			lines = append(lines, fmt.Sprintf("  ↑ %d más", start))
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
			lines = append(lines, fmt.Sprintf("%s%s %s [%s]", prefix, mark, subtask.Title, subtask.Status))
		}
		if end < len(task.Subtasks) {
			lines = append(lines, fmt.Sprintf("  ↓ %d más", len(task.Subtasks)-end))
		}
	}
	if len(task.DependencyIDs) > 0 {
		ids := make([]string, 0, len(task.DependencyIDs))
		for _, id := range task.DependencyIDs {
			ids = append(ids, fmt.Sprint(id))
		}
		lines = append(lines, "", "Depende de: "+strings.Join(ids, ", "))
	}
	markdown := strings.TrimSpace(task.Markdown)
	if markdown != "" {
		fragment := strings.Split(markdown, "\n")
		if len(fragment) > 5 {
			fragment = append(fragment[:5], "…")
		}
		renderer, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(max(16, width-6)))
		if err == nil {
			if rendered, renderErr := renderer.Render(strings.Join(fragment, "\n")); renderErr == nil {
				markdown = strings.TrimSpace(rendered)
			} else {
				markdown = strings.Join(fragment, "\n")
			}
		} else {
			markdown = strings.Join(fragment, "\n")
		}
		lines = append(lines, "", markdown)
	}
	if width < 20 {
		width = 20
	}
	if height > 2 && len(lines) > height-2 {
		capacity := height - 2
		if capacity == 1 {
			lines = lines[:1]
		} else {
			lines = append(lines[:capacity-1], "…")
		}
	}
	return theme.Border.Width(width - 2).Render(strings.Join(lines, "\n"))
}
