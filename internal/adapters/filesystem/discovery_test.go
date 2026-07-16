package filesystem

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverCurrentParentAndNoProject(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "directorio con espacios", "más profundo")
	if err := os.MkdirAll(nested, 0700); err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(root, "proyecto ünicode.tasks")
	touch(t, project)
	for _, start := range []string{root, nested} {
		got, err := Discover(start)
		if err != nil || got != project {
			t.Fatalf("Discover(%q)=%q, %v; want %q", start, got, err, project)
		}
	}
	empty := t.TempDir()
	got, err := Discover(empty)
	if err != nil || got != "" {
		t.Fatalf("empty discovery=%q, %v", got, err)
	}
}

func TestDiscoverStopsAtNearestProjectAndReportsAllConflicts(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	touch(t, filepath.Join(root, "parent.tasks"))
	nearest := filepath.Join(child, "nearest.tasks")
	touch(t, nearest)
	got, err := Discover(child)
	if err != nil || got != nearest {
		t.Fatalf("nearest=%q, %v", got, err)
	}
	other := filepath.Join(child, "another.tasks")
	touch(t, other)
	_, err = Discover(child)
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected ConflictError, got %v", err)
	}
	want := []string{other, nearest}
	if !reflect.DeepEqual(conflict.Files, want) {
		t.Fatalf("conflicts=%v, want %v", conflict.Files, want)
	}
}

func TestDiscoverResolvesProjectSymlinkAndExtensionIsExact(t *testing.T) {
	realDir := t.TempDir()
	realProject := filepath.Join(realDir, "real.tasks")
	touch(t, realProject)
	linkDir := t.TempDir()
	link := filepath.Join(linkDir, "linked.tasks")
	if err := os.Symlink(realProject, link); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(linkDir)
	if err != nil || got != realProject {
		t.Fatalf("symlink discovery=%q, %v", got, err)
	}
	upperDir := t.TempDir()
	touch(t, filepath.Join(upperDir, "not-a-project.TASKS"))
	got, err = Discover(upperDir)
	if err != nil || got != "" {
		t.Fatalf("uppercase extension discovered: %q, %v", got, err)
	}
}

func TestValidateProjectName(t *testing.T) {
	valid := []string{"project.tasks", "con espacios.tasks", "ünicode.tasks"}
	for _, name := range valid {
		if err := ValidateProjectName(name); err != nil {
			t.Errorf("valid name %q: %v", name, err)
		}
	}
	invalid := []string{"project", ".tasks", "project.TASKS", "dir/project.tasks", "../project.tasks"}
	for _, name := range invalid {
		if err := ValidateProjectName(name); err == nil {
			t.Errorf("invalid name accepted: %q", name)
		}
	}
}
