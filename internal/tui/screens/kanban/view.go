package kanban

import (
	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/tui/presenter"
	"github.com/Polo123456789/tasks/internal/tui/theme"
	"github.com/charmbracelet/lipgloss"
	"strings"
)

func View(tasks []presenter.Task, statuses []domain.Status, selected, width int) string {
	if len(tasks) == 0 {
		return theme.Help.Render("No hay tareas. Pulsa n para crear una.")
	}
	cols := len(statuses)
	if cols == 0 {
		cols = 1
	}
	cw := max(18, width/cols-2)
	parts := make([]string, 0, cols)
	idx := 0
	for _, s := range statuses {
		var lines []string
		lines = append(lines, theme.Title.Render(s.Name))
		for _, t := range tasks {
			if t.Status == s.Name {
				style := lipgloss.NewStyle().Width(cw - 2)
				if idx == selected {
					style = theme.Selected.Width(cw - 2)
				}
				lines = append(lines, style.Render(presenter.Badge(t)))
				idx++
			}
		}
		parts = append(parts, theme.Border.Width(cw).Render(strings.Join(lines, "\n")))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}
