package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Mode string

const (
	ModeLocal  Mode = "local"
	ModeGlobal Mode = "global"
)

type OriginKind string

const (
	OriginGlobal    OriginKind = "global"
	OriginProject   OriginKind = "project"
	GlobalOriginKey            = "global"
)

type TaskOrigin struct {
	Kind OriginKind
	Key  string
	Name string
}

func (o TaskOrigin) Identity() string { return string(o.Kind) + ":" + o.Key }

type Capabilities struct {
	CanCreateTask       bool
	CanCreateStatus     bool
	CanCreateSubtask    bool
	CanCreateDependency bool
	CanCreateRecurrence bool
}

type Priority int

const (
	PriorityNone Priority = iota
	PriorityLow
	PriorityMedium
	PriorityHigh
	PriorityUrgent
)

func (p Priority) Valid() bool { return p >= PriorityNone && p <= PriorityUrgent }
func (p Priority) String() string {
	return [...]string{"Ninguna", "Baja", "Media", "Alta", "Urgente"}[p]
}

type StatusKind string

const (
	StatusNormal    StatusKind = "normal"
	StatusCancelled StatusKind = "cancelled"
	StatusDone      StatusKind = "done"
)

type Status struct {
	ID       int64
	Name     string
	Kind     StatusKind
	Position int
	Initial  bool
}
type Task struct {
	ID                   int64
	Origin               TaskOrigin
	Title                string
	StatusID             int64
	Status               Status
	Priority             Priority
	Markdown             string
	Start, Due           *Date
	Recurrence           *Recurrence
	RecurrenceAnchor     *Date
	Version              int64
	DeletedAt            *Date
	CreatedAt, UpdatedAt time.Time
	Subtasks             []Subtask
	DependencyIDs        []int64
	SubtaskDoneCount     int
	SubtaskCount         int
	DependencyCount      int
	Blocked              bool
}
type Subtask struct {
	ID, TaskID int64
	Title      string
	StatusID   int64
	Status     Status
}
type HistoryEvent struct {
	ID, TaskID   int64
	Kind, Detail string
	CreatedAt    time.Time
}

var (
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("concurrent modification")
	ErrForbidden       = errors.New("operation forbidden in this mode")
	ErrDependencyCycle = errors.New("dependency cycle")
	ErrValidation      = errors.New("validation error")
)

type ValidationError struct{ Field, Message string }

func (e ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }
func (e ValidationError) Unwrap() error { return ErrValidation }
func ValidateTask(t Task) error {
	if strings.TrimSpace(t.Title) == "" {
		return ValidationError{"title", "required"}
	}
	if !t.Priority.Valid() {
		return ValidationError{"priority", "invalid"}
	}
	if t.Start != nil && t.Due != nil && t.Due.Before(*t.Start) {
		return ValidationError{"due", "must not precede start"}
	}
	if t.Recurrence != nil && (t.Start != nil || t.Due != nil) {
		return ValidationError{"recurrence", "recurring tasks cannot have dates"}
	}
	if t.Recurrence != nil {
		if err := t.Recurrence.Validate(); err != nil {
			return ValidationError{"recurrence", err.Error()}
		}
	}
	return nil
}
