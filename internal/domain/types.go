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

type Capabilities struct{ CanCreateTask, CanCreateStatus, CanCreateDependency, CanCreateRecurrence bool }

func CapabilitiesFor(m Mode) Capabilities {
	if m == ModeLocal {
		return Capabilities{true, true, true, true}
	}
	return Capabilities{}
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
	Project              string
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
	Blocked              bool
}
type Subtask struct {
	ID, TaskID int64
	Title      string
	StatusID   int64
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
	return nil
}
