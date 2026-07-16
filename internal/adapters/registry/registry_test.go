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
