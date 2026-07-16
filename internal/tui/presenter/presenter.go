package presenter

import (
	"fmt"
	"github.com/Polo123456789/tasks/internal/domain"
	"path/filepath"
)

type Task struct {
	ID                                      int64
	Project, Title, Status, Priority, Dates string
	Blocked, Recurring                      bool
	Version                                 int64
}

func Tasks(in []domain.Task) []Task {
	out := make([]Task, 0, len(in))
	for _, v := range in {
		dates := ""
		if v.Start != nil {
			dates = v.Start.String()
		}
		if v.Due != nil {
			if dates != "" {
				dates += " → "
			}
			dates += v.Due.String()
		}
		project := ""
		if v.Project != "" {
			project = filepath.Base(v.Project)
		}
		out = append(out, Task{v.ID, project, v.Title, v.Status.Name, v.Priority.String(), dates, v.Blocked, v.Recurrence != nil, v.Version})
	}
	return out
}
func Badge(t Task) string {
	flags := ""
	if t.Blocked {
		flags += " 🔒"
	}
	if t.Recurring {
		flags += " ↻"
	}
	return fmt.Sprintf("%s  [%s]%s", t.Title, t.Priority, flags)
}
