package gantt

import (
	"fmt"
	"github.com/Polo123456789/tasks/internal/tui/presenter"
	"github.com/Polo123456789/tasks/internal/tui/theme"
	"strings"
)

func View(tasks []presenter.Task) string {
	lines := []string{theme.Title.Render("Gantt")}
	for _, t := range tasks {
		if t.Dates != "" && !t.Recurring {
			bar := "◆"
			if strings.Contains(t.Dates, "→") {
				bar = "●━━━━━━━━●"
			}
			lines = append(lines, fmt.Sprintf("%-24.24s %s %s", t.Title, bar, t.Dates))
		}
	}
	if len(lines) == 1 {
		lines = append(lines, "No hay tareas planificadas")
	}
	return strings.Join(lines, "\n")
}
