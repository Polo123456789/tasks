package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
