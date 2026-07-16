package trash

import (
	"github.com/Polo123456789/tasks/internal/tui/presenter"
	"github.com/Polo123456789/tasks/internal/tui/theme"
	"strings"
)

func View(tasks []presenter.Task) string {
	lines := []string{theme.Title.Render("Papelera (30 días)")}
	for _, t := range tasks {
		lines = append(lines, "• "+t.Title)
	}
	if len(tasks) == 0 {
		lines = append(lines, "Vacía")
	}
	return strings.Join(lines, "\n")
}
