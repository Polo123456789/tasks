package registry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	adaptersqlite "github.com/Polo123456789/tasks/internal/adapters/sqlite"
	_ "modernc.org/sqlite"
)

type SQLite struct{ db *sql.DB }

func Open(path string) (*SQLite, error) {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	if _, e = db.Exec("PRAGMA journal_mode=DELETE; PRAGMA synchronous=FULL; PRAGMA busy_timeout=3000; CREATE TABLE IF NOT EXISTS projects(path TEXT PRIMARY KEY NOT NULL)"); e != nil {
		db.Close()
		return nil, e
	}
	if e = os.Chmod(path, 0600); e != nil {
		db.Close()
		return nil, e
	}
	return &SQLite{db}, nil
}
func (r *SQLite) Register(ctx context.Context, path string) error {
	p, e := filepath.Abs(path)
	if e != nil {
		return e
	}
	p, e = filepath.EvalSymlinks(p)
	if e != nil {
		return e
	}
	_, e = r.db.ExecContext(ctx, "INSERT OR IGNORE INTO projects(path) VALUES(?)", p)
	return e
}
func (r *SQLite) Unregister(ctx context.Context, path string) error {
	p, e := filepath.Abs(path)
	if e != nil {
		return e
	}
	_, e = r.db.ExecContext(ctx, "DELETE FROM projects WHERE path=?", filepath.Clean(p))
	return e
}
func (r *SQLite) CheckWritable(ctx context.Context) error {
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, "INSERT OR IGNORE INTO projects(path) VALUES(?)", "/.tasks-registry-write-check"); e != nil {
		return e
	}
	return tx.Rollback()
}
func (r *SQLite) Projects(ctx context.Context) ([]string, error) {
	rows, e := r.db.QueryContext(ctx, "SELECT path FROM projects ORDER BY path")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if e = rows.Scan(&p); e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (r *SQLite) Prune(ctx context.Context) ([]string, error) {
	paths, e := r.Projects(ctx)
	if e != nil {
		return nil, e
	}
	var live []string
	var errs []error
	for _, p := range paths {
		st, statErr := os.Stat(p)
		if statErr == nil && !st.IsDir() {
			live = append(live, p)
			continue
		}
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			live = append(live, p)
			errs = append(errs, fmt.Errorf("check registered project %q: %w", p, statErr))
			continue
		}
		if _, e = r.db.ExecContext(ctx, "DELETE FROM projects WHERE path=?", p); e != nil {
			errs = append(errs, e)
		}
	}
	return live, errors.Join(errs...)
}
func (r *SQLite) Close() error { return r.db.Close() }

type Inspection struct {
	Paths     []string
	Integrity string
}

// InspectReadOnly inspects an existing registry without creating directories,
// files, tables, journals, or pruning unavailable entries.
func InspectReadOnly(ctx context.Context, path string) (inspection Inspection, resultErr error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return inspection, err
	}
	if _, err = os.Stat(absolute); err != nil {
		return inspection, err
	}
	if err = adaptersqlite.EnsureQuiescent(absolute); err != nil {
		return inspection, err
	}
	dsn := (&url.URL{Scheme: "file", Path: absolute, RawQuery: "mode=ro"}).String()
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return inspection, err
	}
	database.SetMaxOpenConns(1)
	defer func() { resultErr = errors.Join(resultErr, database.Close()) }()
	if _, err = database.ExecContext(ctx, "PRAGMA query_only=ON; PRAGMA busy_timeout=3000"); err != nil {
		return inspection, err
	}
	rows, err := database.QueryContext(ctx, "PRAGMA integrity_check")
	if err != nil {
		return inspection, err
	}
	var messages []string
	for rows.Next() {
		var message string
		if err = rows.Scan(&message); err != nil {
			rows.Close()
			return inspection, err
		}
		messages = append(messages, message)
	}
	if err = rows.Close(); err != nil {
		return inspection, err
	}
	inspection.Integrity = strings.Join(messages, "; ")
	var exists int
	if err = database.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name='projects')").Scan(&exists); err != nil {
		return inspection, err
	}
	if exists == 0 {
		return inspection, fmt.Errorf("registry schema is missing projects table")
	}
	rows, err = database.QueryContext(ctx, "SELECT path FROM projects ORDER BY path")
	if err != nil {
		return inspection, err
	}
	defer rows.Close()
	for rows.Next() {
		var project string
		if err = rows.Scan(&project); err != nil {
			return inspection, err
		}
		inspection.Paths = append(inspection.Paths, project)
	}
	return inspection, rows.Err()
}

func ProjectsReadOnly(ctx context.Context, path string) ([]string, error) {
	inspection, err := InspectReadOnly(ctx, path)
	return inspection.Paths, err
}
