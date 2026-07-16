package sqlite

import (
	"context"
	"errors"
	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/ports"
	"path/filepath"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, e := Open(filepath.Join(t.TempDir(), "project.tasks"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func TestLifecycleAndOptimisticConflict(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	task, e := s.CreateTask(ctx, domain.Task{Title: "Test", Priority: domain.PriorityHigh})
	if e != nil {
		t.Fatal(e)
	}
	task.Title = "Changed"
	updated, e := s.UpdateTask(ctx, task)
	if e != nil {
		t.Fatal(e)
	}
	if updated.Version != 2 {
		t.Fatalf("version %d", updated.Version)
	}
	if _, e = s.UpdateTask(ctx, task); !errors.Is(e, domain.ErrConflict) {
		t.Fatalf("expected conflict, got %v", e)
	}
	list, e := s.ListTasks(ctx, ports.TaskFilter{IncludeDone: true, IncludeCancelled: true})
	if e != nil || len(list) != 1 {
		t.Fatal(list, e)
	}
}
func TestDependencyCycle(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	a, _ := s.CreateTask(ctx, domain.Task{Title: "a"})
	b, _ := s.CreateTask(ctx, domain.Task{Title: "b"})
	c, _ := s.CreateTask(ctx, domain.Task{Title: "c"})
	if e := s.AddDependency(ctx, a.ID, b.ID); e != nil {
		t.Fatal(e)
	}
	if e := s.AddDependency(ctx, b.ID, c.ID); e != nil {
		t.Fatal(e)
	}
	if e := s.AddDependency(ctx, c.ID, a.ID); !errors.Is(e, domain.ErrDependencyCycle) {
		t.Fatalf("expected cycle: %v", e)
	}
}
func TestTrashAndPurge(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	v, _ := s.CreateTask(ctx, domain.Task{Title: "old"})
	day, _ := domain.ParseDate("2024-01-01")
	if _, e := s.TrashTask(ctx, v.ID, v.Version, day); e != nil {
		t.Fatal(e)
	}
	later := day.AddDays(30)
	if e := s.Maintain(ctx, later); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Task(ctx, v.ID); !errors.Is(e, domain.ErrNotFound) {
		t.Fatalf("expected purge, got %v", e)
	}
}
