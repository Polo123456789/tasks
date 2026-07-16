package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobalHelpAliasesMatchWithoutCreatingConfiguration(t *testing.T) {
	config := filepath.Join(t.TempDir(), "config")
	t.Setenv("XDG_CONFIG_HOME", config)
	var expected string
	for _, argument := range []string{"help", "-h", "--help"} {
		var output bytes.Buffer
		if err := runArgs([]string{argument}, strings.NewReader(""), &output); err != nil {
			t.Fatalf("%s: %v", argument, err)
		}
		if expected == "" {
			expected = output.String()
		} else if output.String() != expected {
			t.Fatalf("%s output differs from global help", argument)
		}
	}
	for _, text := range []string{"tasks — gestor local", "tasks init nombre.tasks", "tasks ai-prompt", "tasks import nombre.tasks", "tasks add [--project ruta.tasks]", "tasks add --help", "tasks new [--priority nivel]", "tasks summary", "tasks is-project", "--project", "--global", "--color=", "-h, --help"} {
		if !strings.Contains(expected, text) {
			t.Fatalf("help missing %q", text)
		}
	}
	if _, err := os.Stat(config); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("help created configuration: %v", err)
	}
}

func TestNewHelpExplainsDestinationsAndOptionsWithoutCreatingConfiguration(t *testing.T) {
	config := filepath.Join(t.TempDir(), "config")
	t.Setenv("XDG_CONFIG_HOME", config)
	var expected string
	for _, argument := range []string{"-h", "--help"} {
		var output bytes.Buffer
		if err := runArgs([]string{"new", argument}, strings.NewReader(""), &output); err != nil {
			t.Fatalf("new %s: %v", argument, err)
		}
		if expected == "" {
			expected = output.String()
		} else if output.String() != expected {
			t.Fatalf("new %s output differs", argument)
		}
	}
	for _, text := range []string{"Dentro de un proyecto", "fuera, Global", "--global", "--project ruta", "--priority", "--start", "--due", "mutuamente excluyentes", "títulos que empiezan por guion", "ruta cuando es un proyecto", "JSON"} {
		if !strings.Contains(expected, text) {
			t.Fatalf("new help missing %q", text)
		}
	}
	if _, err := os.Stat(config); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("new help created configuration: %v", err)
	}
}

func TestAddHelpAliasesDescribeFormatWithoutCreatingConfiguration(t *testing.T) {
	config := filepath.Join(t.TempDir(), "config")
	t.Setenv("XDG_CONFIG_HOME", config)
	var expected string
	for _, argument := range []string{"-h", "--help"} {
		var output bytes.Buffer
		if err := runArgs([]string{"add", argument}, strings.NewReader(""), &output); err != nil {
			t.Fatalf("add %s: %v", argument, err)
		}
		if expected == "" {
			expected = output.String()
		} else if output.String() != expected {
			t.Fatalf("add %s output differs", argument)
		}
	}
	for _, text := range []string{
		"tasks add — agregar tareas desde JSON",
		`"format": "tasks-project"`,
		`"version": 1`,
		`"statuses"`,
		`"tasks"`,
		"priority",
		"recurrence",
		"depends_on",
		"El lote es atómico",
	} {
		if !strings.Contains(expected, text) {
			t.Fatalf("add help missing %q", text)
		}
	}
	if _, err := os.Stat(config); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("add help created configuration: %v", err)
	}
}

func TestParseInvocationRejectsUnknownCommandsOptionsAndBadArity(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "command", args: []string{"missing"}, want: `comando desconocido "missing"`},
		{name: "global option", args: []string{"--missing"}, want: `opción desconocida "--missing"`},
		{name: "init option", args: []string{"init", "--missing"}, want: `opción desconocida "--missing"`},
		{name: "prompt option", args: []string{"ai-prompt", "--missing"}, want: `opción desconocida "--missing"`},
		{name: "is-project option", args: []string{"is-project", "--missing"}, want: `opción desconocida "--missing"`},
		{name: "import target option", args: []string{"import", "--missing"}, want: `opción desconocida "--missing"`},
		{name: "import source option", args: []string{"import", "project.tasks", "--missing"}, want: `opción desconocida "--missing"`},
		{name: "add option", args: []string{"add", "--missing"}, want: `opción desconocida "--missing"`},
		{name: "add missing project", args: []string{"add", "--project"}, want: "uso: tasks add"},
		{name: "add duplicate project", args: []string{"add", "--project=a.tasks", "--project", "b.tasks"}, want: "uso: tasks add"},
		{name: "add extra source", args: []string{"add", "one.json", "two.json"}, want: "uso: tasks add"},
		{name: "new missing title", args: []string{"new", "--priority", "high"}, want: "uso: tasks new"},
		{name: "new unknown option", args: []string{"new", "--missing", "title"}, want: `opción desconocida "--missing"`},
		{name: "new conflicting destination", args: []string{"new", "--global", "--project", "a.tasks", "title"}, want: "uso: tasks new"},
		{name: "new duplicate priority", args: []string{"new", "--priority", "low", "--priority=high", "title"}, want: "uso: tasks new"},
		{name: "new extra title", args: []string{"new", "one", "two"}, want: "uso: tasks new"},
		{name: "help option", args: []string{"help", "--missing"}, want: `opción desconocida "--missing"`},
		{name: "help argument", args: []string{"help", "import"}, want: "uso: tasks help"},
		{name: "init missing", args: []string{"init"}, want: "uso: tasks init nombre.tasks"},
		{name: "init extra", args: []string{"init", "project.tasks", "extra"}, want: "uso: tasks init nombre.tasks"},
		{name: "prompt extra", args: []string{"ai-prompt", "extra"}, want: "uso: tasks ai-prompt"},
		{name: "is-project extra", args: []string{"is-project", "extra"}, want: "uso: tasks is-project"},
		{name: "import missing", args: []string{"import"}, want: "uso: tasks import nombre.tasks"},
		{name: "import extra", args: []string{"import", "project.tasks", "-", "extra"}, want: "uso: tasks import nombre.tasks"},
		{name: "summary argument", args: []string{"summary", "extra"}, want: "uso: tasks summary"},
		{name: "summary option", args: []string{"summary", "--color=sometimes"}, want: `opción desconocida "--color=sometimes"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseInvocation(test.args)
			if err == nil || !strings.Contains(err.Error(), test.want) || !strings.Contains(err.Error(), `Use "tasks help"`) {
				t.Fatalf("error=%v, want %q and help suggestion", err, test.want)
			}
		})
	}
}

func TestParseInvocationPreservesTUIAndImportStdin(t *testing.T) {
	invocation, err := parseInvocation(nil)
	if err != nil || invocation.kind != commandTUI {
		t.Fatalf("bare invocation=%#v err=%v", invocation, err)
	}
	invocation, err = parseInvocation([]string{"import", "project.tasks", "-"})
	if err != nil || invocation.kind != commandImport || invocation.project != "project.tasks" || invocation.source != "-" {
		t.Fatalf("import invocation=%#v err=%v", invocation, err)
	}
	invocation, err = parseInvocation([]string{"is-project"})
	if err != nil || invocation.kind != commandIsProject {
		t.Fatalf("is-project invocation=%#v err=%v", invocation, err)
	}
	invocation, err = parseInvocation([]string{"add", "batch.json", "--project", "project.tasks"})
	if err != nil || invocation.kind != commandAdd || invocation.source != "batch.json" || invocation.project != "project.tasks" || !invocation.projectSet {
		t.Fatalf("add invocation=%#v err=%v", invocation, err)
	}
	invocation, err = parseInvocation([]string{"add", "--project=other.tasks", "-"})
	if err != nil || invocation.kind != commandAdd || invocation.source != "-" || invocation.project != "other.tasks" || !invocation.projectSet {
		t.Fatalf("add equals invocation=%#v err=%v", invocation, err)
	}
	invocation, err = parseInvocation([]string{"add", "--help"})
	if err != nil || invocation.kind != commandAddHelp {
		t.Fatalf("add help invocation=%#v err=%v", invocation, err)
	}
	invocation, err = parseInvocation([]string{"new", "--project=other.tasks", "--priority", "urgent", "--start=2026-07-20", "--due", "2026-07-22", "Entrega"})
	if err != nil || invocation.kind != commandNew || invocation.source != "Entrega" || invocation.project != "other.tasks" || !invocation.projectSet || invocation.global || invocation.priority != "urgent" || invocation.start != "2026-07-20" || invocation.due != "2026-07-22" {
		t.Fatalf("new invocation=%#v err=%v", invocation, err)
	}
	invocation, err = parseInvocation([]string{"new", "--global", "Personal"})
	if err != nil || invocation.kind != commandNew || !invocation.global || invocation.source != "Personal" || invocation.priority != "none" {
		t.Fatalf("global new invocation=%#v err=%v", invocation, err)
	}
	invocation, err = parseInvocation([]string{"new", "--global", "--", "--help"})
	if err != nil || invocation.kind != commandNew || !invocation.global || invocation.source != "--help" {
		t.Fatalf("dash-prefixed title invocation=%#v err=%v", invocation, err)
	}
	invocation, err = parseInvocation([]string{"new", "--help"})
	if err != nil || invocation.kind != commandNewHelp {
		t.Fatalf("new help invocation=%#v err=%v", invocation, err)
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"summary"}, want: "auto"},
		{args: []string{"summary", "--color=always"}, want: "always"},
		{args: []string{"summary", "--no-color"}, want: "never"},
	} {
		invocation, err = parseInvocation(test.args)
		if err != nil || invocation.kind != commandSummary || invocation.color != test.want {
			t.Fatalf("summary invocation=%#v err=%v, want color %q", invocation, err, test.want)
		}
	}
}

func TestUnknownInvocationHasNoRuntimeSideEffects(t *testing.T) {
	config := filepath.Join(t.TempDir(), "config")
	t.Setenv("XDG_CONFIG_HOME", config)
	var output bytes.Buffer
	err := runArgs([]string{"unknown"}, strings.NewReader(""), &output)
	if err == nil || !strings.Contains(err.Error(), "comando desconocido") {
		t.Fatalf("error=%v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("unexpected stdout=%q", output.String())
	}
	if _, statErr := os.Stat(config); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("unknown command created configuration: %v", statErr)
	}
}
