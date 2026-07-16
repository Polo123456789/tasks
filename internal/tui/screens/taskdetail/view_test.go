package taskdetail

import (
	"strings"
	"testing"

	"github.com/Polo123456789/tasks/internal/tui/presenter"
)

func TestViewShowsMetadataAndTruncatedMarkdown(t *testing.T) {
	task := presenter.Task{
		Title: "Detailed", Status: "Pendiente", Priority: "Alta", Dates: "2026-07-15",
		Project: "alpha",
		Blocked: true, Recurring: true, SubtasksDone: 1, SubtasksTotal: 2,
		Dependencies: 3, Markdown: "one\ntwo\nthree\nfour\nfive\nsix",
	}
	view := View(task, 0, 60, 20)
	for _, text := range []string{"Detailed", "proyecto alpha", "bloqueada", "automáticamente", "subtareas", "1/2", "dependencias 3", "one", "…"} {
		if !strings.Contains(view, text) {
			t.Errorf("view lacks %q: %s", text, view)
		}
	}
}
