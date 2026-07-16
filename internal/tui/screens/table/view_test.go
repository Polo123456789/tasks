package table

import (
	"strings"
	"testing"

	"github.com/Polo123456789/tasks/internal/tui/presenter"
)

func TestViewShowsRequiredTaskSummaryAndDisambiguatesProjects(t *testing.T) {
	tasks := []presenter.Task{
		{Title: "First", Status: "Pending", Priority: "Alta", Recurrence: "weekly:mon,thu", Blocked: true, SubtasksDone: 1, SubtasksTotal: 3, Project: "same", Source: "/a/same.tasks"},
		{Title: "Second", Status: "Done", Priority: "Baja", Project: "same", Source: "/b/same.tasks"},
	}
	view := View(tasks, 0, 120, 20)
	for _, expected := range []string{"DATOS", "weekly:mon", "bloqueada", "sub 1/3", "/a/same.tasks", "/b/same.tasks"} {
		if !strings.Contains(view, expected) {
			t.Errorf("missing %q:\n%s", expected, view)
		}
	}
}
