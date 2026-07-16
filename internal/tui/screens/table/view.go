package table

import (
	"fmt"
	"github.com/Polo123456789/tasks/internal/tui/presenter"
	"github.com/Polo123456789/tasks/internal/tui/theme"
	"strings"
)

func View(tasks []presenter.Task, selected int) string {
	if len(tasks) == 0 {
		return "No hay tareas"
	}
	lines := []string{theme.Title.Render(fmt.Sprintf("%-24s %-15s %-10s %-22s %s", "TÍTULO", "ESTADO", "PRIORIDAD", "FECHAS", "PROYECTO"))}
	for i, t := range tasks {
		line := fmt.Sprintf("%-24.24s %-15.15s %-10s %-22s %s", t.Title, t.Status, t.Priority, t.Dates, t.Project)
		if i == selected {
			line = theme.Selected.Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
