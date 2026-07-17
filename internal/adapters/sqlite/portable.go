package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/projectimport"
)

type Inspection struct {
	Path                 string
	SchemaVersion        int
	SupportedVersion     int
	Integrity            string
	ForeignKeyViolations int
	MissingTables        []string
}

var ErrActiveSidecars = errors.New("SQLite database has active sidecars")

// EnsureQuiescent rejects databases that require journal recovery or WAL
// coordination. Read-only commands use it before opening SQLite so inspecting a
// database never creates or updates sidecar files.
func EnsureQuiescent(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		if _, err = os.Lstat(absolute + suffix); err == nil {
			return fmt.Errorf("%w: %s exists; close every process using the database and checkpoint it before retrying", ErrActiveSidecars, absolute+suffix)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	file, err := os.Open(absolute)
	if err != nil {
		return err
	}
	header := make([]byte, 20)
	read, readErr := io.ReadFull(file, header)
	closeErr := file.Close()
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return errors.Join(readErr, closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if read >= 20 && string(header[:16]) == "SQLite format 3\x00" && (header[18] == 2 || header[19] == 2) {
		return fmt.Errorf("%w: %s is configured for WAL; checkpoint it and switch it to DELETE mode before retrying", ErrActiveSidecars, absolute)
	}
	return nil
}

func readOnlyDatabase(path string) (*sql.DB, string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return nil, absolute, err
	}
	if !info.Mode().IsRegular() {
		return nil, absolute, fmt.Errorf("database path is not a regular file")
	}
	if err = EnsureQuiescent(absolute); err != nil {
		return nil, absolute, err
	}
	dsn := (&url.URL{Scheme: "file", Path: absolute, RawQuery: "mode=ro"}).String()
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, absolute, err
	}
	database.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), defaultOpenTimeout)
	defer cancel()
	for _, pragma := range []string{"PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=3000"} {
		if _, err = database.ExecContext(ctx, pragma); err != nil {
			database.Close()
			return nil, absolute, err
		}
	}
	if err = database.PingContext(ctx); err != nil {
		database.Close()
		return nil, absolute, err
	}
	return database, absolute, nil
}

const defaultOpenTimeout = 5 * time.Second

func Inspect(ctx context.Context, path string) (inspection Inspection, resultErr error) {
	database, absolute, err := readOnlyDatabase(path)
	inspection = Inspection{Path: absolute, SupportedVersion: SchemaVersion}
	if err != nil {
		return inspection, err
	}
	defer func() { resultErr = errors.Join(resultErr, database.Close()) }()
	if err = database.QueryRowContext(ctx, "PRAGMA user_version").Scan(&inspection.SchemaVersion); err != nil {
		return inspection, fmt.Errorf("read schema version: %w", err)
	}
	rows, err := database.QueryContext(ctx, "PRAGMA integrity_check")
	if err != nil {
		return inspection, fmt.Errorf("run integrity check: %w", err)
	}
	var integrity []string
	for rows.Next() {
		var message string
		if err = rows.Scan(&message); err != nil {
			rows.Close()
			return inspection, fmt.Errorf("read integrity check: %w", err)
		}
		integrity = append(integrity, message)
	}
	if err = rows.Close(); err != nil {
		return inspection, err
	}
	inspection.Integrity = strings.Join(integrity, "; ")
	foreignRows, err := database.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return inspection, fmt.Errorf("run foreign key check: %w", err)
	}
	for foreignRows.Next() {
		inspection.ForeignKeyViolations++
	}
	if err = foreignRows.Close(); err != nil {
		return inspection, err
	}
	tables := []string{"statuses", "tasks", "subtasks", "dependencies", "history"}
	if inspection.SchemaVersion >= 2 {
		tables = append(tables, "project_config")
	}
	for _, table := range tables {
		var exists int
		if err = database.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name=?)", table).Scan(&exists); err != nil {
			return inspection, fmt.Errorf("inspect table %s: %w", table, err)
		}
		if exists == 0 {
			inspection.MissingTables = append(inspection.MissingTables, table)
		}
	}
	return inspection, nil
}

func openValidatedReadOnly(path string, requireCurrent bool) (*Store, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultOpenTimeout)
	defer cancel()
	inspection, err := Inspect(ctx, path)
	if err != nil {
		return nil, err
	}
	if inspection.SchemaVersion > SchemaVersion || inspection.SchemaVersion < 1 {
		if inspection.SchemaVersion > SchemaVersion {
			return nil, fmt.Errorf("database schema %d is newer than supported version %d", inspection.SchemaVersion, SchemaVersion)
		}
		return nil, fmt.Errorf("database schema %d is not supported", inspection.SchemaVersion)
	}
	if requireCurrent && inspection.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("database schema %d requires migration to version %d; read-only operation refused", inspection.SchemaVersion, SchemaVersion)
	}
	if inspection.Integrity != "ok" || inspection.ForeignKeyViolations != 0 || len(inspection.MissingTables) != 0 {
		return nil, fmt.Errorf("database failed integrity validation")
	}
	database, absolute, err := readOnlyDatabase(path)
	if err != nil {
		return nil, err
	}
	return &Store{db: database, path: absolute}, nil
}

func OpenReadOnly(path string) (*Store, error) { return openValidatedReadOnly(path, true) }

// OpenSnapshotSource accepts every healthy schema version that this binary can
// migrate, while keeping the source itself strictly read-only.
func OpenSnapshotSource(path string) (*Store, error) { return openValidatedReadOnly(path, false) }

func (s *Store) ExportProject(ctx context.Context) (document projectimport.Document, resultErr error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return document, err
	}
	defer tx.Rollback()
	document.Format = projectimport.Format
	document.Version = projectimport.Version
	statusRows, err := tx.QueryContext(ctx, "SELECT id,name,kind,is_initial FROM statuses ORDER BY CASE kind WHEN 'normal' THEN 0 WHEN 'done' THEN 1 ELSE 2 END,position,id")
	if err != nil {
		return document, err
	}
	statusKeys := make(map[int64]string)
	for statusRows.Next() {
		var id int64
		var name string
		var kind domain.StatusKind
		var initial bool
		if err = statusRows.Scan(&id, &name, &kind, &initial); err != nil {
			statusRows.Close()
			return document, err
		}
		key := fmt.Sprintf("status-%d", id)
		switch kind {
		case domain.StatusDone:
			key = projectimport.StatusDone
		case domain.StatusCancelled:
			key = projectimport.StatusCancelled
		default:
			document.Statuses = append(document.Statuses, projectimport.StatusSpec{Key: key, Name: name, Initial: initial})
		}
		statusKeys[id] = key
	}
	if err = statusRows.Close(); err != nil {
		return document, err
	}

	taskRows, err := tx.QueryContext(ctx, "SELECT id,title,status_id,priority,markdown,start_date,due_date,recurrence FROM tasks WHERE deleted_at IS NULL ORDER BY id")
	if err != nil {
		return document, err
	}
	taskIndexes := make(map[int64]int)
	for taskRows.Next() {
		var id, statusID int64
		var title, markdown string
		var priority domain.Priority
		var start, due, recurrence sql.NullString
		if err = taskRows.Scan(&id, &title, &statusID, &priority, &markdown, &start, &due, &recurrence); err != nil {
			taskRows.Close()
			return document, err
		}
		task := projectimport.TaskSpec{Key: fmt.Sprintf("task-%d", id), Title: title, Status: statusKeys[statusID], Priority: priority.Key(), Markdown: markdown}
		if start.Valid {
			value := start.String
			task.Start = &value
		}
		if due.Valid {
			value := due.String
			task.Due = &value
		}
		if recurrence.Valid {
			var parsed domain.Recurrence
			if err = json.Unmarshal([]byte(recurrence.String), &parsed); err != nil {
				taskRows.Close()
				return document, fmt.Errorf("task %d recurrence: %w", id, err)
			}
			value := parsed.Text()
			task.Recurrence = &value
		}
		taskIndexes[id] = len(document.Tasks)
		document.Tasks = append(document.Tasks, task)
	}
	if err = taskRows.Close(); err != nil {
		return document, err
	}

	subtaskRows, err := tx.QueryContext(ctx, "SELECT task_id,title,status_id FROM subtasks WHERE task_id IN (SELECT id FROM tasks WHERE deleted_at IS NULL) ORDER BY task_id,id")
	if err != nil {
		return document, err
	}
	for subtaskRows.Next() {
		var taskID, statusID int64
		var title string
		if err = subtaskRows.Scan(&taskID, &title, &statusID); err != nil {
			subtaskRows.Close()
			return document, err
		}
		if index, ok := taskIndexes[taskID]; ok {
			document.Tasks[index].Subtasks = append(document.Tasks[index].Subtasks, projectimport.SubtaskSpec{Title: title, Status: statusKeys[statusID]})
		}
	}
	if err = subtaskRows.Close(); err != nil {
		return document, err
	}

	dependencyRows, err := tx.QueryContext(ctx, "SELECT task_id,depends_on_id FROM dependencies WHERE task_id IN (SELECT id FROM tasks WHERE deleted_at IS NULL) AND depends_on_id IN (SELECT id FROM tasks WHERE deleted_at IS NULL) ORDER BY task_id,depends_on_id")
	if err != nil {
		return document, err
	}
	for dependencyRows.Next() {
		var taskID, dependsOn int64
		if err = dependencyRows.Scan(&taskID, &dependsOn); err != nil {
			dependencyRows.Close()
			return document, err
		}
		index, taskOK := taskIndexes[taskID]
		dependencyIndex, dependencyOK := taskIndexes[dependsOn]
		if taskOK && dependencyOK {
			document.Tasks[index].DependsOn = append(document.Tasks[index].DependsOn, document.Tasks[dependencyIndex].Key)
		}
	}
	if err = dependencyRows.Close(); err != nil {
		return document, err
	}
	if err = tx.Commit(); err != nil {
		return document, err
	}
	return document, nil
}

// Snapshot creates a private, consistent SQLite snapshot in directory. The
// caller owns the returned file and must remove it when it is no longer needed.
func (s *Store) Snapshot(ctx context.Context, directory string) (path string, resultErr error) {
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	temporary, err := os.CreateTemp(directory, ".tasks-snapshot-")
	if err != nil {
		return "", err
	}
	temporaryPath := temporary.Name()
	path = temporaryPath
	published := false
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, temporary.Close(), os.Remove(temporaryPath))
		}
	}()
	if err = temporary.Chmod(0600); err != nil {
		return "", err
	}
	if _, err = s.db.ExecContext(ctx, "VACUUM INTO ?", temporaryPath); err != nil {
		return "", fmt.Errorf("create consistent SQLite snapshot: %w", err)
	}
	openedInfo, err := temporary.Stat()
	if err != nil {
		return "", err
	}
	pathInfo, err := os.Stat(temporaryPath)
	if err != nil {
		return "", err
	}
	if !os.SameFile(openedInfo, pathInfo) {
		return "", fmt.Errorf("snapshot destination changed while SQLite was writing it")
	}
	if err = temporary.Sync(); err != nil {
		return "", err
	}
	if err = temporary.Close(); err != nil {
		return "", err
	}
	published = true
	return path, nil
}

func (s *Store) Backup(ctx context.Context, target string) error {
	absolute, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if _, err = os.Lstat(absolute); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return fmt.Errorf("backup destination %s already exists", absolute)
		}
		return err
	}
	if err = os.MkdirAll(filepath.Dir(absolute), 0700); err != nil {
		return err
	}
	temporaryPath, err := s.Snapshot(ctx, filepath.Dir(absolute))
	if err != nil {
		return err
	}
	defer os.Remove(temporaryPath)
	if err = os.Link(temporaryPath, absolute); err != nil {
		return fmt.Errorf("publish backup: %w", err)
	}
	return nil
}
