package presenter

import (
	"strings"
	"testing"

	"github.com/Polo123456789/tasks/internal/domain"
)

func TestTasksBuildsViewSummaryAndProjectIdentity(t *testing.T) {
	start, _ := domain.ParseDate("2026-07-01")
	due, _ := domain.ParseDate("2026-07-15")
	input := domain.Task{
		ID: 4, Project: "/workspace/backend.tasks", Title: "Task",
		Status: domain.Status{Name: "En progreso"}, Priority: domain.PriorityHigh,
		Start: &start, Due: &due, Markdown: "# Documentation", Blocked: true,
		Recurrence: &domain.Recurrence{Kind: domain.Daily}, SubtaskDoneCount: 2,
		SubtaskCount: 3, DependencyCount: 4, Version: 7,
	}
	got := Tasks([]domain.Task{input})[0]
	if got.Project != "backend" || got.Source != input.Project || got.Dates != "2026-07-01 → 2026-07-15" {
		t.Fatalf("identity/dates: %#v", got)
	}
	if got.SubtasksDone != 2 || got.SubtasksTotal != 3 || got.Dependencies != 4 || got.Markdown != input.Markdown {
		t.Fatalf("summary: %#v", got)
	}
	badge := Badge(got)
	if !strings.Contains(badge, "🔒") || !strings.Contains(badge, "↻") {
		t.Fatalf("badge=%q", badge)
	}
}
