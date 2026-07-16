package trash

import (
	"fmt"
	"github.com/Polo123456789/tasks/internal/tui/presenter"
	"github.com/Polo123456789/tasks/internal/tui/screens/listutil"
	"github.com/Polo123456789/tasks/internal/tui/theme"
	"strings"
	"time"
)

func View(tasks []presenter.Task, selected, height int) string {
	lines := []string{theme.Title.Render("Papelera (30 días)")}
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
	for index := start; index < end; index++ {
		t := tasks[index]
		lifecycle := ""
		if deleted, err := time.Parse("2006-01-02", t.DeletedAt); err == nil {
			lifecycle = fmt.Sprintf(" · eliminada %s · vence %s", t.DeletedAt, deleted.AddDate(0, 0, 30).Format("2006-01-02"))
		}
		project := ""
		if t.Project != "" {
			name := t.Project
			if len(sources[t.Project]) > 1 {
				name = t.Source
			}
			project = " [" + name + "]"
		}
		line := "  " + t.Title + project + lifecycle
		if index == selected {
			line = theme.Selected.Render("› " + t.Title + project + lifecycle)
		}
		lines = append(lines, line)
	}
	if end < len(tasks) {
		lines = append(lines, fmt.Sprintf("↓ %d tarea(s) más", len(tasks)-end))
	}
	if len(tasks) == 0 {
		lines = append(lines, "Vacía")
	}
	return strings.Join(lines, "\n")
}
