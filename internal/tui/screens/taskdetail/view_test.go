package taskdetail

import (
	"strings"
	"testing"
	"time"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/tui/presenter"
	"github.com/charmbracelet/lipgloss"
)

func TestViewShowsMetadataAndTruncatedMarkdown(t *testing.T) {
	task := presenter.Task{
		Title: "Detailed", Status: "Pendiente", Priority: "Alta", Dates: "2026-07-15",
		Origin:  "alpha",
		Blocked: true, Recurring: true, SubtasksDone: 1, SubtasksTotal: 2,
		Dependencies: 3, Markdown: "one\ntwo\nthree\nfour\nfive\nsix",
	}
	view := View(task, 0, 60, 20)
	for _, text := range []string{"Detailed", "origen alpha", "bloqueada", "automáticamente", "subtareas", "1/2", "dependencias 3", "one", "…"} {
		if !strings.Contains(view, text) {
			t.Errorf("view lacks %q: %s", text, view)
		}
	}
}

func TestInspectorRowsCoverFieldsSubtasksDependenciesAndHistory(t *testing.T) {
	task := presenter.Task{
		ID: 1, Title: "Task", Status: "Pending", Priority: "High", Markdown: "# Notes",
		Subtasks:      []presenter.Subtask{{ID: 2, Title: "Child", Status: "Pending"}},
		DependencyIDs: []int64{9},
	}
	history := []domain.HistoryEvent{{ID: 3, Kind: "created", Detail: "imported", CreatedAt: time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)}}
	rows := Rows(task, history)
	if len(rows) != 10 || rows[0].Kind != RowSubtask || rows[1].Field != "title" || rows[8].Kind != RowDependency || rows[9].Kind != RowHistory {
		t.Fatalf("rows=%#v", rows)
	}
	view := InspectorView(task, history, 9, 60, 12, true, true, true)
	for _, expected := range []string{"Inspector · ACTIVO · EXPANDIDO · FIJADO", "Historial", "creada · imported"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("inspector missing %q:\n%s", expected, view)
		}
	}
}

func TestCompactInspectorShowsSubtasksBeforeTaskFields(t *testing.T) {
	task := presenter.Task{
		Title: "Parent",
		Subtasks: []presenter.Subtask{
			{Title: "First child", Status: "Pending"},
			{Title: "Second child", Status: "Pending"},
		},
	}
	view := InspectorView(task, nil, 0, 80, 8, false, false, false)
	for _, expected := range []string{"First child", "Second child"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("compact inspector hides %q:\n%s", expected, view)
		}
	}
}

func TestInspectorViewNeverExceedsAssignedHeight(t *testing.T) {
	task := presenter.Task{ID: 1, Title: "Crowded"}
	for id := int64(1); id <= 20; id++ {
		task.Subtasks = append(task.Subtasks, presenter.Subtask{ID: id, Title: "Child", Status: "Pending"})
	}
	for _, height := range []int{3, 8, 20} {
		view := InspectorView(task, nil, 12, 90, height, true, height > 8, false)
		if got := lipgloss.Height(view); got > height {
			t.Fatalf("height=%d rendered=%d:\n%s", height, got, view)
		}
	}
}

func TestViewUsesFullWidthAndNeverExceedsHeight(t *testing.T) {
	task := presenter.Task{
		Title: "Wide detail", Status: "Pendiente", Priority: "Urgente",
		Markdown: strings.Repeat("A long markdown paragraph that must remain a preview. ", 20),
		Subtasks: []presenter.Subtask{
			{Title: "First long subtask", Status: "Pendiente"},
			{Title: "Second long subtask", Status: "Pendiente"},
			{Title: "Third long subtask", Status: "Pendiente"},
			{Title: "Fourth long subtask", Status: "Pendiente"},
		},
	}
	view := View(task, 0, 160, 9)
	if lipgloss.Width(view) != 160 {
		t.Fatalf("detail width=%d, want 160:\n%s", lipgloss.Width(view), view)
	}
	if lipgloss.Height(view) > 9 {
		t.Fatalf("detail height=%d, want at most 9:\n%s", lipgloss.Height(view), view)
	}
	for _, expected := range []string{"Subtareas", "Markdown", "First long subtask"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("wide detail lacks %q:\n%s", expected, view)
		}
	}
}

func TestDetailLinesKeepSelectedSubtaskVisibleInTinyColumn(t *testing.T) {
	task := presenter.Task{Subtasks: []presenter.Subtask{
		{Title: "first", Status: "Pendiente"},
		{Title: "second", Status: "Pendiente"},
		{Title: "selected last", Status: "Pendiente"},
	}}
	lines := detailLines(task, 2, 40, 2)
	if !strings.Contains(strings.Join(lines, "\n"), "selected last") {
		t.Fatalf("selected subtask is not visible: %#v", lines)
	}
}

func TestDetailLinesAlignSubtaskStatuses(t *testing.T) {
	task := presenter.Task{Subtasks: []presenter.Subtask{
		{Title: "Corta", Status: "Pendiente"},
		{Title: "Una subtarea con un título largo", Status: "Finalizada", Done: true},
	}}
	lines := detailLines(task, 0, 60, 4)
	if len(lines) < 3 {
		t.Fatalf("subtask rows missing: %#v", lines)
	}
	if lipgloss.Width(lines[1][:strings.Index(lines[1], "[Pendiente]")]) != lipgloss.Width(lines[2][:strings.Index(lines[2], "[Finalizada]")]) {
		t.Fatalf("status columns are not aligned:\n%s\n%s", lines[1], lines[2])
	}
}
