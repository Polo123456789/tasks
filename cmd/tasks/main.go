package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Polo123456789/tasks/internal/adapters/clock"
	"github.com/Polo123456789/tasks/internal/adapters/editor"
	"github.com/Polo123456789/tasks/internal/adapters/filesystem"
	"github.com/Polo123456789/tasks/internal/adapters/registry"
	db "github.com/Polo123456789/tasks/internal/adapters/sqlite"
	"github.com/Polo123456789/tasks/internal/application"
	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/ports"
	tui "github.com/Polo123456789/tasks/internal/tui/app"
	tea "github.com/charmbracelet/bubbletea"
)

type backend struct {
	svc  *application.Service
	path string
}

func (b backend) Mode() domain.Mode                 { return b.svc.Mode }
func (b backend) Capabilities() domain.Capabilities { return b.svc.Capabilities() }
func (b backend) ContextLabel() string {
	if b.svc.Mode == domain.ModeGlobal {
		return "Global"
	}
	return "Local · " + application.ProjectName(b.path)
}
func (b backend) Today() domain.Date               { return b.svc.Clock.Today() }
func (b backend) Maintain(c context.Context) error { return b.svc.Maintain(c) }
func (b backend) List(c context.Context, f ports.TaskFilter) ([]domain.Task, error) {
	return b.svc.ListTasks(c, f)
}
func (b backend) Statuses(c context.Context) ([]domain.Status, error) {
	if len(b.svc.Projects) == 1 {
		return b.svc.Projects[0].Store.Statuses(c)
	}
	return nil, nil
}
func (b backend) Create(c context.Context, title string) (domain.Task, error) {
	return b.svc.CreateTask(c, b.path, domain.Task{Title: title})
}
func (b backend) UpdateTitle(c context.Context, path string, id, version int64, title string) (domain.Task, error) {
	if path == "" {
		path = b.path
	}
	return b.svc.UpdateTaskTitle(c, path, id, version, title)
}
func (b backend) MoveStatus(c context.Context, path string, id, version int64, direction int) (domain.Task, error) {
	if path == "" {
		path = b.path
	}
	return b.svc.MoveTaskStatus(c, path, id, version, direction)
}
func (b backend) SetLifecycle(c context.Context, path string, id, version int64, action string) (domain.Task, error) {
	if path == "" {
		path = b.path
	}
	return b.svc.SetTaskLifecycle(c, path, id, version, action)
}
func (b backend) CyclePriority(c context.Context, path string, id, version int64) (domain.Task, error) {
	if path == "" {
		path = b.path
	}
	return b.svc.CycleTaskPriority(c, path, id, version)
}
func (b backend) UpdateDate(c context.Context, path string, id, version int64, field string, date *domain.Date) (domain.Task, error) {
	if path == "" {
		path = b.path
	}
	return b.svc.UpdateTaskDate(c, path, id, version, field, date)
}
func (b backend) Detail(c context.Context, path string, id int64) (domain.Task, error) {
	if path == "" {
		path = b.path
	}
	return b.svc.Task(c, path, id)
}
func (b backend) History(c context.Context, path string, taskID int64) ([]domain.HistoryEvent, error) {
	if path == "" {
		path = b.path
	}
	return b.svc.History(c, path, taskID)
}
func (b backend) AddSubtask(c context.Context, path string, taskID int64, title string) (domain.Subtask, error) {
	if path == "" {
		path = b.path
	}
	return b.svc.AddSubtask(c, path, taskID, title)
}
func (b backend) RenameSubtask(c context.Context, path string, id int64, title string) (domain.Subtask, error) {
	if path == "" {
		path = b.path
	}
	return b.svc.RenameSubtask(c, path, id, title)
}
func (b backend) ToggleSubtask(c context.Context, path string, taskID, subtaskID int64) error {
	if path == "" {
		path = b.path
	}
	return b.svc.ToggleSubtask(c, path, taskID, subtaskID)
}
func (b backend) MoveSubtaskStatus(c context.Context, path string, taskID, subtaskID int64, direction int) error {
	if path == "" {
		path = b.path
	}
	return b.svc.MoveSubtaskStatus(c, path, taskID, subtaskID, direction)
}
func (b backend) AddDependency(c context.Context, path string, taskID, dependsOn int64) error {
	if path == "" {
		path = b.path
	}
	return b.svc.AddDependency(c, path, taskID, dependsOn)
}
func (b backend) RemoveDependency(c context.Context, path string, taskID, dependsOn int64) error {
	if path == "" {
		path = b.path
	}
	return b.svc.RemoveDependency(c, path, taskID, dependsOn)
}
func (b backend) DependencyCandidates(c context.Context, path string, taskID int64, existingOnly bool) ([]domain.Task, error) {
	if path == "" {
		path = b.path
	}
	return b.svc.DependencyCandidates(c, path, taskID, existingOnly)
}
func (b backend) UpdateRecurrence(c context.Context, path string, id, version int64, recurrence *domain.Recurrence) (domain.Task, error) {
	if path == "" {
		path = b.path
	}
	return b.svc.UpdateTaskRecurrence(c, path, id, version, recurrence)
}
func (b backend) CreateStatus(c context.Context, name string) (domain.Status, error) {
	return b.svc.CreateStatus(c, b.path, name, false)
}
func (b backend) RenameStatus(c context.Context, id int64, name string) error {
	return b.svc.RenameStatus(c, b.path, id, name)
}
func (b backend) SetInitialStatus(c context.Context, id int64) error {
	return b.svc.SetInitialStatus(c, b.path, id)
}
func (b backend) ReorderStatuses(c context.Context, ids []int64) error {
	return b.svc.ReorderStatuses(c, b.path, ids)
}
func (b backend) DeleteStatus(c context.Context, id int64, destination *int64) error {
	return b.svc.DeleteStatus(c, b.path, id, destination)
}
func (b backend) MarkdownEditor(c context.Context, path string, id, version int64) (tea.ExecCommand, func(error) error, error) {
	if path == "" {
		path = b.path
	}
	task, e := b.svc.Task(c, path, id)
	if e != nil {
		return nil, nil, e
	}
	if task.Version != version {
		return nil, nil, domain.ErrConflict
	}
	session, e := editor.NewSession(context.Background(), task.Markdown)
	if e != nil {
		return nil, nil, e
	}
	finish := func(runErr error) error {
		content, finishErr := session.Finish(runErr)
		if finishErr != nil {
			return finishErr
		}
		_, finishErr = b.svc.UpdateTaskMarkdown(context.Background(), path, id, version, content)
		return finishErr
	}
	return session, finish, nil
}
func (b backend) Trash(c context.Context, path string, id, version int64) ([]int64, error) {
	if path == "" {
		path = b.path
	}
	return b.svc.TrashTask(c, path, id, version)
}
func (b backend) DependencyImpact(c context.Context, path string, id int64) ([]domain.Task, error) {
	if path == "" {
		path = b.path
	}
	ids, err := b.svc.DependencyImpact(c, path, id)
	if err != nil {
		return nil, err
	}
	tasks := make([]domain.Task, 0, len(ids))
	for _, affectedID := range ids {
		task, taskErr := b.svc.Task(c, path, affectedID)
		if taskErr != nil {
			return nil, taskErr
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}
func (b backend) Restore(c context.Context, path string, id, version int64) (domain.Task, error) {
	if path == "" {
		path = b.path
	}
	return b.svc.RestoreTask(c, path, id, version)
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "tasks:", e)
		os.Exit(1)
	}
}
func run() error {
	flag.Parse()
	cwd, e := os.Getwd()
	if e != nil {
		return e
	}
	data, e := os.UserConfigDir()
	if e != nil {
		return e
	}
	logFile, e := configureLogging(filepath.Join(data, "tasks", "tasks.log"))
	if e != nil {
		return e
	}
	defer logFile.Close()
	reg, e := registry.Open(filepath.Join(data, "tasks", "registry.sqlite"))
	if e != nil {
		return e
	}
	defer reg.Close()
	var project string
	if flag.NArg() > 0 && flag.Arg(0) == "init" {
		if flag.NArg() != 2 {
			return fmt.Errorf("usage: tasks init nombre.tasks")
		}
		name := flag.Arg(1)
		if e = filesystem.ValidateProjectName(name); e != nil {
			return e
		}
		if existing, e := filesystem.InDirectory(cwd); e != nil {
			return e
		} else if len(existing) != 0 {
			return fmt.Errorf("directory already contains project %s", existing[0])
		}
		project = filepath.Join(cwd, name)
		if _, e = os.Lstat(project); !errors.Is(e, os.ErrNotExist) {
			return fmt.Errorf("%s already exists", project)
		}
		store, e := createProject(project)
		if e != nil {
			return e
		}
		store.Close()
	} else {
		project, e = filesystem.Discover(cwd)
		if e != nil {
			return e
		}
	}
	var projects []application.Project
	mode := domain.ModeLocal
	if project != "" {
		store, e := db.Open(project)
		if e != nil {
			return e
		}
		if e = reg.Register(context.Background(), project); e != nil {
			store.Close()
			return e
		}
		projects = []application.Project{{Path: project, Name: application.ProjectName(project), Store: store}}
	} else {
		mode = domain.ModeGlobal
		paths, e := reg.Prune(context.Background())
		if e != nil {
			return e
		}
		for _, p := range paths {
			store, e := db.Open(p)
			projects = append(projects, application.Project{Path: p, Name: application.ProjectName(p), Store: store, Err: e})
		}
	}
	svc := &application.Service{Mode: mode, Projects: projects, Clock: clock.System{}}
	defer svc.Close()
	if e = svc.Maintain(context.Background()); e != nil {
		slog.Warn("maintenance", "error", e)
	}
	model := tui.New(backend{svc: svc, path: project})
	_, e = tea.NewProgram(model, tea.WithAltScreen()).Run()
	return e
}

func createProject(path string) (*db.Store, error) {
	file, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		if errors.Is(e, os.ErrExist) {
			return nil, fmt.Errorf("%s already exists", path)
		}
		return nil, e
	}
	if e = file.Close(); e != nil {
		os.Remove(path)
		return nil, e
	}
	store, e := db.Open(path)
	if e != nil {
		os.Remove(path)
		return nil, e
	}
	return store, nil
}

func configureLogging(path string) (*os.File, error) {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	file, e := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return nil, e
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{Level: slog.LevelInfo})))
	return file, nil
}
