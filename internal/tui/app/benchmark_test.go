package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/Polo123456789/tasks/internal/tui/presenter"
)

func BenchmarkRenderViews1000Tasks(b *testing.B) {
	backend := &fakeBackend{mode: "local"}
	model := New(backend)
	model.width, model.height, model.loading = 120, 40, false
	model.calendarMonth = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	for index := 0; index < 1000; index++ {
		model.tasks = append(model.tasks, presenter.Task{
			ID: int64(index + 1), Title: fmt.Sprintf("Task %04d", index), Status: "Pendiente",
			Priority: "Media", Start: "2026-07-01", Due: "2026-07-20", Dates: "2026-07-01 → 2026-07-20",
		})
	}
	for _, view := range []int{0, 1, 2, 3} {
		b.Run(fmt.Sprintf("view-%d", view), func(b *testing.B) {
			model.view = view
			for iteration := 0; iteration < b.N; iteration++ {
				_ = model.View()
			}
		})
	}
}
