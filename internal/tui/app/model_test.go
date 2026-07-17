package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/ports"
	"github.com/Polo123456789/tasks/internal/tui/presenter"
	"github.com/Polo123456789/tasks/internal/tui/screens/taskdetail"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type fakeBackend struct {
	mode               domain.Mode
	tasks              []domain.Task
	deleted            []domain.Task
	trashCalls         int
	trashErr           error
	restoreCalls       int
	restoreErr         error
	lastRestoreVersion int64
	updateCalls        int
	moveCalls          int
	moveErr            error
	moveNoop           bool
	lifecycleErr       error
	setStatusCalls     int
	setStatusErr       error
	lastSetStatus      int64
	lastSetVersion     int64
	priorityCalls      int
	dateCalls          int
	addSubCalls        int
	renameSubCalls     int
	toggleSubCalls     int
	addDepCalls        int
	removeDepCalls     int
	impact             []int64
	lastFilter         ports.TaskFilter
	today              domain.Date
	maintainCalls      int
	createStatusCalls  int
	renameStatusCalls  int
	initialStatusCalls int
	reorderStatusCalls int
	deleteStatusCalls  int
	listErr            error
	workflowStatuses   []domain.Status
	saveCalls          int
	savedTask          domain.Task
	saveErr            error
	createCalls        int
	createErr          error
	detailErr          error
	formStatusesErr    error
}

func (b *fakeBackend) Mode() domain.Mode { return b.mode }
func (b *fakeBackend) ContextLabel() string {
	if b.mode == domain.ModeGlobal {
		return "Global"
	}
	return "Local · prueba"
}
func (b *fakeBackend) Today() domain.Date {
	if b.today.IsZero() {
		b.today, _ = domain.ParseDate("2026-07-15")
	}
	return b.today
}
func (b *fakeBackend) Maintain(context.Context) error {
	b.maintainCalls++
	return nil
}
func (b *fakeBackend) Capabilities(source string) domain.Capabilities {
	if b.mode == domain.ModeLocal {
		return domain.Capabilities{CanCreateTask: true, CanCreateStatus: true, CanCreateSubtask: true, CanCreateDependency: true, CanCreateRecurrence: true}
	}
	capabilities := domain.Capabilities{CanCreateTask: true}
	if source == domain.GlobalOriginKey {
		capabilities.CanCreateSubtask = true
		capabilities.CanCreateDependency = true
		capabilities.CanCreateRecurrence = true
	}
	return capabilities
}
func (b *fakeBackend) List(_ context.Context, filter ports.TaskFilter) ([]domain.Task, error) {
	b.lastFilter = filter
	if filter.IncludeDeleted {
		return b.deleted, b.listErr
	}
	return b.tasks, b.listErr
}
func (b *fakeBackend) Statuses(context.Context) ([]domain.Status, error) { return nil, nil }
func (b *fakeBackend) Create(_ context.Context, title string) (domain.Task, error) {
	b.createCalls++
	return domain.Task{ID: 98, Title: title, Origin: domain.TaskOrigin{Key: domain.GlobalOriginKey}}, b.createErr
}
func (b *fakeBackend) SaveTask(_ context.Context, _ string, task domain.Task) (domain.Task, error) {
	b.saveCalls++
	b.savedTask = task
	if task.ID == 0 {
		task.ID = 99
		if b.mode == domain.ModeGlobal {
			task.Origin = domain.TaskOrigin{Kind: domain.OriginGlobal, Key: domain.GlobalOriginKey, Name: "Global"}
		}
	}
	return task, b.saveErr
}
func (b *fakeBackend) FormStatuses(context.Context, string) ([]domain.Status, error) {
	if b.formStatusesErr != nil {
		return nil, b.formStatusesErr
	}
	if len(b.workflowStatuses) > 0 {
		return b.workflowStatuses, nil
	}
	return []domain.Status{{ID: 1, Name: "Pendiente", Kind: domain.StatusNormal, Initial: true}}, nil
}
func (b *fakeBackend) UpdateTitle(context.Context, string, int64, int64, string) (domain.Task, error) {
	b.updateCalls++
	return domain.Task{}, nil
}
func (b *fakeBackend) MoveStatus(_ context.Context, source string, id, version int64, _ int) (domain.Task, error) {
	b.moveCalls++
	if b.moveNoop {
		return domain.Task{ID: id, Version: version, Origin: domain.TaskOrigin{Key: source}}, b.moveErr
	}
	return domain.Task{ID: id, Version: version + 1, Origin: domain.TaskOrigin{Key: source}}, b.moveErr
}
func (b *fakeBackend) SetStatus(_ context.Context, source string, id, status, version int64) (domain.Task, error) {
	b.setStatusCalls++
	b.lastSetStatus, b.lastSetVersion = status, version
	return domain.Task{ID: id, StatusID: status, Version: version + 1, Origin: domain.TaskOrigin{Key: source}}, b.setStatusErr
}
func (b *fakeBackend) SetLifecycle(_ context.Context, source string, id, version int64, _ string) (domain.Task, error) {
	b.moveCalls++
	return domain.Task{ID: id, Version: version + 1, Origin: domain.TaskOrigin{Key: source}}, b.lifecycleErr
}
func (b *fakeBackend) CyclePriority(context.Context, string, int64, int64) (domain.Task, error) {
	b.priorityCalls++
	return domain.Task{}, nil
}
func (b *fakeBackend) UpdateDate(context.Context, string, int64, int64, string, *domain.Date) (domain.Task, error) {
	b.dateCalls++
	return domain.Task{}, nil
}
func (b *fakeBackend) Detail(_ context.Context, _ string, id int64) (domain.Task, error) {
	if b.detailErr != nil {
		return domain.Task{}, b.detailErr
	}
	for _, task := range append(b.tasks, b.deleted...) {
		if task.ID == id {
			return task, nil
		}
	}
	return domain.Task{}, domain.ErrNotFound
}
func (b *fakeBackend) History(context.Context, string, int64) ([]domain.HistoryEvent, error) {
	return []domain.HistoryEvent{{Kind: "created"}}, nil
}
func (b *fakeBackend) AddSubtask(context.Context, string, int64, int64, string) (domain.Subtask, error) {
	b.addSubCalls++
	return domain.Subtask{}, nil
}
func (b *fakeBackend) RenameSubtask(context.Context, string, int64, int64, int64, string) (domain.Subtask, error) {
	b.renameSubCalls++
	return domain.Subtask{}, nil
}
func (b *fakeBackend) ToggleSubtask(context.Context, string, int64, int64, int64) error {
	b.toggleSubCalls++
	return nil
}
func (b *fakeBackend) MoveSubtaskStatus(context.Context, string, int64, int64, int64, int) error {
	return nil
}
func (b *fakeBackend) AddDependency(context.Context, string, int64, int64, int64) error {
	b.addDepCalls++
	return nil
}
func (b *fakeBackend) RemoveDependency(context.Context, string, int64, int64, int64) error {
	b.removeDepCalls++
	return nil
}
func (b *fakeBackend) DependencyCandidates(_ context.Context, _ string, taskID int64, existingOnly bool) ([]domain.Task, error) {
	var selected domain.Task
	for _, task := range b.tasks {
		if task.ID == taskID {
			selected = task
		}
	}
	existing := make(map[int64]bool)
	for _, id := range selected.DependencyIDs {
		existing[id] = true
	}
	var out []domain.Task
	for _, task := range b.tasks {
		if task.ID != taskID && existing[task.ID] == existingOnly {
			out = append(out, task)
		}
	}
	return out, nil
}
func (b *fakeBackend) UpdateRecurrence(context.Context, string, int64, int64, *domain.Recurrence) (domain.Task, error) {
	return domain.Task{}, nil
}
func (b *fakeBackend) CreateStatus(context.Context, string) (domain.Status, error) {
	b.createStatusCalls++
	return domain.Status{}, nil
}
func (b *fakeBackend) RenameStatus(context.Context, int64, string) error {
	b.renameStatusCalls++
	return nil
}
func (b *fakeBackend) SetInitialStatus(context.Context, int64) error {
	b.initialStatusCalls++
	return nil
}
func (b *fakeBackend) ReorderStatuses(context.Context, []int64) error {
	b.reorderStatusCalls++
	return nil
}
func (b *fakeBackend) DeleteStatus(context.Context, int64, *int64) error {
	b.deleteStatusCalls++
	return nil
}
func (b *fakeBackend) MarkdownEditor(context.Context, string, int64, int64) (tea.ExecCommand, func(error) error, error) {
	return nil, nil, domain.ErrNotFound
}
func (b *fakeBackend) Trash(_ context.Context, source string, id, version int64) (domain.Task, []int64, error) {
	b.trashCalls++
	return domain.Task{ID: id, Version: version + 1, Origin: domain.TaskOrigin{Key: source}}, nil, b.trashErr
}
func (b *fakeBackend) DependencyImpact(context.Context, string, int64) ([]domain.Task, error) {
	result := make([]domain.Task, 0, len(b.impact))
	for _, id := range b.impact {
		result = append(result, domain.Task{ID: id, Title: "affected"})
	}
	return result, nil
}
func (b *fakeBackend) Restore(_ context.Context, source string, id, version int64) (domain.Task, error) {
	b.restoreCalls++
	b.lastRestoreVersion = version
	return domain.Task{ID: id, Version: version + 1, Origin: domain.TaskOrigin{Key: source}}, b.restoreErr
}

func key(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

func ctrlP() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyCtrlP} }

func TestGlobalNavigationNeverEntersKanban(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeGlobal}
	model := NewAt(backend, "kanban")
	if model.view != 2 {
		t.Fatalf("global initial view=%d, want calendar", model.view)
	}
	for i := 0; i < 20; i++ {
		updated, _ := model.Update(key("l"))
		model = updated.(Model)
		if model.view == 0 || model.view == 5 {
			t.Fatalf("global navigation entered forbidden view %d", model.view)
		}
	}
}

func TestQuitKeysReturnQuitCommand(t *testing.T) {
	for _, value := range []string{"q", "ctrl+c"} {
		model := New(&fakeBackend{mode: domain.ModeLocal})
		var message tea.KeyMsg
		if value == "ctrl+c" {
			message = tea.KeyMsg{Type: tea.KeyCtrlC}
		} else {
			message = key(value)
		}
		_, command := model.Update(message)
		if command == nil {
			t.Fatalf("%s did not return quit command", value)
		}
		if _, ok := command().(tea.QuitMsg); !ok {
			t.Fatalf("%s returned %T, want QuitMsg", value, command())
		}
	}
}

func TestQuitCancelsPendingDayWatcher(t *testing.T) {
	model := New(&fakeBackend{mode: domain.ModeLocal})
	done := make(chan tea.Msg, 1)
	go func() { done <- model.waitForDayCheck()() }()

	_, command := model.Update(key("q"))
	if command == nil {
		t.Fatal("q did not return quit command")
	}
	select {
	case message := <-done:
		if message != nil {
			t.Fatalf("watcher returned %T, want nil", message)
		}
	case <-time.After(time.Second):
		t.Fatal("pending day watcher was not cancelled")
	}
}

func TestGlobalInitialLoadHidesDoneAndCancelled(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeGlobal}
	model := New(backend)
	_ = model.Init()()
	if backend.lastFilter.IncludeDone || backend.lastFilter.IncludeCancelled {
		t.Fatalf("global default filter=%#v", backend.lastFilter)
	}
}

func TestPartialGlobalResultsRemainVisibleWhenOneProjectFails(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeGlobal, tasks: []domain.Task{{ID: 1, Title: "available"}}, listErr: errors.New("broken.tasks: unavailable")}
	model := New(backend)
	updated, _ := model.Update(model.Init()())
	model = updated.(Model)
	if model.err != nil || len(model.tasks) != 1 || !strings.Contains(model.notice, "broken.tasks") {
		t.Fatalf("partial state: err=%v tasks=%#v notice=%q", model.err, model.tasks, model.notice)
	}
}

func TestTrashAndRestoreCommandsReloadTheirViews(t *testing.T) {
	task := domain.Task{ID: 7, Title: "task", Version: 2, Origin: domain.TaskOrigin{Kind: domain.OriginProject, Key: "/p/project.tasks", Name: "project"}}
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{task}, deleted: []domain.Task{task}}
	model := NewAt(backend, "table")
	msg := model.Init()()
	updated, _ := model.Update(msg)
	model = updated.(Model)
	updated, cmd := model.Update(key("d"))
	model = updated.(Model)
	if cmd == nil || !model.loading {
		t.Fatal("trash did not start asynchronous mutation")
	}
	updated, trashCmd := model.Update(cmd())
	model = updated.(Model)
	if trashCmd == nil {
		t.Fatal("dependency check did not continue with trash")
	}
	updated, reload := model.Update(trashCmd())
	model = updated.(Model)
	if backend.trashCalls != 1 || reload == nil || model.notice != "Tarea enviada a la papelera" {
		t.Fatalf("trash calls=%d notice=%q reload=%v", backend.trashCalls, model.notice, reload != nil)
	}

	model.view = 4
	msg = model.load(true)()
	updated, _ = model.Update(msg)
	model = updated.(Model)
	updated, cmd = model.Update(key("u"))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("restore did not start mutation")
	}
	updated, reload = model.Update(cmd())
	model = updated.(Model)
	if backend.restoreCalls != 1 || reload == nil || model.notice != "Tarea restaurada" {
		t.Fatalf("restore calls=%d notice=%q reload=%v", backend.restoreCalls, model.notice, reload != nil)
	}
}

func TestTrashWithDependenciesRequiresConfirmation(t *testing.T) {
	task := domain.Task{ID: 7, Title: "task", Version: 2}
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{task}, impact: []int64{4, 9}}
	model := New(backend)
	updated, _ := model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	updated, check := model.Update(key("d"))
	model = updated.(Model)
	updated, trashCmd := model.Update(check())
	model = updated.(Model)
	if trashCmd != nil || model.confirmTrash == nil || backend.trashCalls != 0 {
		t.Fatalf("confirmation state=%#v calls=%d", model.confirmTrash, backend.trashCalls)
	}
	updated, trashCmd = model.Update(key("y"))
	model = updated.(Model)
	if trashCmd == nil || model.confirmTrash != nil {
		t.Fatal("confirmation did not start trash")
	}
	_, _ = model.Update(trashCmd())
	if backend.trashCalls != 1 {
		t.Fatalf("trash calls=%d", backend.trashCalls)
	}
}

func TestSelectionClampsWhenSwitchingToShorterResult(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeLocal}
	model := New(backend)
	model.selected = 8
	updated, _ := model.Update(loaded{tasks: []domain.Task{{ID: 1, Title: "only"}}})
	model = updated.(Model)
	if model.selected != 0 {
		t.Fatalf("selected=%d", model.selected)
	}
}

func TestEditTitleAndMoveStatusRunAsynchronousMutations(t *testing.T) {
	task := domain.Task{ID: 3, Title: "old", StatusID: 1, Version: 4}
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{task}}
	model := New(backend)
	updated, _ := model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	updated, formCommand := model.Update(key("e"))
	model = updated.(Model)
	if formCommand == nil || !model.form.open || !model.form.loading {
		t.Fatalf("edit form state: open=%v loading=%v command=%v", model.form.open, model.form.loading, formCommand != nil)
	}
	updated, _ = model.Update(formCommand())
	model = updated.(Model)
	model.form.text[formTitle] = newTextField("new")
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("edit did not return command")
	}
	updated, reload := model.Update(cmd())
	model = updated.(Model)
	if backend.saveCalls != 1 || reload == nil || model.notice != "Tarea actualizada" || backend.savedTask.Title != "new" {
		t.Fatalf("save calls=%d task=%#v notice=%q reload=%v", backend.saveCalls, backend.savedTask, model.notice, reload != nil)
	}
	updated, cmd = model.Update(key("]"))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("move did not return command")
	}
	_, _ = model.Update(cmd())
	if backend.moveCalls != 1 {
		t.Fatalf("move calls=%d", backend.moveCalls)
	}
}

func TestSubtaskAndDependencyInteractions(t *testing.T) {
	task := domain.Task{ID: 3, Title: "parent", Version: 1, Origin: domain.TaskOrigin{Kind: domain.OriginProject, Key: "/p.tasks", Name: "p"}, Subtasks: []domain.Subtask{{ID: 8, Title: "child"}}}
	candidate := domain.Task{ID: 42, Title: "required", Version: 1, Origin: domain.TaskOrigin{Kind: domain.OriginProject, Key: "/p.tasks", Name: "p"}, Status: domain.Status{Name: "Pending"}}
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{task, candidate}}
	model := New(backend)
	updated, cmd := model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("task list did not request detail")
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	if model.detail == nil || len(model.detail.Subtasks) != 1 {
		t.Fatalf("detail=%#v", model.detail)
	}

	updated, _ = model.Update(key("a"))
	model = updated.(Model)
	model.input = "new child"
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	_, _ = model.Update(cmd())
	if backend.addSubCalls != 1 {
		t.Fatalf("add subtask calls=%d", backend.addSubCalls)
	}

	model.inputMode = false
	updated, _ = model.Update(key("J"))
	model = updated.(Model)
	updated, cmd = model.Update(key("t"))
	model = updated.(Model)
	_, _ = model.Update(cmd())
	if backend.toggleSubCalls != 1 {
		t.Fatalf("toggle subtask calls=%d", backend.toggleSubCalls)
	}

	model.inputMode = false
	updated, cmd = model.Update(key("g"))
	model = updated.(Model)
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	_, _ = model.Update(cmd())
	if backend.addDepCalls != 1 {
		t.Fatalf("add dependency calls=%d", backend.addDepCalls)
	}

	model.inputMode = false
	backend.tasks[0].DependencyIDs = []int64{42}
	model.tasks[0].DependencyIDs = []int64{42}
	updated, cmd = model.Update(key("G"))
	model = updated.(Model)
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	_, _ = model.Update(cmd())
	if backend.removeDepCalls != 1 {
		t.Fatalf("remove dependency calls=%d", backend.removeDepCalls)
	}
}

func TestGlobalModeHidesNestedCreationForProjectTasks(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeGlobal, tasks: []domain.Task{{ID: 1, Title: "task"}}}
	model := New(backend)
	updated, _ := model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	for _, forbidden := range []string{"a", "g"} {
		updated, cmd := model.Update(key(forbidden))
		model = updated.(Model)
		if cmd != nil || model.inputMode {
			t.Fatalf("global key %q exposed creation", forbidden)
		}
	}
}

func TestInteractiveSearchFiltersAndSorting(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeLocal}
	model := New(backend)
	model.statuses = []domain.Status{{ID: 3, Name: "Review", Kind: domain.StatusNormal}}
	applyInput := func(keyValue, value string) {
		t.Helper()
		updated, _ := model.Update(key(keyValue))
		model = updated.(Model)
		model.input = value
		updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model = updated.(Model)
		if cmd == nil {
			t.Fatalf("filter %q did not reload", keyValue)
		}
		_ = cmd()
	}
	applyInput("/", "needle")
	if model.filter.Query != "needle" {
		t.Fatalf("title filter=%q", model.filter.Query)
	}
	applyInput("?", "markdown")
	if model.filter.Markdown != "markdown" {
		t.Fatalf("markdown filter=%q", model.filter.Markdown)
	}
	updated, _ := model.Update(key("S"))
	model = updated.(Model)
	updated, _ = model.Update(key("j"))
	model = updated.(Model)
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("status picker did not reload")
	}
	_ = cmd()
	if len(model.filter.StatusIDs) != 1 || model.filter.StatusIDs[0] != 3 {
		t.Fatalf("status filter=%v", model.filter.StatusIDs)
	}
	applyInput("D", "2026-01-01..2026-01-31")
	if model.filter.From == nil || model.filter.To == nil {
		t.Fatalf("date filter=%#v", model.filter)
	}
	for _, keyValue := range []string{"B", "R", "1", "o"} {
		updated, cmd := model.Update(key(keyValue))
		model = updated.(Model)
		if cmd == nil {
			t.Fatalf("toggle %q did not reload", keyValue)
		}
	}
	if !model.filter.OnlyBlocked || !model.filter.OnlyRecurring || len(model.filter.Priorities) != 1 || model.filter.Sort != "priority" {
		t.Fatalf("combined filter=%#v", model.filter)
	}
	updated, cmd = model.Update(key("0"))
	model = updated.(Model)
	if cmd == nil || model.filter.Query != "" || model.filter.Markdown != "" || model.filter.Sort != "updated" {
		t.Fatalf("reset filter=%#v", model.filter)
	}
}

func TestDayChangeRunsMaintenanceAndRefreshes(t *testing.T) {
	initial, _ := domain.ParseDate("2026-07-15")
	next := initial.AddDays(1)
	backend := &fakeBackend{mode: domain.ModeLocal, today: initial}
	model := New(backend)
	model.dayWatchStarted = true
	backend.today = next
	updated, cmd := model.Update(dayCheck{})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("day change did not start maintenance")
	}
	updated, refresh := model.Update(cmd())
	model = updated.(Model)
	if backend.maintainCalls != 1 || refresh == nil || !model.today.Equal(next) {
		t.Fatalf("calls=%d today=%s refresh=%v", backend.maintainCalls, model.today, refresh != nil)
	}
}

func TestLocalStatusAdministrationInteractions(t *testing.T) {
	statuses := []domain.Status{
		{ID: 1, Name: "Pending", Kind: domain.StatusNormal, Position: 1, Initial: true},
		{ID: 2, Name: "Progress", Kind: domain.StatusNormal, Position: 2},
		{ID: 3, Name: "Done", Kind: domain.StatusDone, Position: 3},
	}
	backend := &fakeBackend{mode: domain.ModeLocal}
	model := NewAt(backend, "settings")
	model.statuses = statuses
	updated, _ := model.Update(key("a"))
	model = updated.(Model)
	model.input = "Review"
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	_, _ = model.Update(cmd())
	if backend.createStatusCalls != 1 {
		t.Fatalf("create calls=%d", backend.createStatusCalls)
	}
	model.inputMode = false
	updated, cmd = model.Update(key("i"))
	model = updated.(Model)
	_, _ = model.Update(cmd())
	if backend.initialStatusCalls != 1 {
		t.Fatalf("initial calls=%d", backend.initialStatusCalls)
	}
	model.inputMode = false
	updated, cmd = model.Update(key("]"))
	model = updated.(Model)
	_, _ = model.Update(cmd())
	if backend.reorderStatusCalls != 1 {
		t.Fatalf("reorder calls=%d", backend.reorderStatusCalls)
	}
	model.inputMode = false
	updated, _ = model.Update(key("d"))
	model = updated.(Model)
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	_, _ = model.Update(cmd())
	if backend.deleteStatusCalls != 1 {
		t.Fatalf("delete calls=%d", backend.deleteStatusCalls)
	}
}

func TestHistoryOpensAndClosesFromTaskView(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{{ID: 1, Title: "task"}}}
	model := New(backend)
	updated, _ := model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	updated, cmd := model.Update(key("H"))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("history did not load")
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	if !model.historyOpen || len(model.history) != 1 {
		t.Fatalf("history state=%v %#v", model.historyOpen, model.history)
	}
	updated, _ = model.Update(key("H"))
	model = updated.(Model)
	if model.historyOpen {
		t.Fatal("history did not close")
	}
}

func TestLifecycleActionsAndVisibilityFiltersUseDistinctKeys(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{{ID: 1, Title: "task", Version: 1}}}
	model := New(backend)
	updated, _ := model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	updated, cmd := model.Update(key("f"))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("f did not start direct completion")
	}
	_, _ = model.Update(cmd())
	if backend.moveCalls != 1 {
		t.Fatalf("lifecycle calls=%d", backend.moveCalls)
	}
	before := model.filter.IncludeDone
	updated, cmd = model.Update(key("F"))
	model = updated.(Model)
	if cmd == nil || model.filter.IncludeDone == before {
		t.Fatal("F did not toggle completed visibility")
	}
}

func TestHelpAndContextAreVisibleAtSmallTerminal(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeLocal}
	model := New(backend)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 90, Height: 40})
	model = updated.(Model)
	updated, _ = model.Update(key("f1"))
	model = updated.(Model)
	view := model.View()
	for _, expected := range []string{"Local · prueba", "AYUDA DE TASKS", "F1 o Esc"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("missing %q in help:\n%s", expected, view)
		}
	}
}

func TestContextualFooterExposesEveryTaskActionAtMinimumSize(t *testing.T) {
	task := domain.Task{ID: 1, Title: "task", Version: 1, Recurrence: &domain.Recurrence{Kind: domain.Daily}, DependencyIDs: []int64{2}}
	task.Subtasks = []domain.Subtask{{ID: 11, Title: "subtask"}}
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{task}}
	model := New(backend)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 90, Height: 40})
	model = updated.(Model)
	updated, _ = model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	updated, _ = model.Update(detailLoaded{task: task})
	model = updated.(Model)
	model.notice = "Cambio guardado"

	view := model.View()
	for _, expected := range []string{
		"←/→ cambiar vista", "n nueva tarea", "e título", "p prioridad", "[/] estado",
		"f finalizar", "C cancelar", "z reabrir", "s inicio", "v vencimiento", "m Markdown",
		"c recurrencia", "d papelera", "H historial", "a añadir subtarea", "g agregar dependencia",
		"G quitar dependencia", "↑/↓ seleccionar", "E renombrar", "t completar/reabrir",
		"{/} cambiar estado", "/ título", "? Markdown", "S estado", "D fechas", "1 prioridad",
		"B bloqueadas", "R recurrentes", "F mostrar/ocultar finalizadas", "X mostrar/ocultar",
		"o ordenar", "0 limpiar", "r recargar", "q salir",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("missing contextual action %q:\n%s", expected, view)
		}
	}
	if lines := strings.Count(view, "\n") + 1; lines > 40 {
		t.Fatalf("rendered %d lines in the supported 90x40 terminal:\n%s", lines, view)
	}
}

func TestContextualFooterChangesForTransientModesAndKeepsNotices(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{{ID: 1, Title: "task"}}}
	model := New(backend)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 90, Height: 40})
	model = updated.(Model)
	updated, _ = model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)

	model.notice = "Operación terminada"
	view := model.View()
	for _, expected := range []string{"✓ ÉXITO", "Operación terminada", "ACCIONES", "n nueva tarea"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("notice replaced contextual help; missing %q:\n%s", expected, view)
		}
	}

	model.inputMode = true
	model.inputAction = "start"
	view = model.View()
	for _, expected := range []string{"FORMULARIO", "AAAA-MM-DD", "Enter guardar o aplicar", "Esc cancelar"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("input footer missing %q:\n%s", expected, view)
		}
	}
	if strings.Contains(view, "ACCIONES") {
		t.Fatalf("normal footer remained visible during input:\n%s", view)
	}

	model.inputMode = false
	model.openPicker("recurrence-weekly-days", model.tasks[0], weekdayPickerOptions())
	view = model.View()
	for _, expected := range []string{"SELECTOR", "Espacio marcar/desmarcar", "Enter confirmar", "Esc cancelar"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("picker footer missing %q:\n%s", expected, view)
		}
	}
}

func TestContextualFooterOnlyShowsActionsAvailableInGlobalMode(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeGlobal, tasks: []domain.Task{{ID: 1, Title: "task", DependencyCount: 1}}}
	model := NewAt(backend, "table")
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 90, Height: 40})
	model = updated.(Model)
	updated, _ = model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	view := model.View()
	for _, unavailable := range []string{"a añadir subtarea", "g agregar dependencia", "c recurrencia"} {
		if strings.Contains(view, unavailable) {
			t.Fatalf("global footer exposes unavailable action %q:\n%s", unavailable, view)
		}
	}
	for _, available := range []string{"n nueva tarea", "P origen", "G quitar dependencia", "e título", "F mostrar/ocultar finalizadas", "X mostrar/ocultar"} {
		if !strings.Contains(view, available) {
			t.Fatalf("global footer missing %q:\n%s", available, view)
		}
	}
}

func TestCalendarSelectionSkipsTasksNotShownInMonth(t *testing.T) {
	due, _ := domain.ParseDate("2026-07-20")
	backend := &fakeBackend{mode: domain.ModeGlobal, tasks: []domain.Task{{ID: 1, Title: "hidden"}, {ID: 2, Title: "visible", Due: &due}}}
	model := NewAt(backend, "calendar")
	updated, _ := model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	if model.selected != 1 {
		t.Fatalf("calendar selected task index=%d, want visible index 1", model.selected)
	}
}

func TestCalendarWithoutEventsDoesNotExposeAnInvisibleTask(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeGlobal, tasks: []domain.Task{{ID: 1, Title: "hidden task"}}}
	model := NewAt(backend, "calendar")
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updated.(Model)
	updated, _ = model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	if strings.Contains(model.View(), "hidden task") || model.loadDetail() != nil {
		t.Fatalf("calendar exposed a task absent from the current month:\n%s", model.View())
	}
}

func TestChangingTaskViewReloadsFullDetail(t *testing.T) {
	task := domain.Task{ID: 1, Title: "task", Subtasks: []domain.Subtask{{ID: 2, Title: "child"}}}
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{task}}
	model := New(backend)
	updated, _ := model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	model.detail = &task

	updated, command := model.Update(key("l"))
	model = updated.(Model)
	if command == nil || model.detail != nil {
		t.Fatalf("view change command=%v detail=%#v", command != nil, model.detail)
	}
	updated, _ = model.Update(command())
	model = updated.(Model)
	if model.detail == nil || len(model.detail.Subtasks) != 1 {
		t.Fatalf("full detail was not reloaded: %#v", model.detail)
	}
}

func TestGlobalTaskCreationOpensOwnTaskForm(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeGlobal, tasks: []domain.Task{{ID: 1, Title: "task"}}}
	model := New(backend)
	updated, _ := model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	updated, command := model.Update(key("n"))
	model = updated.(Model)
	if command == nil || !model.form.open || !model.form.loading {
		t.Fatalf("form=%#v command=%v notice=%q", model.form, command != nil, model.notice)
	}
	updated, _ = model.Update(command())
	model = updated.(Model)
	if !model.form.open || model.form.destination != "Global · origen propio" {
		t.Fatalf("loaded form=%#v", model.form)
	}
}

func TestGlobalOwnedTaskExposesNestedCreation(t *testing.T) {
	origin := domain.TaskOrigin{Kind: domain.OriginGlobal, Key: domain.GlobalOriginKey, Name: "Global"}
	backend := &fakeBackend{mode: domain.ModeGlobal, tasks: []domain.Task{{ID: 1, Title: "task", Origin: origin}}}
	model := NewAt(backend, "table")
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model = updated.(Model)
	updated, _ = model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	view := model.View()
	for _, available := range []string{"a añadir subtarea", "g agregar dependencia", "c recurrencia"} {
		if !strings.Contains(view, available) {
			t.Fatalf("global task footer missing %q:\n%s", available, view)
		}
	}
}

func TestRecurrenceStartsWithGuidedPicker(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{{ID: 1, Title: "task", Version: 1}}}
	model := New(backend)
	updated, _ := model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	updated, _ = model.Update(key("c"))
	model = updated.(Model)
	if !model.pickerOpen || model.pickerAction != "recurrence-kind" || len(model.pickerOptions) < 5 {
		t.Fatalf("recurrence picker=%v action=%q options=%d", model.pickerOpen, model.pickerAction, len(model.pickerOptions))
	}
	model.pickerSelected = 2 // semanal
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.pickerAction != "recurrence-weekly-days" || len(model.pickerOptions) != 7 {
		t.Fatalf("weekly picker action=%q options=%d", model.pickerAction, len(model.pickerOptions))
	}
	updated, _ = model.Update(key(" "))
	model = updated.(Model)
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("weekly day selection did not produce recurrence mutation")
	}
	if message := cmd().(mutated); message.err != nil {
		t.Fatalf("weekly recurrence mutation: %v", message.err)
	}
}

func TestCrowdedKanbanFitsMinimumNinetyByForty(t *testing.T) {
	statuses := []domain.Status{{ID: 1, Name: "Pending", Kind: domain.StatusNormal}, {ID: 2, Name: "Progress", Kind: domain.StatusNormal}, {ID: 3, Name: "Blocked", Kind: domain.StatusNormal}, {ID: 4, Name: "Cancelled", Kind: domain.StatusCancelled}, {ID: 5, Name: "Done", Kind: domain.StatusDone}}
	tasks := make([]domain.Task, 40)
	for index := range tasks {
		tasks[index] = domain.Task{ID: int64(index + 1), Title: "long task title", StatusID: 1, Status: statuses[0]}
	}
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: tasks}
	model := New(backend)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 90, Height: 40})
	model = updated.(Model)
	updated, _ = model.Update(loaded{tasks: tasks, statuses: statuses})
	model = updated.(Model)
	if lines := strings.Count(model.View(), "\n") + 1; lines > 40 {
		t.Fatalf("rendered %d lines in supported 90x40 terminal:\n%s", lines, model.View())
	}
}

func TestReloadPreservesSelectedTaskIdentityAfterResorting(t *testing.T) {
	first := domain.Task{ID: 1, Title: "first", Origin: domain.TaskOrigin{Kind: domain.OriginProject, Key: "/p.tasks", Name: "p"}}
	second := domain.Task{ID: 2, Title: "second", Origin: domain.TaskOrigin{Kind: domain.OriginProject, Key: "/p.tasks", Name: "p"}}
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{first, second}}
	model := New(backend)
	updated, _ := model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	model.selected = 1
	updated, _ = model.Update(mutated{action: "updated"})
	model = updated.(Model)
	updated, _ = model.Update(loaded{tasks: []domain.Task{second, first}})
	model = updated.(Model)
	if model.selected != 0 || model.tasks[model.selected].ID != second.ID {
		t.Fatalf("selected index=%d task=%#v", model.selected, model.tasks[model.selected])
	}
}

func TestCommandPaletteSearchesNamesDescriptionsAndSynonyms(t *testing.T) {
	task := domain.Task{ID: 1, Title: "task", Version: 1}
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{task}}
	model := NewAt(backend, "table")
	updated, _ := model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	updated, _ = model.Update(ctrlP())
	model = updated.(Model)
	if !model.paletteOpen {
		t.Fatal("Ctrl+P did not open the command palette")
	}
	for _, query := range []string{"Editar título", "Modificar el título", "título renombrar", "editar titulo"} {
		model.paletteQuery = query
		entries := model.paletteEntries()
		found := false
		for _, entry := range entries {
			found = found || entry.Action.Name == "Editar título"
		}
		if !found {
			t.Fatalf("query %q did not find Editar título: %#v", query, entries)
		}
	}
}

func TestCommandPaletteNavigationAvailabilityMatchesVisibleViewportAndEdges(t *testing.T) {
	dueOutsideMonth, _ := domain.ParseDate("2027-01-20")
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{{ID: 1, Title: "outside", Due: &dueOutsideMonth}}}
	model := NewAt(backend, "calendar")
	updated, _ := model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	model.paletteQuery = "seleccionar"
	for _, entry := range model.paletteEntries() {
		if (entry.Action.Name == "Seleccionar siguiente" || entry.Action.Name == "Seleccionar anterior") && entry.Enabled {
			t.Fatalf("invisible calendar navigation marked available: %#v", entry)
		}
	}

	statusBackend := &fakeBackend{mode: domain.ModeLocal}
	model = NewAt(statusBackend, "settings")
	model.statuses = []domain.Status{{ID: 1, Name: "Primero", Kind: domain.StatusNormal}, {ID: 2, Name: "Último", Kind: domain.StatusNormal}}
	model.selected = 0
	if enabled, _ := model.paletteAvailability(paletteNormalStatusLeft); enabled {
		t.Fatal("first status can incorrectly move left")
	}
	if enabled, _ := model.paletteAvailability(paletteNormalStatusRight); !enabled {
		t.Fatal("first status should be able to move right")
	}

	detailTask := domain.Task{ID: 1, Title: "parent", Subtasks: []domain.Subtask{{ID: 2, Title: "only"}}}
	model = NewAt(&fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{detailTask}}, "table")
	model.tasks = presenter.Tasks([]domain.Task{detailTask})
	model.detail = &detailTask
	if enabled, _ := model.paletteAvailability(paletteSubtaskNext); enabled {
		t.Fatal("only subtask can incorrectly move next")
	}
	if enabled, _ := model.paletteAvailability(paletteSubtaskPrevious); enabled {
		t.Fatal("only subtask can incorrectly move previous")
	}
}

func TestCommandPaletteSortsAvailableActionsFirstAndExplainsDisabledOnes(t *testing.T) {
	model := NewAt(&fakeBackend{mode: domain.ModeLocal}, "table")
	model.loading = false
	model.width, model.height = 100, 40
	model.paletteOpen = true
	model.paletteQuery = "título"
	entries := model.paletteEntries()
	if len(entries) < 2 || entries[0].Action.Name != "Buscar por título" || !entries[0].Enabled {
		t.Fatalf("available action is not first: %#v", entries)
	}
	foundDisabled := false
	for _, entry := range entries {
		if entry.Action.Name == "Editar título" {
			foundDisabled = !entry.Enabled && strings.Contains(entry.Reason, "selecciona una tarea")
		}
	}
	if !foundDisabled || !strings.Contains(model.View(), "No disponible: selecciona una tarea") {
		t.Fatalf("disabled reason missing; entries=%#v\n%s", entries, model.View())
	}
}

func TestCommandPaletteExecutesTheExistingShortcutPath(t *testing.T) {
	task := domain.Task{ID: 1, Title: "task", Version: 1}
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{task}}
	model := NewAt(backend, "table")
	updated, _ := model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	updated, _ = model.Update(ctrlP())
	model = updated.(Model)
	updated, _ = model.Update(key("avanzar la prioridad"))
	model = updated.(Model)
	entries := model.paletteEntries()
	if len(entries) != 1 || entries[0].Action.Name != "Cambiar prioridad" || !entries[0].Enabled {
		t.Fatalf("priority search entries=%#v", entries)
	}
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.paletteOpen || command == nil {
		t.Fatalf("paletteOpen=%v command=%v", model.paletteOpen, command != nil)
	}
	message, ok := command().(mutated)
	if !ok || message.err != nil || backend.priorityCalls != 1 {
		t.Fatalf("message=%#v priority calls=%d", message, backend.priorityCalls)
	}
}

func TestCancellingCommandPalettePreservesContext(t *testing.T) {
	first := domain.Task{ID: 1, Title: "first"}
	second := domain.Task{ID: 2, Title: "second"}
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{first, second}}
	model := NewAt(backend, "gantt")
	updated, _ := model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	model.selected = 1
	model.selectedSubtask = 3
	model.filter.Query = "original"
	model.ganttStartDay = 15
	month := model.calendarMonth
	updated, _ = model.Update(ctrlP())
	model = updated.(Model)
	updated, _ = model.Update(key("prioridad"))
	model = updated.(Model)
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if command != nil || model.paletteOpen || model.selected != 1 || model.selectedSubtask != 3 || model.filter.Query != "original" || model.ganttStartDay != 15 || !model.calendarMonth.Equal(month) {
		t.Fatalf("context changed on cancel: %#v", model)
	}
}

func TestTransientInteractionsTakePrecedenceOverCommandPalette(t *testing.T) {
	task := domain.Task{ID: 1, Title: "task"}
	base := NewAt(&fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{task}}, "table")
	base.tasks = presenter.Tasks([]domain.Task{task})
	states := map[string]func(*Model){
		"input":        func(m *Model) { m.inputMode = true; m.inputAction = "edit" },
		"picker":       func(m *Model) { m.openPicker("filter-status", presenter.Task{}, []pickerOption{{Label: "Todos"}}) },
		"history":      func(m *Model) { m.historyOpen = true },
		"confirmation": func(m *Model) { selected := m.tasks[0]; m.confirmTrash = &selected },
		"help":         func(m *Model) { m.helpOpen = true },
	}
	for name, prepare := range states {
		t.Run(name, func(t *testing.T) {
			model := base
			prepare(&model)
			updated, _ := model.Update(ctrlP())
			model = updated.(Model)
			if model.paletteOpen {
				t.Fatalf("palette opened over %s", name)
			}
		})
	}
}

func TestAsyncTransientResultsCloseCommandPaletteAndTakePrecedence(t *testing.T) {
	task := domain.Task{ID: 1, Title: "task", Version: 1, Origin: domain.TaskOrigin{Kind: domain.OriginProject, Key: "/p.tasks", Name: "p"}}
	other := domain.Task{ID: 2, Title: "other", Version: 1, Origin: task.Origin}
	for _, test := range []struct {
		name     string
		startKey string
		backend  *fakeBackend
		opened   func(Model) bool
	}{
		{name: "history", startKey: "H", backend: &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{task, other}}, opened: func(m Model) bool { return m.historyOpen }},
		{name: "picker", startKey: "g", backend: &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{task, other}}, opened: func(m Model) bool { return m.pickerOpen }},
		{name: "confirmation", startKey: "d", backend: &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{task, other}, impact: []int64{2}}, opened: func(m Model) bool { return m.confirmTrash != nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := NewAt(test.backend, "table")
			updated, _ := model.Update(loaded{tasks: test.backend.tasks})
			model = updated.(Model)
			updated, pending := model.Update(key(test.startKey))
			model = updated.(Model)
			if pending == nil {
				t.Fatalf("%s did not start an asynchronous interaction", test.startKey)
			}
			updated, _ = model.Update(ctrlP())
			model = updated.(Model)
			if !model.paletteOpen {
				t.Fatal("palette did not open while result was pending")
			}
			updated, _ = model.Update(pending())
			model = updated.(Model)
			if model.paletteOpen || !test.opened(model) {
				t.Fatalf("async modal did not take precedence: palette=%v model=%#v", model.paletteOpen, model)
			}
		})
	}
}

func TestCommandPaletteOpensInEveryReachableViewAndFitsMinimumTerminal(t *testing.T) {
	for _, test := range []struct {
		mode  domain.Mode
		views []int
	}{{domain.ModeLocal, []int{0, 1, 2, 3, 4, 5}}, {domain.ModeGlobal, []int{2, 3, 4}}} {
		for _, view := range test.views {
			backend := &fakeBackend{mode: test.mode}
			model := New(backend)
			model.view = view
			model.loading = false
			updated, _ := model.Update(tea.WindowSizeMsg{Width: 90, Height: 40})
			model = updated.(Model)
			updated, _ = model.Update(ctrlP())
			model = updated.(Model)
			if !model.paletteOpen {
				t.Fatalf("mode=%s view=%d did not open palette", test.mode, view)
			}
			palette := model.paletteView(36)
			if width, height := lipgloss.Width(palette), lipgloss.Height(palette); width > 90 || height > 36 {
				t.Fatalf("mode=%s view=%d palette rendered %dx%d:\n%s", test.mode, view, width, height, palette)
			}
			if lines := strings.Count(model.View(), "\n") + 1; lines > 40 {
				t.Fatalf("mode=%s view=%d full view rendered %d lines:\n%s", test.mode, view, lines, model.View())
			}
		}
	}
}

func TestCommandPaletteTruncatesLongQueryAndDisabledReasonToTerminalWidth(t *testing.T) {
	model := NewAt(&fakeBackend{mode: domain.ModeLocal}, "table")
	model.width = 90
	model.paletteOpen = true
	model.paletteQuery = strings.Repeat("consulta muy larga ", 20)
	palette := model.paletteView(8)
	if width, height := lipgloss.Width(palette), lipgloss.Height(palette); width > 90 || height > 8 {
		t.Fatalf("long palette rendered %dx%d:\n%s", width, height, palette)
	}
	model.paletteQuery = "título"
	palette = model.paletteView(8)
	if width := lipgloss.Width(palette); width > 90 {
		t.Fatalf("disabled reason rendered %d columns:\n%s", width, palette)
	}
}

func TestPanelFocusNavigatesInspectorRowsAndScopesNestedActions(t *testing.T) {
	start, _ := domain.ParseDate("2026-07-15")
	task := domain.Task{
		ID: 1, Title: "parent", Version: 3, Start: &start,
		Origin:        domain.TaskOrigin{Kind: domain.OriginProject, Key: "/p.tasks", Name: "p"},
		Subtasks:      []domain.Subtask{{ID: 8, Title: "child", Status: domain.Status{Name: "Pending"}}},
		DependencyIDs: []int64{42},
	}
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{task}}
	model := NewAt(backend, "table")
	updated, _ := model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	events := []domain.HistoryEvent{{ID: 9, Kind: "created", CreatedAt: time.Now()}}
	updated, _ = model.Update(detailLoaded{task: task, history: events})
	model = updated.(Model)

	updated, command := model.Update(key("t"))
	model = updated.(Model)
	if command != nil || backend.toggleSubCalls != 0 {
		t.Fatal("inactive inspector accepted a subtask action")
	}
	// J/K are the direct path from the task preview into its subtasks: no
	// inspector expansion or field-by-field traversal is required.
	updated, _ = model.Update(key("J"))
	model = updated.(Model)
	if model.panelFocus != focusInspector || model.selected != 0 {
		t.Fatalf("focus=%v selected=%d", model.panelFocus, model.selected)
	}
	row, ok := model.focusedInspectorRow()
	if !ok || row.Kind != taskdetail.RowSubtask || model.selectedSubtask != 0 {
		t.Fatalf("focused row=%#v ok=%v", row, ok)
	}
	updated, command = model.Update(key("t"))
	model = updated.(Model)
	if command == nil {
		t.Fatal("focused subtask action did not run")
	}
	_ = command()
	if backend.toggleSubCalls != 1 {
		t.Fatalf("toggle calls=%d", backend.toggleSubCalls)
	}
	for range 8 {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(Model)
	}
	row, _ = model.focusedInspectorRow()
	if row.Kind != taskdetail.RowDependency || row.ID != 42 {
		t.Fatalf("dependency row=%#v", row)
	}
	updated, command = model.Update(key("G"))
	model = updated.(Model)
	if command == nil {
		t.Fatal("focused dependency action did not run")
	}
	_ = command()
	if backend.removeDepCalls != 1 {
		t.Fatalf("remove dependency calls=%d", backend.removeDepCalls)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if !model.historyOpen {
		t.Fatal("history row did not open the history panel")
	}
}

func TestPanelNavigationIsRememberedPerViewAndPinKeepsLayout(t *testing.T) {
	tasks := []domain.Task{{ID: 1, Title: "one"}, {ID: 2, Title: "two"}}
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: tasks}
	model := NewAt(backend, "table")
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 90, Height: 40})
	model = updated.(Model)
	updated, _ = model.Update(loaded{tasks: tasks})
	model = updated.(Model)
	model.selected = 1
	updated, _ = model.Update(detailLoaded{task: tasks[1]})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(Model)
	for range 3 {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(Model)
	}
	updated, _ = model.Update(key("I"))
	model = updated.(Model)
	if model.inspectorMode != inspectorExpanded {
		t.Fatal("inspector did not expand")
	}

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	model = updated.(Model)
	if command == nil || model.view != 0 || model.selected != 0 || model.panelFocus != focusMain || model.inspectorMode != inspectorNormal {
		t.Fatalf("kanban state view=%d selected=%d focus=%v mode=%v", model.view, model.selected, model.panelFocus, model.inspectorMode)
	}
	updated, _ = model.Update(command())
	model = updated.(Model)
	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyRight})
	model = updated.(Model)
	if command == nil || model.view != 1 || model.selected != 1 || model.panelFocus != focusInspector || model.inspectorCursor != 3 || model.inspectorMode != inspectorExpanded {
		t.Fatalf("restored table state view=%d selected=%d focus=%v cursor=%d mode=%v", model.view, model.selected, model.panelFocus, model.inspectorCursor, model.inspectorMode)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeySpace})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	model = updated.(Model)
	if !model.inspectorPinned || model.inspectorMode != inspectorExpanded || model.panelFocus != focusInspector {
		t.Fatalf("pinned layout was not retained: pinned=%v mode=%v focus=%v", model.inspectorPinned, model.inspectorMode, model.panelFocus)
	}
}

func TestInspectorCanHideExpandAndRenderWithinResizedTerminal(t *testing.T) {
	task := domain.Task{ID: 1, Title: "task", Subtasks: []domain.Subtask{{ID: 2, Title: "child"}}}
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{task}}
	model := NewAt(backend, "table")
	updated, _ := model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	updated, _ = model.Update(detailLoaded{task: task})
	model = updated.(Model)
	for _, size := range []tea.WindowSizeMsg{{Width: 90, Height: 40}, {Width: 130, Height: 50}} {
		updated, _ = model.Update(size)
		model = updated.(Model)
		updated, _ = model.Update(key("I"))
		model = updated.(Model)
		view := model.View()
		if !strings.Contains(view, "EXPANDIDO") || strings.Contains(view, "Vista principal") || lipgloss.Width(view) > size.Width || lipgloss.Height(view) > size.Height {
			t.Fatalf("expanded render %dx%d rendered %dx%d:\n%s", size.Width, size.Height, lipgloss.Width(view), lipgloss.Height(view), view)
		}
		updated, _ = model.Update(key("I"))
		model = updated.(Model)
		if view = model.View(); strings.Contains(view, "Inspector ·") || !strings.Contains(view, "Vista principal · ACTIVA") {
			t.Fatalf("hidden inspector render:\n%s", view)
		}
		updated, _ = model.Update(key("I"))
		model = updated.(Model)
	}
}

func TestExpandedInspectorAlwaysOwnsFocusAndTabRestoresMainPanel(t *testing.T) {
	tasks := []domain.Task{{ID: 1, Title: "one"}, {ID: 2, Title: "two"}}
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: tasks}
	model := NewAt(backend, "table")
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 90, Height: 40})
	model = updated.(Model)
	updated, _ = model.Update(loaded{tasks: tasks})
	model = updated.(Model)
	updated, _ = model.Update(detailLoaded{task: tasks[0]})
	model = updated.(Model)
	updated, _ = model.Update(key("I"))
	model = updated.(Model)
	if model.inspectorMode != inspectorExpanded || model.panelFocus != focusInspector || !strings.Contains(model.View(), "Inspector · ACTIVO · EXPANDIDO") {
		t.Fatalf("expanded focus mode=%v focus=%v\n%s", model.inspectorMode, model.panelFocus, model.View())
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.selected != 0 || model.inspectorCursor != 1 {
		t.Fatalf("expanded down selected=%d cursor=%d", model.selected, model.inspectorCursor)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(Model)
	if model.inspectorMode != inspectorNormal || model.panelFocus != focusMain || !strings.Contains(model.View(), "Vista principal · ACTIVA") {
		t.Fatalf("tab from expanded mode=%v focus=%v\n%s", model.inspectorMode, model.panelFocus, model.View())
	}
}

func TestCalendarAndGanttRememberIndependentTemporalViewports(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeLocal}
	model := NewAt(backend, "calendar")
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	model = updated.(Model)
	calendarMonth := model.calendarMonth
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRight})
	model = updated.(Model)
	if model.view != 3 || model.calendarMonth.Month() != time.July {
		t.Fatalf("new gantt viewport view=%d month=%s", model.view, model.calendarMonth.Month())
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	model = updated.(Model)
	ganttMonth := model.calendarMonth
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	model = updated.(Model)
	if !model.calendarMonth.Equal(calendarMonth) || model.calendarMonth.Equal(ganttMonth) {
		t.Fatalf("calendar=%v want=%v gantt=%v", model.calendarMonth, calendarMonth, ganttMonth)
	}
}

func TestHiddenInspectorDisablesNestedNavigationAndContextHelp(t *testing.T) {
	task := domain.Task{ID: 1, Title: "parent", Subtasks: []domain.Subtask{{ID: 2, Title: "one"}, {ID: 3, Title: "two"}}}
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{task}}
	model := NewAt(backend, "table")
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 90, Height: 40})
	model = updated.(Model)
	updated, _ = model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	updated, _ = model.Update(detailLoaded{task: task})
	model = updated.(Model)
	model.inspectorMode = inspectorHidden
	model.panelFocus = focusMain
	if enabled, reason := model.paletteAvailability(paletteSubtaskNext); enabled || !strings.Contains(reason, "muestra el inspector") {
		t.Fatalf("hidden subtask palette enabled=%v reason=%q", enabled, reason)
	}
	footer := model.footerContent()
	for _, hidden := range []string{"Enter actuar sobre la fila", "Espacio fijar", "Subtarea"} {
		if strings.Contains(footer, hidden) {
			t.Fatalf("hidden inspector footer exposes %q:\n%s", hidden, footer)
		}
	}
}
