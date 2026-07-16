package calendar

import (
	"fmt"
	"github.com/Polo123456789/tasks/internal/tui/presenter"
	"github.com/Polo123456789/tasks/internal/tui/theme"
	"strings"
	"time"
)

func View(tasks []presenter.Task, now time.Time) string {
	lines := []string{theme.Title.Render(now.Format("January 2006"))}
	for _, t := range tasks {
		if t.Dates != "" && !t.Recurring {
			lines = append(lines, fmt.Sprintf("• %-22s %s", t.Dates, t.Title))
		}
	}
	if len(lines) == 1 {
		lines = append(lines, "No hay tareas con fechas")
	}
	return strings.Join(lines, "\n")
}
