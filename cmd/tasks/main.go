package main

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	"github.com/Polo123456789/tasks/internal/projectimport"
	tui "github.com/Polo123456789/tasks/internal/tui/app"
	tea "github.com/charmbracelet/bubbletea"
)

type backend struct {
	svc  *application.Service
	path string
}

func (b backend) Mode() domain.Mode { return b.svc.Mode }
func (b backend) Capabilities(source string) domain.Capabilities {
	return b.svc.Capabilities(source)
}
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
	if b.svc.Mode == domain.ModeLocal && len(b.svc.Sources) == 1 {
		return b.svc.Sources[0].Store.Statuses(c)
	}
	return nil, nil
}
func (b backend) Create(c context.Context, title string) (domain.Task, error) {
	return b.svc.CreateTask(c, domain.Task{Title: title})
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
func (b backend) AddSubtask(c context.Context, path string, taskID, version int64, title string) (domain.Subtask, error) {
	if path == "" {
		path = b.path
	}
	return b.svc.AddSubtask(c, path, taskID, version, title)
}
func (b backend) RenameSubtask(c context.Context, path string, taskID, id, version int64, title string) (domain.Subtask, error) {
	if path == "" {
		path = b.path
	}
	return b.svc.RenameSubtask(c, path, taskID, id, version, title)
}
func (b backend) ToggleSubtask(c context.Context, path string, taskID, subtaskID, version int64) error {
	if path == "" {
		path = b.path
	}
	return b.svc.ToggleSubtask(c, path, taskID, subtaskID, version)
}
func (b backend) MoveSubtaskStatus(c context.Context, path string, taskID, subtaskID, version int64, direction int) error {
	if path == "" {
		path = b.path
	}
	return b.svc.MoveSubtaskStatus(c, path, taskID, subtaskID, version, direction)
}
func (b backend) AddDependency(c context.Context, path string, taskID, dependsOn, version int64) error {
	if path == "" {
		path = b.path
	}
	return b.svc.AddDependency(c, path, taskID, dependsOn, version)
}
func (b backend) RemoveDependency(c context.Context, path string, taskID, dependsOn, version int64) error {
	if path == "" {
		path = b.path
	}
	return b.svc.RemoveDependency(c, path, taskID, dependsOn, version)
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
		if runErr != nil {
			_, finishErr := session.Finish(runErr)
			return finishErr
		}
		content, readErr := session.Read()
		if readErr != nil {
			return fmt.Errorf("read editor output; temporary file preserved at %s: %w", session.Path(), readErr)
		}
		if _, updateErr := b.svc.UpdateTaskMarkdown(context.Background(), path, id, version, content); updateErr != nil {
			return fmt.Errorf("save markdown; edits preserved at %s: %w", session.Path(), updateErr)
		}
		if cleanupErr := session.Cleanup(); cleanupErr != nil {
			return fmt.Errorf("markdown saved, but remove temporary file %s: %w", session.Path(), cleanupErr)
		}
		return nil
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
	return runArgs(os.Args[1:], os.Stdin, os.Stdout)
}

func runArgs(args []string, stdin io.Reader, stdout io.Writer) (resultErr error) {
	invocation, e := parseInvocation(args)
	if e != nil {
		return e
	}
	if invocation.kind == commandHelp {
		_, e = io.WriteString(stdout, helpText)
		return e
	}
	systemClock := clock.System{}
	if invocation.kind == commandAIPrompt {
		_, e := io.WriteString(stdout, projectimport.Prompt(systemClock.Today()))
		return e
	}
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
	defer func() { resultErr = errors.Join(resultErr, logFile.Close()) }()
	reg, e := registry.Open(filepath.Join(data, "tasks", "registry.sqlite"))
	if e != nil {
		return e
	}
	defer func() { resultErr = errors.Join(resultErr, reg.Close()) }()
	var project string
	if invocation.kind == commandImport {
		summary, importedPath, importErr := importProject(context.Background(), cwd, invocation.project, invocation.source, stdin, reg, systemClock)
		if importErr != nil {
			return importErr
		}
		_, e = fmt.Fprintf(stdout, "Proyecto importado: %s (%d estados, %d tareas, %d subtareas, %d dependencias)\n", importedPath, summary.Statuses, summary.Tasks, summary.Subtasks, summary.Dependencies)
		return e
	}
	if invocation.kind == commandInit {
		name := invocation.project
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
		if e = store.Close(); e != nil {
			return e
		}
	} else {
		project, e = filesystem.Discover(cwd)
		if e != nil {
			return e
		}
	}
	var sources []application.Source
	var writableSource string
	summaryPartial := false
	mode := domain.ModeLocal
	if project != "" {
		store, e := db.Open(project)
		if e != nil {
			return e
		}
		if e = reg.Register(context.Background(), project); e != nil {
			return errors.Join(e, store.Close())
		}
		origin := domain.TaskOrigin{Kind: domain.OriginProject, Key: project, Name: application.ProjectName(project)}
		sources = []application.Source{{Origin: origin, Store: store}}
		writableSource = project
	} else {
		mode = domain.ModeGlobal
		writableSource = domain.GlobalOriginKey
		globalOrigin := domain.TaskOrigin{Kind: domain.OriginGlobal, Key: domain.GlobalOriginKey, Name: "Global"}
		globalStore, globalErr := openGlobalStore(filepath.Join(data, "tasks", "global.sqlite"))
		globalSource := application.Source{Origin: globalOrigin}
		if globalErr != nil {
			globalSource.Err = fmt.Errorf("Global: %w", globalErr)
		} else {
			globalSource.Store = globalStore
		}
		sources = append(sources, globalSource)
		paths, pruneErr := reg.Prune(context.Background())
		if pruneErr != nil {
			if invocation.kind != commandSummary {
				return pruneErr
			}
			summaryPartial = true
			slog.Warn("registry prune", "error", pruneErr)
		}
		for _, p := range paths {
			store, openErr := db.Open(p)
			origin := domain.TaskOrigin{Kind: domain.OriginProject, Key: p, Name: application.ProjectName(p)}
			opened := application.Source{Origin: origin}
			if openErr == nil {
				opened.Store = store
			} else {
				opened.Err = fmt.Errorf("%s: %w", origin.Name, openErr)
			}
			sources = append(sources, opened)
		}
	}
	svc := &application.Service{Mode: mode, Sources: sources, WritableSource: writableSource, Clock: clock.System{}}
	defer func() { resultErr = errors.Join(resultErr, svc.Close()) }()
	maintenanceErr := svc.Maintain(context.Background())
	if maintenanceErr != nil {
		slog.Warn("maintenance", "error", maintenanceErr)
	}
	if invocation.kind == commandSummary {
		return writeSummary(context.Background(), stdout, svc, systemClock.Today(), invocation.color, summaryPartial || maintenanceErr != nil)
	}
	model := tui.New(backend{svc: svc, path: project})
	_, e = tea.NewProgram(model, tea.WithAltScreen()).Run()
	return e
}

func importProject(ctx context.Context, cwd, name, source string, stdin io.Reader, reg ports.Registry, importClock ports.Clock) (projectimport.Summary, string, error) {
	if e := filesystem.ValidateProjectName(name); e != nil {
		return projectimport.Summary{}, "", e
	}
	var reader io.Reader = stdin
	var input *os.File
	if source != "" && source != "-" {
		var e error
		input, e = os.Open(source)
		if e != nil {
			return projectimport.Summary{}, "", e
		}
		defer input.Close()
		reader = input
	}
	document, e := projectimport.Decode(reader)
	if e != nil {
		return projectimport.Summary{}, "", e
	}
	seed, e := projectimport.Normalize(document, importClock.Today())
	if e != nil {
		return projectimport.Summary{}, "", e
	}
	if existing, directoryErr := filesystem.InDirectory(cwd); directoryErr != nil {
		return projectimport.Summary{}, "", directoryErr
	} else if len(existing) != 0 {
		return projectimport.Summary{}, "", fmt.Errorf("directory already contains project %s", existing[0])
	}
	target := filepath.Join(cwd, name)
	if _, e = os.Lstat(target); !errors.Is(e, os.ErrNotExist) {
		return projectimport.Summary{}, "", fmt.Errorf("%s already exists", target)
	}

	temporary, e := os.CreateTemp(cwd, ".tasks-import-")
	if e != nil {
		return projectimport.Summary{}, "", e
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if e = temporary.Chmod(0600); e == nil {
		e = temporary.Close()
	} else {
		_ = temporary.Close()
	}
	if e != nil {
		return projectimport.Summary{}, "", e
	}
	store, e := db.Open(temporaryPath)
	if e != nil {
		return projectimport.Summary{}, "", e
	}
	summary, importErr := store.ImportProject(ctx, seed, importClock.Now())
	closeErr := store.Close()
	if importErr != nil || closeErr != nil {
		return projectimport.Summary{}, "", errors.Join(importErr, closeErr)
	}
	if e = os.Link(temporaryPath, target); e != nil {
		return projectimport.Summary{}, "", fmt.Errorf("publish project: %w", e)
	}
	cleanupErr := os.Remove(temporaryPath)
	registerErr := reg.Register(ctx, target)
	if cleanupErr != nil || registerErr != nil {
		var publicationErrs []error
		if cleanupErr != nil {
			publicationErrs = append(publicationErrs, fmt.Errorf("project imported at %s, but staging cleanup failed: %w", target, cleanupErr))
		}
		if registerErr != nil {
			publicationErrs = append(publicationErrs, fmt.Errorf("project imported at %s, but global registration failed: %w", target, registerErr))
		}
		return summary, target, errors.Join(publicationErrs...)
	}
	return summary, target, nil
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
		return nil, errors.Join(e, removeFile(path))
	}
	store, e := db.Open(path)
	if e != nil {
		return nil, errors.Join(e, removeFile(path))
	}
	return store, nil
}

func openGlobalStore(path string) (*db.Store, error) {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	created := false
	file, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e == nil {
		created = true
		if closeErr := file.Close(); closeErr != nil {
			return nil, errors.Join(closeErr, removeFile(path))
		}
	} else if !errors.Is(e, os.ErrExist) {
		return nil, e
	}
	if e = os.Chmod(path, 0600); e != nil {
		if created {
			return nil, errors.Join(e, removeFile(path))
		}
		return nil, e
	}
	store, e := db.Open(path)
	if e != nil && created {
		return nil, errors.Join(e, removeFile(path))
	}
	return store, e
}

func removeFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s after failure: %w", path, err)
	}
	return nil
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
