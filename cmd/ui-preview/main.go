package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/ports"
	ui "github.com/Polo123456789/tasks/internal/tui/app"
	tea "github.com/charmbracelet/bubbletea"
	"os"
	"time"
)

type fake struct{ fixture string }

func (fake) Mode() domain.Mode                 { return domain.ModeLocal }
func (fake) Capabilities() domain.Capabilities { return domain.CapabilitiesFor(domain.ModeLocal) }
func (f fake) Statuses(context.Context) ([]domain.Status, error) {
	return []domain.Status{{ID: 1, Name: "Pendiente", Kind: domain.StatusNormal, Initial: true}, {ID: 2, Name: "En progreso", Kind: domain.StatusNormal}, {ID: 3, Name: "Bloqueada", Kind: domain.StatusNormal}, {ID: 4, Name: "Cancelada", Kind: domain.StatusCancelled}, {ID: 5, Name: "Finalizada", Kind: domain.StatusDone}}, nil
}
func (f fake) List(context.Context, ports.TaskFilter) ([]domain.Task, error) {
	if f.fixture == "error" {
		return nil, fmt.Errorf("fallo simulado")
	}
	if f.fixture == "empty" {
		return nil, nil
	}
	today := domain.DateFromTime(time.Now())
	due := today.AddDays(4)
	tasks := []domain.Task{{ID: 1, Title: "Diseñar interfaz Unicode ✓", StatusID: 1, Status: domain.Status{ID: 1, Name: "Pendiente"}, Priority: domain.PriorityUrgent, Version: 1}, {ID: 2, Title: "Implementar almacenamiento", StatusID: 2, Status: domain.Status{ID: 2, Name: "En progreso"}, Priority: domain.PriorityHigh, Start: &today, Due: &due, Blocked: true, Version: 1}, {ID: 3, Title: "Documentar el proyecto", StatusID: 1, Status: domain.Status{ID: 1, Name: "Pendiente"}, Priority: domain.PriorityLow, Recurrence: &domain.Recurrence{Kind: domain.Weekly, Weekdays: []time.Weekday{time.Monday}}, Version: 1}}
	if f.fixture == "crowded" {
		for i := 0; i < 30; i++ {
			t := tasks[i%3]
			t.ID = int64(i + 10)
			t.Title = fmt.Sprintf("Tarea de prueba muy larga %02d", i)
			tasks = append(tasks, t)
		}
	}
	return tasks, nil
}
func (fake) Create(context.Context, string) (domain.Task, error)          { return domain.Task{}, nil }
func (fake) Trash(context.Context, string, int64, int64) ([]int64, error) { return nil, nil }
func main() {
	fixture := flag.String("fixture", "default", "default|empty|crowded|error")
	screen := flag.String("screen", "kanban", "kanban|table|calendar|gantt|trash")
	flag.Parse()
	if _, e := tea.NewProgram(ui.NewAt(fake{*fixture}, *screen), tea.WithAltScreen()).Run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
