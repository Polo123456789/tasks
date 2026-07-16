package settings

import (
	"fmt"
	"strings"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/tui/screens/listutil"
	"github.com/Polo123456789/tasks/internal/tui/theme"
)

func View(statuses []domain.Status, selected, height int) string {
	lines := []string{theme.Title.Render("Estados del proyecto")}
	start, end := listutil.Bounds(len(statuses), selected, max(1, height-5))
	if start > 0 {
		lines = append(lines, fmt.Sprintf("↑ %d estado(s) más", start))
	}
	for index := start; index < end; index++ {
		status := statuses[index]
		kind := "normal"
		if status.Kind == domain.StatusDone {
			kind = "especial · finalizada"
		} else if status.Kind == domain.StatusCancelled {
			kind = "especial · cancelada"
		}
		if status.Initial {
			kind += " · inicial"
		}
		line := fmt.Sprintf("pos. %-2d  #%-3d  %-24s %s", status.Position, status.ID, status.Name, kind)
		if index == selected {
			line = theme.Selected.Render("› " + line)
		} else {
			line = "  " + line
		}
		lines = append(lines, line)
	}
	if end < len(statuses) {
		lines = append(lines, fmt.Sprintf("↓ %d estado(s) más", len(statuses)-end))
	}
	lines = append(lines, "", "a crear · e renombrar · i hacer inicial · [/] reordenar · d eliminar")
	return strings.Join(lines, "\n")
}
