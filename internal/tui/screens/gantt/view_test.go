package gantt

import (
	"strings"
	"testing"
	"time"

	"github.com/Polo123456789/tasks/internal/tui/presenter"
)

func TestViewRendersIntervalsMilestonesDependenciesAndProjects(t *testing.T) {
	tasks := []presenter.Task{
		{Title: "Interval", Start: "2026-07-02", Due: "2026-07-05", Project: "alpha", DependencyIDs: []int64{9}},
		{Title: "Start only", Start: "2026-07-10"},
		{Title: "Due only", Due: "2026-07-12"},
		{Title: "Recurring", Start: "2026-07-01", Recurring: true},
		{Title: "Undated"},
	}
	view := View(tasks, time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC), 0, 100, 20, 1)
	for _, expected := range []string{"Interval [alpha]", "●━━●", "Start only", "Due only", "◆", "depende de #9"} {
		if !strings.Contains(view, expected) {
			t.Errorf("missing %q:\n%s", expected, view)
		}
	}
	for _, excluded := range []string{"Recurring", "Undated"} {
		if strings.Contains(view, excluded) {
			t.Errorf("unexpected %q:\n%s", excluded, view)
		}
	}
}

func TestViewClipsIntervalsToVisibleMonth(t *testing.T) {
	tasks := []presenter.Task{{Title: "Crossing", Start: "2026-06-20", Due: "2026-08-05"}}
	view := View(tasks, time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC), 0, 50, 20, 1)
	if !strings.Contains(view, "Crossing") || !strings.Contains(view, "●") {
		t.Fatalf("clipped interval:\n%s", view)
	}
}

func TestViewDisambiguatesSameNamedProjects(t *testing.T) {
	tasks := []presenter.Task{
		{Title: "One", Due: "2026-07-01", Project: "same", Source: "/a/same.tasks"},
		{Title: "Two", Due: "2026-07-02", Project: "same", Source: "/b/same.tasks"},
	}
	view := View(tasks, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), 0, 120, 20, 1)
	if !strings.Contains(view, "/a/same.tasks") || !strings.Contains(view, "/b/same.tasks") {
		t.Fatalf("projects not disambiguated:\n%s", view)
	}
}
