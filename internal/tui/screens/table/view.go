package table

import (
	"fmt"
	"github.com/Polo123456789/tasks/internal/tui/presenter"
	"github.com/Polo123456789/tasks/internal/tui/screens/listutil"
	"github.com/Polo123456789/tasks/internal/tui/theme"
	"strings"
)

func View(tasks []presenter.Task, selected, width, height int) string {
	if len(tasks) == 0 {
		return "No hay tareas"
	}
	lines := []string{theme.Title.Render("ID · TÍTULO · ESTADO · PRIORIDAD · DATOS")}
	sources := make(map[string]map[string]struct{})
	for _, task := range tasks {
		if sources[task.Project] == nil {
			sources[task.Project] = make(map[string]struct{})
		}
		sources[task.Project][task.Source] = struct{}{}
	}
	start, end := listutil.Bounds(len(tasks), selected, max(1, height-3))
	if start > 0 {
		lines = append(lines, fmt.Sprintf("↑ %d tarea(s) más", start))
	}
	for i := start; i < end; i++ {
		t := tasks[i]
		project := t.Project
		if project != "" && len(sources[project]) > 1 {
			project = t.Source
		}
		blocked := ""
		if t.Blocked {
			blocked = "sí"
		}
		subtasks := ""
		if t.SubtasksTotal > 0 {
			subtasks = fmt.Sprintf("%d/%d", t.SubtasksDone, t.SubtasksTotal)
		}
		extra := make([]string, 0, 5)
		if t.Dates != "" {
			extra = append(extra, t.Dates)
		}
		if t.Recurrence != "" {
			extra = append(extra, "↻ "+t.Recurrence)
		}
		if blocked != "" {
			extra = append(extra, "bloqueada")
		}
		if subtasks != "" {
			extra = append(extra, "sub "+subtasks)
		}
		if project != "" {
			extra = append(extra, "proyecto "+project)
		}
		line := fmt.Sprintf("#%d · %s · %s · %s", t.ID, t.Title, t.Status, t.Priority)
		if len(extra) > 0 {
			line += " · " + strings.Join(extra, " · ")
		}
		line = listutil.Truncate(line, max(1, width-2))
		if i == selected {
			line = theme.Selected.Render("› " + line)
		} else {
			line = "  " + line
		}
		lines = append(lines, line)
	}
	if end < len(tasks) {
		lines = append(lines, fmt.Sprintf("↓ %d tarea(s) más", len(tasks)-end))
	}
	return strings.Join(lines, "\n")
}
