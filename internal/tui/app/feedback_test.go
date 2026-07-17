package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/tui/presenter"
	tea "github.com/charmbracelet/bubbletea"
)

func loadedModel(t *testing.T, backend *fakeBackend, task domain.Task) Model {
	t.Helper()
	backend.tasks = []domain.Task{task}
	model := NewAt(backend, "table")
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model = updated.(Model)
	updated, _ = model.Update(loaded{tasks: backend.tasks})
	return updated.(Model)
}

func TestReloadKeepsCurrentContentAndShowsActivity(t *testing.T) {
	task := domain.Task{ID: 1, Title: "Contexto visible", StatusID: 1, Version: 2}
	backend := &fakeBackend{mode: domain.ModeLocal}
	model := loadedModel(t, backend, task)
	updated, command := model.Update(key("r"))
	model = updated.(Model)
	if command == nil || !model.loading || !model.loadedOnce {
		t.Fatalf("reload state loading=%v loadedOnce=%v command=%v", model.loading, model.loadedOnce, command != nil)
	}
	view := model.View()
	if !strings.Contains(view, "Contexto visible") || !strings.Contains(view, "⟳ Actualizando…") || strings.Contains(view, "\nCargando…\n") {
		t.Fatalf("reload replaced context:\n%s", view)
	}

	updated, _ = model.Update(loaded{err: errors.New("storage unavailable")})
	model = updated.(Model)
	view = model.View()
	if !strings.Contains(view, "Contexto visible") || !strings.Contains(view, "⚠ ADVERTENCIA") || !strings.Contains(view, "storage unavailable") {
		t.Fatalf("failed reload lost context or feedback:\n%s", view)
	}
}

func TestSuccessfulMutationAndFailedRefreshReportBothOutcomes(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeLocal}
	model := loadedModel(t, backend, domain.Task{ID: 1, Title: "task", Version: 1})
	updated, _ := model.Update(mutated{action: "Tarea finalizada"})
	model = updated.(Model)
	updated, _ = model.Update(loaded{err: errors.New("offline")})
	model = updated.(Model)
	view := model.View()
	for _, expected := range []string{"✓ ÉXITO", "Tarea finalizada", "⚠ ADVERTENCIA", "no se pudo actualizar la vista", "offline"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("combined outcome missing %q:\n%s", expected, view)
		}
	}
}

func TestFeedbackUsesTextSymbolsAndClearsOnNextInteraction(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeLocal}
	model := loadedModel(t, backend, domain.Task{ID: 1, Title: "task"})
	model.notice = "Guardado"
	if view := model.View(); !strings.Contains(view, "✓ ÉXITO") {
		t.Fatalf("success feedback lacks semantic text/symbol:\n%s", view)
	}
	model.notice = "No se puede completar ahora"
	if view := model.View(); !strings.Contains(view, "⚠ ADVERTENCIA") {
		t.Fatalf("warning feedback lacks semantic text/symbol:\n%s", view)
	}
	model.err = errors.New("falló")
	if view := model.View(); !strings.Contains(view, "✗ ERROR") {
		t.Fatalf("error feedback lacks semantic text/symbol:\n%s", view)
	}
	updated, _ := model.Update(key("x"))
	model = updated.(Model)
	if model.notice != "" || model.err != nil {
		t.Fatalf("transient feedback survived interaction: notice=%q err=%v", model.notice, model.err)
	}
}

func TestStatusChangeCanBeUndoneWithOptimisticVersion(t *testing.T) {
	task := domain.Task{ID: 7, Title: "Mover", StatusID: 3, Version: 4, Origin: domain.TaskOrigin{Key: "project.tasks", Name: "Proyecto"}}
	backend := &fakeBackend{mode: domain.ModeLocal}
	model := loadedModel(t, backend, task)
	updated, mutation := model.Update(key("]"))
	model = updated.(Model)
	if mutation == nil {
		t.Fatal("status mutation command missing")
	}
	updated, _ = model.Update(mutation())
	model = updated.(Model)
	if model.undo == nil || model.undo.version != 5 || model.undo.previousStatusID != 3 {
		t.Fatalf("undo snapshot=%#v", model.undo)
	}
	if !strings.Contains(model.View(), "U revertir") {
		t.Fatalf("undo is not discoverable:\n%s", model.View())
	}
	updated, undo := model.Update(key("U"))
	model = updated.(Model)
	if undo == nil {
		t.Fatal("undo command missing")
	}
	updated, reload := model.Update(undo())
	model = updated.(Model)
	if backend.setStatusCalls != 1 || backend.lastSetStatus != 3 || backend.lastSetVersion != 5 || model.undo != nil || reload == nil {
		t.Fatalf("set calls=%d status=%d version=%d undo=%#v reload=%v", backend.setStatusCalls, backend.lastSetStatus, backend.lastSetVersion, model.undo, reload != nil)
	}
}

func TestStatusBoundaryNoopDoesNotCreateUndoOrFalseSuccess(t *testing.T) {
	task := domain.Task{ID: 7, Title: "Límite", StatusID: 3, Version: 4, Origin: domain.TaskOrigin{Key: "project.tasks"}}
	backend := &fakeBackend{mode: domain.ModeLocal, moveNoop: true}
	model := loadedModel(t, backend, task)
	updated, command := model.Update(key("]"))
	model = updated.(Model)
	updated, _ = model.Update(command())
	model = updated.(Model)
	if model.undo != nil || model.noticeKind != feedbackWarning || !strings.Contains(model.notice, "límite") {
		t.Fatalf("no-op undo=%#v kind=%v notice=%q", model.undo, model.noticeKind, model.notice)
	}
}

func TestLifecycleChangesAllCaptureUndoSnapshot(t *testing.T) {
	for _, shortcut := range []string{"f", "C", "z"} {
		t.Run(shortcut, func(t *testing.T) {
			task := domain.Task{ID: 7, Title: "Ciclo", StatusID: 3, Version: 4, Origin: domain.TaskOrigin{Key: "project.tasks", Name: "Proyecto"}}
			backend := &fakeBackend{mode: domain.ModeLocal}
			model := loadedModel(t, backend, task)
			updated, command := model.Update(key(shortcut))
			model = updated.(Model)
			if command == nil {
				t.Fatal("lifecycle command missing")
			}
			updated, _ = model.Update(command())
			model = updated.(Model)
			if model.undo == nil || model.undo.kind != undoStatus || model.undo.previousStatusID != task.StatusID || model.undo.version != task.Version+1 {
				t.Fatalf("shortcut %s undo=%#v", shortcut, model.undo)
			}
		})
	}
}

func TestUndoConflictNeverOverwritesExternalChange(t *testing.T) {
	task := domain.Task{ID: 7, Title: "Mover", StatusID: 3, Version: 4, Origin: domain.TaskOrigin{Key: "project.tasks", Name: "Proyecto"}}
	backend := &fakeBackend{mode: domain.ModeLocal, setStatusErr: domain.ErrConflict}
	model := loadedModel(t, backend, task)
	model.undo = &undoAction{kind: undoStatus, source: "project.tasks", title: task.Title, id: task.ID, version: 5, previousStatusID: 3}
	updated, command := model.Update(key("U"))
	model = updated.(Model)
	updated, reload := model.Update(command())
	model = updated.(Model)
	if reload != nil || model.undo == nil || model.conflict == nil || !errors.Is(model.err, domain.ErrConflict) || backend.lastSetVersion != 5 {
		t.Fatalf("conflicting undo reload=%v undo=%#v conflict=%#v err=%v version=%d", reload != nil, model.undo, model.conflict, model.err, backend.lastSetVersion)
	}
	view := model.View()
	for _, expected := range []string{"✗ ERROR", "r recargar", "v revisar cambio"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("conflict feedback missing %q:\n%s", expected, view)
		}
	}
	updated, review := model.Update(key("v"))
	model = updated.(Model)
	if review == nil {
		t.Fatal("conflict review command missing")
	}
	updated, _ = model.Update(review())
	model = updated.(Model)
	if model.conflict != nil || model.detail == nil || model.inspectorMode != inspectorExpanded || model.panelFocus != focusInspector {
		t.Fatalf("remote review state conflict=%#v detail=%#v mode=%v focus=%v", model.conflict, model.detail, model.inspectorMode, model.panelFocus)
	}
}

func TestRepeatedUndoIsDisabledWhileFirstRequestIsPending(t *testing.T) {
	task := domain.Task{ID: 7, Title: "Mover", StatusID: 3, Version: 4, Origin: domain.TaskOrigin{Key: "project.tasks"}}
	backend := &fakeBackend{mode: domain.ModeLocal}
	model := loadedModel(t, backend, task)
	model.undo = &undoAction{kind: undoStatus, source: "project.tasks", id: task.ID, version: 5, previousStatusID: 3}
	updated, first := model.Update(key("U"))
	model = updated.(Model)
	updated, second := model.Update(key("U"))
	model = updated.(Model)
	if first == nil || second != nil || !model.undoPending {
		t.Fatalf("first=%v second=%v pending=%v", first != nil, second != nil, model.undoPending)
	}
	if enabled, _ := model.paletteAvailability(paletteUndo); enabled {
		t.Fatal("palette enabled undo while request was pending")
	}
}

func TestTrashCanBeUndoneUsingPostTrashVersion(t *testing.T) {
	task := domain.Task{ID: 9, Title: "Papelera", StatusID: 1, Version: 6, Origin: domain.TaskOrigin{Key: "project.tasks", Name: "Proyecto"}}
	backend := &fakeBackend{mode: domain.ModeLocal}
	model := loadedModel(t, backend, task)
	message := model.trash(presenter.Tasks([]domain.Task{task})[0])()
	updated, _ := model.Update(message)
	model = updated.(Model)
	if model.undo == nil || model.undo.kind != undoTrash || model.undo.version != 7 {
		t.Fatalf("trash undo=%#v", model.undo)
	}
	updated, command := model.Update(key("U"))
	model = updated.(Model)
	updated, _ = model.Update(command())
	model = updated.(Model)
	if backend.restoreCalls != 1 || backend.lastRestoreVersion != 7 || model.undo != nil {
		t.Fatalf("restore calls=%d version=%d undo=%#v", backend.restoreCalls, backend.lastRestoreVersion, model.undo)
	}
}

func TestUndoWhileViewingTrashReloadsTrashList(t *testing.T) {
	deletedAt, _ := domain.ParseDate("2026-07-15")
	task := domain.Task{ID: 9, Title: "Papelera", Version: 7, DeletedAt: &deletedAt, Origin: domain.TaskOrigin{Key: "project.tasks"}}
	backend := &fakeBackend{mode: domain.ModeLocal, deleted: []domain.Task{task}}
	model := NewAt(backend, "trash")
	updated, _ := model.Update(loaded{tasks: backend.deleted, trash: true})
	model = updated.(Model)
	model.undo = &undoAction{kind: undoTrash, source: task.Origin.Key, id: task.ID, version: task.Version}
	updated, command := model.Update(key("U"))
	model = updated.(Model)
	updated, reload := model.Update(command())
	model = updated.(Model)
	if reload == nil {
		t.Fatal("undo did not request trash reload")
	}
	message, ok := reload().(loaded)
	if !ok || !message.trash || !backend.lastFilter.IncludeDeleted {
		t.Fatalf("reload message=%#v filter=%#v", message, backend.lastFilter)
	}
}

func TestManualRestoreConflictOffersReloadAndReviewInTrash(t *testing.T) {
	deletedAt, _ := domain.ParseDate("2026-07-15")
	task := domain.Task{ID: 9, Title: "Papelera", StatusID: 1, Version: 7, DeletedAt: &deletedAt, Origin: domain.TaskOrigin{Key: "project.tasks", Name: "Proyecto"}}
	backend := &fakeBackend{mode: domain.ModeLocal, deleted: []domain.Task{task}, restoreErr: domain.ErrConflict}
	model := NewAt(backend, "trash")
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model = updated.(Model)
	updated, _ = model.Update(loaded{tasks: backend.deleted, trash: true})
	model = updated.(Model)
	updated, command := model.Update(key("u"))
	model = updated.(Model)
	updated, _ = model.Update(command())
	model = updated.(Model)
	if model.conflict == nil || model.conflict.id != task.ID || model.conflict.source != task.Origin.Key {
		t.Fatalf("restore conflict=%#v", model.conflict)
	}
	for _, expected := range []string{"r recargar", "v revisar cambio"} {
		if !strings.Contains(model.View(), expected) {
			t.Fatalf("restore conflict missing %q:\n%s", expected, model.View())
		}
	}
	updated, review := model.Update(key("v"))
	model = updated.(Model)
	updated, _ = model.Update(review())
	model = updated.(Model)
	if !strings.Contains(model.notice, "Versión remota #9") {
		t.Fatalf("deleted task review notice=%q", model.notice)
	}
}

func TestLateConflictReviewIsIgnoredAfterConflictCloses(t *testing.T) {
	task := domain.Task{ID: 7, Title: "Remota", Version: 3, Origin: domain.TaskOrigin{Key: "project.tasks"}}
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{task}}
	model := loadedModel(t, backend, task)
	model.setConflict(task.Origin.Key, task.ID, task.Title)
	updated, review := model.Update(key("v"))
	model = updated.(Model)
	updated, _ = model.Update(key("esc"))
	model = updated.(Model)
	updated, _ = model.Update(review())
	model = updated.(Model)
	if model.detail != nil || model.notice != "" || model.conflict != nil {
		t.Fatalf("late review changed state: detail=%#v notice=%q conflict=%#v", model.detail, model.notice, model.conflict)
	}
}

func TestFormConflictOffersReloadReviewAndKeepDraft(t *testing.T) {
	task := domain.Task{ID: 4, Title: "Original", StatusID: 1, Version: 2, Origin: domain.TaskOrigin{Key: "project.tasks", Name: "Proyecto"}}
	backend := &fakeBackend{mode: domain.ModeLocal, saveErr: domain.ErrConflict}
	model := loadedModel(t, backend, task)
	updated, loadForm := model.Update(key("e"))
	model = updated.(Model)
	updated, _ = model.Update(loadForm())
	model = updated.(Model)
	model.form.text[formTitle] = newTextField("Borrador local")
	updated, save := model.Update(key("enter"))
	model = updated.(Model)
	updated, _ = model.Update(save())
	model = updated.(Model)
	if !model.form.conflict || model.form.text[formTitle].String() != "Borrador local" {
		t.Fatalf("conflict lost draft: %#v", model.form)
	}
	for _, expected := range []string{"r recargar remoto", "v revisar cambio", "k conservar borrador local"} {
		if !strings.Contains(model.View(), expected) {
			t.Fatalf("conflict option missing %q:\n%s", expected, model.View())
		}
	}
	backend.tasks[0].Title, backend.tasks[0].Version = "Versión remota", 3
	updated, _ = model.Update(key("r"))
	model = updated.(Model)
	if !model.form.confirmRemoteReload || model.form.text[formTitle].String() != "Borrador local" {
		t.Fatalf("reload did not request discard confirmation: %#v", model.form)
	}
	updated, _ = model.Update(key("n"))
	model = updated.(Model)
	updated, review := model.Update(key("v"))
	model = updated.(Model)
	updated, _ = model.Update(review())
	model = updated.(Model)
	if model.form.remote == nil || model.form.remote.Title != "Versión remota" || model.form.text[formTitle].String() != "Borrador local" || !strings.Contains(model.View(), "REMOTO v3") {
		t.Fatalf("review did not preserve both versions: %#v", model.form)
	}
	updated, _ = model.Update(key("k"))
	model = updated.(Model)
	if model.form.conflict || model.form.task.Version != 3 || model.form.text[formTitle].String() != "Borrador local" {
		t.Fatalf("keep-local option lost draft: %#v", model.form)
	}
	backend.saveErr = nil
	updated, save = model.Update(key("enter"))
	model = updated.(Model)
	updated, reload := model.Update(save())
	model = updated.(Model)
	if reload == nil || model.form.open || backend.savedTask.Version != 3 {
		t.Fatalf("rebased save form=%#v saved=%#v command=%v", model.form, backend.savedTask, reload != nil)
	}
}
