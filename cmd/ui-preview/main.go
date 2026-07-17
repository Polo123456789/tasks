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

type fake struct {
	fixture string
	mode    domain.Mode
}

func (f fake) Mode() domain.Mode { return f.mode }
func (f fake) Capabilities(source string) domain.Capabilities {
	if f.mode == domain.ModeLocal || source == domain.GlobalOriginKey || source == "" {
		return domain.Capabilities{CanCreateTask: true, CanCreateStatus: f.mode == domain.ModeLocal, CanCreateSubtask: true, CanCreateDependency: true, CanCreateRecurrence: true}
	}
	return domain.Capabilities{}
}
func (f fake) ContextLabel() string {
	if f.mode == domain.ModeGlobal {
		return "Global · vista previa"
	}
	return "Local · vista previa"
}
func (fake) Today() domain.Date {
	today, _ := domain.ParseDate("2026-07-15")
	return today
}
func (fake) Maintain(context.Context) error { return nil }
func (f fake) Statuses(context.Context) ([]domain.Status, error) {
	return []domain.Status{{ID: 1, Name: "Pendiente", Kind: domain.StatusNormal, Position: 1, Initial: true}, {ID: 2, Name: "En progreso", Kind: domain.StatusNormal, Position: 2}, {ID: 3, Name: "Bloqueada", Kind: domain.StatusNormal, Position: 3}, {ID: 4, Name: "Cancelada", Kind: domain.StatusCancelled, Position: 4}, {ID: 5, Name: "Finalizada", Kind: domain.StatusDone, Position: 5}}, nil
}
func (f fake) List(ctx context.Context, filter ports.TaskFilter) ([]domain.Task, error) {
	if f.fixture == "error" {
		return nil, fmt.Errorf("fallo simulado")
	}
	if f.fixture == "conflict" {
		return nil, domain.ErrConflict
	}
	if f.fixture == "loading" {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if f.fixture == "empty" {
		return nil, nil
	}
	today, _ := domain.ParseDate("2026-07-15")
	due := today.AddDays(4)
	tasks := []domain.Task{{ID: 1, Title: "Diseñar interfaz Unicode ✓", StatusID: 1, Status: domain.Status{ID: 1, Name: "Pendiente"}, Priority: domain.PriorityUrgent, Version: 1}, {ID: 2, Title: "Implementar almacenamiento", StatusID: 2, Status: domain.Status{ID: 2, Name: "En progreso"}, Priority: domain.PriorityHigh, Start: &today, Due: &due, Blocked: true, Version: 1}, {ID: 3, Title: "Documentar el proyecto", StatusID: 1, Status: domain.Status{ID: 1, Name: "Pendiente"}, Priority: domain.PriorityLow, Recurrence: &domain.Recurrence{Kind: domain.Weekly, Weekdays: []time.Weekday{time.Monday}}, Version: 1}}
	if filter.IncludeDeleted {
		deletedAt := today.AddDays(-3)
		return []domain.Task{
			{ID: 91, Title: "Tarea eliminada seleccionable", StatusID: 1, Status: domain.Status{ID: 1, Name: "Pendiente"}, DeletedAt: &deletedAt, Version: 1},
			{ID: 92, Title: "Otra tarea en papelera", StatusID: 2, Status: domain.Status{ID: 2, Name: "En progreso"}, DeletedAt: &deletedAt, Version: 1},
		}, nil
	}
	if f.fixture == "crowded" {
		for i := 0; i < 30; i++ {
			t := tasks[i%3]
			t.ID = int64(i + 10)
			t.Title = fmt.Sprintf("Tarea de prueba muy larga %02d", i)
			tasks = append(tasks, t)
		}
	}
	if f.fixture == "dependencies" {
		tasks[1].DependencyIDs = []int64{tasks[0].ID}
		tasks[1].DependencyCount = 1
		tasks[1].Blocked = true
	}
	if f.mode == domain.ModeGlobal {
		for index := range tasks {
			if index%2 == 0 {
				tasks[index].Origin = domain.TaskOrigin{Kind: domain.OriginGlobal, Key: domain.GlobalOriginKey, Name: "Global"}
			} else {
				tasks[index].Origin = domain.TaskOrigin{Kind: domain.OriginProject, Key: "/other/alpha.tasks", Name: "alpha"}
			}
		}
	}
	return tasks, nil
}
func (fake) Create(context.Context, string) (domain.Task, error) { return domain.Task{}, nil }
func (fake) SaveTask(context.Context, string, domain.Task) (domain.Task, error) {
	return domain.Task{}, nil
}
func (fake) FormStatuses(context.Context, string) ([]domain.Status, error) {
	return (fake{}).Statuses(context.Background())
}
func (fake) UpdateTitle(context.Context, string, int64, int64, string) (domain.Task, error) {
	return domain.Task{}, nil
}
func (fake) MoveStatus(context.Context, string, int64, int64, int) (domain.Task, error) {
	return domain.Task{}, nil
}
func (fake) SetLifecycle(context.Context, string, int64, int64, string) (domain.Task, error) {
	return domain.Task{}, nil
}
func (fake) CyclePriority(context.Context, string, int64, int64) (domain.Task, error) {
	return domain.Task{}, nil
}
func (fake) UpdateDate(context.Context, string, int64, int64, string, *domain.Date) (domain.Task, error) {
	return domain.Task{}, nil
}
func (f fake) Detail(_ context.Context, _ string, id int64) (domain.Task, error) {
	tasks, _ := f.List(context.Background(), ports.TaskFilter{})
	for _, task := range tasks {
		if task.ID == id {
			return task, nil
		}
	}
	return domain.Task{}, domain.ErrNotFound
}
func (fake) History(context.Context, string, int64) ([]domain.HistoryEvent, error) {
	return []domain.HistoryEvent{{Kind: "created", Detail: "fixture", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}}, nil
}
func (fake) AddSubtask(context.Context, string, int64, int64, string) (domain.Subtask, error) {
	return domain.Subtask{}, nil
}
func (fake) RenameSubtask(context.Context, string, int64, int64, int64, string) (domain.Subtask, error) {
	return domain.Subtask{}, nil
}
func (fake) ToggleSubtask(context.Context, string, int64, int64, int64) error { return nil }
func (fake) MoveSubtaskStatus(context.Context, string, int64, int64, int64, int) error {
	return nil
}
func (fake) AddDependency(context.Context, string, int64, int64, int64) error { return nil }
func (fake) RemoveDependency(context.Context, string, int64, int64, int64) error {
	return nil
}
func (f fake) DependencyCandidates(ctx context.Context, _ string, taskID int64, existingOnly bool) ([]domain.Task, error) {
	tasks, err := f.List(ctx, ports.TaskFilter{})
	if err != nil {
		return nil, err
	}
	var selected domain.Task
	for _, task := range tasks {
		if task.ID == taskID {
			selected = task
			break
		}
	}
	existing := make(map[int64]bool)
	for _, id := range selected.DependencyIDs {
		existing[id] = true
	}
	out := make([]domain.Task, 0)
	for _, task := range tasks {
		if task.ID != taskID && existing[task.ID] == existingOnly {
			out = append(out, task)
		}
	}
	return out, nil
}
func (fake) UpdateRecurrence(context.Context, string, int64, int64, *domain.Recurrence) (domain.Task, error) {
	return domain.Task{}, nil
}
func (fake) CreateStatus(context.Context, string) (domain.Status, error) {
	return domain.Status{}, nil
}
func (fake) RenameStatus(context.Context, int64, string) error { return nil }
func (fake) SetInitialStatus(context.Context, int64) error     { return nil }
func (fake) ReorderStatuses(context.Context, []int64) error    { return nil }
func (fake) DeleteStatus(context.Context, int64, *int64) error { return nil }
func (fake) MarkdownEditor(context.Context, string, int64, int64) (tea.ExecCommand, func(error) error, error) {
	return nil, nil, fmt.Errorf("editor deshabilitado en ui-preview")
}
func (fake) Trash(context.Context, string, int64, int64) ([]int64, error) { return nil, nil }
func (fake) DependencyImpact(context.Context, string, int64) ([]domain.Task, error) {
	return nil, nil
}
func (fake) Restore(context.Context, string, int64, int64) (domain.Task, error) {
	return domain.Task{}, nil
}
func main() {
	fixture := flag.String("fixture", "default", "default|empty|crowded|dependencies|loading|error|conflict")
	screen := flag.String("screen", "kanban", "kanban|table|calendar|gantt|trash|settings")
	mode := flag.String("mode", "local", "local|global")
	flag.Parse()
	selectedMode := domain.Mode(*mode)
	if selectedMode != domain.ModeLocal && selectedMode != domain.ModeGlobal {
		fmt.Fprintln(os.Stderr, "mode must be local or global")
		os.Exit(2)
	}
	if _, e := tea.NewProgram(ui.NewAt(fake{fixture: *fixture, mode: selectedMode}, *screen), tea.WithAltScreen()).Run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
