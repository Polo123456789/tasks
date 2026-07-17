package app

import (
	"strings"
	"testing"
	"time"

	"github.com/Polo123456789/tasks/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func mustDate(t *testing.T, value string) domain.Date {
	t.Helper()
	date, err := domain.ParseDate(value)
	if err != nil {
		t.Fatal(err)
	}
	return date
}

func TestDatePickerNavigatesLeapDaysWeeksMonthsAndYears(t *testing.T) {
	today := mustDate(t, "2024-02-28")
	model := Model{datePicker: datePickerState{open: true, focused: today, today: today}}
	updated, _ := model.Update(key("right"))
	model = updated.(Model)
	if got := model.datePicker.focused.String(); got != "2024-02-29" {
		t.Fatalf("leap day=%s", got)
	}
	updated, _ = model.Update(key("right"))
	model = updated.(Model)
	if got := model.datePicker.focused.String(); got != "2024-03-01" {
		t.Fatalf("year/month boundary=%s", got)
	}
	updated, _ = model.Update(key("pgdown"))
	model = updated.(Model)
	if got := model.datePicker.focused.String(); got != "2024-03-08" {
		t.Fatalf("week navigation=%s", got)
	}
	jan31 := mustDate(t, "2024-01-31")
	if got := shiftPickerMonth(jan31, 1).String(); got != "2024-02-29" {
		t.Fatalf("month clamp=%s", got)
	}
	dec31 := mustDate(t, "2024-12-31")
	if got := shiftPickerMonth(dec31, 1).String(); got != "2025-01-31" {
		t.Fatalf("year change=%s", got)
	}
}

func TestDatePickerRendersTodaySavedAndFocusWithoutColor(t *testing.T) {
	today := mustDate(t, "2024-02-28")
	saved := mustDate(t, "2024-02-29")
	picker := datePickerState{open: true, target: dateFormDue, focused: saved, current: &saved, today: today}
	view := picker.view(90, 30)
	for _, expected := range []string{"febrero 2024", "> foco", "= guardada/aplicada", "* hoy", ">=29", "*28", "Enter elegir", "[/] mes"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("calendar missing %q:\n%s", expected, view)
		}
	}
	if lines := strings.Count(view, "\n") + 1; lines > 30 {
		t.Fatalf("calendar exceeds viewport with %d lines:\n%s", lines, view)
	}
}

func TestDatePickerWeekdayHeadersUseVisualWidth(t *testing.T) {
	days := []string{"Lun", "Mar", "Mié", "Jue", "Vie", "Sáb", "Dom"}
	var header strings.Builder
	for _, day := range days {
		cell := padVisual(day, 6)
		if got := lipgloss.Width(cell); got != 6 {
			t.Fatalf("visual width for %q=%d, want 6", day, got)
		}
		header.WriteString(cell)
	}
	if got := lipgloss.Width(header.String()); got != 42 {
		t.Fatalf("header visual width=%d, want 42", got)
	}
}

func TestFormCalendarMarksPersistedDateNotDraft(t *testing.T) {
	today := mustDate(t, "2026-07-15")
	saved := mustDate(t, "2026-07-18")
	model := Model{
		today: today,
		form: taskForm{
			open:  true,
			field: formStart,
			task:  domain.Task{Start: &saved},
			text:  map[int]textField{formStart: newTextField("2026-07-20")},
		},
	}
	if !model.openFormDatePicker() {
		t.Fatal("date picker did not open")
	}
	if model.datePicker.current == nil || model.datePicker.current.String() != "2026-07-18" || model.datePicker.focused.String() != "2026-07-20" {
		t.Fatalf("picker current/focus=%#v", model.datePicker)
	}
}

func TestFormCalendarCancelConfirmClearAndPreserveDraft(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeLocal, today: mustDate(t, "2026-07-15")}
	model := loadCreationForm(t, backend)
	model.form.text[formTitle] = newTextField("Borrador intacto")
	model.form.field = formStart
	model.form.text[formStart] = newTextField("2026-07-20")
	updated, _ := model.Update(key("ctrl+o"))
	model = updated.(Model)
	if !model.datePicker.open || model.datePicker.focused.String() != "2026-07-20" {
		t.Fatalf("date picker=%#v", model.datePicker)
	}
	updated, _ = model.Update(key("right"))
	model = updated.(Model)
	updated, _ = model.Update(key("esc"))
	model = updated.(Model)
	if model.datePicker.open || model.form.text[formStart].String() != "2026-07-20" || model.form.text[formTitle].String() != "Borrador intacto" {
		t.Fatalf("cancel changed draft: picker=%#v form=%#v", model.datePicker, model.form)
	}
	updated, _ = model.Update(key("ctrl+o"))
	model = updated.(Model)
	model.datePicker.focused = mustDate(t, "2026-07-22")
	updated, _ = model.Update(key("enter"))
	model = updated.(Model)
	if model.datePicker.open || model.form.text[formStart].String() != "2026-07-22" {
		t.Fatalf("confirmed start picker=%#v value=%q", model.datePicker, model.form.text[formStart].String())
	}
	updated, _ = model.Update(key("ctrl+o"))
	model = updated.(Model)
	updated, _ = model.Update(key("x"))
	model = updated.(Model)
	if model.form.text[formStart].String() != "" {
		t.Fatalf("clear left value=%q", model.form.text[formStart].String())
	}
}

func TestFormCalendarExplainsInvalidRangeBeforeChangingDraft(t *testing.T) {
	backend := &fakeBackend{mode: domain.ModeLocal, today: mustDate(t, "2026-07-15")}
	model := loadCreationForm(t, backend)
	model.form.text[formStart] = newTextField("2026-07-20")
	model.form.text[formDue] = newTextField("2026-07-25")
	model.form.field = formDue
	updated, _ := model.Update(key("ctrl+o"))
	model = updated.(Model)
	model.datePicker.focused = mustDate(t, "2026-07-19")
	updated, _ = model.Update(key("enter"))
	model = updated.(Model)
	if !model.datePicker.open || model.datePicker.error == "" || model.form.text[formDue].String() != "2026-07-25" {
		t.Fatalf("invalid range picker=%#v due=%q", model.datePicker, model.form.text[formDue].String())
	}
	if !strings.Contains(model.View(), "no puede ser anterior") {
		t.Fatalf("range error is not visible:\n%s", model.View())
	}
	model.datePicker.focused = mustDate(t, "2026-07-21")
	updated, _ = model.Update(key("enter"))
	model = updated.(Model)
	if model.datePicker.open || model.form.text[formDue].String() != "2026-07-21" {
		t.Fatalf("valid due was not accepted: %#v", model.form)
	}
}

func TestFilterCalendarBuildsRangeDraftBeforeApplyingFilter(t *testing.T) {
	today := mustDate(t, "2026-07-15")
	backend := &fakeBackend{mode: domain.ModeLocal, today: today}
	model := NewAt(backend, "table")
	model.inputMode, model.inputAction, model.input = true, "filter-dates", "2026-07-01..2026-07-31"
	updated, _ := model.Update(key("ctrl+o"))
	model = updated.(Model)
	if !model.datePicker.open || model.datePicker.target != dateFilterFrom {
		t.Fatalf("filter picker=%#v", model.datePicker)
	}
	model.datePicker.focused = mustDate(t, "2026-07-10")
	updated, _ = model.Update(key("enter"))
	model = updated.(Model)
	if model.datePicker.target != dateFilterTo || model.filter.From != nil || model.filter.To != nil {
		t.Fatalf("first endpoint mutated active filter: picker=%#v filter=%#v", model.datePicker, model.filter)
	}
	model.datePicker.focused = mustDate(t, "2026-07-09")
	updated, _ = model.Update(key("enter"))
	model = updated.(Model)
	if !model.datePicker.open || model.datePicker.error == "" {
		t.Fatalf("invalid filter range accepted: %#v", model.datePicker)
	}
	model.datePicker.focused = mustDate(t, "2026-07-20")
	updated, _ = model.Update(key("enter"))
	model = updated.(Model)
	if model.datePicker.open || model.input != "2026-07-10..2026-07-20" || model.filter.From != nil || model.filter.To != nil {
		t.Fatalf("range draft=%q filter=%#v picker=%#v", model.input, model.filter, model.datePicker)
	}
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if command == nil || model.filter.From == nil || model.filter.To == nil || model.filter.From.String() != "2026-07-10" || model.filter.To.String() != "2026-07-20" {
		t.Fatalf("filter was not applied atomically: %#v command=%v", model.filter, command != nil)
	}
}

func TestDatePickerUsesDomainDateAtUTCMidnight(t *testing.T) {
	date := mustDate(t, "2026-12-31")
	if date.Time().Location() != time.UTC || date.Time().Hour() != 0 || date.Time().Minute() != 0 {
		t.Fatalf("date introduced time or timezone: %v", date.Time())
	}
}

func TestOpenDatePickerRefreshesTodayWithoutMovingFocus(t *testing.T) {
	oldToday := mustDate(t, "2026-07-15")
	newToday := mustDate(t, "2026-07-16")
	focus := mustDate(t, "2026-08-01")
	backend := &fakeBackend{mode: domain.ModeLocal, today: newToday}
	model := NewAt(backend, "table")
	model.today = oldToday
	model.datePicker = datePickerState{open: true, focused: focus, today: oldToday}
	updated, _ := model.Update(maintenanceFinished{today: newToday})
	model = updated.(Model)
	if !model.datePicker.today.Equal(newToday) {
		t.Fatalf("picker today=%s, want %s", model.datePicker.today, newToday)
	}
	if !model.datePicker.focused.Equal(focus) {
		t.Fatalf("picker focus moved to %s", model.datePicker.focused)
	}
}
