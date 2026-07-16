package gantt

import (
	"strings"
	"testing"
	"time"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/tui/presenter"
	"github.com/Polo123456789/tasks/internal/tui/theme"
	"github.com/charmbracelet/lipgloss"
)

func TestViewRendersIntervalsMilestonesDependenciesAndProjects(t *testing.T) {
	tasks := []presenter.Task{
		{Title: "Interval", Start: "2026-07-02", Due: "2026-07-05", Origin: "alpha", DependencyIDs: []int64{9}},
		{Title: "Start only", Start: "2026-07-10"},
		{Title: "Due only", Due: "2026-07-12"},
		{Title: "Recurring", Start: "2026-07-01", Recurring: true},
		{Title: "Undated"},
	}
	view := View(tasks, time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC), 0, 100, 20, 1)
	for _, expected := range []string{"Interval [alpha]", "●━━━━━━●", "Start only", "Due only", "◆", "↳#9"} {
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

func TestStatusLegendNamesAndColorsRemainVisible(t *testing.T) {
	items := []legendItem{
		{name: "En progreso", kind: domain.StatusNormal},
		{name: "Finalizada", kind: domain.StatusDone},
	}
	legend := statusLegend(items, 100)
	for _, expected := range []string{
		theme.Status(domain.StatusNormal, "En progreso").Render("En progreso"),
		theme.Status(domain.StatusDone, "Finalizada").Render("Finalizada"),
	} {
		if !strings.Contains(legend, expected) {
			t.Fatalf("legend lacks styled status %q:\n%s", expected, legend)
		}
	}
}

func TestViewUsesWideTerminalForReadableDayCells(t *testing.T) {
	tasks := []presenter.Task{{Title: "Wide interval", Start: "2026-07-01", Due: "2026-07-31"}}
	view := View(tasks, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), 0, 160, 20, 1)
	lines := strings.Split(view, "\n")
	if len(lines) < 3 || lipgloss.Width(lines[1]) < 155 {
		t.Fatalf("Gantt did not use the available width:\n%s", view)
	}
	for _, day := range []string{" 1  ", " 10 ", " 31 "} {
		if !strings.Contains(lines[1], day) {
			t.Fatalf("header lacks readable day %q:\n%s", day, lines[1])
		}
	}
}

func TestViewSpacesDayLabelsAtMinimumWidth(t *testing.T) {
	tasks := []presenter.Task{{Title: "Interval", Start: "2026-07-01", Due: "2026-07-31"}}
	view := View(tasks, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), 0, 90, 20, 1)
	header := strings.Split(view, "\n")[1]
	if strings.Contains(header, "101112") {
		t.Fatalf("narrow day labels run together:\n%s", header)
	}
	for _, day := range []string{" 5", "10", "15", "20", "25", "30"} {
		if !strings.Contains(header, day) {
			t.Fatalf("narrow header lacks %q:\n%s", day, header)
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
		{Title: "One", Due: "2026-07-01", Origin: "same", Source: "/a/same.tasks"},
		{Title: "Two", Due: "2026-07-02", Origin: "same", Source: "/b/same.tasks"},
	}
	view := View(tasks, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), 0, 120, 20, 1)
	if !strings.Contains(view, "/a/same.tasks") || !strings.Contains(view, "/b/same.tasks") {
		t.Fatalf("projects not disambiguated:\n%s", view)
	}
}
