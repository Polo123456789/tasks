package registry

import (
	"context"
	"database/sql"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
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
	if _, e = db.Exec("PRAGMA journal_mode=DELETE; PRAGMA busy_timeout=3000; CREATE TABLE IF NOT EXISTS projects(path TEXT PRIMARY KEY NOT NULL)"); e != nil {
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
	for _, p := range paths {
		if st, e := os.Stat(p); e == nil && !st.IsDir() {
			live = append(live, p)
		} else {
			if _, e = r.db.ExecContext(ctx, "DELETE FROM projects WHERE path=?", p); e != nil {
				return nil, e
			}
		}
	}
	return live, nil
}
func (r *SQLite) Close() error { return r.db.Close() }
