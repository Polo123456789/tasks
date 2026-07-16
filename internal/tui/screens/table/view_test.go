package table

import (
	"strconv"
	"strings"
	"testing"

	"github.com/Polo123456789/tasks/internal/tui/presenter"
	"github.com/charmbracelet/lipgloss"
)

func TestViewRendersAlignedColumnsAndDisambiguatesProjects(t *testing.T) {
	tasks := []presenter.Task{
		{Title: "First", Status: "Pending", Priority: "Alta", Recurrence: "weekly:mon,thu", Blocked: true, SubtasksDone: 1, SubtasksTotal: 3, Project: "same", Source: "/a/same.tasks"},
		{Title: "Second", Status: "Done", Priority: "Baja", Project: "same", Source: "/b/same.tasks"},
	}
	view := View(tasks, 0, 120, 20, true)
	for _, expected := range []string{"PLANIFICACIÓN", "│", "─┼─", "weekly:mon", "sí", "1/3", "/a/same.tasks", "/b/same.tasks"} {
		if !strings.Contains(view, expected) {
			t.Errorf("missing %q:\n%s", expected, view)
		}
	}
	columns := tableColumns(120, true)
	header := renderRow(columns, headerValues(columns))
	row := renderRow(columns, rowValues(tasks[0], "/a/same.tasks", 23))
	if dividerPositions(header) != dividerPositions(row) {
		t.Fatalf("columns are not aligned:\n%s\n%s", header, row)
	}
}

func TestNarrowPlanningUsesReadableCompactDates(t *testing.T) {
	task := presenter.Task{Start: "2026-07-16", Due: "2026-07-18", Dates: "2026-07-16 → 2026-07-18"}
	if got := planning(task, 17); got != "16–18 jul 2026" {
		t.Fatalf("compact planning=%q", got)
	}
	if got := planning(task, 23); got != task.Dates {
		t.Fatalf("wide planning=%q", got)
	}
	task.Due = task.Start
	if got := planning(task, 17); got != "16 jul 2026" {
		t.Fatalf("compact milestone=%q", got)
	}
}

func TestViewAdaptsToMinimumWidth(t *testing.T) {
	tasks := []presenter.Task{{
		ID: 42, Title: "A very long task title that needs truncation", Status: "En progreso",
		Priority: "Urgente", Dates: "2026-07-16 → 2026-08-15", Blocked: true,
		SubtasksDone: 3, SubtasksTotal: 10, Project: "alpha", Source: "/work/alpha.tasks",
	}}
	view := View(tasks, 0, 90, 10, true)
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > 90 {
			t.Fatalf("line width=%d, want at most 90:\n%s", lipgloss.Width(line), view)
		}
	}
	for _, expected := range []string{"TÍTULO", "ESTADO", "PRIOR.", "SUB", "BLOQ.", "[alpha]", "›"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("minimum-width table lacks %q:\n%s", expected, view)
		}
	}
}

func TestWideGlobalViewUsesProjectColumn(t *testing.T) {
	tasks := []presenter.Task{{ID: 1, Title: "Task", Status: "Pending", Priority: "Alta", Project: "alpha", Source: "/work/alpha.tasks"}}
	view := View(tasks, 0, 160, 10, true)
	if !strings.Contains(view, "PROYECTO") || !strings.Contains(view, "alpha") {
		t.Fatalf("wide global table lacks project column:\n%s", view)
	}
}

func dividerPositions(value string) string {
	positions := make([]string, 0)
	for index, r := range []rune(value) {
		if r == '│' {
			positions = append(positions, strconv.Itoa(index))
		}
	}
	return strings.Join(positions, ",")
}
