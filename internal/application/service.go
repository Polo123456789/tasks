package application

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/ports"
)

type Source struct {
	Origin domain.TaskOrigin
	Store  ports.TaskStore
	Err    error
}
type Service struct {
	Mode           domain.Mode
	Sources        []Source
	WritableSource string
	Clock          ports.Clock
	locks          sync.Map
}

func (s *Service) Capabilities(source string) domain.Capabilities {
	if s.Mode == domain.ModeLocal {
		available := s.sourceAvailable(s.writableKey())
		return domain.Capabilities{
			CanCreateTask:       available,
			CanCreateStatus:     available,
			CanCreateSubtask:    available,
			CanCreateDependency: available,
			CanCreateRecurrence: available,
		}
	}
	globalAvailable := s.sourceAvailable(s.WritableSource)
	capabilities := domain.Capabilities{CanCreateTask: globalAvailable}
	if source == s.WritableSource && globalAvailable {
		capabilities.CanCreateSubtask = true
		capabilities.CanCreateDependency = true
		capabilities.CanCreateRecurrence = true
	}
	return capabilities
}

// Statuses returns the workflow statuses for a specific source. An empty
// source resolves to the writable source, which is what creation forms need in
// both local and global mode.
func (s *Service) Statuses(ctx context.Context, source string) ([]domain.Status, error) {
	if source == "" {
		source = s.writableKey()
	}
	p, err := s.source(source)
	if err != nil {
		return nil, err
	}
	return p.Store.Statuses(ctx)
}
func (s *Service) CreateStatus(ctx context.Context, path, name string, initial bool) (domain.Status, error) {
	if !s.Capabilities("").CanCreateStatus {
		return domain.Status{}, domain.ErrForbidden
	}
	p, e := s.source(path)
	if e != nil {
		return domain.Status{}, e
	}
	v, _ := s.locks.LoadOrStore(path, &sync.Mutex{})
	m := v.(*sync.Mutex)
	m.Lock()
	defer m.Unlock()
	return p.Store.CreateStatus(ctx, name, initial)
}
func (s *Service) RenameStatus(ctx context.Context, path string, id int64, name string) error {
	if !s.Capabilities("").CanCreateStatus {
		return domain.ErrForbidden
	}
	p, e := s.source(path)
	if e != nil {
		return e
	}
	return s.serialError(path, func() error { return p.Store.RenameStatus(ctx, id, name) })
}
func (s *Service) SetInitialStatus(ctx context.Context, path string, id int64) error {
	if !s.Capabilities("").CanCreateStatus {
		return domain.ErrForbidden
	}
	p, e := s.source(path)
	if e != nil {
		return e
	}
	return s.serialError(path, func() error { return p.Store.SetInitialStatus(ctx, id) })
}
func (s *Service) ReorderStatuses(ctx context.Context, path string, ids []int64) error {
	if !s.Capabilities("").CanCreateStatus {
		return domain.ErrForbidden
	}
	p, e := s.source(path)
	if e != nil {
		return e
	}
	return s.serialError(path, func() error { return p.Store.ReorderStatuses(ctx, ids) })
}
func (s *Service) DeleteStatus(ctx context.Context, path string, id int64, destination *int64) error {
	if !s.Capabilities("").CanCreateStatus {
		return domain.ErrForbidden
	}
	p, e := s.source(path)
	if e != nil {
		return e
	}
	return s.serialError(path, func() error { return p.Store.DeleteStatus(ctx, id, destination) })
}
func (s *Service) ListTasks(ctx context.Context, f ports.TaskFilter) ([]domain.Task, error) {
	var out []domain.Task
	var errs []error
	for _, p := range s.Sources {
		if f.Origin != "" && f.Origin != p.Origin.Key && f.Origin != p.Origin.Name {
			continue
		}
		if p.Err != nil {
			errs = append(errs, p.Err)
			continue
		}
		if p.Store == nil {
			errs = append(errs, fmt.Errorf("%s: origen no disponible", p.Origin.Name))
			continue
		}
		tasks, e := p.Store.ListTasks(ctx, f)
		if e != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.Origin.Name, e))
			continue
		}
		for i := range tasks {
			tasks[i].Origin = p.Origin
		}
		out = append(out, tasks...)
	}
	sort.SliceStable(out, func(i, j int) bool { return taskLess(out[i], out[j], f.Sort) })
	return out, errors.Join(errs...)
}

func taskLess(left, right domain.Task, order string) bool {
	different := false
	less := false
	switch order {
	case "priority":
		different, less = left.Priority != right.Priority, left.Priority > right.Priority
	case "title":
		leftTitle, rightTitle := strings.ToLower(left.Title), strings.ToLower(right.Title)
		different, less = leftTitle != rightTitle, leftTitle < rightTitle
	case "status":
		leftRank, rightRank := statusRank(left.Status.Kind), statusRank(right.Status.Kind)
		if leftRank != rightRank {
			different, less = true, leftRank < rightRank
		} else if left.Status.Position != right.Status.Position {
			different, less = true, left.Status.Position < right.Status.Position
		} else {
			leftStatus, rightStatus := strings.ToLower(left.Status.Name), strings.ToLower(right.Status.Name)
			different, less = leftStatus != rightStatus, leftStatus < rightStatus
		}
	case "start":
		different, less = compareOptionalDate(left.Start, right.Start)
	case "due":
		different, less = compareOptionalDate(left.Due, right.Due)
	default:
		different, less = !left.UpdatedAt.Equal(right.UpdatedAt), left.UpdatedAt.After(right.UpdatedAt)
	}
	if different {
		return less
	}
	if left.Origin.Identity() != right.Origin.Identity() {
		return left.Origin.Identity() < right.Origin.Identity()
	}
	return left.ID < right.ID
}

func statusRank(kind domain.StatusKind) int {
	switch kind {
	case domain.StatusCancelled:
		return 1
	case domain.StatusDone:
		return 2
	default:
		return 0
	}
}

func compareOptionalDate(left, right *domain.Date) (different, less bool) {
	if left == nil && right == nil {
		return false, false
	}
	if left == nil {
		return true, false
	}
	if right == nil {
		return true, true
	}
	if left.Equal(*right) {
		return false, false
	}
	return true, left.Before(*right)
}
func (s *Service) source(key string) (Source, error) {
	if key == "" {
		key = s.writableKey()
	}
	for _, p := range s.Sources {
		if p.Origin.Key == key {
			if p.Err != nil {
				return p, p.Err
			}
			if p.Store == nil {
				return p, domain.ErrNotFound
			}
			return p, nil
		}
	}
	return Source{}, domain.ErrNotFound
}
func (s *Service) writableKey() string {
	if s.WritableSource != "" {
		return s.WritableSource
	}
	if len(s.Sources) == 1 {
		return s.Sources[0].Origin.Key
	}
	return ""
}
func (s *Service) sourceAvailable(key string) bool {
	if key == "" {
		return false
	}
	for _, source := range s.Sources {
		if source.Origin.Key == key {
			return source.Store != nil && source.Err == nil
		}
	}
	return false
}
func (s *Service) CreateTask(ctx context.Context, t domain.Task) (domain.Task, error) {
	key := s.writableKey()
	p, e := s.source(key)
	if e != nil {
		return t, e
	}
	if t.Recurrence != nil && t.RecurrenceAnchor == nil {
		if s.Clock == nil {
			return t, domain.ValidationError{Field: "recurrence", Message: "clock is required"}
		}
		today := s.Clock.Today()
		t.RecurrenceAnchor = &today
	}
	created, e := s.serial(key, func() (domain.Task, error) { return p.Store.CreateTask(ctx, t) })
	created.Origin = p.Origin
	return created, e
}
func (s *Service) UpdateTask(ctx context.Context, path string, t domain.Task) (domain.Task, error) {
	p, e := s.source(path)
	if e != nil {
		return t, e
	}
	v, _ := s.locks.LoadOrStore(path, &sync.Mutex{})
	m := v.(*sync.Mutex)
	m.Lock()
	defer m.Unlock()
	current, e := p.Store.Task(ctx, t.ID)
	if e != nil {
		return t, e
	}
	if current.Version != t.Version {
		return t, domain.ErrConflict
	}
	if current.Recurrence == nil && t.Recurrence != nil && !s.Capabilities(path).CanCreateRecurrence {
		return t, domain.ErrForbidden
	}
	if t.Recurrence == nil {
		t.RecurrenceAnchor = nil
	} else if t.RecurrenceAnchor == nil {
		if s.Clock == nil {
			return t, domain.ValidationError{Field: "recurrence", Message: "clock is required"}
		}
		anchor := s.Clock.Today()
		t.RecurrenceAnchor = &anchor
	}
	updated, e := p.Store.UpdateTask(ctx, t)
	updated.Origin = p.Origin
	return updated, e
}
func (s *Service) SetStatus(ctx context.Context, path string, id, status, version int64) (domain.Task, error) {
	p, e := s.source(path)
	if e != nil {
		return domain.Task{}, e
	}
	return s.serial(path, func() (domain.Task, error) { return p.Store.SetTaskStatus(ctx, id, status, version) })
}

// SetTaskLifecycle performs a direct lifecycle transition without forcing the
// caller to know origin-specific status IDs or walk through intermediate
// columns. Supported actions are "complete", "cancel", and "reopen".
func (s *Service) SetTaskLifecycle(ctx context.Context, path string, id, version int64, action string) (domain.Task, error) {
	p, e := s.source(path)
	if e != nil {
		return domain.Task{}, e
	}
	statuses, e := p.Store.Statuses(ctx)
	if e != nil {
		return domain.Task{}, e
	}
	var target int64
	for _, status := range statuses {
		switch action {
		case "complete":
			if status.Kind == domain.StatusDone {
				target = status.ID
			}
		case "cancel":
			if status.Kind == domain.StatusCancelled {
				target = status.ID
			}
		case "reopen":
			if status.Kind == domain.StatusNormal && status.Initial {
				target = status.ID
			}
		default:
			return domain.Task{}, domain.ValidationError{Field: "lifecycle", Message: "acción inválida"}
		}
	}
	if target == 0 {
		return domain.Task{}, domain.ErrNotFound
	}
	return s.SetStatus(ctx, path, id, target, version)
}
func (s *Service) MoveTaskStatus(ctx context.Context, path string, id, version int64, direction int) (domain.Task, error) {
	if direction != -1 && direction != 1 {
		return domain.Task{}, domain.ValidationError{Field: "direction", Message: "must be -1 or 1"}
	}
	p, e := s.source(path)
	if e != nil {
		return domain.Task{}, e
	}
	v, _ := s.locks.LoadOrStore(path, &sync.Mutex{})
	m := v.(*sync.Mutex)
	m.Lock()
	defer m.Unlock()
	task, e := p.Store.Task(ctx, id)
	if e != nil {
		return domain.Task{}, e
	}
	if task.Version != version {
		return domain.Task{}, domain.ErrConflict
	}
	statuses, e := p.Store.Statuses(ctx)
	if e != nil {
		return domain.Task{}, e
	}
	index := -1
	for i := range statuses {
		if statuses[i].ID == task.StatusID {
			index = i
			break
		}
	}
	if index == -1 {
		return domain.Task{}, domain.ErrNotFound
	}
	target := index + direction
	if target < 0 || target >= len(statuses) {
		task.Origin = p.Origin
		return task, nil
	}
	updated, e := p.Store.SetTaskStatus(ctx, id, statuses[target].ID, version)
	updated.Origin = p.Origin
	return updated, e
}
func (s *Service) UpdateTaskTitle(ctx context.Context, path string, id, version int64, title string) (domain.Task, error) {
	return s.updateTask(ctx, path, id, version, func(task *domain.Task) { task.Title = title })
}
func (s *Service) UpdateTaskMarkdown(ctx context.Context, path string, id, version int64, markdown string) (domain.Task, error) {
	return s.updateTask(ctx, path, id, version, func(task *domain.Task) { task.Markdown = markdown })
}
func (s *Service) CycleTaskPriority(ctx context.Context, path string, id, version int64) (domain.Task, error) {
	p, e := s.source(path)
	if e != nil {
		return domain.Task{}, e
	}
	v, _ := s.locks.LoadOrStore(path, &sync.Mutex{})
	m := v.(*sync.Mutex)
	m.Lock()
	defer m.Unlock()
	task, e := p.Store.Task(ctx, id)
	if e != nil {
		return domain.Task{}, e
	}
	if task.Version != version {
		return domain.Task{}, domain.ErrConflict
	}
	priority := (task.Priority + 1) % (domain.PriorityUrgent + 1)
	updated, e := p.Store.SetTaskPriority(ctx, id, priority, version)
	updated.Origin = p.Origin
	return updated, e
}
func (s *Service) UpdateTaskDate(ctx context.Context, path string, id, version int64, field string, date *domain.Date) (domain.Task, error) {
	if field != "start" && field != "due" {
		return domain.Task{}, domain.ValidationError{Field: "date", Message: "field must be start or due"}
	}
	return s.updateTask(ctx, path, id, version, func(task *domain.Task) {
		if field == "start" {
			task.Start = date
		} else {
			task.Due = date
		}
	})
}
func (s *Service) UpdateTaskRecurrence(ctx context.Context, path string, id, version int64, recurrence *domain.Recurrence) (domain.Task, error) {
	p, e := s.source(path)
	if e != nil {
		return domain.Task{}, e
	}
	v, _ := s.locks.LoadOrStore(path, &sync.Mutex{})
	m := v.(*sync.Mutex)
	m.Lock()
	defer m.Unlock()
	task, e := p.Store.Task(ctx, id)
	if e != nil {
		return domain.Task{}, e
	}
	if task.Version != version {
		return domain.Task{}, domain.ErrConflict
	}
	if task.Recurrence == nil && recurrence != nil && !s.Capabilities(path).CanCreateRecurrence {
		return domain.Task{}, domain.ErrForbidden
	}
	if recurrence == nil {
		task.Recurrence = nil
		task.RecurrenceAnchor = nil
	} else {
		if e = recurrence.Validate(); e != nil {
			return domain.Task{}, domain.ValidationError{Field: "recurrence", Message: e.Error()}
		}
		task.Recurrence = recurrence
		if task.RecurrenceAnchor == nil {
			if s.Clock == nil {
				return domain.Task{}, domain.ValidationError{Field: "recurrence", Message: "clock is required"}
			}
			anchor := s.Clock.Today()
			task.RecurrenceAnchor = &anchor
		}
	}
	updated, e := p.Store.UpdateTask(ctx, task)
	updated.Origin = p.Origin
	return updated, e
}
func (s *Service) Task(ctx context.Context, path string, id int64) (domain.Task, error) {
	p, e := s.source(path)
	if e != nil {
		return domain.Task{}, e
	}
	task, e := p.Store.Task(ctx, id)
	task.Origin = p.Origin
	return task, e
}
func (s *Service) History(ctx context.Context, path string, taskID int64) ([]domain.HistoryEvent, error) {
	p, e := s.source(path)
	if e != nil {
		return nil, e
	}
	return p.Store.History(ctx, taskID)
}
func (s *Service) updateTask(ctx context.Context, path string, id, version int64, change func(*domain.Task)) (domain.Task, error) {
	p, e := s.source(path)
	if e != nil {
		return domain.Task{}, e
	}
	v, _ := s.locks.LoadOrStore(path, &sync.Mutex{})
	m := v.(*sync.Mutex)
	m.Lock()
	defer m.Unlock()
	task, e := p.Store.Task(ctx, id)
	if e != nil {
		return domain.Task{}, e
	}
	if task.Version != version {
		return domain.Task{}, domain.ErrConflict
	}
	change(&task)
	updated, e := p.Store.UpdateTask(ctx, task)
	updated.Origin = p.Origin
	return updated, e
}
func (s *Service) AddSubtask(ctx context.Context, path string, taskID, version int64, title string) (domain.Subtask, error) {
	if !s.Capabilities(path).CanCreateSubtask {
		return domain.Subtask{}, domain.ErrForbidden
	}
	p, e := s.source(path)
	if e != nil {
		return domain.Subtask{}, e
	}
	v, _ := s.locks.LoadOrStore(path, &sync.Mutex{})
	m := v.(*sync.Mutex)
	m.Lock()
	defer m.Unlock()
	return p.Store.AddSubtask(ctx, taskID, version, title)
}
func (s *Service) RenameSubtask(ctx context.Context, path string, taskID, id, version int64, title string) (domain.Subtask, error) {
	p, e := s.source(path)
	if e != nil {
		return domain.Subtask{}, e
	}
	v, _ := s.locks.LoadOrStore(path, &sync.Mutex{})
	m := v.(*sync.Mutex)
	m.Lock()
	defer m.Unlock()
	return p.Store.RenameSubtask(ctx, taskID, id, version, title)
}
func (s *Service) SetSubtaskStatus(ctx context.Context, path string, taskID, id, statusID, version int64) error {
	p, e := s.source(path)
	if e != nil {
		return e
	}
	return s.serialError(path, func() error { return p.Store.SetSubtaskStatus(ctx, taskID, id, statusID, version) })
}
func (s *Service) ToggleSubtask(ctx context.Context, path string, taskID, subtaskID, version int64) error {
	p, e := s.source(path)
	if e != nil {
		return e
	}
	return s.serialError(path, func() error {
		task, taskErr := p.Store.Task(ctx, taskID)
		if taskErr != nil {
			return taskErr
		}
		var current *domain.Subtask
		for i := range task.Subtasks {
			if task.Subtasks[i].ID == subtaskID {
				current = &task.Subtasks[i]
				break
			}
		}
		if current == nil {
			return domain.ErrNotFound
		}
		statuses, statusErr := p.Store.Statuses(ctx)
		if statusErr != nil {
			return statusErr
		}
		var initialID, doneID int64
		for _, status := range statuses {
			if status.Initial {
				initialID = status.ID
			}
			if status.Kind == domain.StatusDone {
				doneID = status.ID
			}
		}
		target := doneID
		if current.Status.Kind == domain.StatusDone {
			target = initialID
		}
		if target == 0 {
			return domain.ErrNotFound
		}
		return p.Store.SetSubtaskStatus(ctx, taskID, subtaskID, target, version)
	})
}
func (s *Service) MoveSubtaskStatus(ctx context.Context, path string, taskID, subtaskID, version int64, direction int) error {
	if direction != -1 && direction != 1 {
		return domain.ValidationError{Field: "direction", Message: "must be -1 or 1"}
	}
	p, e := s.source(path)
	if e != nil {
		return e
	}
	return s.serialError(path, func() error {
		task, taskErr := p.Store.Task(ctx, taskID)
		if taskErr != nil {
			return taskErr
		}
		currentStatus := int64(0)
		for _, subtask := range task.Subtasks {
			if subtask.ID == subtaskID {
				currentStatus = subtask.StatusID
				break
			}
		}
		if currentStatus == 0 {
			return domain.ErrNotFound
		}
		statuses, statusErr := p.Store.Statuses(ctx)
		if statusErr != nil {
			return statusErr
		}
		current := -1
		for index, status := range statuses {
			if status.ID == currentStatus {
				current = index
				break
			}
		}
		target := current + direction
		if current < 0 || target < 0 || target >= len(statuses) {
			return nil
		}
		return p.Store.SetSubtaskStatus(ctx, taskID, subtaskID, statuses[target].ID, version)
	})
}
func (s *Service) AddDependency(ctx context.Context, path string, taskID, dependsOn, version int64) error {
	if !s.Capabilities(path).CanCreateDependency {
		return domain.ErrForbidden
	}
	p, e := s.source(path)
	if e != nil {
		return e
	}
	return s.serialError(path, func() error { return p.Store.AddDependency(ctx, taskID, dependsOn, version) })
}
func (s *Service) RemoveDependency(ctx context.Context, path string, taskID, dependsOn, version int64) error {
	p, e := s.source(path)
	if e != nil {
		return e
	}
	return s.serialError(path, func() error { return p.Store.RemoveDependency(ctx, taskID, dependsOn, version) })
}

// DependencyCandidates returns complete origin-local choices for the TUI.
// It intentionally bypasses the active view filters so a dependency never
// becomes impossible to manage just because its task is currently hidden.
func (s *Service) DependencyCandidates(ctx context.Context, path string, taskID int64, existingOnly bool) ([]domain.Task, error) {
	p, e := s.source(path)
	if e != nil {
		return nil, e
	}
	tasks, e := p.Store.ListTasks(ctx, ports.TaskFilter{IncludeDone: true, IncludeCancelled: true, Sort: "title"})
	if e != nil {
		return nil, e
	}
	existing := make(map[int64]struct{})
	if existingOnly {
		task, taskErr := p.Store.Task(ctx, taskID)
		if taskErr != nil {
			return nil, taskErr
		}
		for _, id := range task.DependencyIDs {
			existing[id] = struct{}{}
		}
	}
	out := make([]domain.Task, 0, len(tasks))
	for _, task := range tasks {
		if task.ID == taskID {
			continue
		}
		_, isExisting := existing[task.ID]
		if existingOnly != isExisting {
			continue
		}
		task.Origin = p.Origin
		out = append(out, task)
	}
	return out, nil
}
func (s *Service) TrashTask(ctx context.Context, path string, id, version int64) (domain.Task, []int64, error) {
	p, e := s.source(path)
	if e != nil {
		return domain.Task{}, nil, e
	}
	if s.Clock == nil {
		return domain.Task{}, nil, domain.ValidationError{Field: "trash", Message: "clock is required"}
	}
	v, _ := s.locks.LoadOrStore(path, &sync.Mutex{})
	m := v.(*sync.Mutex)
	m.Lock()
	defer m.Unlock()
	affected, e := p.Store.TrashTask(ctx, id, version, s.Clock.Today())
	if e != nil {
		return domain.Task{}, nil, e
	}
	task, e := p.Store.Task(ctx, id)
	task.Origin = p.Origin
	return task, affected, e
}
func (s *Service) DependencyImpact(ctx context.Context, path string, id int64) ([]int64, error) {
	p, e := s.source(path)
	if e != nil {
		return nil, e
	}
	return p.Store.DependencyImpact(ctx, id)
}
func (s *Service) RestoreTask(ctx context.Context, path string, id, version int64) (domain.Task, error) {
	p, e := s.source(path)
	if e != nil {
		return domain.Task{}, e
	}
	return s.serial(path, func() (domain.Task, error) { return p.Store.RestoreTask(ctx, id, version) })
}
func (s *Service) serial(key string, fn func() (domain.Task, error)) (domain.Task, error) {
	v, _ := s.locks.LoadOrStore(key, &sync.Mutex{})
	m := v.(*sync.Mutex)
	m.Lock()
	defer m.Unlock()
	task, err := fn()
	if err == nil {
		if source, sourceErr := s.source(key); sourceErr == nil {
			task.Origin = source.Origin
		}
	}
	return task, err
}
func (s *Service) serialError(key string, fn func() error) error {
	v, _ := s.locks.LoadOrStore(key, &sync.Mutex{})
	m := v.(*sync.Mutex)
	m.Lock()
	defer m.Unlock()
	return fn()
}
func (s *Service) Maintain(ctx context.Context) error {
	var errs []error
	for _, p := range s.Sources {
		if p.Store != nil {
			errs = append(errs, p.Store.Maintain(ctx, s.Clock.Today()))
		}
	}
	return errors.Join(errs...)
}
func (s *Service) Close() error {
	var errs []error
	for _, p := range s.Sources {
		if p.Store != nil {
			errs = append(errs, p.Store.Close())
		}
	}
	return errors.Join(errs...)
}
func ProjectName(path string) string { return filepath.Base(path[:len(path)-len(filepath.Ext(path))]) }
