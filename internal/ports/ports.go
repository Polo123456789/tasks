package ports

import (
	"context"
	"github.com/Polo123456789/tasks/internal/domain"
	"time"
)

type Clock interface {
	Now() time.Time
	Today() domain.Date
}
type Registry interface {
	Register(context.Context, string) error
	Projects(context.Context) ([]string, error)
	Prune(context.Context) ([]string, error)
	Close() error
}
type TaskFilter struct {
	Query, Markdown, Project                                                  string
	StatusIDs                                                                 []int64
	Priorities                                                                []domain.Priority
	IncludeDone, IncludeCancelled, IncludeDeleted, OnlyBlocked, OnlyRecurring bool
	From, To                                                                  *domain.Date
	Sort                                                                      string
}
type TaskStore interface {
	Statuses(context.Context) ([]domain.Status, error)
	CreateStatus(context.Context, string, bool) (domain.Status, error)
	RenameStatus(context.Context, int64, string) error
	DeleteStatus(context.Context, int64, *int64) error
	ListTasks(context.Context, TaskFilter) ([]domain.Task, error)
	Task(context.Context, int64) (domain.Task, error)
	CreateTask(context.Context, domain.Task) (domain.Task, error)
	UpdateTask(context.Context, domain.Task) (domain.Task, error)
	SetTaskStatus(context.Context, int64, int64, int64) (domain.Task, error)
	TrashTask(context.Context, int64, int64, domain.Date) ([]int64, error)
	RestoreTask(context.Context, int64, int64) (domain.Task, error)
	AddSubtask(context.Context, int64, string) (domain.Subtask, error)
	SetSubtaskStatus(context.Context, int64, int64) error
	AddDependency(context.Context, int64, int64) error
	RemoveDependency(context.Context, int64, int64) error
	History(context.Context, int64) ([]domain.HistoryEvent, error)
	Maintain(context.Context, domain.Date) error
	Close() error
}
