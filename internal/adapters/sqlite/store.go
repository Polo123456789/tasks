package sqlite

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/ports"
	_ "modernc.org/sqlite"
)

const SchemaVersion = 2

//go:embed schema.sql
var schema string

//go:embed migration_002.sql
var migration002 string

type Store struct {
	db   *sql.DB
	path string
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, path: path}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, p := range []string{"PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=3000", "PRAGMA journal_mode=DELETE", "PRAGMA synchronous=FULL"} {
		if _, err = db.ExecContext(ctx, p); err != nil {
			db.Close()
			return nil, err
		}
	}
	var version int
	if err = db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		db.Close()
		return nil, err
	}
	if version > SchemaVersion {
		db.Close()
		return nil, fmt.Errorf("database schema %d is newer than supported version %d", version, SchemaVersion)
	}
	if version == 0 {
		var objects int
		if err = db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema WHERE name NOT LIKE 'sqlite_%'").Scan(&objects); err != nil {
			db.Close()
			return nil, fmt.Errorf("inspect database schema: %w", err)
		}
		if objects != 0 {
			db.Close()
			return nil, fmt.Errorf("unrecognized database schema: version is zero but the database is not empty")
		}
		if _, err = db.ExecContext(ctx, schema); err != nil {
			db.Close()
			return nil, fmt.Errorf("initialize database: %w", err)
		}
		version = SchemaVersion
	} else if err = validateSchema(ctx, db, version); err != nil {
		db.Close()
		return nil, err
	}
	for version < SchemaVersion {
		switch version + 1 {
		case 2:
			_, err = db.ExecContext(ctx, migration002)
		default:
			err = fmt.Errorf("no migration available from schema version %d", version)
		}
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("migrate database from version %d: %w", version, err)
		}
		version++
	}
	if err = validateSchema(ctx, db, SchemaVersion); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func validateSchema(ctx context.Context, db *sql.DB, version int) error {
	tables := []string{"statuses", "tasks", "subtasks", "dependencies", "history"}
	if version >= 2 {
		tables = append(tables, "project_config")
	}
	for _, table := range tables {
		var exists int
		if err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name=?)", table).Scan(&exists); err != nil {
			return fmt.Errorf("validate database schema: %w", err)
		}
		if exists == 0 {
			return fmt.Errorf("unrecognized database schema version %d: missing table %s", version, table)
		}
	}
	return nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Statuses(ctx context.Context) ([]domain.Status, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT id,name,kind,position,is_initial FROM statuses ORDER BY CASE kind WHEN 'done' THEN 2 WHEN 'cancelled' THEN 1 ELSE 0 END,position")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.Status
	for rows.Next() {
		var v domain.Status
		if e = rows.Scan(&v.ID, &v.Name, &v.Kind, &v.Position, &v.Initial); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) CreateStatus(ctx context.Context, name string, initial bool) (domain.Status, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Status{}, domain.ValidationError{Field: "name", Message: "required"}
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return domain.Status{}, e
	}
	defer tx.Rollback()
	if initial {
		if _, e = tx.ExecContext(ctx, "UPDATE statuses SET is_initial=0 WHERE kind='normal'"); e != nil {
			return domain.Status{}, e
		}
	}
	r, e := tx.ExecContext(ctx, "INSERT INTO statuses(name,kind,position,is_initial) VALUES(?,'normal',(SELECT COALESCE(MAX(position),0)+1 FROM statuses WHERE kind='normal'),?)", name, initial)
	if e != nil {
		return domain.Status{}, e
	}
	id, e := r.LastInsertId()
	if e != nil {
		return domain.Status{}, e
	}
	if e = tx.Commit(); e != nil {
		return domain.Status{}, e
	}
	return domain.Status{ID: id, Name: name, Kind: domain.StatusNormal, Initial: initial}, nil
}
func (s *Store) RenameStatus(ctx context.Context, id int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.ValidationError{Field: "name", Message: "required"}
	}
	r, e := s.db.ExecContext(ctx, "UPDATE statuses SET name=? WHERE id=? AND kind='normal'", name, id)
	if e == nil {
		n, rowsErr := r.RowsAffected()
		if rowsErr != nil {
			return rowsErr
		}
		if n == 0 {
			return domain.ErrNotFound
		}
	}
	return e
}
func (s *Store) SetInitialStatus(ctx context.Context, id int64) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var kind string
	if e = tx.QueryRowContext(ctx, "SELECT kind FROM statuses WHERE id=?", id).Scan(&kind); errors.Is(e, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	if e != nil {
		return e
	}
	if kind != string(domain.StatusNormal) {
		return domain.ValidationError{Field: "status", Message: "initial status must be normal"}
	}
	if _, e = tx.ExecContext(ctx, "UPDATE statuses SET is_initial=0 WHERE is_initial=1"); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "UPDATE statuses SET is_initial=1 WHERE id=?", id); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) ReorderStatuses(ctx context.Context, ids []int64) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var normalCount int
	if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM statuses WHERE kind='normal'").Scan(&normalCount); e != nil {
		return e
	}
	if len(ids) != normalCount {
		return domain.ValidationError{Field: "statuses", Message: "order must contain every normal status exactly once"}
	}
	seen := make(map[int64]struct{}, len(ids))
	for position, id := range ids {
		if _, duplicate := seen[id]; duplicate {
			return domain.ValidationError{Field: "statuses", Message: "order contains duplicate status"}
		}
		seen[id] = struct{}{}
		r, execErr := tx.ExecContext(ctx, "UPDATE statuses SET position=? WHERE id=? AND kind='normal'", position+1, id)
		if execErr != nil {
			return execErr
		}
		changed, rowsErr := r.RowsAffected()
		if rowsErr != nil {
			return rowsErr
		}
		if changed != 1 {
			return domain.ValidationError{Field: "statuses", Message: "order contains unknown or special status"}
		}
	}
	return tx.Commit()
}
func (s *Store) DeleteStatus(ctx context.Context, id int64, dest *int64) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var initial bool
	var kind string
	if e = tx.QueryRowContext(ctx, "SELECT is_initial,kind FROM statuses WHERE id=?", id).Scan(&initial, &kind); errors.Is(e, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	if e != nil {
		return e
	}
	if initial || kind != "normal" {
		return domain.ValidationError{Field: "status", Message: "initial and special statuses cannot be deleted"}
	}
	var count int
	if e = tx.QueryRowContext(ctx, "SELECT (SELECT count(*) FROM tasks WHERE status_id=?)+(SELECT count(*) FROM subtasks WHERE status_id=?)", id, id).Scan(&count); e != nil {
		return e
	}
	if count > 0 && dest == nil {
		return domain.ValidationError{Field: "destination", Message: "required for non-empty status"}
	}
	if dest != nil {
		if *dest == id {
			return domain.ValidationError{Field: "destination", Message: "must differ from deleted status"}
		}
		var destinationKind string
		if e = tx.QueryRowContext(ctx, "SELECT kind FROM statuses WHERE id=?", *dest).Scan(&destinationKind); errors.Is(e, sql.ErrNoRows) {
			return domain.ErrNotFound
		}
		if e != nil {
			return e
		}
		if destinationKind != string(domain.StatusNormal) {
			return domain.ValidationError{Field: "destination", Message: "must be a normal status"}
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO history(task_id,kind,detail) SELECT id,'status_changed',? FROM tasks WHERE status_id=?", fmt.Sprintf("moved to status %d before deleting status", *dest), id); e != nil {
			return e
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		if _, e = tx.ExecContext(ctx, "INSERT INTO history(task_id,kind,detail,created_at) SELECT DISTINCT t.id,'subtask_changed',?,? FROM tasks t JOIN subtasks st ON st.task_id=t.id WHERE st.status_id=? AND t.status_id<>?", fmt.Sprintf("moved subtasks to status %d before deleting status", *dest), now, id, id); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE tasks SET version=version+1,updated_at=? WHERE id IN (SELECT st.task_id FROM subtasks st JOIN tasks t ON t.id=st.task_id WHERE st.status_id=? AND t.status_id<>?)", now, id, id); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE tasks SET status_id=?,version=version+1,updated_at=? WHERE status_id=?", *dest, now, id); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE subtasks SET status_id=? WHERE status_id=?", *dest, id); e != nil {
			return e
		}
	}
	_, e = tx.ExecContext(ctx, "DELETE FROM statuses WHERE id=?", id)
	if e != nil {
		return e
	}
	return tx.Commit()
}

func (s *Store) ListTasks(ctx context.Context, f ports.TaskFilter) ([]domain.Task, error) {
	q := `SELECT t.id,t.title,t.status_id,s.name,s.kind,s.position,s.is_initial,t.priority,t.markdown,t.start_date,t.due_date,t.recurrence,t.recurrence_anchor,t.version,t.deleted_at,t.created_at,t.updated_at, EXISTS(SELECT 1 FROM dependencies d JOIN tasks p ON p.id=d.depends_on_id JOIN statuses ps ON ps.id=p.status_id WHERE d.task_id=t.id AND ps.kind!='done'), (SELECT count(*) FROM subtasks st JOIN statuses ss ON ss.id=st.status_id WHERE st.task_id=t.id AND ss.kind='done'), (SELECT count(*) FROM subtasks st WHERE st.task_id=t.id), (SELECT count(*) FROM dependencies d WHERE d.task_id=t.id), (SELECT group_concat(depends_on_id, ',') FROM dependencies d WHERE d.task_id=t.id) FROM tasks t JOIN statuses s ON s.id=t.status_id WHERE 1=1`
	var a []any
	if !f.IncludeDeleted {
		q += " AND t.deleted_at IS NULL"
	}
	if f.IncludeDeleted {
		q += " AND t.deleted_at IS NOT NULL"
	}
	if f.Query != "" {
		q += " AND t.title LIKE ?"
		a = append(a, "%"+f.Query+"%")
	}
	if f.Markdown != "" {
		q += " AND t.markdown LIKE ?"
		a = append(a, "%"+f.Markdown+"%")
	}
	if len(f.StatusIDs) > 0 {
		q += " AND t.status_id IN (" + placeholders(len(f.StatusIDs)) + ")"
		for _, id := range f.StatusIDs {
			a = append(a, id)
		}
	}
	if len(f.StatusNames) > 0 {
		q += " AND s.name IN (" + placeholders(len(f.StatusNames)) + ")"
		for _, name := range f.StatusNames {
			a = append(a, name)
		}
	}
	if len(f.Priorities) > 0 {
		q += " AND t.priority IN (" + placeholders(len(f.Priorities)) + ")"
		for _, priority := range f.Priorities {
			a = append(a, priority)
		}
	}
	if f.From != nil {
		q += " AND COALESCE(t.due_date,t.start_date)>=?"
		a = append(a, f.From.String())
	}
	if f.To != nil {
		q += " AND COALESCE(t.start_date,t.due_date)<=?"
		a = append(a, f.To.String())
	}
	if !f.IncludeDone {
		q += " AND s.kind!='done'"
	}
	if !f.IncludeCancelled {
		q += " AND s.kind!='cancelled'"
	}
	if f.OnlyBlocked {
		q += " AND EXISTS(SELECT 1 FROM dependencies d JOIN tasks p ON p.id=d.depends_on_id JOIN statuses ps ON ps.id=p.status_id WHERE d.task_id=t.id AND ps.kind!='done')"
	}
	if f.OnlyRecurring {
		q += " AND t.recurrence IS NOT NULL"
	}
	switch f.Sort {
	case "title":
		q += " ORDER BY t.title COLLATE NOCASE,t.id"
	case "priority":
		q += " ORDER BY t.priority DESC,t.updated_at DESC,t.id"
	case "status":
		q += " ORDER BY CASE s.kind WHEN 'done' THEN 2 WHEN 'cancelled' THEN 1 ELSE 0 END,s.position,t.title COLLATE NOCASE,t.id"
	case "start":
		q += " ORDER BY t.start_date IS NULL,t.start_date,t.title COLLATE NOCASE,t.id"
	case "due":
		q += " ORDER BY t.due_date IS NULL,t.due_date,t.title COLLATE NOCASE,t.id"
	case "updated":
		q += " ORDER BY t.updated_at DESC,t.id"
	default:
		q += " ORDER BY t.updated_at DESC,t.id"
	}
	rows, e := s.db.QueryContext(ctx, q, a...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.Task
	for rows.Next() {
		t, e := scanTask(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

type scanner interface{ Scan(...any) error }

func scanTask(r scanner) (domain.Task, error) {
	var t domain.Task
	var start, due, del, rec, anchor, dependencies sql.NullString
	var created, updated string
	e := r.Scan(&t.ID, &t.Title, &t.StatusID, &t.Status.Name, &t.Status.Kind, &t.Status.Position, &t.Status.Initial, &t.Priority, &t.Markdown, &start, &due, &rec, &anchor, &t.Version, &del, &created, &updated, &t.Blocked, &t.SubtaskDoneCount, &t.SubtaskCount, &t.DependencyCount, &dependencies)
	if e != nil {
		return t, e
	}
	t.Status.ID = t.StatusID
	if t.Start, e = parseStoredDate("start_date", start); e != nil {
		return t, fmt.Errorf("task %d: %w", t.ID, e)
	}
	if t.Due, e = parseStoredDate("due_date", due); e != nil {
		return t, fmt.Errorf("task %d: %w", t.ID, e)
	}
	if t.DeletedAt, e = parseStoredDate("deleted_at", del); e != nil {
		return t, fmt.Errorf("task %d: %w", t.ID, e)
	}
	if t.RecurrenceAnchor, e = parseStoredDate("recurrence_anchor", anchor); e != nil {
		return t, fmt.Errorf("task %d: %w", t.ID, e)
	}
	if rec.Valid {
		var v domain.Recurrence
		if e = json.Unmarshal([]byte(rec.String), &v); e != nil {
			return t, fmt.Errorf("task %d recurrence: %w", t.ID, e)
		}
		if e = v.Validate(); e != nil {
			return t, fmt.Errorf("task %d recurrence: %w", t.ID, e)
		}
		t.Recurrence = &v
	}
	if dependencies.Valid {
		for _, raw := range strings.Split(dependencies.String, ",") {
			id, parseErr := strconv.ParseInt(raw, 10, 64)
			if parseErr != nil {
				return t, fmt.Errorf("task %d dependency id %q: %w", t.ID, raw, parseErr)
			}
			t.DependencyIDs = append(t.DependencyIDs, id)
		}
	}
	if t.CreatedAt, e = time.Parse(time.RFC3339Nano, created); e != nil {
		return t, fmt.Errorf("task %d created_at: %w", t.ID, e)
	}
	if t.UpdatedAt, e = time.Parse(time.RFC3339Nano, updated); e != nil {
		return t, fmt.Errorf("task %d updated_at: %w", t.ID, e)
	}
	if t.Status.Kind != domain.StatusNormal && t.Status.Kind != domain.StatusCancelled && t.Status.Kind != domain.StatusDone {
		return t, fmt.Errorf("task %d status kind %q is invalid", t.ID, t.Status.Kind)
	}
	if e = domain.ValidateTask(t); e != nil {
		return t, fmt.Errorf("task %d stored data: %w", t.ID, e)
	}
	return t, nil
}

func parseStoredDate(field string, raw sql.NullString) (*domain.Date, error) {
	if !raw.Valid {
		return nil, nil
	}
	value, err := domain.ParseDate(raw.String)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", field, err)
	}
	return &value, nil
}

const taskSelect = `SELECT t.id,t.title,t.status_id,s.name,s.kind,s.position,s.is_initial,t.priority,t.markdown,t.start_date,t.due_date,t.recurrence,t.recurrence_anchor,t.version,t.deleted_at,t.created_at,t.updated_at, EXISTS(SELECT 1 FROM dependencies d JOIN tasks p ON p.id=d.depends_on_id JOIN statuses ps ON ps.id=p.status_id WHERE d.task_id=t.id AND ps.kind!='done'), (SELECT count(*) FROM subtasks st JOIN statuses ss ON ss.id=st.status_id WHERE st.task_id=t.id AND ss.kind='done'), (SELECT count(*) FROM subtasks st WHERE st.task_id=t.id), (SELECT count(*) FROM dependencies d WHERE d.task_id=t.id), (SELECT group_concat(depends_on_id, ',') FROM dependencies d WHERE d.task_id=t.id) FROM tasks t JOIN statuses s ON s.id=t.status_id WHERE t.id=?`

func (s *Store) Task(ctx context.Context, id int64) (domain.Task, error) {
	t, e := scanTask(s.db.QueryRowContext(ctx, taskSelect, id))
	if errors.Is(e, sql.ErrNoRows) {
		return t, domain.ErrNotFound
	}
	if e != nil {
		return t, e
	}
	rows, e := s.db.QueryContext(ctx, "SELECT st.id,st.task_id,st.title,st.status_id,s.name,s.kind,s.position,s.is_initial FROM subtasks st JOIN statuses s ON s.id=st.status_id WHERE st.task_id=? ORDER BY st.id", id)
	if e != nil {
		return t, e
	}
	defer rows.Close()
	for rows.Next() {
		var v domain.Subtask
		if e = rows.Scan(&v.ID, &v.TaskID, &v.Title, &v.StatusID, &v.Status.Name, &v.Status.Kind, &v.Status.Position, &v.Status.Initial); e != nil {
			return t, e
		}
		if strings.TrimSpace(v.Title) == "" {
			return t, fmt.Errorf("subtask %d has an empty title", v.ID)
		}
		if v.Status.Kind != domain.StatusNormal && v.Status.Kind != domain.StatusCancelled && v.Status.Kind != domain.StatusDone {
			return t, fmt.Errorf("subtask %d status kind %q is invalid", v.ID, v.Status.Kind)
		}
		v.Status.ID = v.StatusID
		t.Subtasks = append(t.Subtasks, v)
	}
	if e = rows.Err(); e != nil {
		return t, e
	}
	return t, nil
}
func nullableDate(d *domain.Date) any {
	if d == nil {
		return nil
	}
	return d.String()
}
func recurrenceJSON(r *domain.Recurrence) (any, error) {
	if r == nil {
		return nil, nil
	}
	if e := r.Validate(); e != nil {
		return nil, e
	}
	b, e := json.Marshal(r)
	return string(b), e
}
func (s *Store) CreateTask(ctx context.Context, t domain.Task) (domain.Task, error) {
	if e := domain.ValidateTask(t); e != nil {
		return t, e
	}
	rec, e := recurrenceJSON(t.Recurrence)
	if e != nil {
		return t, e
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return t, e
	}
	defer tx.Rollback()
	if t.StatusID == 0 {
		e = tx.QueryRowContext(ctx, "SELECT id FROM statuses WHERE is_initial=1").Scan(&t.StatusID)
		if e != nil {
			return t, e
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	r, e := tx.ExecContext(ctx, "INSERT INTO tasks(title,status_id,priority,markdown,start_date,due_date,recurrence,recurrence_anchor,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)", strings.TrimSpace(t.Title), t.StatusID, t.Priority, t.Markdown, nullableDate(t.Start), nullableDate(t.Due), rec, nullableDate(t.RecurrenceAnchor), now, now)
	if e != nil {
		return t, e
	}
	t.ID, e = r.LastInsertId()
	if e != nil {
		return t, e
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO history(task_id,kind,detail,created_at) VALUES(?,'created','',?)", t.ID, now)
	if e != nil {
		return t, e
	}
	if e = tx.Commit(); e != nil {
		return t, e
	}
	return s.Task(ctx, t.ID)
}
func (s *Store) UpdateTask(ctx context.Context, t domain.Task) (domain.Task, error) {
	if e := domain.ValidateTask(t); e != nil {
		return t, e
	}
	rec, e := recurrenceJSON(t.Recurrence)
	if e != nil {
		return t, e
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return t, e
	}
	defer tx.Rollback()
	r, e := tx.ExecContext(ctx, "UPDATE tasks SET title=?,priority=?,markdown=?,start_date=?,due_date=?,recurrence=?,recurrence_anchor=?,version=version+1,updated_at=? WHERE id=? AND version=? AND deleted_at IS NULL", strings.TrimSpace(t.Title), t.Priority, t.Markdown, nullableDate(t.Start), nullableDate(t.Due), rec, nullableDate(t.RecurrenceAnchor), now, t.ID, t.Version)
	if e != nil {
		return t, e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return t, e
	}
	if n == 0 {
		return t, domain.ErrConflict
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO history(task_id,kind,detail,created_at) VALUES(?,'edited','',?)", t.ID, now)
	if e != nil {
		return t, e
	}
	if e = tx.Commit(); e != nil {
		return t, e
	}
	return s.Task(ctx, t.ID)
}
func (s *Store) SetTaskStatus(ctx context.Context, id, statusID, version int64) (domain.Task, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return domain.Task{}, e
	}
	defer tx.Rollback()
	var kind, oldKind string
	if e = tx.QueryRowContext(ctx, "SELECT kind FROM statuses WHERE id=?", statusID).Scan(&kind); errors.Is(e, sql.ErrNoRows) {
		return domain.Task{}, domain.ErrNotFound
	}
	if e != nil {
		return domain.Task{}, e
	}
	if e = tx.QueryRowContext(ctx, "SELECT s.kind FROM tasks t JOIN statuses s ON s.id=t.status_id WHERE t.id=?", id).Scan(&oldKind); errors.Is(e, sql.ErrNoRows) {
		return domain.Task{}, domain.ErrNotFound
	}
	if e != nil {
		return domain.Task{}, e
	}
	r, e := tx.ExecContext(ctx, "UPDATE tasks SET status_id=?,version=version+1,updated_at=? WHERE id=? AND version=? AND deleted_at IS NULL", statusID, time.Now().UTC().Format(time.RFC3339Nano), id, version)
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
	if kind == "done" || kind == "cancelled" {
		_, e = tx.ExecContext(ctx, "UPDATE subtasks SET status_id=? WHERE task_id=?", statusID, id)
		if e != nil {
			return domain.Task{}, e
		}
	} else if oldKind == "done" || oldKind == "cancelled" {
		_, e = tx.ExecContext(ctx, "UPDATE subtasks SET status_id=(SELECT id FROM statuses WHERE is_initial=1) WHERE task_id=?", id)
		if e != nil {
			return domain.Task{}, e
		}
	}
	eventKind := "status_changed"
	if kind == string(domain.StatusDone) {
		eventKind = "completed"
	} else if kind == string(domain.StatusCancelled) {
		eventKind = "cancelled"
	} else if oldKind == string(domain.StatusDone) || oldKind == string(domain.StatusCancelled) {
		eventKind = "reopened"
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO history(task_id,kind,detail) VALUES(?,?,?)", id, eventKind, kind)
	if e != nil {
		return domain.Task{}, e
	}
	if e = tx.Commit(); e != nil {
		return domain.Task{}, e
	}
	return s.Task(ctx, id)
}

func (s *Store) SetTaskPriority(ctx context.Context, id int64, priority domain.Priority, version int64) (domain.Task, error) {
	if !priority.Valid() {
		return domain.Task{}, domain.ValidationError{Field: "priority", Message: "invalid"}
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return domain.Task{}, e
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	r, e := tx.ExecContext(ctx, "UPDATE tasks SET priority=?,version=version+1,updated_at=? WHERE id=? AND version=? AND deleted_at IS NULL", priority, now, id, version)
	if e != nil {
		return domain.Task{}, e
	}
	changed, e := r.RowsAffected()
	if e != nil {
		return domain.Task{}, e
	}
	if changed == 0 {
		return domain.Task{}, domain.ErrConflict
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO history(task_id,kind,detail,created_at) VALUES(?,'priority_changed',?,?)", id, priority.String(), now); e != nil {
		return domain.Task{}, e
	}
	if e = tx.Commit(); e != nil {
		return domain.Task{}, e
	}
	return s.Task(ctx, id)
}
