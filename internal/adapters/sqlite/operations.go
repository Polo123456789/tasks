package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Polo123456789/tasks/internal/domain"
)

func (s *Store) TrashTask(ctx context.Context, id, version int64, today domain.Date) ([]int64, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, "SELECT task_id FROM dependencies WHERE depends_on_id=? UNION SELECT depends_on_id FROM dependencies WHERE task_id=?", id, id)
	if e != nil {
		return nil, e
	}
	var affected []int64
	for rows.Next() {
		var x int64
		_ = rows.Scan(&x)
		affected = append(affected, x)
	}
	rows.Close()
	r, e := tx.ExecContext(ctx, "UPDATE tasks SET deleted_at=?,version=version+1 WHERE id=? AND version=? AND deleted_at IS NULL", today.String(), id, version)
	if e != nil {
		return nil, e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return nil, domain.ErrConflict
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
func (s *Store) RestoreTask(ctx context.Context, id, version int64) (domain.Task, error) {
	r, e := s.db.ExecContext(ctx, "UPDATE tasks SET deleted_at=NULL,version=version+1,updated_at=? WHERE id=? AND version=? AND deleted_at IS NOT NULL", time.Now().UTC().Format(time.RFC3339Nano), id, version)
	if e != nil {
		return domain.Task{}, e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return domain.Task{}, domain.ErrConflict
	}
	_, _ = s.db.ExecContext(ctx, "INSERT INTO history(task_id,kind) VALUES(?,'restored')", id)
	return s.Task(ctx, id)
}
func (s *Store) AddSubtask(ctx context.Context, taskID int64, title string) (domain.Subtask, error) {
	var status int64
	if e := s.db.QueryRowContext(ctx, "SELECT id FROM statuses WHERE is_initial=1").Scan(&status); e != nil {
		return domain.Subtask{}, e
	}
	r, e := s.db.ExecContext(ctx, "INSERT INTO subtasks(task_id,title,status_id) VALUES(?,?,?)", taskID, title, status)
	if e != nil {
		return domain.Subtask{}, e
	}
	id, _ := r.LastInsertId()
	_, _ = s.db.ExecContext(ctx, "INSERT INTO history(task_id,kind,detail) VALUES(?,'subtask_created',?)", taskID, title)
	return domain.Subtask{ID: id, TaskID: taskID, Title: title, StatusID: status}, nil
}
func (s *Store) SetSubtaskStatus(ctx context.Context, id, statusID int64) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var taskID int64
	if e = tx.QueryRowContext(ctx, "SELECT task_id FROM subtasks WHERE id=?", id).Scan(&taskID); errors.Is(e, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	if e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "UPDATE subtasks SET status_id=? WHERE id=?", statusID, id); e != nil {
		return e
	}
	var total, done int
	_ = tx.QueryRowContext(ctx, "SELECT count(*),sum(CASE WHEN s.kind='done' THEN 1 ELSE 0 END) FROM subtasks st JOIN statuses s ON s.id=st.status_id WHERE task_id=?", taskID).Scan(&total, &done)
	if total >= 2 && done == total {
		_, e = tx.ExecContext(ctx, "UPDATE tasks SET status_id=(SELECT id FROM statuses WHERE kind='done'),version=version+1 WHERE id=?", taskID)
		if e != nil {
			return e
		}
	}
	_, _ = tx.ExecContext(ctx, "INSERT INTO history(task_id,kind) VALUES(?,'subtask_changed')", taskID)
	return tx.Commit()
}
func (s *Store) AddDependency(ctx context.Context, taskID, dependsOn int64) error {
	if taskID == dependsOn {
		return domain.ErrDependencyCycle
	}
	var cycle int
	e := s.db.QueryRowContext(ctx, `WITH RECURSIVE reach(id) AS (SELECT depends_on_id FROM dependencies WHERE task_id=? UNION SELECT d.depends_on_id FROM dependencies d JOIN reach r ON d.task_id=r.id) SELECT EXISTS(SELECT 1 FROM reach WHERE id=?)`, dependsOn, taskID).Scan(&cycle)
	if e != nil {
		return e
	}
	if cycle != 0 {
		return domain.ErrDependencyCycle
	}
	_, e = s.db.ExecContext(ctx, "INSERT INTO dependencies(task_id,depends_on_id) VALUES(?,?)", taskID, dependsOn)
	if e == nil {
		_, _ = s.db.ExecContext(ctx, "INSERT INTO history(task_id,kind,detail) VALUES(?,'dependency_added',?)", taskID, fmt.Sprint(dependsOn))
	}
	return e
}
func (s *Store) RemoveDependency(ctx context.Context, taskID, dependsOn int64) error {
	r, e := s.db.ExecContext(ctx, "DELETE FROM dependencies WHERE task_id=? AND depends_on_id=?", taskID, dependsOn)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	_, _ = s.db.ExecContext(ctx, "INSERT INTO history(task_id,kind,detail) VALUES(?,'dependency_removed',?)", taskID, fmt.Sprint(dependsOn))
	return nil
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
		v.CreatedAt, _ = time.Parse(time.RFC3339Nano, ts)
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
	rows, e := tx.QueryContext(ctx, "SELECT id,recurrence,recurrence_anchor,status_id FROM tasks WHERE recurrence IS NOT NULL AND recurrence_anchor IS NOT NULL AND deleted_at IS NULL")
	if e != nil {
		return e
	}
	type reset struct {
		id      int64
		anchor  domain.Date
		skipped int
	}
	var resets []reset
	for rows.Next() {
		var id, status int64
		var raw, anchor string
		if e = rows.Scan(&id, &raw, &anchor, &status); e != nil {
			return e
		}
		var rec domain.Recurrence
		if e = json.Unmarshal([]byte(raw), &rec); e != nil {
			continue
		}
		a, _ := domain.ParseDate(anchor)
		next, _ := rec.Next(a)
		skipped := 0
		for !next.After(today) {
			a = next
			skipped++
			next, _ = rec.Next(a)
		}
		if skipped > 0 {
			resets = append(resets, reset{id, a, skipped - 1})
		}
	}
	rows.Close()
	for _, r := range resets {
		_, e = tx.ExecContext(ctx, "UPDATE tasks SET status_id=(SELECT id FROM statuses WHERE is_initial=1),recurrence_anchor=?,version=version+1 WHERE id=?", r.anchor.String(), r.id)
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
