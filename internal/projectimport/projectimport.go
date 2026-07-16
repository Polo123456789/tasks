package projectimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Polo123456789/tasks/internal/domain"
)

const (
	Format  = "tasks-project"
	Version = 1

	StatusDone      = "done"
	StatusCancelled = "cancelled"
)

type Document struct {
	Format   string       `json:"format"`
	Version  int          `json:"version"`
	Statuses []StatusSpec `json:"statuses"`
	Tasks    []TaskSpec   `json:"tasks"`
}

type StatusSpec struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Initial bool   `json:"initial,omitempty"`
}

type TaskSpec struct {
	Key        string        `json:"key"`
	Title      string        `json:"title"`
	Status     string        `json:"status,omitempty"`
	Priority   string        `json:"priority,omitempty"`
	Markdown   string        `json:"markdown,omitempty"`
	Start      *string       `json:"start,omitempty"`
	Due        *string       `json:"due,omitempty"`
	Recurrence *string       `json:"recurrence,omitempty"`
	Subtasks   []SubtaskSpec `json:"subtasks,omitempty"`
	DependsOn  []string      `json:"depends_on,omitempty"`
}

type SubtaskSpec struct {
	Title  string `json:"title"`
	Status string `json:"status,omitempty"`
}

type ProjectSeed struct {
	Statuses []StatusSeed
	Tasks    []TaskSeed
}

type StatusSeed struct {
	Key     string
	Name    string
	Initial bool
}

type TaskSeed struct {
	Key       string
	Task      domain.Task
	StatusKey string
	Subtasks  []SubtaskSeed
	DependsOn []string
}

type SubtaskSeed struct {
	Title     string
	StatusKey string
}

type Summary struct {
	Statuses     int
	Tasks        int
	Subtasks     int
	Dependencies int
}

// Importer is the bootstrap-only persistence port used to populate a new
// project. It intentionally remains separate from the interactive TaskStore.
type Importer interface {
	ImportProject(context.Context, ProjectSeed, time.Time) (Summary, error)
}

func Decode(r io.Reader) (Document, error) {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	var document Document
	if err := decoder.Decode(&document); err != nil {
		return Document{}, fmt.Errorf("decode project JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Document{}, fmt.Errorf("decode project JSON: expected exactly one JSON object")
		}
		return Document{}, fmt.Errorf("decode project JSON: trailing content: %w", err)
	}
	return document, nil
}

func Normalize(document Document, today domain.Date) (ProjectSeed, error) {
	if document.Format != Format {
		return ProjectSeed{}, validation("format", fmt.Sprintf("must be %q", Format))
	}
	if document.Version != Version {
		return ProjectSeed{}, validation("version", fmt.Sprintf("unsupported version %d; expected %d", document.Version, Version))
	}
	if len(document.Statuses) == 0 {
		return ProjectSeed{}, validation("statuses", "at least one normal status is required")
	}

	seed := ProjectSeed{Statuses: make([]StatusSeed, 0, len(document.Statuses)), Tasks: make([]TaskSeed, 0, len(document.Tasks))}
	statusKeys := map[string]struct{}{StatusDone: {}, StatusCancelled: {}}
	statusNames := map[string]struct{}{
		strings.ToLower("Finalizada"): {},
		strings.ToLower("Cancelada"):  {},
	}
	initialKey := ""
	for index, status := range document.Statuses {
		path := fmt.Sprintf("statuses[%d]", index)
		status.Key = strings.TrimSpace(status.Key)
		status.Name = strings.TrimSpace(status.Name)
		if status.Key == "" {
			return ProjectSeed{}, validation(path+".key", "required")
		}
		if status.Name == "" {
			return ProjectSeed{}, validation(path+".name", "required")
		}
		if _, exists := statusKeys[status.Key]; exists {
			return ProjectSeed{}, validation(path+".key", "duplicate or reserved status key")
		}
		nameKey := strings.ToLower(status.Name)
		if _, exists := statusNames[nameKey]; exists {
			return ProjectSeed{}, validation(path+".name", "duplicate or reserved status name")
		}
		statusKeys[status.Key] = struct{}{}
		statusNames[nameKey] = struct{}{}
		if status.Initial {
			if initialKey != "" {
				return ProjectSeed{}, validation("statuses", "exactly one status must be initial")
			}
			initialKey = status.Key
		}
		seed.Statuses = append(seed.Statuses, StatusSeed{Key: status.Key, Name: status.Name, Initial: status.Initial})
	}
	if initialKey == "" {
		return ProjectSeed{}, validation("statuses", "exactly one status must be initial")
	}

	taskIndexes := make(map[string]int, len(document.Tasks))
	for index, task := range document.Tasks {
		path := fmt.Sprintf("tasks[%d]", index)
		task.Key = strings.TrimSpace(task.Key)
		task.Title = strings.TrimSpace(task.Title)
		if task.Key == "" {
			return ProjectSeed{}, validation(path+".key", "required")
		}
		if _, exists := taskIndexes[task.Key]; exists {
			return ProjectSeed{}, validation(path+".key", "duplicate task key")
		}
		if task.Title == "" {
			return ProjectSeed{}, validation(path+".title", "required")
		}
		taskIndexes[task.Key] = index

		statusKey := strings.TrimSpace(task.Status)
		if statusKey == "" {
			statusKey = initialKey
		}
		if _, exists := statusKeys[statusKey]; !exists {
			return ProjectSeed{}, validation(path+".status", fmt.Sprintf("unknown status key %q", statusKey))
		}
		priority, err := parsePriority(task.Priority)
		if err != nil {
			return ProjectSeed{}, validation(path+".priority", err.Error())
		}
		start, err := parseOptionalDate(task.Start)
		if err != nil {
			return ProjectSeed{}, validation(path+".start", err.Error())
		}
		due, err := parseOptionalDate(task.Due)
		if err != nil {
			return ProjectSeed{}, validation(path+".due", err.Error())
		}
		var recurrence *domain.Recurrence
		var anchor *domain.Date
		if task.Recurrence != nil {
			value := strings.TrimSpace(*task.Recurrence)
			if value == "" {
				return ProjectSeed{}, validation(path+".recurrence", "must not be empty")
			}
			parsed, parseErr := domain.ParseRecurrence(value)
			if parseErr != nil {
				return ProjectSeed{}, validation(path+".recurrence", parseErr.Error())
			}
			recurrence = &parsed
			anchorValue := today
			anchor = &anchorValue
		}
		domainTask := domain.Task{Title: task.Title, Priority: priority, Markdown: task.Markdown, Start: start, Due: due, Recurrence: recurrence, RecurrenceAnchor: anchor}
		if err = domain.ValidateTask(domainTask); err != nil {
			return ProjectSeed{}, validation(path, err.Error())
		}

		subtasks := make([]SubtaskSeed, 0, len(task.Subtasks))
		for subtaskIndex, subtask := range task.Subtasks {
			subtaskPath := fmt.Sprintf("%s.subtasks[%d]", path, subtaskIndex)
			title := strings.TrimSpace(subtask.Title)
			if title == "" {
				return ProjectSeed{}, validation(subtaskPath+".title", "required")
			}
			subtaskStatus := strings.TrimSpace(subtask.Status)
			if subtaskStatus == "" {
				subtaskStatus = initialKey
			}
			if _, exists := statusKeys[subtaskStatus]; !exists {
				return ProjectSeed{}, validation(subtaskPath+".status", fmt.Sprintf("unknown status key %q", subtaskStatus))
			}
			subtasks = append(subtasks, SubtaskSeed{Title: title, StatusKey: subtaskStatus})
		}
		if statusKey == StatusDone || statusKey == StatusCancelled {
			for subtaskIndex := range subtasks {
				subtasks[subtaskIndex].StatusKey = statusKey
			}
		} else if len(subtasks) >= 2 {
			allDone := true
			for _, subtask := range subtasks {
				allDone = allDone && subtask.StatusKey == StatusDone
			}
			if allDone {
				statusKey = StatusDone
			}
		}

		dependencies := make([]string, 0, len(task.DependsOn))
		seenDependencies := make(map[string]struct{}, len(task.DependsOn))
		for dependencyIndex, dependency := range task.DependsOn {
			dependency = strings.TrimSpace(dependency)
			if dependency == "" {
				return ProjectSeed{}, validation(fmt.Sprintf("%s.depends_on[%d]", path, dependencyIndex), "required")
			}
			if _, duplicate := seenDependencies[dependency]; duplicate {
				return ProjectSeed{}, validation(path+".depends_on", fmt.Sprintf("duplicate dependency %q", dependency))
			}
			seenDependencies[dependency] = struct{}{}
			dependencies = append(dependencies, dependency)
		}
		seed.Tasks = append(seed.Tasks, TaskSeed{Key: task.Key, Task: domainTask, StatusKey: statusKey, Subtasks: subtasks, DependsOn: dependencies})
	}

	for index, task := range seed.Tasks {
		for dependencyIndex, dependency := range task.DependsOn {
			if dependency == task.Key {
				return ProjectSeed{}, validation(fmt.Sprintf("tasks[%d].depends_on[%d]", index, dependencyIndex), "task cannot depend on itself")
			}
			if _, exists := taskIndexes[dependency]; !exists {
				return ProjectSeed{}, validation(fmt.Sprintf("tasks[%d].depends_on[%d]", index, dependencyIndex), fmt.Sprintf("unknown task key %q", dependency))
			}
		}
	}
	if err := validateAcyclic(seed.Tasks); err != nil {
		return ProjectSeed{}, err
	}
	return seed, nil
}

func parsePriority(value string) (domain.Priority, error) {
	switch strings.TrimSpace(value) {
	case "", "none":
		return domain.PriorityNone, nil
	case "low":
		return domain.PriorityLow, nil
	case "medium":
		return domain.PriorityMedium, nil
	case "high":
		return domain.PriorityHigh, nil
	case "urgent":
		return domain.PriorityUrgent, nil
	default:
		return domain.PriorityNone, fmt.Errorf("must be one of none, low, medium, high, urgent")
	}
}

func parseOptionalDate(value *string) (*domain.Date, error) {
	if value == nil {
		return nil, nil
	}
	if strings.TrimSpace(*value) == "" {
		return nil, fmt.Errorf("must not be empty")
	}
	date, err := domain.ParseDate(*value)
	if err != nil {
		return nil, err
	}
	return &date, nil
}

func validateAcyclic(tasks []TaskSeed) error {
	edges := make(map[string][]string, len(tasks))
	for _, task := range tasks {
		edges[task.Key] = task.DependsOn
	}
	const (
		unvisited = iota
		visiting
		visited
	)
	states := make(map[string]int, len(tasks))
	var visit func(string) error
	visit = func(key string) error {
		switch states[key] {
		case visiting:
			return validation("tasks.depends_on", fmt.Sprintf("dependency cycle involving %q", key))
		case visited:
			return nil
		}
		states[key] = visiting
		for _, dependency := range edges[key] {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		states[key] = visited
		return nil
	}
	for _, task := range tasks {
		if err := visit(task.Key); err != nil {
			return err
		}
	}
	return nil
}

func validation(field, message string) error {
	return domain.ValidationError{Field: field, Message: message}
}

func Prompt(today domain.Date) string {
	return fmt.Sprintf(`Convierte el timeline y la planificación del proyecto que hemos estado hablando a un único objeto JSON para importarlo en la aplicación local "tasks".

La fecha actual es %s. Convierte fechas relativas solo cuando la conversación permita resolverlas con claridad; si una fecha no fue acordada, omítela. No inventes tareas, fechas ni decisiones.

Responde únicamente con JSON puro: sin explicación, sin bloque Markdown y sin texto antes o después.

Contrato:
- format debe ser "tasks-project" y version debe ser 1.
- statuses contiene los estados normales en el orden del Kanban. Cada estado tiene key portable, name visible y exactamente uno tiene initial=true. Las claves "done" y "cancelled" están reservadas para los estados especiales y no se declaran en statuses.
- tasks puede estar vacío. Cada tarea requiere key única y title. status referencia una key normal, "done" o "cancelled"; si se omite usa el estado inicial.
- priority admite none, low, medium, high o urgent; si se omite usa none.
- markdown contiene las notas y decisiones relevantes. No existe un campo description separado.
- start y due usan YYYY-MM-DD. due no puede ser anterior a start.
- recurrence, si existe, reemplaza las fechas y usa una de estas formas: daily, weekly:mon,thu, monthly:15, month-end, monthly-weekday:first:mon o monthly-weekday:last:fri.
- subtasks solo admite un nivel; cada elemento tiene title y status opcional.
- Una tarea con status done o cancelled propaga ese estado a sus subtareas; si una tarea normal tiene dos o más subtareas y todas están done, la tarea principal también queda done.
- depends_on contiene keys de otras tareas del mismo documento. No crees autorreferencias ni ciclos.
- Omite campos sin valor. No incluyas IDs internos, timestamps, historial, papelera ni valores calculados.

Ejemplo válido:
{
  "format": "tasks-project",
  "version": 1,
  "statuses": [
    {"key": "pending", "name": "Pendiente", "initial": true},
    {"key": "in_progress", "name": "En progreso"},
    {"key": "blocked", "name": "Bloqueada"}
  ],
  "tasks": [
    {
      "key": "define-scope",
      "title": "Definir alcance",
      "status": "done",
      "priority": "high",
      "markdown": "Decisiones y contexto del proyecto.",
      "due": "%s"
    },
    {
      "key": "implementation",
      "title": "Implementar primera versión",
      "status": "pending",
      "priority": "medium",
      "subtasks": [
        {"title": "Implementar parser", "status": "pending"}
      ],
      "depends_on": ["define-scope"]
    }
  ]
}
`, today.String(), today.String())
}
