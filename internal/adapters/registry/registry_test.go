package registry

import (
	"context"
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
