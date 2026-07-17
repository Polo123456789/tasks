package presenter

import (
	"fmt"
	"github.com/Polo123456789/tasks/internal/domain"
)

type Task struct {
	ID                                             int64
	Source, Origin, Title, Status, Priority, Dates string
	StatusID                                       int64
	PriorityValue                                  domain.Priority
	SourceKind                                     domain.OriginKind
	StatusKind                                     domain.StatusKind
	Start, Due                                     string
	DeletedAt                                      string
	Markdown                                       string
	Recurrence                                     string
	RecurrenceText                                 string
	Blocked, Recurring                             bool
	SubtasksDone, SubtasksTotal, Dependencies      int
	Subtasks                                       []Subtask
	DependencyIDs                                  []int64
	Version                                        int64
}

type Subtask struct {
	ID     int64
	Title  string
	Status string
	Done   bool
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
		done, total, dependencies := v.SubtaskDoneCount, v.SubtaskCount, v.DependencyCount
		if len(v.Subtasks) > 0 {
			total = len(v.Subtasks)
			done = 0
			for _, subtask := range v.Subtasks {
				if subtask.Status.Kind == domain.StatusDone {
					done++
				}
			}
		}
		if len(v.DependencyIDs) > 0 {
			dependencies = len(v.DependencyIDs)
		}
		subtasks := make([]Subtask, 0, len(v.Subtasks))
		for _, subtask := range v.Subtasks {
			subtasks = append(subtasks, Subtask{ID: subtask.ID, Title: subtask.Title, Status: subtask.Status.Name, Done: subtask.Status.Kind == domain.StatusDone})
		}
		out = append(out, Task{
			ID: v.ID, Source: v.Origin.Key, Origin: v.Origin.Name, SourceKind: v.Origin.Kind, Title: v.Title,
			Status: v.Status.Name, StatusID: v.StatusID, StatusKind: v.Status.Kind,
			Priority: v.Priority.String(), PriorityValue: v.Priority, Dates: dates,
			Markdown: v.Markdown, Blocked: v.Blocked, Recurring: v.Recurrence != nil,
			SubtasksDone: done, SubtasksTotal: total, Dependencies: dependencies,
			Subtasks: subtasks, DependencyIDs: append([]int64(nil), v.DependencyIDs...),
			Version: v.Version,
		})
		if v.Recurrence != nil {
			out[len(out)-1].Recurrence = v.Recurrence.HumanText()
			out[len(out)-1].RecurrenceText = v.Recurrence.Text()
		}
		if v.Start != nil {
			out[len(out)-1].Start = v.Start.String()
		}
		if v.Due != nil {
			out[len(out)-1].Due = v.Due.String()
		}
		if v.DeletedAt != nil {
			out[len(out)-1].DeletedAt = v.DeletedAt.String()
		}
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
