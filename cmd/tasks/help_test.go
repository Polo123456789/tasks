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
	for _, text := range []string{"tasks — gestor local", "tasks init nombre.tasks", "tasks ai-prompt", "tasks import nombre.tasks", "-h, --help"} {
		if !strings.Contains(expected, text) {
			t.Fatalf("help missing %q", text)
		}
	}
	if _, err := os.Stat(config); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("help created configuration: %v", err)
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
		{name: "import target option", args: []string{"import", "--missing"}, want: `opción desconocida "--missing"`},
		{name: "import source option", args: []string{"import", "project.tasks", "--missing"}, want: `opción desconocida "--missing"`},
		{name: "help option", args: []string{"help", "--missing"}, want: `opción desconocida "--missing"`},
		{name: "help argument", args: []string{"help", "import"}, want: "uso: tasks help"},
		{name: "init missing", args: []string{"init"}, want: "uso: tasks init nombre.tasks"},
		{name: "init extra", args: []string{"init", "project.tasks", "extra"}, want: "uso: tasks init nombre.tasks"},
		{name: "prompt extra", args: []string{"ai-prompt", "extra"}, want: "uso: tasks ai-prompt"},
		{name: "import missing", args: []string{"import"}, want: "uso: tasks import nombre.tasks"},
		{name: "import extra", args: []string{"import", "project.tasks", "-", "extra"}, want: "uso: tasks import nombre.tasks"},
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
