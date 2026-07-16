package kanban

import (
	"fmt"
	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/tui/presenter"
	"github.com/Polo123456789/tasks/internal/tui/screens/listutil"
	"github.com/Polo123456789/tasks/internal/tui/theme"
	"github.com/charmbracelet/lipgloss"
	"strings"
)

func View(tasks []presenter.Task, statuses []domain.Status, selected, width, height int) string {
	if len(tasks) == 0 {
		return theme.Help.Render("No hay tareas. Pulsa n para crear una.")
	}
	if len(statuses) == 0 {
		return theme.Help.Render("El proyecto no tiene estados disponibles.")
	}
	focusStatus := statuses[0].Name
	if selected >= 0 && selected < len(tasks) {
		focusStatus = tasks[selected].Status
	}
	focusColumn := 0
	for index, status := range statuses {
		if status.Name == focusStatus {
			focusColumn = index
			break
		}
	}
	visibleColumns := min(len(statuses), max(1, width/20))
	columnStart, columnEnd := listutil.Bounds(len(statuses), focusColumn, visibleColumns)
	visibleStatuses := statuses[columnStart:columnEnd]
	cw := max(18, width/len(visibleStatuses)-2)
	parts := make([]string, 0, len(visibleStatuses))
	for visibleIndex, s := range visibleStatuses {
		var lines []string
		title := s.Name
		if visibleIndex == 0 && columnStart > 0 {
			title = "← " + title
		}
		if visibleIndex == len(visibleStatuses)-1 && columnEnd < len(statuses) {
			title += " →"
		}
		lines = append(lines, theme.Title.Render(listutil.Truncate(title, cw-2)))
		type item struct {
			index int
			task  presenter.Task
		}
		var items []item
		for taskIndex, t := range tasks {
			if t.Status == s.Name {
				items = append(items, item{index: taskIndex, task: t})
			}
		}
		selectedItem := 0
		for index, item := range items {
			if item.index == selected {
				selectedItem = index
				break
			}
		}
		start, end := listutil.Bounds(len(items), selectedItem, max(1, height-5))
		if start > 0 {
			lines = append(lines, theme.Help.Render(fmt.Sprintf("↑ %d más", start)))
		}
		for _, item := range items[start:end] {
			style := lipgloss.NewStyle().Width(cw - 2)
			label := listutil.Truncate(presenter.Badge(item.task), cw-4)
			if item.index == selected {
				style = theme.Selected.Width(cw - 2)
				label = "› " + label
			} else {
				label = "  " + label
			}
			lines = append(lines, style.Render(label))
		}
		if end < len(items) {
			lines = append(lines, theme.Help.Render(fmt.Sprintf("↓ %d más", len(items)-end)))
		}
		parts = append(parts, theme.Border.Width(cw).Render(strings.Join(lines, "\n")))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}
