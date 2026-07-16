package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Polo123456789/tasks/internal/adapters/editor"
	db "github.com/Polo123456789/tasks/internal/adapters/sqlite"
	"github.com/Polo123456789/tasks/internal/application"
	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/ports"
)

const importJSON = `{
  "format": "tasks-project",
  "version": 1,
  "statuses": [
    {"key": "todo", "name": "Por hacer", "initial": true},
    {"key": "doing", "name": "En curso"}
  ],
  "tasks": [
    {"key": "first", "title": "Primera", "status": "done", "priority": "high"},
    {"key": "second", "title": "Segunda", "depends_on": ["first"], "subtasks": [{"title": "Detalle"}]}
  ]
}`

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time     { return c.now }
func (c fixedClock) Today() domain.Date { return domain.DateFromTime(c.now) }

type recordingRegistry struct {
	path string
	err  error
}

func (r *recordingRegistry) Register(_ context.Context, path string) error {
	r.path = path
	return r.err
}
func (*recordingRegistry) Projects(context.Context) ([]string, error) { return nil, nil }
func (*recordingRegistry) Prune(context.Context) ([]string, error)    { return nil, nil }
func (*recordingRegistry) Close() error                               { return nil }

var _ ports.Registry = (*recordingRegistry)(nil)

func TestConfigureLoggingWritesOnlyToRequestedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "tasks.log")
	file, err := configureLogging(path)
	if err != nil {
		t.Fatal(err)
	}
	slog.Info("diagnostic", "project", "alpha")
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "diagnostic") || !strings.Contains(string(content), "alpha") {
		t.Fatalf("log content=%q", content)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf("log permissions=%o", info.Mode().Perm())
	}
}

func TestMarkdownConflictPreservesEditedTemporaryFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is for supported Unix platforms")
	}
	directory := t.TempDir()
	projectPath := filepath.Join(directory, "project.tasks")
	store, err := db.Open(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	service := &application.Service{Mode: domain.ModeLocal, Clock: fixedClock{now: time.Now()}, Projects: []application.Project{{Path: projectPath, Store: store}}}
	defer service.Close()
	task, err := service.CreateTask(context.Background(), projectPath, domain.Task{Title: "original", Markdown: "before"})
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(directory, "editor")
	if err = os.WriteFile(script, []byte("#!/bin/sh\nprintf 'edited content' > \"$1\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", script)
	command, finish, err := (backend{svc: service, path: projectPath}).MarkdownEditor(context.Background(), projectPath, task.ID, task.Version)
	if err != nil {
		t.Fatal(err)
	}
	session, ok := command.(*editor.Session)
	if !ok {
		t.Fatalf("editor command type %T", command)
	}
	if err = command.Run(); err != nil {
		t.Fatal(err)
	}
	if _, err = service.UpdateTaskTitle(context.Background(), projectPath, task.ID, task.Version, "concurrent"); err != nil {
		t.Fatal(err)
	}
	if err = finish(nil); !errors.Is(err, domain.ErrConflict) || !strings.Contains(err.Error(), session.Path()) {
		t.Fatalf("conflict did not expose recovery path: %v", err)
	}
	content, err := os.ReadFile(session.Path())
	if err != nil || string(content) != "edited content" {
		t.Fatalf("preserved content=%q err=%v", content, err)
	}
	if err = session.Cleanup(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateProjectIsExclusiveAndPortable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.tasks")
	store, err := createProject(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = createProject(path); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second creation error=%v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf("project permissions=%o", info.Mode().Perm())
	}
}

func TestAIPromptWritesStandaloneInstructions(t *testing.T) {
	var output bytes.Buffer
	if err := runArgs([]string{"ai-prompt"}, strings.NewReader(""), &output); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"tasks-project", "JSON puro", "depends_on"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("prompt missing %q", expected)
		}
	}
}

func TestImportProjectFromStdinPublishesAndRegisters(t *testing.T) {
	directory := t.TempDir()
	registry := &recordingRegistry{}
	clock := fixedClock{now: time.Date(2026, 7, 16, 12, 30, 0, 0, time.FixedZone("local", -6*60*60))}
	summary, path, err := importProject(context.Background(), directory, "nuevo.tasks", "", strings.NewReader(importJSON), registry, clock)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Tasks != 2 || summary.Statuses != 2 || summary.Subtasks != 1 || summary.Dependencies != 1 {
		t.Fatalf("summary=%#v", summary)
	}
	if path != filepath.Join(directory, "nuevo.tasks") || registry.path != path {
		t.Fatalf("path=%q registered=%q", path, registry.path)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf("project mode=%v", info.Mode())
	}
	store, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := store.ListTasks(context.Background(), ports.TaskFilter{IncludeDone: true, IncludeCancelled: true})
	closeErr := store.Close()
	if err != nil || closeErr != nil || len(tasks) != 2 {
		t.Fatalf("tasks=%#v err=%v close=%v", tasks, err, closeErr)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 || entries[0].Name() != "nuevo.tasks" {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
}

func TestImportProjectReadsFileAndRejectsExistingProject(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "result.json")
	if err := os.WriteFile(source, []byte(importJSON), 0600); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "project")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	clock := fixedClock{now: time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)}
	if _, _, err := importProject(context.Background(), directory, "from-file.tasks", source, strings.NewReader("ignored"), &recordingRegistry{}, clock); err != nil {
		t.Fatal(err)
	}
	if _, _, err := importProject(context.Background(), directory, "other.tasks", source, strings.NewReader(""), &recordingRegistry{}, clock); err == nil || !strings.Contains(err.Error(), "already contains project") {
		t.Fatalf("existing project error=%v", err)
	}
}

func TestImportValidationFailureLeavesNoProjectOrStagingFile(t *testing.T) {
	directory := t.TempDir()
	clock := fixedClock{now: time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)}
	input := "```json\n" + importJSON + "\n```"
	if _, _, err := importProject(context.Background(), directory, "failed.tasks", "-", strings.NewReader(input), &recordingRegistry{}, clock); err == nil {
		t.Fatal("expected import error")
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
}

func TestImportKeepsCompleteProjectWhenRegistrationFails(t *testing.T) {
	directory := t.TempDir()
	registry := &recordingRegistry{err: errors.New("registry failed")}
	clock := fixedClock{now: time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)}
	summary, path, err := importProject(context.Background(), directory, "recoverable.tasks", "-", strings.NewReader(importJSON), registry, clock)
	if err == nil || !strings.Contains(err.Error(), "project imported at") || summary.Tasks != 2 {
		t.Fatalf("summary=%#v path=%q err=%v", summary, path, err)
	}
	if path != filepath.Join(directory, "recoverable.tasks") {
		t.Fatalf("path=%q", path)
	}
	store, openErr := db.Open(path)
	if openErr != nil {
		t.Fatalf("published project is not recoverable: %v", openErr)
	}
	defer store.Close()
	tasks, listErr := store.ListTasks(context.Background(), ports.TaskFilter{IncludeDone: true, IncludeCancelled: true})
	if listErr != nil || len(tasks) != 2 {
		t.Fatalf("published tasks=%#v err=%v", tasks, listErr)
	}
}
