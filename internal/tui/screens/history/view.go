package history

import (
	"fmt"
	"strings"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/tui/theme"
)

func View(events []domain.HistoryEvent, height int) string {
	lines := []string{theme.Title.Render("Historial de la tarea")}
	start := max(0, len(events)-max(1, height-6))
	if start > 0 {
		lines = append(lines, fmt.Sprintf("↑ %d evento(s) anterior(es)", start))
	}
	for _, event := range events[start:] {
		date := event.CreatedAt.Local().Format("2006-01-02 15:04")
		line := fmt.Sprintf("%s · %s", date, eventName(event.Kind))
		if event.Detail != "" {
			line += " · " + event.Detail
		}
		lines = append(lines, line)
	}
	if len(events) == 0 {
		lines = append(lines, "Sin eventos")
	}
	lines = append(lines, "", "H cerrar historial")
	return theme.Border.Render(strings.Join(lines, "\n"))
}

func eventName(kind string) string {
	names := map[string]string{
		"created": "creada", "updated": "editada", "title_changed": "título cambiado",
		"status_changed": "estado cambiado", "priority_changed": "prioridad cambiada",
		"completed": "finalizada", "cancelled": "cancelada", "reopened": "reabierta",
		"subtask_added": "subtarea creada", "subtask_updated": "subtarea actualizada",
		"dependency_added": "dependencia creada", "dependency_removed": "dependencia eliminada",
		"trashed": "enviada a papelera", "restored": "restaurada", "recurrence_reset": "recurrencia reiniciada",
	}
	if name, ok := names[kind]; ok {
		return name
	}
	return kind
}
