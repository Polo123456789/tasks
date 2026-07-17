package registry

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRegisterUsesCanonicalUniquePathsAndPruneRemovesMissing(t *testing.T) {
	root := t.TempDir()
	registry, err := Open(filepath.Join(root, "config", "registry.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()
	project := filepath.Join(root, "project.tasks")
	if err = os.WriteFile(project, nil, 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "alias.tasks")
	if err = os.Symlink(project, link); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, path := range []string{project, link, project} {
		if err = registry.Register(ctx, path); err != nil {
			t.Fatal(err)
		}
	}
	paths, err := registry.Projects(ctx)
	if err != nil || !reflect.DeepEqual(paths, []string{project}) {
		t.Fatalf("paths=%v, %v", paths, err)
	}
	if err = os.Remove(project); err != nil {
		t.Fatal(err)
	}
	live, err := registry.Prune(ctx)
	if err != nil || len(live) != 0 {
		t.Fatalf("live=%v, %v", live, err)
	}
	paths, _ = registry.Projects(ctx)
	if len(paths) != 0 {
		t.Fatalf("stale registry paths: %v", paths)
	}
}

func TestPruneKeepsEntryWhenFilesystemCheckFailsTransiently(t *testing.T) {
	root := t.TempDir()
	registry, err := Open(filepath.Join(root, "registry.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()
	loop := filepath.Join(root, "loop.tasks")
	if err = os.Symlink(loop, loop); err != nil {
		t.Fatal(err)
	}
	if _, err = registry.db.Exec("INSERT INTO projects(path) VALUES(?)", loop); err != nil {
		t.Fatal(err)
	}
	live, err := registry.Prune(context.Background())
	if err == nil {
		t.Fatal("expected the filesystem error to be reported")
	}
	if !reflect.DeepEqual(live, []string{loop}) {
		t.Fatalf("transiently unavailable project omitted from partial result: %v", live)
	}
	paths, err := registry.Projects(context.Background())
	if err != nil || !reflect.DeepEqual(paths, []string{loop}) {
		t.Fatalf("transient failure removed registry entry: paths=%v err=%v", paths, err)
	}
}

func TestProjectsReadOnlyDoesNotCreateOrPruneRegistry(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing", "registry.sqlite")
	if _, err := ProjectsReadOnly(context.Background(), missing); !os.IsNotExist(err) {
		t.Fatalf("missing registry error=%v", err)
	}
	if _, err := os.Stat(filepath.Dir(missing)); !os.IsNotExist(err) {
		t.Fatalf("read-only inspection created directory: %v", err)
	}
	path := filepath.Join(root, "registry.sqlite")
	registry, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(root, "gone.tasks")
	if _, err = registry.db.Exec("INSERT INTO projects(path) VALUES(?)", project); err != nil {
		t.Fatal(err)
	}
	if err = registry.Close(); err != nil {
		t.Fatal(err)
	}
	paths, err := ProjectsReadOnly(context.Background(), path)
	if err != nil || !reflect.DeepEqual(paths, []string{project}) {
		t.Fatalf("paths=%v err=%v", paths, err)
	}
	inspection, err := InspectReadOnly(context.Background(), path)
	if err != nil || inspection.Integrity != "ok" || !reflect.DeepEqual(inspection.Paths, []string{project}) {
		t.Fatalf("inspection=%#v err=%v", inspection, err)
	}
	registry, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()
	paths, err = registry.Projects(context.Background())
	if err != nil || !reflect.DeepEqual(paths, []string{project}) {
		t.Fatalf("read-only inspection pruned entry: paths=%v err=%v", paths, err)
	}
}

func TestProjectsReadOnlyRejectsWALWithoutCreatingSHM(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "registry.sqlite")
	registry, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = registry.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err = database.Exec("PRAGMA journal_mode=WAL; INSERT INTO projects(path) VALUES('/wal.tasks')"); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(path + "-shm"); err != nil {
		t.Fatal(err)
	}
	if _, err = ProjectsReadOnly(context.Background(), path); err == nil {
		t.Fatal("WAL registry was inspected as quiescent")
	}
	if _, err = os.Lstat(path + "-shm"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read-only registry inspection created SHM: %v", err)
	}
}
