package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Polo123456789/tasks/internal/adapters/registry"
	db "github.com/Polo123456789/tasks/internal/adapters/sqlite"
	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/ports"
	"github.com/creack/pty"
)

type synchronizedBuffer struct {
	sync.Mutex
	bytes.Buffer
}

func (b *synchronizedBuffer) writeFrom(reader io.Reader) {
	buffer := make([]byte, 4096)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			b.Lock()
			_, _ = b.Buffer.Write(buffer[:n])
			b.Unlock()
		}
		if err != nil {
			return
		}
	}
}

func (b *synchronizedBuffer) contains(value string) bool {
	b.Lock()
	defer b.Unlock()
	return strings.Contains(b.String(), value)
}

func (b *synchronizedBuffer) text() string {
	b.Lock()
	defer b.Unlock()
	return b.String()
}

func waitForText(t *testing.T, output *synchronizedBuffer, value string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if output.contains(value) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q; output:\n%s", value, output.text())
}

func waitForTask(t *testing.T, projectPath string, predicate func(domain.Task) bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		store, err := db.Open(projectPath)
		if err == nil {
			task, taskErr := store.Task(context.Background(), 1)
			_ = store.Close()
			if taskErr == nil && predicate(task) {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for persisted task state")
}

func runPTY(t *testing.T, binary, directory, home string, arguments ...string) (*os.File, *exec.Cmd, *synchronizedBuffer) {
	t.Helper()
	command := exec.Command(binary, arguments...)
	command.Dir = directory
	command.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, "config"), "TERM=xterm-256color")
	terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 40, Cols: 120})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_ = terminal.Close()
	})
	output := &synchronizedBuffer{}
	go output.writeFrom(terminal)
	return terminal, command, output
}

func stopPTY(t *testing.T, terminal *os.File, command *exec.Cmd, output *synchronizedBuffer) {
	t.Helper()
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		_ = terminal.Close()
		if err != nil {
			t.Fatalf("TUI process: %v", err)
		}
	case <-time.After(20 * time.Second):
		_ = command.Process.Kill()
		_ = terminal.Close()
		t.Fatalf("TUI did not exit after SIGTERM; output:\n%s", output.text())
	}
}

func terminatePTY(terminal *os.File, command *exec.Cmd) {
	_ = command.Process.Kill()
	_, _ = command.Process.Wait()
	_ = terminal.Close()
}

func buildBinary(t *testing.T, root string) string {
	t.Helper()
	binary := filepath.Join(root, "tasks")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	return binary
}

func TestE2EInitCreateCloseAndReopen(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("initial release supports Linux and macOS")
	}
	root := t.TempDir()
	binary := buildBinary(t, root)
	projectDir := filepath.Join(root, "project")
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(projectDir, 0700); err != nil {
		t.Fatal(err)
	}
	editorScript := filepath.Join(root, "fake-editor")
	if err := os.WriteFile(editorScript, []byte("#!/bin/sh\nprintf '# E2E markdown\\n' > \"$1\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", editorScript)
	projectPath := filepath.Join(projectDir, "e2e.tasks")
	terminal, command, output := runPTY(t, binary, projectDir, home, "init", "e2e.tasks")
	waitForText(t, output, "Kanban")
	if _, err := terminal.Write([]byte("n")); err != nil {
		t.Fatal(err)
	}
	waitForText(t, output, "Nueva tarea")
	if _, err := terminal.Write([]byte("E2E task")); err != nil {
		t.Fatal(err)
	}
	waitForText(t, output, "E2E task")
	if _, err := terminal.Write([]byte("\r")); err != nil {
		t.Fatal(err)
	}
	waitForText(t, output, "Tarea creada")
	if _, err := terminal.Write([]byte("a")); err != nil {
		t.Fatal(err)
	}
	waitForText(t, output, "Nueva subtarea")
	if err := pty.Setsize(terminal, &pty.Winsize{Rows: 40, Cols: 90}); err != nil {
		t.Fatal(err)
	}
	if _, err := terminal.Write([]byte("Child task\r")); err != nil {
		t.Fatal(err)
	}
	waitForText(t, output, "Subtarea creada")
	if _, err := terminal.Write([]byte("t")); err != nil {
		t.Fatal(err)
	}
	waitForText(t, output, "Subtarea actualizada")
	waitForText(t, output, "subtareas 1/1")
	if err := pty.Setsize(terminal, &pty.Winsize{Rows: 40, Cols: 120}); err != nil {
		t.Fatal(err)
	}
	if _, err := terminal.Write([]byte("m")); err != nil {
		t.Fatal(err)
	}
	waitForText(t, output, "Markdown actualizado")
	waitForTask(t, projectPath, func(task domain.Task) bool { return task.Markdown == "# E2E markdown\n" })
	terminatePTY(terminal, command)

	store, err := db.Open(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := store.ListTasks(context.Background(), ports.TaskFilter{IncludeDone: true, IncludeCancelled: true})
	if closeErr := store.Close(); err == nil {
		err = closeErr
	}
	if err != nil || len(tasks) != 1 || tasks[0].Title != "E2E task" {
		t.Fatalf("persisted tasks=%#v err=%v", tasks, err)
	}
	store, err = db.Open(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	detail, err := store.Task(context.Background(), tasks[0].ID)
	_ = store.Close()
	if err != nil || detail.Markdown != "# E2E markdown\n" || len(detail.Subtasks) != 1 || detail.Status.Name != "Pendiente" {
		t.Fatalf("persisted detail=%#v err=%v", detail, err)
	}

	terminal, command, output = runPTY(t, binary, projectDir, home)
	waitForText(t, output, "E2E task")
	stopPTY(t, terminal, command, output)
}

func TestE2EGlobalTaskPersistsAndDoesNotAppearLocally(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("initial release supports Linux and macOS")
	}
	root := t.TempDir()
	binary := buildBinary(t, root)
	home := filepath.Join(root, "home")
	globalDirectory := filepath.Join(root, "outside")
	localDirectory := filepath.Join(root, "project")
	if err := os.MkdirAll(globalDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(localDirectory, 0700); err != nil {
		t.Fatal(err)
	}

	terminal, command, output := runPTY(t, binary, globalDirectory, home)
	waitForText(t, output, "Calendario")
	if _, err := terminal.Write([]byte("n")); err != nil {
		t.Fatal(err)
	}
	waitForText(t, output, "Nueva tarea")
	if _, err := terminal.Write([]byte("Tarea global E2E\r")); err != nil {
		t.Fatal(err)
	}
	waitForText(t, output, "Tarea creada")
	stopPTY(t, terminal, command, output)

	globalPath := filepath.Join(home, "config", "tasks", "global.sqlite")
	store, err := db.Open(globalPath)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := store.ListTasks(context.Background(), ports.TaskFilter{IncludeDone: true, IncludeCancelled: true})
	if closeErr := store.Close(); err == nil {
		err = closeErr
	}
	if err != nil || len(tasks) != 1 || tasks[0].Title != "Tarea global E2E" {
		t.Fatalf("global tasks=%#v err=%v", tasks, err)
	}
	info, err := os.Stat(globalPath)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("global store info=%v err=%v", info, err)
	}

	localPath := filepath.Join(localDirectory, "local.tasks")
	localStore, err := db.Open(localPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = localStore.Close(); err != nil {
		t.Fatal(err)
	}
	terminal, command, output = runPTY(t, binary, localDirectory, home)
	waitForText(t, output, "Kanban")
	if output.contains("Tarea global E2E") {
		t.Fatalf("local TUI leaked global task:\n%s", output.text())
	}
	stopPTY(t, terminal, command, output)
}

func TestE2EImportCommandCreatesRegisteredProjectAndExits(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("initial release supports Linux and macOS")
	}
	root := t.TempDir()
	binary := buildBinary(t, root)
	projectDir := filepath.Join(root, "project")
	home := filepath.Join(root, "home")
	config := filepath.Join(home, "config")
	if err := os.MkdirAll(projectDir, 0700); err != nil {
		t.Fatal(err)
	}
	var globalHelp []byte
	for _, argument := range []string{"help", "-h", "--help"} {
		help := exec.Command(binary, argument)
		help.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+config)
		helpOutput, helpErr := help.CombinedOutput()
		if helpErr != nil || !strings.Contains(string(helpOutput), "tasks — gestor local") {
			t.Fatalf("%s err=%v output=%s", argument, helpErr, helpOutput)
		}
		if globalHelp == nil {
			globalHelp = helpOutput
		} else if !bytes.Equal(globalHelp, helpOutput) {
			t.Fatalf("%s output differs from global help", argument)
		}
	}
	if _, err := os.Stat(config); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("help created configuration: %v", err)
	}
	unknown := exec.Command(binary, "unknown")
	unknown.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+config)
	unknownOutput, unknownErr := unknown.CombinedOutput()
	var exitError *exec.ExitError
	if !errors.As(unknownErr, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(string(unknownOutput), `comando desconocido "unknown"`) || !strings.Contains(string(unknownOutput), `Use "tasks help"`) {
		t.Fatalf("unknown err=%v output=%s", unknownErr, unknownOutput)
	}
	if _, err := os.Stat(config); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unknown command created configuration: %v", err)
	}
	prompt := exec.Command(binary, "ai-prompt")
	prompt.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+config)
	promptOutput, err := prompt.CombinedOutput()
	if err != nil || !strings.Contains(string(promptOutput), `"tasks-project"`) || !strings.Contains(string(promptOutput), "JSON puro") {
		t.Fatalf("ai-prompt err=%v output=%s", err, promptOutput)
	}

	input := `{"format":"tasks-project","version":1,"statuses":[{"key":"todo","name":"Pendiente","initial":true}],"tasks":[{"key":"plan","title":"Plan importado","priority":"urgent"}]}`
	command := exec.Command(binary, "import", "imported.tasks", "-")
	command.Dir = projectDir
	command.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+config)
	command.Stdin = strings.NewReader(input)
	output, err := command.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "1 estados, 1 tareas, 0 subtareas, 0 dependencias") {
		t.Fatalf("import err=%v output=%s", err, output)
	}
	projectPath := filepath.Join(projectDir, "imported.tasks")
	store, err := db.Open(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := store.ListTasks(context.Background(), ports.TaskFilter{IncludeDone: true, IncludeCancelled: true})
	closeErr := store.Close()
	if err != nil || closeErr != nil || len(tasks) != 1 || tasks[0].Title != "Plan importado" || tasks[0].Priority != domain.PriorityUrgent {
		t.Fatalf("tasks=%#v err=%v close=%v", tasks, err, closeErr)
	}
	index, err := registry.Open(filepath.Join(config, "tasks", "registry.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := index.Projects(context.Background())
	closeErr = index.Close()
	if err != nil || closeErr != nil || len(paths) != 1 || paths[0] != projectPath {
		t.Fatalf("registry paths=%v err=%v close=%v", paths, err, closeErr)
	}
}
