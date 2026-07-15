package application

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"sync"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/ports"
)

type Project struct {
	Path, Name string
	Store      ports.TaskStore
	Err        error
}
type Service struct {
	Mode     domain.Mode
	Projects []Project
	Clock    ports.Clock
	locks    sync.Map
}

func (s *Service) Capabilities() domain.Capabilities { return domain.CapabilitiesFor(s.Mode) }
func (s *Service) ListTasks(ctx context.Context, f ports.TaskFilter) ([]domain.Task, error) {
	var out []domain.Task
	var errs []error
	for _, p := range s.Projects {
		if p.Err != nil {
			errs = append(errs, p.Err)
			continue
		}
		tasks, e := p.Store.ListTasks(ctx, f)
		if e != nil {
			errs = append(errs, e)
			continue
		}
		for i := range tasks {
			tasks[i].Project = p.Path
		}
		out = append(out, tasks...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			if out[i].Project == out[j].Project {
				return out[i].ID < out[j].ID
			}
			return out[i].Project < out[j].Project
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, errors.Join(errs...)
}
func (s *Service) project(path string) (Project, error) {
	if s.Mode == domain.ModeLocal && len(s.Projects) == 1 {
		return s.Projects[0], nil
	}
	for _, p := range s.Projects {
		if p.Path == path {
			return p, nil
		}
	}
	return Project{}, domain.ErrNotFound
}
func (s *Service) CreateTask(ctx context.Context, path string, t domain.Task) (domain.Task, error) {
	if !s.Capabilities().CanCreateTask {
		return t, domain.ErrForbidden
	}
	p, e := s.project(path)
	if e != nil {
		return t, e
	}
	return s.serial(path, func() (domain.Task, error) { return p.Store.CreateTask(ctx, t) })
}
func (s *Service) UpdateTask(ctx context.Context, path string, t domain.Task) (domain.Task, error) {
	p, e := s.project(path)
	if e != nil {
		return t, e
	}
	return s.serial(path, func() (domain.Task, error) { return p.Store.UpdateTask(ctx, t) })
}
func (s *Service) SetStatus(ctx context.Context, path string, id, status, version int64) (domain.Task, error) {
	p, e := s.project(path)
	if e != nil {
		return domain.Task{}, e
	}
	return s.serial(path, func() (domain.Task, error) { return p.Store.SetTaskStatus(ctx, id, status, version) })
}
func (s *Service) serial(key string, fn func() (domain.Task, error)) (domain.Task, error) {
	v, _ := s.locks.LoadOrStore(key, &sync.Mutex{})
	m := v.(*sync.Mutex)
	m.Lock()
	defer m.Unlock()
	return fn()
}
func (s *Service) Maintain(ctx context.Context) error {
	var errs []error
	for _, p := range s.Projects {
		if p.Store != nil {
			errs = append(errs, p.Store.Maintain(ctx, s.Clock.Today()))
		}
	}
	return errors.Join(errs...)
}
func (s *Service) Close() error {
	var errs []error
	for _, p := range s.Projects {
		if p.Store != nil {
			errs = append(errs, p.Store.Close())
		}
	}
	return errors.Join(errs...)
}
func ProjectName(path string) string { return filepath.Base(path[:len(path)-len(filepath.Ext(path))]) }
