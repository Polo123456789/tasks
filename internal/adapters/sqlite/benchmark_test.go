package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/ports"
)

func BenchmarkListTasks1000(b *testing.B) {
	store, err := Open(filepath.Join(b.TempDir(), "benchmark.tasks"))
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	for index := 0; index < 1000; index++ {
		if _, err = store.CreateTask(ctx, domain.Task{Title: fmt.Sprintf("Task %04d", index), Priority: domain.Priority(index % 5)}); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		if _, err = store.ListTasks(ctx, ports.TaskFilter{IncludeDone: true, IncludeCancelled: true, Sort: "priority"}); err != nil {
			b.Fatal(err)
		}
	}
}
