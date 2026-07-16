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
	svc   *application.Service
	path  string
	today domain.Date
}

func (b backend) Mode() domain.Mode                 { return b.svc.Mode }
func (b backend) Capabilities() domain.Capabilities { return b.svc.Capabilities() }
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
func (b backend) Trash(c context.Context, path string, id, version int64) ([]int64, error) {
	if path == "" {
		path = b.path
	}
	for _, p := range b.svc.Projects {
		if p.Path == path {
			return p.Store.TrashTask(c, id, version, b.today)
		}
	}
	return nil, domain.ErrNotFound
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
		if existing, e := filesystem.Discover(cwd); e != nil {
			return e
		} else if existing != "" {
			return fmt.Errorf("directory tree already contains project %s", existing)
		}
		project = filepath.Join(cwd, name)
		if _, e = os.Lstat(project); !errors.Is(e, os.ErrNotExist) {
			return fmt.Errorf("%s already exists", project)
		}
		store, e := db.Open(project)
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
		if e = reg.Register(context.Background(), project); e != nil {
			return e
		}
		store, e := db.Open(project)
		if e != nil {
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
	model := tui.New(backend{svc, project, clock.System{}.Today()})
	_, e = tea.NewProgram(model, tea.WithAltScreen()).Run()
	return e
}
