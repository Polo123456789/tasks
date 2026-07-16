package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/ports"
	"path/filepath"
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

func TestOptimisticConflictAcrossIndependentConnections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "concurrent.tasks")
	first, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	created, _ := first.CreateTask(context.Background(), domain.Task{Title: "original"})
	second, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	stale, _ := second.Task(context.Background(), created.ID)
	created.Title = "first writer"
	if _, err = first.UpdateTask(context.Background(), created); err != nil {
		t.Fatal(err)
	}
	stale.Title = "lost update"
	if _, err = second.UpdateTask(context.Background(), stale); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected cross-connection conflict, got %v", err)
	}
}

func TestBusyDatabaseReturnsBoundedError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "busy.tasks")
	first, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if _, err = first.db.Exec("BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	defer first.db.Exec("ROLLBACK")
	if _, err = second.db.Exec("PRAGMA busy_timeout=25"); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	_, err = second.CreateTask(context.Background(), domain.Task{Title: "blocked"})
	if err == nil {
		t.Fatal("write unexpectedly succeeded while database was locked")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("busy error was not bounded: %s", elapsed)
	}
}

func TestTaskAndHistoryRollbackTogether(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if _, err := s.db.Exec(`CREATE TRIGGER reject_history BEFORE INSERT ON history BEGIN SELECT RAISE(ABORT,'history failed'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTask(ctx, domain.Task{Title: "must rollback"}); err == nil {
		t.Fatal("expected history failure")
	}
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM tasks").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("task committed without history: count=%d", count)
	}
}

func TestMigrationFromVersionOnePreservesData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.tasks")
	legacy := strings.Replace(schema, "CREATE TABLE project_config(key TEXT PRIMARY KEY NOT NULL, value TEXT NOT NULL);\n", "", 1)
	legacy = strings.Replace(legacy, "INSERT INTO project_config(key,value) VALUES ('trash_retention_days','30');\n", "", 1)
	legacy = strings.Replace(legacy, "PRAGMA user_version=2;", "PRAGMA user_version=1;", 1)
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(legacy); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec("INSERT INTO tasks(title,status_id,created_at,updated_at) VALUES('legacy',1,'2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')"); err != nil {
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var version int
	if err = store.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != SchemaVersion {
		t.Fatalf("version=%d err=%v", version, err)
	}
	var retention string
	if err = store.db.QueryRow("SELECT value FROM project_config WHERE key='trash_retention_days'").Scan(&retention); err != nil || retention != "30" {
		t.Fatalf("retention=%q err=%v", retention, err)
	}
	task, err := store.Task(context.Background(), 1)
	if err != nil || task.Title != "legacy" {
		t.Fatalf("legacy task=%#v err=%v", task, err)
	}
}

func TestOpenRejectsFutureAndCorruptDatabases(t *testing.T) {
	future := filepath.Join(t.TempDir(), "future.tasks")
	database, err := sql.Open("sqlite", future)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec("PRAGMA user_version=99"); err != nil {
		t.Fatal(err)
	}
	database.Close()
	if _, err = Open(future); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("future database error=%v", err)
	}
	corrupt := filepath.Join(t.TempDir(), "corrupt.tasks")
	if err = os.WriteFile(corrupt, []byte("not a SQLite database"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Open(corrupt); err == nil {
		t.Fatal("corrupt database was accepted")
	}
}

func TestClosedProjectIsSinglePortableFileWithoutSidecars(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "portable.tasks")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateTask(context.Background(), domain.Task{Title: "portable"}); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "portable.tasks" {
		t.Fatalf("project created sidecars: %v", entries)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if task, err := reopened.Task(context.Background(), 1); err != nil || task.Title != "portable" {
		t.Fatalf("reopened task=%#v err=%v", task, err)
	}
}

func TestListTasksCombinedFiltersAndSorts(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	statuses, _ := s.Statuses(ctx)
	progress := statuses[1]
	jan1, _ := domain.ParseDate("2026-01-01")
	jan5, _ := domain.ParseDate("2026-01-05")
	jan10, _ := domain.ParseDate("2026-01-10")
	jan20, _ := domain.ParseDate("2026-01-20")
	tasks := []domain.Task{
		{Title: "Zulu", StatusID: progress.ID, Priority: domain.PriorityHigh, Markdown: "needle", Start: &jan1, Due: &jan10},
		{Title: "Alpha", StatusID: progress.ID, Priority: domain.PriorityUrgent, Markdown: "needle", Due: &jan5},
		{Title: "Outside", StatusID: progress.ID, Priority: domain.PriorityHigh, Markdown: "needle", Start: &jan20},
		{Title: "Wrong status", Priority: domain.PriorityHigh, Markdown: "needle", Start: &jan5},
		{Title: "Wrong markdown", StatusID: progress.ID, Priority: domain.PriorityHigh, Markdown: "haystack", Start: &jan5},
	}
	for _, task := range tasks {
		if _, err := s.CreateTask(ctx, task); err != nil {
			t.Fatal(err)
		}
	}
	from, _ := domain.ParseDate("2026-01-03")
	to, _ := domain.ParseDate("2026-01-12")
	got, err := s.ListTasks(ctx, ports.TaskFilter{
		Markdown:   "needle",
		StatusIDs:  []int64{progress.ID},
		Priorities: []domain.Priority{domain.PriorityHigh, domain.PriorityUrgent},
		From:       &from,
		To:         &to,
		Sort:       "title",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Title != "Alpha" || got[1].Title != "Zulu" {
		t.Fatalf("filtered tasks: %#v", got)
	}
	for _, sortName := range []string{"priority", "status", "start", "due", "updated"} {
		if _, err = s.ListTasks(ctx, ports.TaskFilter{Sort: sortName}); err != nil {
			t.Fatalf("sort %s: %v", sortName, err)
		}
	}
}

func TestListTasksFiltersStatusByPortableName(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	review, _ := s.CreateStatus(ctx, "Review", false)
	_, _ = s.CreateTask(ctx, domain.Task{Title: "matching", StatusID: review.ID})
	_, _ = s.CreateTask(ctx, domain.Task{Title: "other"})
	got, err := s.ListTasks(ctx, ports.TaskFilter{StatusNames: []string{"Review"}})
	if err != nil || len(got) != 1 || got[0].Title != "matching" {
		t.Fatalf("status-name filter=%#v err=%v", got, err)
	}
}

func TestStatusAdministrationInvariants(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	created, err := s.CreateStatus(ctx, "Revisión", false)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.RenameStatus(ctx, created.ID, "  "); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("blank rename: %v", err)
	}
	if err = s.SetInitialStatus(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	statuses, _ := s.Statuses(ctx)
	var normalIDs []int64
	for _, status := range statuses {
		if status.Kind == domain.StatusNormal {
			normalIDs = append(normalIDs, status.ID)
			if status.Initial != (status.ID == created.ID) {
				t.Fatalf("unexpected initial state: %#v", statuses)
			}
		}
	}
	for left, right := 0, len(normalIDs)-1; left < right; left, right = left+1, right-1 {
		normalIDs[left], normalIDs[right] = normalIDs[right], normalIDs[left]
	}
	if err = s.ReorderStatuses(ctx, normalIDs); err != nil {
		t.Fatal(err)
	}
	statuses, _ = s.Statuses(ctx)
	for index, id := range normalIDs {
		if statuses[index].ID != id || statuses[index].Position != index+1 {
			t.Fatalf("statuses not reordered: %#v", statuses)
		}
	}
	if err = s.ReorderStatuses(ctx, []int64{normalIDs[0]}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("incomplete order: %v", err)
	}
	done := statusByKind(t, s, domain.StatusDone)
	if err = s.SetInitialStatus(ctx, done.ID); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("special initial status: %v", err)
	}
	if err = s.DeleteStatus(ctx, created.ID, nil); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("delete initial status: %v", err)
	}
}

func TestDeleteStatusMovesTasksAndSubtasksToNormalDestination(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	source, _ := s.CreateStatus(ctx, "Source", false)
	destination, _ := s.CreateStatus(ctx, "Destination", false)
	task, _ := s.CreateTask(ctx, domain.Task{Title: "task", StatusID: source.ID})
	subtask, _ := s.AddSubtask(ctx, task.ID, "subtask")
	if err := s.SetSubtaskStatus(ctx, subtask.ID, source.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteStatus(ctx, source.ID, nil); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("missing destination: %v", err)
	}
	done := statusByKind(t, s, domain.StatusDone)
	if err := s.DeleteStatus(ctx, source.ID, &done.ID); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("special destination: %v", err)
	}
	if err := s.DeleteStatus(ctx, source.ID, &destination.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Task(ctx, task.ID)
	if got.StatusID != destination.ID || len(got.Subtasks) != 1 || got.Subtasks[0].StatusID != destination.ID {
		t.Fatalf("status migration failed: %#v", got)
	}
	events, _ := s.History(ctx, task.ID)
	if len(events) == 0 || events[0].Kind != "status_changed" {
		t.Fatalf("status migration history missing: %#v", events)
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

func statusByKind(t *testing.T, s *Store, kind domain.StatusKind) domain.Status {
	t.Helper()
	statuses, err := s.Statuses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range statuses {
		if status.Kind == kind {
			return status
		}
	}
	t.Fatalf("missing status kind %s", kind)
	return domain.Status{}
}

func TestSubtaskCompletionRulesAndParentPropagation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	initial := statusByKind(t, s, domain.StatusNormal)
	done := statusByKind(t, s, domain.StatusDone)
	cancelled := statusByKind(t, s, domain.StatusCancelled)
	parent, _ := s.CreateTask(ctx, domain.Task{Title: "parent"})
	one, err := s.AddSubtask(ctx, parent.ID, " first ")
	if err != nil || one.Title != "first" {
		t.Fatalf("add first: %#v %v", one, err)
	}
	if err = s.SetSubtaskStatus(ctx, one.ID, done.ID); err != nil {
		t.Fatal(err)
	}
	parent, _ = s.Task(ctx, parent.ID)
	if parent.Status.Kind == domain.StatusDone {
		t.Fatal("one completed subtask must not complete its parent")
	}
	two, _ := s.AddSubtask(ctx, parent.ID, "second")
	if err = s.SetSubtaskStatus(ctx, two.ID, done.ID); err != nil {
		t.Fatal(err)
	}
	parent, _ = s.Task(ctx, parent.ID)
	if parent.Status.Kind != domain.StatusDone {
		t.Fatal("two completed subtasks must complete their parent")
	}
	parent, err = s.SetTaskStatus(ctx, parent.ID, initial.ID, parent.Version)
	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range parent.Subtasks {
		if sub.StatusID != initial.ID {
			t.Fatalf("reopen did not reset subtask %#v", sub)
		}
	}
	parent, err = s.SetTaskStatus(ctx, parent.ID, cancelled.ID, parent.Version)
	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range parent.Subtasks {
		if sub.StatusID != cancelled.ID {
			t.Fatalf("cancel did not propagate to subtask %#v", sub)
		}
	}
}

func TestStatusAndPriorityHistoryKinds(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	initial := statusByKind(t, s, domain.StatusNormal)
	done := statusByKind(t, s, domain.StatusDone)
	cancelled := statusByKind(t, s, domain.StatusCancelled)
	task, _ := s.CreateTask(ctx, domain.Task{Title: "history"})
	task, _ = s.SetTaskPriority(ctx, task.ID, domain.PriorityHigh, task.Version)
	task, _ = s.SetTaskStatus(ctx, task.ID, done.ID, task.Version)
	task, _ = s.SetTaskStatus(ctx, task.ID, initial.ID, task.Version)
	task, _ = s.SetTaskStatus(ctx, task.ID, cancelled.ID, task.Version)
	events, err := s.History(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"priority_changed": false, "completed": false, "reopened": false, "cancelled": false}
	for _, event := range events {
		if _, ok := want[event.Kind]; ok {
			want[event.Kind] = true
		}
	}
	for kind, found := range want {
		if !found {
			t.Errorf("missing %s event in %#v", kind, events)
		}
	}
	if _, err = s.SetTaskPriority(ctx, task.ID, domain.Priority(99), task.Version); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("invalid priority: %v", err)
	}
}

func TestDependencyBlockingTracksDoneCancelledAndReopened(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	initial := statusByKind(t, s, domain.StatusNormal)
	done := statusByKind(t, s, domain.StatusDone)
	cancelled := statusByKind(t, s, domain.StatusCancelled)
	dependent, _ := s.CreateTask(ctx, domain.Task{Title: "dependent"})
	prerequisite, _ := s.CreateTask(ctx, domain.Task{Title: "prerequisite"})
	if err := s.AddDependency(ctx, dependent.ID, prerequisite.ID); err != nil {
		t.Fatal(err)
	}
	assertBlocked := func(want bool) domain.Task {
		t.Helper()
		got, err := s.Task(ctx, dependent.ID)
		if err != nil || got.Blocked != want {
			t.Fatalf("blocked=%v, want %v (err=%v)", got.Blocked, want, err)
		}
		return got
	}
	assertBlocked(true)
	prerequisite, _ = s.SetTaskStatus(ctx, prerequisite.ID, done.ID, prerequisite.Version)
	assertBlocked(false)
	prerequisite, _ = s.SetTaskStatus(ctx, prerequisite.ID, initial.ID, prerequisite.Version)
	assertBlocked(true)
	prerequisite, _ = s.SetTaskStatus(ctx, prerequisite.ID, cancelled.ID, prerequisite.Version)
	assertBlocked(true)
}

func TestSubtaskAndDependencyValidation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	task, _ := s.CreateTask(ctx, domain.Task{Title: "task"})
	if _, err := s.AddSubtask(ctx, task.ID, "  "); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("blank subtask: %v", err)
	}
	if _, err := s.AddSubtask(ctx, 9999, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing parent: %v", err)
	}
	if err := s.AddDependency(ctx, task.ID, 9999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing dependency: %v", err)
	}
	sub, _ := s.AddSubtask(ctx, task.ID, "old")
	sub, err := s.RenameSubtask(ctx, sub.ID, " new ")
	if err != nil || sub.Title != "new" {
		t.Fatalf("rename subtask: %#v %v", sub, err)
	}
	if _, err = s.RenameSubtask(ctx, sub.ID, " "); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("blank rename: %v", err)
	}
}

func TestTrashedTaskSubtasksCannotBeEdited(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	task, _ := s.CreateTask(ctx, domain.Task{Title: "parent"})
	subtask, _ := s.AddSubtask(ctx, task.ID, "child")
	task, _ = s.Task(ctx, task.ID)
	today, _ := domain.ParseDate("2026-07-15")
	if _, err := s.TrashTask(ctx, task.ID, task.Version, today); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RenameSubtask(ctx, subtask.ID, "changed"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("renamed trashed subtask: %v", err)
	}
	initial := statusByKind(t, s, domain.StatusNormal)
	if err := s.SetSubtaskStatus(ctx, subtask.ID, initial.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("changed trashed subtask: %v", err)
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

func TestTrashRemovesDependenciesPermanentlyOnRestore(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	a, _ := s.CreateTask(ctx, domain.Task{Title: "a"})
	b, _ := s.CreateTask(ctx, domain.Task{Title: "b"})
	if err := s.AddDependency(ctx, a.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	today, _ := domain.ParseDate("2026-07-15")
	affected, err := s.TrashTask(ctx, b.ID, b.Version, today)
	if err != nil || len(affected) != 1 || affected[0] != a.ID {
		t.Fatalf("affected=%v err=%v", affected, err)
	}
	b, err = s.Task(ctx, b.ID)
	if err != nil || b.DeletedAt == nil {
		t.Fatalf("trashed task: %#v %v", b, err)
	}
	b, err = s.RestoreTask(ctx, b.ID, b.Version)
	if err != nil || b.DeletedAt != nil {
		t.Fatalf("restored task: %#v %v", b, err)
	}
	a, _ = s.Task(ctx, a.ID)
	if len(a.DependencyIDs) != 0 || a.Blocked {
		t.Fatalf("dependency restored unexpectedly: %#v", a)
	}
	events, _ := s.History(ctx, b.ID)
	var kinds []string
	for _, event := range events {
		kinds = append(kinds, event.Kind)
	}
	joined := strings.Join(kinds, ",")
	if !strings.Contains(joined, "restored") || !strings.Contains(joined, "trashed") {
		t.Fatalf("history lacks trash lifecycle: %s", joined)
	}
}

func TestPurgeBoundaryIsExactlyThirtyDays(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	day, _ := domain.ParseDate("2024-01-01")
	before, _ := s.CreateTask(ctx, domain.Task{Title: "before"})
	_, _ = s.TrashTask(ctx, before.ID, before.Version, day)
	if err := s.Maintain(ctx, day.AddDays(29)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Task(ctx, before.ID); err != nil {
		t.Fatalf("purged before 30 days: %v", err)
	}
	if err := s.Maintain(ctx, day.AddDays(30)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Task(ctx, before.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("not purged at 30 days: %v", err)
	}
}

func TestHistoryTimestampsAreParseable(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	task, _ := s.CreateTask(ctx, domain.Task{Title: "history"})
	events, err := s.History(ctx, task.ID)
	if err != nil || len(events) != 1 || events[0].CreatedAt.Equal(time.Time{}) {
		t.Fatalf("events=%#v err=%v", events, err)
	}
}

func TestRecurrenceMaintenanceResetsCurrentCycleAndRecordsSkipped(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	done := statusByKind(t, s, domain.StatusDone)
	anchor, _ := domain.ParseDate("2024-01-01")
	today, _ := domain.ParseDate("2024-01-04")
	recurrence := domain.Recurrence{Kind: domain.Daily}
	task, err := s.CreateTask(ctx, domain.Task{
		Title:            "daily",
		Priority:         domain.PriorityUrgent,
		Markdown:         "keep me",
		Recurrence:       &recurrence,
		RecurrenceAnchor: &anchor,
	})
	if err != nil {
		t.Fatal(err)
	}
	sub, _ := s.AddSubtask(ctx, task.ID, "child")
	task, _ = s.Task(ctx, task.ID)
	task, _ = s.SetTaskStatus(ctx, task.ID, done.ID, task.Version)
	dependency, _ := s.CreateTask(ctx, domain.Task{Title: "dependency"})
	if err = s.AddDependency(ctx, task.ID, dependency.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.Maintain(ctx, today); err != nil {
		t.Fatal(err)
	}
	got, err := s.Task(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status.Kind != domain.StatusNormal || got.RecurrenceAnchor == nil || got.RecurrenceAnchor.String() != today.String() {
		t.Fatalf("task was not advanced to current cycle: %#v", got)
	}
	if got.Markdown != "keep me" || got.Priority != domain.PriorityUrgent || len(got.DependencyIDs) != 1 {
		t.Fatalf("recurrence reset lost task data: %#v", got)
	}
	if len(got.Subtasks) != 1 || got.Subtasks[0].ID != sub.ID || got.Subtasks[0].StatusID != got.StatusID {
		t.Fatalf("subtask was not reset: %#v", got.Subtasks)
	}
	events, _ := s.History(ctx, task.ID)
	var completed, reset bool
	for _, event := range events {
		if event.Kind == "recurrence_cycle_completed" && event.Detail == "skipped=2" {
			completed = true
		}
		if event.Kind == "recurrence_reset" && event.Detail == "skipped=2" {
			reset = true
		}
	}
	if !completed || !reset {
		t.Fatalf("missing recurrence history: %#v", events)
	}
	version := got.Version
	if err = s.Maintain(ctx, today); err != nil {
		t.Fatal(err)
	}
	again, _ := s.Task(ctx, task.ID)
	if again.Version != version {
		t.Fatalf("maintenance is not idempotent: version %d -> %d", version, again.Version)
	}
}

func TestRecurrenceMaintenanceRecordsIncompleteCycle(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	anchor, _ := domain.ParseDate("2024-02-01")
	today := anchor.AddDays(1)
	recurrence := domain.Recurrence{Kind: domain.Daily}
	task, _ := s.CreateTask(ctx, domain.Task{Title: "unfinished", Recurrence: &recurrence, RecurrenceAnchor: &anchor})
	if err := s.Maintain(ctx, today); err != nil {
		t.Fatal(err)
	}
	events, _ := s.History(ctx, task.ID)
	for _, event := range events {
		if event.Kind == "recurrence_cycle_incomplete" {
			return
		}
	}
	t.Fatalf("missing incomplete cycle event: %#v", events)
}
