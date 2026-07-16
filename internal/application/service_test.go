package application

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	db "github.com/Polo123456789/tasks/internal/adapters/sqlite"
	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/ports"
)

type fixedClock struct{ today domain.Date }

func (c fixedClock) Today() domain.Date { return c.today }
func (c fixedClock) Now() time.Time     { return c.today.Time() }

func testSource(kind domain.OriginKind, key, name string, store ports.TaskStore) Source {
	return Source{Origin: domain.TaskOrigin{Kind: kind, Key: key, Name: name}, Store: store}
}

func TestCreateRecurringTaskSetsAnchorFromClock(t *testing.T) {
	store, err := db.Open(filepath.Join(t.TempDir(), "project.tasks"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	today, _ := domain.ParseDate("2026-07-15")
	service := Service{
		Mode:           domain.ModeLocal,
		Clock:          fixedClock{today: today},
		Sources:        []Source{testSource(domain.OriginProject, "project.tasks", "project", store)},
		WritableSource: "project.tasks",
	}
	recurrence := domain.Recurrence{Kind: domain.Daily}
	task, err := service.CreateTask(context.Background(), domain.Task{Title: "daily", Recurrence: &recurrence})
	if err != nil {
		t.Fatal(err)
	}
	if task.RecurrenceAnchor == nil || *task.RecurrenceAnchor != today {
		t.Fatalf("anchor=%v, want %s", task.RecurrenceAnchor, today)
	}
}

func TestGlobalModeCreatesCompleteTasksOnlyInGlobalSource(t *testing.T) {
	globalStore, err := db.Open(filepath.Join(t.TempDir(), "global.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer globalStore.Close()
	projectStore, err := db.Open(filepath.Join(t.TempDir(), "project.tasks"))
	if err != nil {
		t.Fatal(err)
	}
	defer projectStore.Close()
	today, _ := domain.ParseDate("2026-07-15")
	service := Service{
		Mode:           domain.ModeGlobal,
		Clock:          fixedClock{today: today},
		WritableSource: domain.GlobalOriginKey,
		Sources: []Source{
			testSource(domain.OriginGlobal, domain.GlobalOriginKey, "Global", globalStore),
			testSource(domain.OriginProject, "project.tasks", "project", projectStore),
		},
	}
	recurrence := domain.Recurrence{Kind: domain.Daily}
	created, err := service.CreateTask(context.Background(), domain.Task{Title: "daily", Recurrence: &recurrence})
	if err != nil || created.Origin.Kind != domain.OriginGlobal {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	plain, err := service.CreateTask(context.Background(), domain.Task{Title: "plain"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.AddSubtask(context.Background(), domain.GlobalOriginKey, created.ID, created.Version, "child"); err != nil {
		t.Fatalf("global subtask: %v", err)
	}
	created, _ = service.Task(context.Background(), domain.GlobalOriginKey, created.ID)
	if err = service.AddDependency(context.Background(), domain.GlobalOriginKey, created.ID, plain.ID, created.Version); err != nil {
		t.Fatalf("global dependency: %v", err)
	}
	projectTask, _ := projectStore.CreateTask(context.Background(), domain.Task{Title: "project"})
	if _, err = service.AddSubtask(context.Background(), "project.tasks", projectTask.ID, projectTask.Version, "forbidden"); err != domain.ErrForbidden {
		t.Fatalf("project subtask: got %v, want forbidden", err)
	}
}

func TestUnavailableGlobalSourceDisablesCreationAndReturnsItsError(t *testing.T) {
	unavailable := errors.New("global store unavailable")
	service := Service{
		Mode:           domain.ModeGlobal,
		WritableSource: domain.GlobalOriginKey,
		Sources: []Source{{
			Origin: domain.TaskOrigin{Kind: domain.OriginGlobal, Key: domain.GlobalOriginKey, Name: "Global"},
			Err:    unavailable,
		}},
	}
	if service.Capabilities("").CanCreateTask {
		t.Fatal("creation remained enabled for an unavailable global source")
	}
	if _, err := service.CreateTask(context.Background(), domain.Task{Title: "blocked"}); !errors.Is(err, unavailable) {
		t.Fatalf("create error=%v", err)
	}
}

func TestGlobalModeCanModifyExistingRecurrenceButNotAddOne(t *testing.T) {
	store, err := db.Open(filepath.Join(t.TempDir(), "project.tasks"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	today, _ := domain.ParseDate("2026-07-15")
	daily := domain.Recurrence{Kind: domain.Daily}
	recurring, _ := store.CreateTask(context.Background(), domain.Task{Title: "recurring", Recurrence: &daily, RecurrenceAnchor: &today})
	plain, _ := store.CreateTask(context.Background(), domain.Task{Title: "plain"})
	path := "/project.tasks"
	service := Service{Mode: domain.ModeGlobal, Clock: fixedClock{today}, Sources: []Source{testSource(domain.OriginProject, path, "project", store)}, WritableSource: domain.GlobalOriginKey}
	weekly := domain.Recurrence{Kind: domain.Weekly, Weekdays: []time.Weekday{time.Monday}}
	updated, err := service.UpdateTaskRecurrence(context.Background(), path, recurring.ID, recurring.Version, &weekly)
	if err != nil || updated.Recurrence == nil || updated.Recurrence.Kind != domain.Weekly {
		t.Fatalf("modify recurrence: %#v %v", updated, err)
	}
	updated, err = service.UpdateTaskRecurrence(context.Background(), path, updated.ID, updated.Version, nil)
	if err != nil || updated.Recurrence != nil || updated.RecurrenceAnchor != nil {
		t.Fatalf("remove recurrence: %#v %v", updated, err)
	}
	if _, err = service.UpdateTaskRecurrence(context.Background(), path, plain.ID, plain.Version, &daily); err != domain.ErrForbidden {
		t.Fatalf("add recurrence globally: %v", err)
	}
}

func TestGlobalModeRejectsSubtaskAndDependencyCreation(t *testing.T) {
	service := Service{Mode: domain.ModeGlobal}
	if _, err := service.AddSubtask(context.Background(), "project.tasks", 1, 1, "child"); err != domain.ErrForbidden {
		t.Fatalf("subtask: got %v, want forbidden", err)
	}
	if err := service.AddDependency(context.Background(), "project.tasks", 1, 2, 1); err != domain.ErrForbidden {
		t.Fatalf("dependency: got %v, want forbidden", err)
	}
	if _, err := service.CreateStatus(context.Background(), "project.tasks", "New", false); err != domain.ErrForbidden {
		t.Fatalf("status: got %v, want forbidden", err)
	}
	if err := service.RenameStatus(context.Background(), "project.tasks", 1, "Renamed"); err != domain.ErrForbidden {
		t.Fatalf("rename status: got %v, want forbidden", err)
	}
}

func TestMoveSubtaskAcrossProjectStatuses(t *testing.T) {
	store, err := db.Open(filepath.Join(t.TempDir(), "subtasks.tasks"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	task, _ := store.CreateTask(context.Background(), domain.Task{Title: "parent"})
	subtask, _ := store.AddSubtask(context.Background(), task.ID, task.Version, "child")
	task, _ = store.Task(context.Background(), task.ID)
	service := Service{Mode: domain.ModeLocal, Sources: []Source{testSource(domain.OriginProject, "project.tasks", "project", store)}, WritableSource: "project.tasks"}
	if err = service.MoveSubtaskStatus(context.Background(), "", task.ID, subtask.ID, task.Version, 1); err != nil {
		t.Fatal(err)
	}
	updated, _ := store.Task(context.Background(), task.ID)
	if len(updated.Subtasks) != 1 || updated.Subtasks[0].Status.Name != "En progreso" {
		t.Fatalf("subtask status=%#v", updated.Subtasks)
	}
}

func TestListTasksFiltersProjectsByPathOrName(t *testing.T) {
	open := func(name, title string) *db.Store {
		t.Helper()
		store, err := db.Open(filepath.Join(t.TempDir(), name+".tasks"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = store.CreateTask(context.Background(), domain.Task{Title: title}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { store.Close() })
		return store
	}
	firstPath := "/projects/first.tasks"
	service := Service{Mode: domain.ModeGlobal, Sources: []Source{
		testSource(domain.OriginProject, firstPath, "first", open("first", "one")),
		testSource(domain.OriginProject, "/other/second.tasks", "second", open("second", "two")),
	}}
	for _, filter := range []string{firstPath, "first"} {
		got, err := service.ListTasks(context.Background(), ports.TaskFilter{Origin: filter})
		if err != nil || len(got) != 1 || got[0].Title != "one" || got[0].Origin.Key != firstPath {
			t.Fatalf("filter %q: %#v %v", filter, got, err)
		}
	}
}

func TestGlobalMutationTargetsProjectPathWithDuplicateNamesAndIDs(t *testing.T) {
	first := func(path string) (*db.Store, domain.Task) {
		t.Helper()
		store, err := db.Open(filepath.Join(t.TempDir(), path))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { store.Close() })
		task, err := store.CreateTask(context.Background(), domain.Task{Title: "original"})
		if err != nil {
			t.Fatal(err)
		}
		return store, task
	}
	firstStore, firstTask := first("first.tasks")
	secondStore, secondTask := first("second.tasks")
	firstPath, secondPath := "/a/same.tasks", "/b/same.tasks"
	service := Service{Mode: domain.ModeGlobal, Sources: []Source{
		testSource(domain.OriginProject, firstPath, "same", firstStore),
		testSource(domain.OriginProject, secondPath, "same", secondStore),
	}}
	if firstTask.ID != secondTask.ID {
		t.Fatal("fixture did not produce duplicate IDs")
	}
	updated, err := service.UpdateTaskTitle(context.Background(), secondPath, secondTask.ID, secondTask.Version, "second only")
	if err != nil || updated.Title != "second only" {
		t.Fatalf("targeted update=%#v err=%v", updated, err)
	}
	firstAfter, _ := firstStore.Task(context.Background(), firstTask.ID)
	secondAfter, _ := secondStore.Task(context.Background(), secondTask.ID)
	if firstAfter.Title != "original" || secondAfter.Title != "second only" {
		t.Fatalf("wrong project mutated: first=%q second=%q", firstAfter.Title, secondAfter.Title)
	}
}

func TestAggregatedSortingHonorsRequestedOrder(t *testing.T) {
	open := func(name string, tasks []domain.Task) *db.Store {
		t.Helper()
		store, err := db.Open(filepath.Join(t.TempDir(), name+".tasks"))
		if err != nil {
			t.Fatal(err)
		}
		for _, task := range tasks {
			if _, err = store.CreateTask(context.Background(), task); err != nil {
				t.Fatal(err)
			}
		}
		t.Cleanup(func() { store.Close() })
		return store
	}
	early, _ := domain.ParseDate("2026-01-01")
	late, _ := domain.ParseDate("2026-02-01")
	service := Service{Mode: domain.ModeGlobal, Sources: []Source{
		testSource(domain.OriginProject, "/z.tasks", "z", open("z", []domain.Task{{Title: "Zulu", Priority: domain.PriorityLow, Due: &late}})),
		testSource(domain.OriginProject, "/a.tasks", "a", open("a", []domain.Task{{Title: "Alpha", Priority: domain.PriorityUrgent, Due: &early}})),
	}}
	cases := []struct {
		sort, first string
	}{{"title", "Alpha"}, {"priority", "Alpha"}, {"due", "Alpha"}}
	for _, tc := range cases {
		got, err := service.ListTasks(context.Background(), ports.TaskFilter{Sort: tc.sort})
		if err != nil || len(got) != 2 || got[0].Title != tc.first {
			t.Fatalf("sort %s: %#v err=%v", tc.sort, got, err)
		}
	}
}

func TestSetTaskLifecycleJumpsDirectlyToSpecialAndInitialStatuses(t *testing.T) {
	store, err := db.Open(filepath.Join(t.TempDir(), "lifecycle.tasks"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	task, err := store.CreateTask(context.Background(), domain.Task{Title: "task"})
	if err != nil {
		t.Fatal(err)
	}
	service := Service{Mode: domain.ModeLocal, Sources: []Source{testSource(domain.OriginProject, "project.tasks", "project", store)}, WritableSource: "project.tasks"}
	completed, err := service.SetTaskLifecycle(context.Background(), "", task.ID, task.Version, "complete")
	if err != nil || completed.Status.Kind != domain.StatusDone {
		t.Fatalf("complete=%#v err=%v", completed, err)
	}
	reopened, err := service.SetTaskLifecycle(context.Background(), "", completed.ID, completed.Version, "reopen")
	if err != nil || reopened.Status.Kind != domain.StatusNormal || !reopened.Status.Initial {
		t.Fatalf("reopen=%#v err=%v", reopened, err)
	}
	cancelled, err := service.SetTaskLifecycle(context.Background(), "", reopened.ID, reopened.Version, "cancel")
	if err != nil || cancelled.Status.Kind != domain.StatusCancelled {
		t.Fatalf("cancel=%#v err=%v", cancelled, err)
	}
}

func TestDependencyCandidatesIgnoreViewFiltersAndNameExistingChoices(t *testing.T) {
	store, err := db.Open(filepath.Join(t.TempDir(), "dependencies.tasks"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	parent, _ := store.CreateTask(context.Background(), domain.Task{Title: "parent"})
	first, _ := store.CreateTask(context.Background(), domain.Task{Title: "first"})
	second, _ := store.CreateTask(context.Background(), domain.Task{Title: "second"})
	service := Service{Mode: domain.ModeLocal, Sources: []Source{testSource(domain.OriginProject, "project.tasks", "project", store)}, WritableSource: "project.tasks"}
	choices, err := service.DependencyCandidates(context.Background(), "", parent.ID, false)
	if err != nil || len(choices) != 2 {
		t.Fatalf("choices=%#v err=%v", choices, err)
	}
	if err = service.AddDependency(context.Background(), "", parent.ID, second.ID, parent.Version); err != nil {
		t.Fatal(err)
	}
	existing, err := service.DependencyCandidates(context.Background(), "", parent.ID, true)
	if err != nil || len(existing) != 1 || existing[0].ID != second.ID || existing[0].Title != "second" {
		t.Fatalf("existing=%#v err=%v (first=%d)", existing, err, first.ID)
	}
}
