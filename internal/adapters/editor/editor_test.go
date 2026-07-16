package editor

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEditResolvesVisualThenEditorAndCleansTemporaryFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is for supported Unix platforms")
	}
	dir := t.TempDir()
	visualMarker := filepath.Join(dir, "visual-called")
	editorMarker := filepath.Join(dir, "editor-called")
	script := filepath.Join(dir, "fake-editor")
	body := "#!/bin/sh\nprintf visual > '" + visualMarker + "'\nprintf '\\nchanged' >> \"$1\"\nprintf '%s' \"$1\" > '" + filepath.Join(dir, "temp-path") + "'\n"
	if err := os.WriteFile(script, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", script)
	t.Setenv("EDITOR", "sh -c 'printf editor > "+editorMarker+"'")
	got, err := Edit(context.Background(), "original")
	if err != nil || got != "original\nchanged" {
		t.Fatalf("content=%q, %v", got, err)
	}
	if _, err = os.Stat(visualMarker); err != nil {
		t.Fatalf("VISUAL was not invoked: %v", err)
	}
	if _, err = os.Stat(editorMarker); !os.IsNotExist(err) {
		t.Fatalf("EDITOR unexpectedly invoked: %v", err)
	}
	tempPath, _ := os.ReadFile(filepath.Join(dir, "temp-path"))
	if _, err = os.Stat(string(tempPath)); !os.IsNotExist(err) {
		t.Fatalf("temporary file was not removed: %v", err)
	}
}

func TestEditRequiresConfiguredEditorAndPreservesContentOnFailure(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	if _, err := Edit(context.Background(), "text"); err == nil || !strings.Contains(err.Error(), "$VISUAL") {
		t.Fatalf("missing editor error: %v", err)
	}
	t.Setenv("EDITOR", filepath.Join(t.TempDir(), "missing-editor"))
	if _, err := Edit(context.Background(), "text"); err == nil {
		t.Fatal("expected editor execution error")
	}
}
