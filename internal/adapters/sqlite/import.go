package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/projectimport"
)

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

	timestamp := importedAt.UTC().Format(time.RFC3339Nano)
	taskIDs := make(map[string]int64, len(seed.Tasks))
	for _, task := range seed.Tasks {
		if err = domain.ValidateTask(task.Task); err != nil {
			return projectimport.Summary{}, err
		}
		statusID := statusIDs[task.StatusKey]
		if statusID == 0 {
			return projectimport.Summary{}, domain.ValidationError{Field: "tasks.status", Message: fmt.Sprintf("unknown status key %q", task.StatusKey)}
		}
		if task.Key == "" || taskIDs[task.Key] != 0 {
			return projectimport.Summary{}, domain.ValidationError{Field: "tasks.key", Message: "duplicate or empty task key"}
		}
		recurrence, recurrenceErr := recurrenceJSON(task.Task.Recurrence)
		if recurrenceErr != nil {
			return projectimport.Summary{}, recurrenceErr
		}
		result, insertErr := tx.ExecContext(ctx, `INSERT INTO tasks(title,status_id,priority,markdown,start_date,due_date,recurrence,recurrence_anchor,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, task.Task.Title, statusID, task.Task.Priority, task.Task.Markdown, nullableDate(task.Task.Start), nullableDate(task.Task.Due), recurrence, nullableDate(task.Task.RecurrenceAnchor), timestamp, timestamp)
		if insertErr != nil {
			return projectimport.Summary{}, fmt.Errorf("insert task %q: %w", task.Key, insertErr)
		}
		id, insertErr := result.LastInsertId()
		if insertErr != nil {
			return projectimport.Summary{}, insertErr
		}
		taskIDs[task.Key] = id
		if _, insertErr = tx.ExecContext(ctx, "INSERT INTO history(task_id,kind,detail,created_at) VALUES(?,'created','imported',?)", id, timestamp); insertErr != nil {
			return projectimport.Summary{}, insertErr
		}
	}

	for _, task := range seed.Tasks {
		taskID := taskIDs[task.Key]
		for _, subtask := range task.Subtasks {
			statusID := statusIDs[subtask.StatusKey]
			if statusID == 0 {
				return projectimport.Summary{}, domain.ValidationError{Field: "subtasks.status", Message: fmt.Sprintf("unknown status key %q", subtask.StatusKey)}
			}
			if _, err = tx.ExecContext(ctx, "INSERT INTO subtasks(task_id,title,status_id) VALUES(?,?,?)", taskID, subtask.Title, statusID); err != nil {
				return projectimport.Summary{}, err
			}
			summary.Subtasks++
		}
		for _, dependency := range task.DependsOn {
			dependencyID := taskIDs[dependency]
			if dependencyID == 0 {
				return projectimport.Summary{}, domain.ValidationError{Field: "tasks.depends_on", Message: fmt.Sprintf("unknown task key %q", dependency)}
			}
			if dependencyID == taskID {
				return projectimport.Summary{}, domain.ErrDependencyCycle
			}
			var cycle int
			if err = tx.QueryRowContext(ctx, `WITH RECURSIVE reach(id) AS (SELECT depends_on_id FROM dependencies WHERE task_id=? UNION SELECT d.depends_on_id FROM dependencies d JOIN reach r ON d.task_id=r.id) SELECT EXISTS(SELECT 1 FROM reach WHERE id=?)`, dependencyID, taskID).Scan(&cycle); err != nil {
				return projectimport.Summary{}, err
			}
			if cycle != 0 {
				return projectimport.Summary{}, domain.ErrDependencyCycle
			}
			if _, err = tx.ExecContext(ctx, "INSERT INTO dependencies(task_id,depends_on_id) VALUES(?,?)", taskID, dependencyID); err != nil {
				return projectimport.Summary{}, err
			}
			summary.Dependencies++
		}
	}

	if err = tx.Commit(); err != nil {
		return projectimport.Summary{}, err
	}
	return summary, nil
}

var _ projectimport.Importer = (*Store)(nil)
