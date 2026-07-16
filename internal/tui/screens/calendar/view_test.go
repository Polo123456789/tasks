package calendar

import (
	"strings"
	"testing"
	"time"

	"github.com/Polo123456789/tasks/internal/tui/presenter"
)

func TestViewRendersMonthEventsAndExcludesRecurring(t *testing.T) {
	tasks := []presenter.Task{
		{Title: "Interval", Start: "2026-07-02", Due: "2026-07-05", Origin: "alpha"},
		{Title: "Milestone", Due: "2026-07-15"},
		{Title: "Recurring", Due: "2026-07-16", Recurring: true},
		{Title: "Other month", Due: "2026-08-01"},
	}
	view := View(tasks, time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC), 0, 80, 20)
	for _, expected := range []string{"julio 2026", "02 · Interval [alpha]", "05 · Interval [alpha]", "15 · Milestone", "•1"} {
		if !strings.Contains(view, expected) {
			t.Errorf("missing %q:\n%s", expected, view)
		}
	}
	for _, excluded := range []string{"Recurring", "Other month"} {
		if strings.Contains(view, excluded) {
			t.Errorf("unexpected %q:\n%s", excluded, view)
		}
	}
}

func TestViewDegradesAtSmallWidth(t *testing.T) {
	view := View(nil, time.Date(2024, time.February, 1, 0, 0, 0, 0, time.UTC), 0, 20, 20)
	if !strings.Contains(view, "29") || !strings.Contains(view, "No hay tareas") {
		t.Fatalf("small calendar:\n%s", view)
	}
}

func TestViewDisambiguatesSameNamedProjects(t *testing.T) {
	tasks := []presenter.Task{
		{Title: "One", Due: "2026-07-01", Origin: "same", Source: "/a/same.tasks"},
		{Title: "Two", Due: "2026-07-02", Origin: "same", Source: "/b/same.tasks"},
	}
	view := View(tasks, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), 0, 100, 20)
	if !strings.Contains(view, "/a/same.tasks") || !strings.Contains(view, "/b/same.tasks") {
		t.Fatalf("projects not disambiguated:\n%s", view)
	}
}

func TestViewKeepsStatusTextAlongsideColor(t *testing.T) {
	tasks := []presenter.Task{{Title: "Task", Due: "2026-07-10", Status: "En progreso"}}
	view := View(tasks, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), -1, 90, 20)
	if !strings.Contains(view, "Task · En progreso") {
		t.Fatalf("calendar event lacks textual status:\n%s", view)
	}
}
