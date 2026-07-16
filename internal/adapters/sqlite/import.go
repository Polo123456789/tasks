package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/projectimport"
)

func (s *Store) AddTasks(ctx context.Context, seed projectimport.ProjectSeed, addedAt time.Time) (projectimport.AddResult, error) {
	if len(seed.Statuses) == 0 {
		return projectimport.AddResult{}, domain.ValidationError{Field: "statuses", Message: "at least one normal status is required"}
	}
	if len(seed.Tasks) == 0 {
		return projectimport.AddResult{}, domain.ValidationError{Field: "tasks", Message: "at least one task is required"}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return projectimport.AddResult{}, err
	}
	defer tx.Rollback()

	statusIDs, err := resolveExistingStatuses(ctx, tx, seed.Statuses)
	if err != nil {
		return projectimport.AddResult{}, err
	}
	result, err := insertTaskBatch(ctx, tx, seed.Tasks, statusIDs, addedAt, "added")
	if err != nil {
		return projectimport.AddResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return projectimport.AddResult{}, err
	}
	return result, nil
}

type existingStatus struct {
	id      int64
	initial bool
}

func resolveExistingStatuses(ctx context.Context, tx *sql.Tx, statuses []projectimport.StatusSeed) (map[string]int64, error) {
	rows, err := tx.QueryContext(ctx, "SELECT id,name,kind,is_initial FROM statuses")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byName := make(map[string]existingStatus)
	statusIDs := make(map[string]int64, len(statuses)+2)
	for rows.Next() {
		var id int64
		var name string
		var kind domain.StatusKind
		var initial bool
		if err = rows.Scan(&id, &name, &kind, &initial); err != nil {
			return nil, err
		}
		if kind == domain.StatusNormal {
			byName[name] = existingStatus{id: id, initial: initial}
		} else {
			statusIDs[string(kind)] = id
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if statusIDs[projectimport.StatusDone] == 0 || statusIDs[projectimport.StatusCancelled] == 0 {
		return nil, fmt.Errorf("project is missing special statuses")
	}
	initialCount := 0
	for index, status := range statuses {
		if status.Key == "" || statusIDs[status.Key] != 0 {
			return nil, domain.ValidationError{Field: fmt.Sprintf("statuses[%d].key", index), Message: "duplicate, reserved, or empty status key"}
		}
		stored, ok := byName[status.Name]
		if !ok {
			return nil, domain.ValidationError{Field: fmt.Sprintf("statuses[%d].name", index), Message: fmt.Sprintf("status %q does not exist in destination", status.Name)}
		}
		if status.Initial {
			initialCount++
			if !stored.initial {
				return nil, domain.ValidationError{Field: fmt.Sprintf("statuses[%d].initial", index), Message: fmt.Sprintf("status %q is not the destination's initial status", status.Name)}
			}
		}
		statusIDs[status.Key] = stored.id
	}
	if initialCount != 1 {
		return nil, domain.ValidationError{Field: "statuses", Message: "exactly one status must be initial"}
	}
	return statusIDs, nil
}

func (s *Store) ImportProject(ctx context.Context, seed projectimport.ProjectSeed, importedAt time.Time) (projectimport.Summary, error) {
	summary := projectimport.Summary{Statuses: len(seed.Statuses), Tasks: len(seed.Tasks)}
	if len(seed.Statuses) == 0 {
		return projectimport.Summary{}, domain.ValidationError{Field: "statuses", Message: "at least one normal status is required"}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return projectimport.Summary{}, err
	}
	defer tx.Rollback()

	var existingTasks int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM tasks").Scan(&existingTasks); err != nil {
		return projectimport.Summary{}, err
	}
	if existingTasks != 0 {
		return projectimport.Summary{}, domain.ValidationError{Field: "project", Message: "import requires an empty project"}
	}

	statusIDs := make(map[string]int64, len(seed.Statuses)+2)
	rows, err := tx.QueryContext(ctx, "SELECT id,kind FROM statuses WHERE kind IN ('done','cancelled')")
	if err != nil {
		return projectimport.Summary{}, err
	}
	for rows.Next() {
		var id int64
		var kind string
		if err = rows.Scan(&id, &kind); err != nil {
			rows.Close()
			return projectimport.Summary{}, err
		}
		statusIDs[kind] = id
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return projectimport.Summary{}, err
	}
	if err = rows.Close(); err != nil {
		return projectimport.Summary{}, err
	}
	if statusIDs[projectimport.StatusDone] == 0 || statusIDs[projectimport.StatusCancelled] == 0 {
		return projectimport.Summary{}, fmt.Errorf("project is missing special statuses")
	}
	if _, err = tx.ExecContext(ctx, "UPDATE statuses SET is_initial=0 WHERE kind='normal'"); err != nil {
		return projectimport.Summary{}, err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM statuses WHERE kind='normal'"); err != nil {
		return projectimport.Summary{}, err
	}
	initialCount := 0
	for index, status := range seed.Statuses {
		if status.Initial {
			initialCount++
		}
		result, insertErr := tx.ExecContext(ctx, "INSERT INTO statuses(name,kind,position,is_initial) VALUES(?,'normal',?,?)", status.Name, index+1, status.Initial)
		if insertErr != nil {
			return projectimport.Summary{}, fmt.Errorf("insert status %q: %w", status.Key, insertErr)
		}
		id, insertErr := result.LastInsertId()
		if insertErr != nil {
			return projectimport.Summary{}, insertErr
		}
		if status.Key == "" || statusIDs[status.Key] != 0 {
			return projectimport.Summary{}, domain.ValidationError{Field: "statuses", Message: "duplicate or empty status key"}
		}
		statusIDs[status.Key] = id
	}
	if initialCount != 1 {
		return projectimport.Summary{}, domain.ValidationError{Field: "statuses", Message: "exactly one status must be initial"}
	}

	result, err := insertTaskBatch(ctx, tx, seed.Tasks, statusIDs, importedAt, "imported")
	if err != nil {
		return projectimport.Summary{}, err
	}
	summary.Subtasks = result.Summary.Subtasks
	summary.Dependencies = result.Summary.Dependencies
	if err = tx.Commit(); err != nil {
		return projectimport.Summary{}, err
	}
	return summary, nil
}

func insertTaskBatch(ctx context.Context, tx *sql.Tx, tasks []projectimport.TaskSeed, statusIDs map[string]int64, at time.Time, historyDetail string) (projectimport.AddResult, error) {
	result := projectimport.AddResult{
		Summary: projectimport.Summary{Tasks: len(tasks)},
		Tasks:   make([]projectimport.CreatedTask, 0, len(tasks)),
	}
	timestamp := at.UTC().Format(time.RFC3339Nano)
	taskIDs := make(map[string]int64, len(tasks))
	for _, task := range tasks {
		if err := domain.ValidateTask(task.Task); err != nil {
			return projectimport.AddResult{}, err
		}
		statusID := statusIDs[task.StatusKey]
		if statusID == 0 {
			return projectimport.AddResult{}, domain.ValidationError{Field: "tasks.status", Message: fmt.Sprintf("unknown status key %q", task.StatusKey)}
		}
		if task.Key == "" || taskIDs[task.Key] != 0 {
			return projectimport.AddResult{}, domain.ValidationError{Field: "tasks.key", Message: "duplicate or empty task key"}
		}
		recurrence, err := recurrenceJSON(task.Task.Recurrence)
		if err != nil {
			return projectimport.AddResult{}, err
		}
		inserted, err := tx.ExecContext(ctx, `INSERT INTO tasks(title,status_id,priority,markdown,start_date,due_date,recurrence,recurrence_anchor,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, task.Task.Title, statusID, task.Task.Priority, task.Task.Markdown, nullableDate(task.Task.Start), nullableDate(task.Task.Due), recurrence, nullableDate(task.Task.RecurrenceAnchor), timestamp, timestamp)
		if err != nil {
			return projectimport.AddResult{}, fmt.Errorf("insert task %q: %w", task.Key, err)
		}
		id, err := inserted.LastInsertId()
		if err != nil {
			return projectimport.AddResult{}, err
		}
		taskIDs[task.Key] = id
		result.Tasks = append(result.Tasks, projectimport.CreatedTask{Key: task.Key, ID: id})
		if _, err = tx.ExecContext(ctx, "INSERT INTO history(task_id,kind,detail,created_at) VALUES(?,'created',?,?)", id, historyDetail, timestamp); err != nil {
			return projectimport.AddResult{}, err
		}
	}

	for _, task := range tasks {
		taskID := taskIDs[task.Key]
		for _, subtask := range task.Subtasks {
			statusID := statusIDs[subtask.StatusKey]
			if statusID == 0 {
				return projectimport.AddResult{}, domain.ValidationError{Field: "subtasks.status", Message: fmt.Sprintf("unknown status key %q", subtask.StatusKey)}
			}
			if _, err := tx.ExecContext(ctx, "INSERT INTO subtasks(task_id,title,status_id) VALUES(?,?,?)", taskID, subtask.Title, statusID); err != nil {
				return projectimport.AddResult{}, err
			}
			result.Summary.Subtasks++
		}
		for _, dependency := range task.DependsOn {
			dependencyID := taskIDs[dependency]
			if dependencyID == 0 {
				return projectimport.AddResult{}, domain.ValidationError{Field: "tasks.depends_on", Message: fmt.Sprintf("unknown task key %q", dependency)}
			}
			if dependencyID == taskID {
				return projectimport.AddResult{}, domain.ErrDependencyCycle
			}
			var cycle int
			if err := tx.QueryRowContext(ctx, `WITH RECURSIVE reach(id) AS (SELECT depends_on_id FROM dependencies WHERE task_id=? UNION SELECT d.depends_on_id FROM dependencies d JOIN reach r ON d.task_id=r.id) SELECT EXISTS(SELECT 1 FROM reach WHERE id=?)`, dependencyID, taskID).Scan(&cycle); err != nil {
				return projectimport.AddResult{}, err
			}
			if cycle != 0 {
				return projectimport.AddResult{}, domain.ErrDependencyCycle
			}
			if _, err := tx.ExecContext(ctx, "INSERT INTO dependencies(task_id,depends_on_id) VALUES(?,?)", taskID, dependencyID); err != nil {
				return projectimport.AddResult{}, err
			}
			result.Summary.Dependencies++
		}
	}
	return result, nil
}

var _ projectimport.Importer = (*Store)(nil)
var _ projectimport.Appender = (*Store)(nil)
