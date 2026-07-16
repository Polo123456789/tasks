package taskdetail

import (
	"strings"
	"testing"

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
