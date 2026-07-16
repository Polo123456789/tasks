package kanban

import (
	"strings"
	"testing"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/tui/presenter"
)

func TestViewUsesOriginalTaskSelectionAndKeepsDoneLast(t *testing.T) {
	statuses := []domain.Status{
		{ID: 1, Name: "Pending"}, {ID: 2, Name: "Progress"},
		{ID: 3, Name: "Cancelled", Kind: domain.StatusCancelled},
		{ID: 4, Name: "Done", Kind: domain.StatusDone},
	}
	tasks := []presenter.Task{
		{Title: "progress task", Status: "Progress", Priority: "Alta"},
		{Title: "pending task", Status: "Pending", Priority: "Baja"},
	}
	view := View(tasks, statuses, 0, 120, 20)
	if strings.Index(view, "Done") < strings.Index(view, "Cancelled") {
		t.Fatalf("done column is not last:\n%s", view)
	}
	if !strings.Contains(view, "progress task") || !strings.Contains(view, "pending task") {
		t.Fatalf("missing tasks:\n%s", view)
	}
}

func TestViewPagesColumnsInSmallTerminal(t *testing.T) {
	statuses := []domain.Status{{ID: 1, Name: "One"}, {ID: 2, Name: "Two"}, {ID: 3, Name: "Three"}}
	view := View([]presenter.Task{{Title: "task", Status: "One", Priority: "Ninguna"}}, statuses, 0, 30, 20)
	if !strings.Contains(view, "One") || !strings.Contains(view, "→") {
		t.Fatalf("small Kanban:\n%s", view)
	}
}
