package app

import (
	"strings"
	"testing"

	"github.com/Polo123456789/tasks/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
)

func pasted(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value), Paste: true}
}

func TestTextFieldSupportsCursorWordsPasteAndLineDeletion(t *testing.T) {
	field := newTextField("uno dos")
	field.update(key("ctrl+left"))
	if field.cursor != 4 {
		t.Fatalf("word-left cursor=%d", field.cursor)
	}
	field.update(key("ctrl+w"))
	if got := field.String(); got != "dos" {
		t.Fatalf("word deletion=%q", got)
	}
	field.update(key("end"))
	field.update(pasted(" cuatro\ncinco"))
	if got := field.String(); got != "dos cuatro cinco" {
		t.Fatalf("paste=%q", got)
	}
	field.update(key("home"))
	field.update(key("ctrl+k"))
	if got := field.String(); got != "" || field.cursor != 0 {
		t.Fatalf("line deletion=%q cursor=%d", got, field.cursor)
	}
	field.update(pasted("antes después"))
	field.update(key("ctrl+left"))
	field.update(key("ctrl+u"))
	if got := field.String(); got != "después" || field.cursor != 0 {
		t.Fatalf("delete to start=%q cursor=%d", got, field.cursor)
	}
}

func loadCreationForm(t *testing.T, backend *fakeBackend) Model {
	t.Helper()
	model := NewAt(backend, "table")
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model = updated.(Model)
	updated, command := model.Update(key("n"))
	model = updated.(Model)
	if command == nil {
		t.Fatal("creation form did not load asynchronously")
	}
	updated, _ = model.Update(command())
	return updated.(Model)
}

func TestUnifiedCreationFormSavesEveryFieldOnce(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeLocal, workflowStatuses: []domain.Status{
		{ID: 1, Name: "Pendiente", Kind: domain.StatusNormal, Initial: true},
		{ID: 2, Name: "En curso", Kind: domain.StatusNormal},
	}}
	model := loadCreationForm(t, backend)
	if model.form.field != formTitle || model.form.destination != "Local · prueba" {
		t.Fatalf("initial form=%#v", model.form)
	}
	keys := []tea.KeyMsg{
		pasted("Preparar entrega"), key("tab"), key("right"), key("tab"), key("right"), key("right"),
		key("tab"), pasted("2026-07-20"), key("tab"), pasted("2026-07-25"),
	}
	for _, message := range keys {
		updated, _ := model.Update(message)
		model = updated.(Model)
	}
	updated, save := model.Update(key("enter"))
	model = updated.(Model)
	if save == nil || !model.form.saving || backend.saveCalls != 0 {
		t.Fatalf("saving=%v command=%v calls=%d", model.form.saving, save != nil, backend.saveCalls)
	}
	updated, reload := model.Update(save())
	model = updated.(Model)
	if backend.saveCalls != 1 || reload == nil || model.form.open {
		t.Fatalf("save calls=%d reload=%v form=%#v", backend.saveCalls, reload != nil, model.form)
	}
	saved := backend.savedTask
	if saved.Title != "Preparar entrega" || saved.StatusID != 2 || saved.Priority != domain.PriorityMedium || saved.Start.String() != "2026-07-20" || saved.Due.String() != "2026-07-25" {
		t.Fatalf("saved task=%#v", saved)
	}
}

func TestUnifiedFormRejectsDateAndRecurrenceConflictBeforeMutation(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeLocal}
	model := loadCreationForm(t, backend)
	model.form.text[formTitle] = newTextField("Repetir")
	model.form.text[formStart] = newTextField("2026-07-20")
	model.form.text[formRecurrence] = newTextField("daily")
	updated, command := model.Update(key("enter"))
	model = updated.(Model)
	if command != nil || backend.saveCalls != 0 || model.form.errors["recurrence"] == "" {
		t.Fatalf("command=%v calls=%d errors=%#v", command != nil, backend.saveCalls, model.form.errors)
	}
	if !strings.Contains(model.View(), "⚠") || !strings.Contains(model.View(), "no admite fechas") {
		t.Fatalf("field-local validation not rendered:\n%s", model.View())
	}
}

func TestUnifiedFormPreservesDraftAfterRecoverableSaveError(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeLocal, saveErr: domain.ErrConflict}
	model := loadCreationForm(t, backend)
	model.form.text[formTitle] = newTextField("Borrador local")
	updated, command := model.Update(key("enter"))
	model = updated.(Model)
	if command == nil {
		t.Fatal("save command missing")
	}
	updated, _ = model.Update(command())
	model = updated.(Model)
	if !model.form.open || model.form.text[formTitle].String() != "Borrador local" || model.form.errors["form"] == "" {
		t.Fatalf("draft was not preserved: %#v", model.form)
	}
}

func TestUnifiedFormOnlyConfirmsDiscardWhenDirty(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeLocal}
	model := loadCreationForm(t, backend)
	updated, _ := model.Update(key("esc"))
	model = updated.(Model)
	if model.form.open {
		t.Fatal("pristine form asked for confirmation")
	}
	model = loadCreationForm(t, backend)
	updated, _ = model.Update(pasted("cambio"))
	model = updated.(Model)
	updated, _ = model.Update(key("esc"))
	model = updated.(Model)
	if !model.form.open || !model.form.confirmDiscard {
		t.Fatalf("dirty form did not ask for confirmation: %#v", model.form)
	}
	updated, _ = model.Update(key("n"))
	model = updated.(Model)
	if !model.form.open || model.form.confirmDiscard || model.form.text[formTitle].String() != "cambio" {
		t.Fatalf("cancel confirmation lost draft: %#v", model.form)
	}
	updated, _ = model.Update(key("esc"))
	model = updated.(Model)
	updated, _ = model.Update(key("enter"))
	model = updated.(Model)
	if model.form.open {
		t.Fatal("confirmed discard did not close form")
	}
}

func TestUnifiedEditFormLoadsExactTaskAndKeepsSelection(t *testing.T) {
	start, _ := domain.ParseDate("2026-07-20")
	recurrence := domain.Recurrence{Kind: domain.Daily}
	task := domain.Task{ID: 7, Title: "Editar", StatusID: 2, Priority: domain.PriorityHigh, Start: &start, Version: 3,
		Origin: domain.TaskOrigin{Key: "/project/tasks.sqlite", Name: "Proyecto"}}
	backend := &fakeBackend{mode: domain.ModeGlobal, tasks: []domain.Task{task}, workflowStatuses: []domain.Status{
		{ID: 1, Name: "Pendiente", Initial: true}, {ID: 2, Name: "En curso"},
	}}
	model := NewAt(backend, "table")
	updated, _ := model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	updated, command := model.Update(key("e"))
	model = updated.(Model)
	updated, _ = model.Update(command())
	model = updated.(Model)
	if model.selected != 0 || model.form.text[formTitle].String() != "Editar" || model.form.selectedStatusID() != 2 || model.form.priority != domain.PriorityHigh || model.form.destination != "Proyecto" {
		t.Fatalf("edit form=%#v selected=%d", model.form, model.selected)
	}
	// An incompatible recurrence must still be rejected without mutating the
	// task loaded from the backend.
	model.form.text[formRecurrence] = newTextField(recurrence.Text())
	updated, save := model.Update(key("enter"))
	model = updated.(Model)
	if save != nil || model.form.errors["recurrence"] == "" {
		t.Fatalf("incompatible edit save=%v errors=%#v", save != nil, model.form.errors)
	}
}

func TestUnifiedFormPreservesSelectionViewportAndFitsMinimumTerminal(t *testing.T) {
	due, _ := domain.ParseDate("2026-07-22")
	task := domain.Task{ID: 8, Title: "Visible", StatusID: 1, Due: &due, Origin: domain.TaskOrigin{Key: "project.tasks", Name: "Proyecto"}}
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{task}}
	model := NewAt(backend, "calendar")
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 90, Height: 40})
	model = updated.(Model)
	updated, _ = model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	month, startDay, selected := model.calendarMonth, model.ganttStartDay, model.selected
	updated, command := model.Update(key("e"))
	model = updated.(Model)
	updated, _ = model.Update(command())
	model = updated.(Model)
	view := model.View()
	if lines := strings.Count(view, "\n") + 1; lines > 40 {
		t.Fatalf("form exceeds 90x40 with %d lines:\n%s", lines, view)
	}
	for _, expected := range []string{"Título:", "Estado:", "Prioridad:", "Inicio:", "Vencimiento:", "Recurrencia:", "Destino/origen: Proyecto"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("form missing %q:\n%s", expected, view)
		}
	}
	updated, _ = model.Update(key("esc"))
	model = updated.(Model)
	if model.selected != selected || !model.calendarMonth.Equal(month) || model.ganttStartDay != startDay {
		t.Fatalf("navigation changed: selected=%d month=%v start=%d", model.selected, model.calendarMonth, model.ganttStartDay)
	}
}

func TestCompactCaptureReusesEditorConfirmationAndErrorRecovery(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeGlobal}
	model := New(backend)
	updated, command := model.Update(key("N"))
	model = updated.(Model)
	if command != nil || !model.form.open || !model.form.compact || model.form.destination != "Global · origen propio" {
		t.Fatalf("compact capture=%#v command=%v", model.form, command != nil)
	}
	for _, message := range []tea.KeyMsg{pasted("Uno dos"), key("ctrl+left"), key("ctrl+w"), pasted("Plan ")} {
		updated, _ = model.Update(message)
		model = updated.(Model)
	}
	if model.form.text[formTitle].String() != "Plan dos" {
		t.Fatalf("compact editor=%q", model.form.text[formTitle].String())
	}
	updated, _ = model.Update(key("esc"))
	model = updated.(Model)
	if !model.form.confirmDiscard {
		t.Fatal("dirty compact capture did not confirm discard")
	}
	updated, _ = model.Update(key("n"))
	model = updated.(Model)
	updated, command = model.Update(key("enter"))
	model = updated.(Model)
	updated, _ = model.Update(command())
	model = updated.(Model)
	if model.form.open || backend.savedTask.Title != "Plan dos" || model.preserveSelectionSource != domain.GlobalOriginKey {
		t.Fatalf("compact save form=%#v task=%#v source=%q", model.form, backend.savedTask, model.preserveSelectionSource)
	}

	backend.saveErr = domain.ErrConflict
	updated, _ = model.Update(key("N"))
	model = updated.(Model)
	updated, _ = model.Update(pasted("Conservar este borrador"))
	model = updated.(Model)
	updated, command = model.Update(key("enter"))
	model = updated.(Model)
	updated, _ = model.Update(command())
	model = updated.(Model)
	if !model.form.open || model.form.text[formTitle].String() != "Conservar este borrador" {
		t.Fatalf("compact draft was lost after error: %#v", model.form)
	}
}

func TestEditDetailLoadFailureCanNeverCreateTask(t *testing.T) {
	task := domain.Task{ID: 7, Title: "Original", StatusID: 1, Origin: domain.TaskOrigin{Key: "project.tasks", Name: "Proyecto"}}
	backend := &fakeBackend{mode: domain.ModeGlobal, tasks: []domain.Task{task}, detailErr: domain.ErrNotFound}
	model := NewAt(backend, "table")
	updated, _ := model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	updated, command := model.Update(key("e"))
	model = updated.(Model)
	updated, _ = model.Update(command())
	model = updated.(Model)
	if !model.form.open || !model.form.loadFailed || model.form.task.ID != 0 {
		t.Fatalf("failed form=%#v", model.form)
	}
	updated, save := model.Update(key("enter"))
	model = updated.(Model)
	if save != nil || backend.saveCalls != 0 || backend.createCalls != 0 {
		t.Fatalf("unsafe save=%v saveCalls=%d createCalls=%d", save != nil, backend.saveCalls, backend.createCalls)
	}
}

func TestStaleFormLoadCannotReplaceNewerForm(t *testing.T) {
	task := domain.Task{ID: 7, Title: "Editar", StatusID: 1, Origin: domain.TaskOrigin{Key: "project.tasks", Name: "Proyecto"}}
	backend := &fakeBackend{mode: domain.ModeLocal, tasks: []domain.Task{task}}
	model := NewAt(backend, "table")
	updated, _ := model.Update(loaded{tasks: backend.tasks})
	model = updated.(Model)
	updated, creationLoad := model.Update(key("n"))
	model = updated.(Model)
	firstRequest := model.form.requestID
	updated, _ = model.Update(key("esc"))
	model = updated.(Model)
	updated, editLoad := model.Update(key("e"))
	model = updated.(Model)
	secondRequest := model.form.requestID
	if firstRequest == secondRequest {
		t.Fatal("form request IDs were reused")
	}
	updated, _ = model.Update(creationLoad())
	model = updated.(Model)
	if !model.form.loading || model.form.requestID != secondRequest || !model.form.editing {
		t.Fatalf("stale response replaced form: %#v", model.form)
	}
	updated, _ = model.Update(editLoad())
	model = updated.(Model)
	if model.form.loading || !model.form.editing || model.form.text[formTitle].String() != "Editar" {
		t.Fatalf("current response did not load edit: %#v", model.form)
	}
}
