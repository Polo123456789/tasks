package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Polo123456789/tasks/internal/domain"
)

func (s *Store) TrashTask(ctx context.Context, id, version int64, today domain.Date) ([]int64, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, "SELECT task_id FROM dependencies WHERE depends_on_id=? UNION SELECT depends_on_id FROM dependencies WHERE task_id=? ORDER BY 1", id, id)
	if e != nil {
		return nil, e
	}
	var affected []int64
	for rows.Next() {
		var x int64
		if e = rows.Scan(&x); e != nil {
			rows.Close()
			return nil, e
		}
		affected = append(affected, x)
	}
	if e = rows.Close(); e != nil {
		return nil, e
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	r, e := tx.ExecContext(ctx, "UPDATE tasks SET deleted_at=?,version=version+1,updated_at=? WHERE id=? AND version=? AND deleted_at IS NULL", today.String(), now, id, version)
	if e != nil {
		return nil, e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return nil, e
	}
	if n == 0 {
		return nil, domain.ErrConflict
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO history(task_id,kind,detail,created_at) SELECT task_id,'dependency_removed',?,? FROM dependencies WHERE depends_on_id=?", fmt.Sprint(id), now, id); e != nil {
		return nil, e
	}
	if _, e = tx.ExecContext(ctx, "UPDATE tasks SET version=version+1,updated_at=? WHERE id IN (SELECT task_id FROM dependencies WHERE depends_on_id=?)", now, id); e != nil {
		return nil, e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM dependencies WHERE task_id=? OR depends_on_id=?", id, id); e != nil {
		return nil, e
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO history(task_id,kind,detail) VALUES(?,'trashed',?)", id, fmt.Sprintf("removed %d dependency relations", len(affected)))
	if e != nil {
		return nil, e
	}
	return affected, tx.Commit()
}
func (s *Store) DependencyImpact(ctx context.Context, id int64) ([]int64, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT task_id FROM dependencies WHERE depends_on_id=? UNION SELECT depends_on_id FROM dependencies WHERE task_id=? ORDER BY 1", id, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var affected []int64
	for rows.Next() {
		var taskID int64
		if e = rows.Scan(&taskID); e != nil {
			return nil, e
		}
		affected = append(affected, taskID)
	}
	return affected, rows.Err()
}
func (s *Store) RestoreTask(ctx context.Context, id, version int64) (domain.Task, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return domain.Task{}, e
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	r, e := tx.ExecContext(ctx, "UPDATE tasks SET deleted_at=NULL,version=version+1,updated_at=? WHERE id=? AND version=? AND deleted_at IS NOT NULL", now, id, version)
	if e != nil {
		return domain.Task{}, e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return domain.Task{}, e
	}
	if n == 0 {
		return domain.Task{}, domain.ErrConflict
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO history(task_id,kind,created_at) VALUES(?,'restored',?)", id, now); e != nil {
		return domain.Task{}, e
	}
	if e = tx.Commit(); e != nil {
		return domain.Task{}, e
	}
	return s.Task(ctx, id)
}
func (s *Store) AddSubtask(ctx context.Context, taskID, version int64, title string) (domain.Subtask, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return domain.Subtask{}, domain.ValidationError{Field: "title", Message: "required"}
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return domain.Subtask{}, e
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if e = bumpTaskVersion(ctx, tx, taskID, version, now); e != nil {
		return domain.Subtask{}, e
	}
	var status int64
	if e = tx.QueryRowContext(ctx, "SELECT id FROM statuses WHERE is_initial=1").Scan(&status); e != nil {
		return domain.Subtask{}, e
	}
	r, e := tx.ExecContext(ctx, "INSERT INTO subtasks(task_id,title,status_id) VALUES(?,?,?)", taskID, title, status)
	if e != nil {
		return domain.Subtask{}, e
	}
	id, e := r.LastInsertId()
	if e != nil {
		return domain.Subtask{}, e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO history(task_id,kind,detail) VALUES(?,'subtask_created',?)", taskID, title); e != nil {
		return domain.Subtask{}, e
	}
	if e = tx.Commit(); e != nil {
		return domain.Subtask{}, e
	}
	return domain.Subtask{ID: id, TaskID: taskID, Title: title, StatusID: status}, nil
}
func (s *Store) RenameSubtask(ctx context.Context, taskID, id, version int64, title string) (domain.Subtask, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return domain.Subtask{}, domain.ValidationError{Field: "title", Message: "required"}
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return domain.Subtask{}, e
	}
	defer tx.Rollback()
	var sub domain.Subtask
	if e = tx.QueryRowContext(ctx, "SELECT st.id,st.task_id,st.title,st.status_id FROM subtasks st JOIN tasks t ON t.id=st.task_id WHERE st.id=? AND st.task_id=? AND t.deleted_at IS NULL", id, taskID).Scan(&sub.ID, &sub.TaskID, &sub.Title, &sub.StatusID); errors.Is(e, sql.ErrNoRows) {
		return domain.Subtask{}, domain.ErrNotFound
	}
	if e != nil {
		return domain.Subtask{}, e
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if e = bumpTaskVersion(ctx, tx, taskID, version, now); e != nil {
		return domain.Subtask{}, e
	}
	if _, e = tx.ExecContext(ctx, "UPDATE subtasks SET title=? WHERE id=?", title, id); e != nil {
		return domain.Subtask{}, e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO history(task_id,kind,detail) VALUES(?,'subtask_edited',?)", sub.TaskID, title); e != nil {
		return domain.Subtask{}, e
	}
	if e = tx.Commit(); e != nil {
		return domain.Subtask{}, e
	}
	sub.Title = title
	return sub, nil
}
func (s *Store) SetSubtaskStatus(ctx context.Context, taskID, id, statusID, version int64) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var storedTaskID int64
	if e = tx.QueryRowContext(ctx, "SELECT st.task_id FROM subtasks st JOIN tasks t ON t.id=st.task_id WHERE st.id=? AND st.task_id=? AND t.deleted_at IS NULL", id, taskID).Scan(&storedTaskID); errors.Is(e, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	if e != nil {
		return e
	}
	if storedTaskID != taskID {
		return domain.ErrNotFound
	}
	var statusKind string
	if e = tx.QueryRowContext(ctx, "SELECT kind FROM statuses WHERE id=?", statusID).Scan(&statusKind); errors.Is(e, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	if e != nil {
		return e
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if e = bumpTaskVersion(ctx, tx, taskID, version, now); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "UPDATE subtasks SET status_id=? WHERE id=?", statusID, id); e != nil {
		return e
	}
	var total, done int
	if e = tx.QueryRowContext(ctx, "SELECT count(*),COALESCE(sum(CASE WHEN s.kind='done' THEN 1 ELSE 0 END),0) FROM subtasks st JOIN statuses s ON s.id=st.status_id WHERE task_id=?", taskID).Scan(&total, &done); e != nil {
		return e
	}
	if total >= 2 && done == total {
		r, updateErr := tx.ExecContext(ctx, "UPDATE tasks SET status_id=(SELECT id FROM statuses WHERE kind='done') WHERE id=? AND status_id<>(SELECT id FROM statuses WHERE kind='done')", taskID)
		e = updateErr
		if e != nil {
			return e
		}
		changed, rowsErr := r.RowsAffected()
		if rowsErr != nil {
			return rowsErr
		}
		if changed == 1 {
			if _, e = tx.ExecContext(ctx, "INSERT INTO history(task_id,kind,detail) VALUES(?,'completed','all subtasks completed')", taskID); e != nil {
				return e
			}
		}
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO history(task_id,kind,detail) VALUES(?,'subtask_changed',?)", taskID, statusKind); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) AddDependency(ctx context.Context, taskID, dependsOn, version int64) error {
	if taskID == dependsOn {
		return domain.ErrDependencyCycle
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var count int
	if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM tasks WHERE id IN (?,?) AND deleted_at IS NULL", taskID, dependsOn).Scan(&count); e != nil {
		return e
	}
	if count != 2 {
		return domain.ErrNotFound
	}
	var cycle int
	e = tx.QueryRowContext(ctx, `WITH RECURSIVE reach(id) AS (SELECT depends_on_id FROM dependencies WHERE task_id=? UNION SELECT d.depends_on_id FROM dependencies d JOIN reach r ON d.task_id=r.id) SELECT EXISTS(SELECT 1 FROM reach WHERE id=?)`, dependsOn, taskID).Scan(&cycle)
	if e != nil {
		return e
	}
	if cycle != 0 {
		return domain.ErrDependencyCycle
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if e = bumpTaskVersion(ctx, tx, taskID, version, now); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO dependencies(task_id,depends_on_id) VALUES(?,?)", taskID, dependsOn); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO history(task_id,kind,detail) VALUES(?,'dependency_added',?)", taskID, fmt.Sprint(dependsOn)); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) RemoveDependency(ctx context.Context, taskID, dependsOn, version int64) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var exists int
	if e = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM dependencies WHERE task_id=? AND depends_on_id=?)", taskID, dependsOn).Scan(&exists); e != nil {
		return e
	}
	if exists == 0 {
		return domain.ErrNotFound
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if e = bumpTaskVersion(ctx, tx, taskID, version, now); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM dependencies WHERE task_id=? AND depends_on_id=?", taskID, dependsOn); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO history(task_id,kind,detail) VALUES(?,'dependency_removed',?)", taskID, fmt.Sprint(dependsOn)); e != nil {
		return e
	}
	return tx.Commit()
}

func bumpTaskVersion(ctx context.Context, tx *sql.Tx, taskID, version int64, now string) error {
	result, err := tx.ExecContext(ctx, "UPDATE tasks SET version=version+1,updated_at=? WHERE id=? AND version=? AND deleted_at IS NULL", now, taskID, version)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 1 {
		return nil
	}
	var exists int
	if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM tasks WHERE id=? AND deleted_at IS NULL)", taskID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return domain.ErrNotFound
	}
	return domain.ErrConflict
}
func (s *Store) History(ctx context.Context, taskID int64) ([]domain.HistoryEvent, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT id,task_id,kind,detail,created_at FROM history WHERE task_id=? ORDER BY id DESC", taskID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.HistoryEvent
	for rows.Next() {
		var v domain.HistoryEvent
		var ts string
		if e = rows.Scan(&v.ID, &v.TaskID, &v.Kind, &v.Detail, &ts); e != nil {
			return nil, e
		}
		v.CreatedAt, e = time.Parse(time.RFC3339Nano, ts)
		if e != nil {
			return nil, fmt.Errorf("history event %d created_at: %w", v.ID, e)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) Maintain(ctx context.Context, today domain.Date) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	cutoff := today.AddDays(-30).String()
	if _, e = tx.ExecContext(ctx, "DELETE FROM tasks WHERE deleted_at IS NOT NULL AND deleted_at<=?", cutoff); e != nil {
		return e
	}
	rows, e := tx.QueryContext(ctx, "SELECT t.id,t.recurrence,t.recurrence_anchor,s.kind FROM tasks t JOIN statuses s ON s.id=t.status_id WHERE t.recurrence IS NOT NULL AND t.recurrence_anchor IS NOT NULL AND t.deleted_at IS NULL")
	if e != nil {
		return e
	}
	defer rows.Close()
	type reset struct {
		id          int64
		anchor      domain.Date
		skipped     int
		wasComplete bool
	}
	var resets []reset
	for rows.Next() {
		var id int64
		var raw, anchor, statusKind string
		if e = rows.Scan(&id, &raw, &anchor, &statusKind); e != nil {
			return e
		}
		var rec domain.Recurrence
		if e = json.Unmarshal([]byte(raw), &rec); e != nil {
			return fmt.Errorf("task %d recurrence: %w", id, e)
		}
		if e = rec.Validate(); e != nil {
			return fmt.Errorf("task %d recurrence: %w", id, e)
		}
		a, parseErr := domain.ParseDate(anchor)
		if parseErr != nil {
			return fmt.Errorf("task %d recurrence anchor: %w", id, parseErr)
		}
		next, nextErr := rec.Next(a)
		if nextErr != nil {
			return fmt.Errorf("task %d recurrence: %w", id, nextErr)
		}
		skipped := 0
		for !next.After(today) {
			a = next
			skipped++
			next, nextErr = rec.Next(a)
			if nextErr != nil {
				return fmt.Errorf("task %d recurrence: %w", id, nextErr)
			}
		}
		if skipped > 0 {
			resets = append(resets, reset{id: id, anchor: a, skipped: skipped - 1, wasComplete: statusKind == string(domain.StatusDone)})
		}
	}
	if e = rows.Err(); e != nil {
		return e
	}
	if e = rows.Close(); e != nil {
		return e
	}
	for _, r := range resets {
		cycleKind := "recurrence_cycle_incomplete"
		if r.wasComplete {
			cycleKind = "recurrence_cycle_completed"
		}
		_, e = tx.ExecContext(ctx, "INSERT INTO history(task_id,kind,detail) VALUES(?,?,?)", r.id, cycleKind, fmt.Sprintf("skipped=%d", r.skipped))
		if e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, "UPDATE tasks SET status_id=(SELECT id FROM statuses WHERE is_initial=1),recurrence_anchor=?,version=version+1,updated_at=? WHERE id=?", r.anchor.String(), time.Now().UTC().Format(time.RFC3339Nano), r.id)
		if e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, "UPDATE subtasks SET status_id=(SELECT id FROM statuses WHERE is_initial=1) WHERE task_id=?", r.id)
		if e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, "INSERT INTO history(task_id,kind,detail) VALUES(?,'recurrence_reset',?)", r.id, fmt.Sprintf("skipped=%d", r.skipped))
		if e != nil {
			return e
		}
	}
	return tx.Commit()
}
