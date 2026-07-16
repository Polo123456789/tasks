package application

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	db "github.com/Polo123456789/tasks/internal/adapters/sqlite"
	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/ports"
)

func BenchmarkGlobalListTwentyProjects(b *testing.B) {
	ctx := context.Background()
	service := Service{Mode: domain.ModeGlobal}
	for projectIndex := 0; projectIndex < 20; projectIndex++ {
		path := filepath.Join(b.TempDir(), fmt.Sprintf("project-%02d.tasks", projectIndex))
		store, err := db.Open(path)
		if err != nil {
			b.Fatal(err)
		}
		for taskIndex := 0; taskIndex < 100; taskIndex++ {
			if _, err = store.CreateTask(ctx, domain.Task{Title: fmt.Sprintf("Task %04d", taskIndex)}); err != nil {
				b.Fatal(err)
			}
		}
		service.Projects = append(service.Projects, Project{Path: path, Name: fmt.Sprintf("project-%02d", projectIndex), Store: store})
	}
	defer service.Close()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		if _, err := service.ListTasks(ctx, ports.TaskFilter{}); err != nil {
			b.Fatal(err)
		}
	}
}
