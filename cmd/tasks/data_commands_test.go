package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	registrydb "github.com/Polo123456789/tasks/internal/adapters/registry"
	db "github.com/Polo123456789/tasks/internal/adapters/sqlite"
	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/ports"
	"github.com/Polo123456789/tasks/internal/projectimport"
)

func createDataCommandStore(t *testing.T, path, title string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	store, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	start, _ := domain.ParseDate("2026-07-20")
	due, _ := domain.ParseDate("2026-07-22")
	base, err := store.CreateTask(context.Background(), domain.Task{Title: "Base", Priority: domain.PriorityHigh, Markdown: "# Uno\nDos", Start: &start, Due: &due})
	if err != nil {
		t.Fatal(err)
	}
	delivery, err := store.CreateTask(context.Background(), domain.Task{Title: title})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.AddSubtask(context.Background(), delivery.ID, delivery.Version, "Detalle"); err != nil {
		t.Fatal(err)
	}
	delivery, err = store.Task(context.Background(), delivery.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.AddDependency(context.Background(), delivery.ID, base.ID, delivery.Version); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
}

func hasTaskTitle(tasks []domain.Task, title string) bool {
	for _, task := range tasks {
		if task.Title == title {
			return true
		}
	}
	return false
}

func TestExportCommandIsReadOnlyAndSupportsPortableMarkdownAndCSV(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project.tasks")
	createDataCommandStore(t, project, "Entrega, final")
	before, err := os.ReadFile(project)
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "config")
	t.Setenv("XDG_CONFIG_HOME", config)

	for _, format := range []string{"json", "markdown", "csv"} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			if err := exportTasks(context.Background(), root, invocation{kind: commandExport, format: format}, &output); err != nil {
				t.Fatal(err)
			}
			switch format {
			case "json":
				var document projectimport.Document
				if err := json.Unmarshal(output.Bytes(), &document); err != nil || document.Format != projectimport.Format || len(document.Tasks) != 2 || len(document.Tasks[1].Subtasks) != 1 || len(document.Tasks[1].DependsOn) != 1 {
					t.Fatalf("document=%#v output=%s err=%v", document, output.String(), err)
				}
				if _, err = projectimport.Normalize(document, domain.DateFromTime(fixedClock{}.now)); err != nil {
					// Normalize only needs a non-zero date when recurrence exists; this fixture has none.
					t.Fatalf("portable export rejected: %v", err)
				}
			case "markdown":
				for _, value := range []string{"# Tareas exportadas", "Entrega, final", "Subtarea: Detalle", "Depende de: task-1"} {
					if !strings.Contains(output.String(), value) {
						t.Fatalf("markdown missing %q:\n%s", value, output.String())
					}
				}
			case "csv":
				records, readErr := csv.NewReader(bytes.NewReader(output.Bytes())).ReadAll()
				if readErr != nil || len(records) != 3 || records[0][0] != "key" || records[2][1] != "Entrega, final" {
					t.Fatalf("records=%#v err=%v output=%s", records, readErr, output.String())
				}
			}
		})
	}
	after, err := os.ReadFile(project)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("export modified database: err=%v", err)
	}
	for _, sidecar := range []string{"-journal", "-wal", "-shm"} {
		if _, err = os.Stat(project + sidecar); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("export left sidecar %s: %v", sidecar, err)
		}
	}
	if _, err = os.Stat(config); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("local export created global configuration: %v", err)
	}
}

func TestExportRequiresExplicitGlobalSelectionOutsideProjects(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "config")
	t.Setenv("XDG_CONFIG_HOME", config)
	var output bytes.Buffer
	if err := exportTasks(context.Background(), root, invocation{kind: commandExport, format: "json"}, &output); err == nil || !strings.Contains(err.Error(), "--global") {
		t.Fatalf("implicit global export error=%v", err)
	}
	globalPath := filepath.Join(config, "tasks", "global.sqlite")
	createDataCommandStore(t, globalPath, "Global")
	if err := exportTasks(context.Background(), root, invocation{kind: commandExport, format: "json", global: true}, &output); err != nil {
		t.Fatal(err)
	}
	var document projectimport.Document
	if err := json.Unmarshal(output.Bytes(), &document); err != nil || len(document.Tasks) != 2 {
		t.Fatalf("global document=%#v err=%v", document, err)
	}
	globalBackup := filepath.Join(root, "global.tasks.bak")
	if err := backupTasks(context.Background(), root, invocation{kind: commandBackup, global: true, source: globalBackup}, &output); err != nil {
		t.Fatalf("global backup: %v", err)
	}
	inspection, err := db.Inspect(context.Background(), globalBackup)
	if err != nil || inspection.Integrity != "ok" || inspection.SchemaVersion != db.SchemaVersion {
		t.Fatalf("global backup inspection=%#v err=%v", inspection, err)
	}
}

func TestBackupAndRestoreCommandsPublishAtomicallyAndRequireForce(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "config")
	t.Setenv("XDG_CONFIG_HOME", config)
	project := filepath.Join(root, "project.tasks")
	createDataCommandStore(t, project, "Primera versión")
	backupPath := filepath.Join(root, "backup.tasks.bak")
	var output bytes.Buffer
	if err := backupTasks(context.Background(), root, invocation{kind: commandBackup, source: "backup.tasks.bak"}, &output); err != nil || !strings.Contains(output.String(), backupPath) {
		t.Fatalf("backup output=%q err=%v", output.String(), err)
	}
	if err := backupTasks(context.Background(), root, invocation{kind: commandBackup, source: "backup.tasks.bak"}, &output); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("backup overwrite error=%v", err)
	}

	restoredPath := filepath.Join(root, "restored.tasks")
	output.Reset()
	restoreInvocation := invocation{kind: commandRestore, source: backupPath, project: restoredPath, projectSet: true}
	if err := restoreTasks(context.Background(), root, restoreInvocation, &output); err != nil || !strings.Contains(output.String(), "Restaurado") {
		t.Fatalf("restore output=%q err=%v", output.String(), err)
	}
	restored, err := db.OpenReadOnly(restoredPath)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := restored.ListTasks(context.Background(), ports.TaskFilter{IncludeDone: true, IncludeCancelled: true})
	closeErr := restored.Close()
	if err != nil || closeErr != nil || len(tasks) != 2 || !hasTaskTitle(tasks, "Primera versión") {
		t.Fatalf("restored tasks=%#v err=%v close=%v", tasks, err, closeErr)
	}
	indexPath := filepath.Join(config, "tasks", "registry.sqlite")
	paths, err := registrydb.ProjectsReadOnly(context.Background(), indexPath)
	if err != nil || len(paths) != 1 || paths[0] != restoredPath {
		t.Fatalf("registered paths=%v err=%v", paths, err)
	}

	before, err := os.ReadFile(restoredPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = restoreTasks(context.Background(), root, restoreInvocation, &output); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("silent overwrite error=%v", err)
	}
	after, err := os.ReadFile(restoredPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("rejected restore changed destination: %v", err)
	}

	secondProject := filepath.Join(root, "second.tasks")
	createDataCommandStore(t, secondProject, "Segunda versión")
	secondBackup := filepath.Join(root, "second-backup.tasks.bak")
	secondStore, err := db.Open(secondProject)
	if err != nil {
		t.Fatal(err)
	}
	if err = secondStore.Backup(context.Background(), secondBackup); err != nil {
		t.Fatal(err)
	}
	if err = secondStore.Close(); err != nil {
		t.Fatal(err)
	}
	restoreInvocation.source = secondBackup
	restoreInvocation.force = true
	if err = restoreTasks(context.Background(), root, restoreInvocation, &output); err != nil {
		t.Fatal(err)
	}
	restored, err = db.OpenReadOnly(restoredPath)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err = restored.ListTasks(context.Background(), ports.TaskFilter{IncludeDone: true, IncludeCancelled: true})
	closeErr = restored.Close()
	if err != nil || closeErr != nil || len(tasks) != 2 || !hasTaskTitle(tasks, "Segunda versión") {
		t.Fatalf("force restored tasks=%#v err=%v close=%v", tasks, err, closeErr)
	}
}

func TestRestoreRejectsCorruptionBeforePublishingAndSupportsExplicitGlobal(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "config")
	t.Setenv("XDG_CONFIG_HOME", config)
	corrupt := filepath.Join(root, "corrupt.tasks")
	if err := os.WriteFile(corrupt, []byte("not sqlite"), 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "never.tasks")
	err := restoreTasks(context.Background(), root, invocation{kind: commandRestore, source: corrupt, project: target, projectSet: true}, &bytes.Buffer{})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "corrupt") {
		t.Fatalf("corrupt restore error=%v", err)
	}
	if _, statErr := os.Stat(target); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("corrupt restore published target: %v", statErr)
	}
	createDataCommandStore(t, target, "Conservar")
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	err = restoreTasks(context.Background(), root, invocation{kind: commandRestore, source: corrupt, project: target, projectSet: true, force: true}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("forced corrupt restore unexpectedly succeeded")
	}
	after, readErr := os.ReadFile(target)
	if readErr != nil || !bytes.Equal(before, after) {
		t.Fatalf("invalid forced restore changed destination: err=%v read=%v", err, readErr)
	}

	sourceProject := filepath.Join(root, "source.tasks")
	createDataCommandStore(t, sourceProject, "Global restaurada")
	backup := filepath.Join(root, "global-backup.tasks.bak")
	store, err := db.Open(sourceProject)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Backup(context.Background(), backup); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	if err = restoreTasks(context.Background(), root, invocation{kind: commandRestore, source: backup, global: true}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	global, err := db.OpenReadOnly(filepath.Join(config, "tasks", "global.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer global.Close()
	tasks, err := global.ListTasks(context.Background(), ports.TaskFilter{IncludeDone: true, IncludeCancelled: true})
	if err != nil || len(tasks) != 2 || !hasTaskTitle(tasks, "Global restaurada") {
		t.Fatalf("global restored tasks=%#v err=%v", tasks, err)
	}
}

func TestRestoreMigratesCompatibleOldBackupBeforePublishing(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "old.tasks.bak")
	createDataCommandStore(t, old, "Antigua")
	database, err := sql.Open("sqlite", old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec("DROP TABLE project_config; PRAGMA user_version=1"); err != nil {
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "migrated.tasks")
	if err = restoreDatabase(context.Background(), old, target, false); err != nil {
		t.Fatal(err)
	}
	inspection, err := db.Inspect(context.Background(), target)
	if err != nil || inspection.SchemaVersion != db.SchemaVersion || inspection.Integrity != "ok" {
		t.Fatalf("inspection=%#v err=%v", inspection, err)
	}
	sourceInspection, err := db.Inspect(context.Background(), old)
	if err != nil || sourceInspection.SchemaVersion != 1 {
		t.Fatalf("restore modified old source: inspection=%#v err=%v", sourceInspection, err)
	}
}

func TestDoctorIsReadOnlyStructuredAndDistinguishesIssueKinds(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project.tasks")
	createDataCommandStore(t, project, "Doctor")
	before, err := os.ReadFile(project)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = doctorTasks(context.Background(), root, invocation{kind: commandDoctor, structured: true}, &output); err != nil {
		t.Fatalf("healthy doctor output=%s err=%v", output.String(), err)
	}
	var report doctorReport
	if err = json.Unmarshal(output.Bytes(), &report); err != nil || !report.OK || len(report.Checks) < 3 {
		t.Fatalf("report=%#v err=%v", report, err)
	}
	after, err := os.ReadFile(project)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("doctor modified project: %v", err)
	}

	if err = os.Chmod(project, 0644); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	err = doctorTasks(context.Background(), root, invocation{kind: commandDoctor, structured: true}, &output)
	if !errors.Is(err, errDoctorIssues) {
		t.Fatalf("permission doctor error=%v output=%s", err, output.String())
	}
	if err = json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	foundRepairable := false
	for _, check := range report.Checks {
		foundRepairable = foundRepairable || check.Kind == "repairable"
	}
	if report.OK || !foundRepairable {
		t.Fatalf("permission report=%#v", report)
	}

	future := filepath.Join(root, "future.tasks")
	createDataCommandStore(t, future, "Future")
	store, err := db.Open(future)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Task(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	// Change only user_version without reopening through the version-validating adapter.
	database, err := sql.Open("sqlite", future)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec("PRAGMA user_version=99"); err != nil {
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	err = doctorTasks(context.Background(), root, invocation{kind: commandDoctor, project: future, projectSet: true, structured: true}, &output)
	if !errors.Is(err, errDoctorIssues) || !strings.Contains(output.String(), `"kind": "incompatible"`) {
		t.Fatalf("future doctor err=%v output=%s", err, output.String())
	}
}

func TestGlobalDoctorReportsUnavailableRegistryEntriesWithoutPruning(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "config")
	t.Setenv("XDG_CONFIG_HOME", config)
	globalPath := filepath.Join(config, "tasks", "global.sqlite")
	createDataCommandStore(t, globalPath, "Global")
	project := filepath.Join(root, "gone.tasks")
	createDataCommandStore(t, project, "Gone")
	indexPath := filepath.Join(config, "tasks", "registry.sqlite")
	index, err := registrydb.Open(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = index.Register(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	if err = index.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(project); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err = doctorTasks(context.Background(), root, invocation{kind: commandDoctor, global: true, structured: true}, &output)
	if !errors.Is(err, errDoctorIssues) || !strings.Contains(output.String(), "no disponible") || !strings.Contains(output.String(), "repairable") {
		t.Fatalf("global doctor err=%v output=%s", err, output.String())
	}
	paths, err := registrydb.ProjectsReadOnly(context.Background(), indexPath)
	if err != nil || len(paths) != 1 || paths[0] != project {
		t.Fatalf("doctor pruned registry: paths=%v err=%v", paths, err)
	}
}

func TestReadOnlyCommandsRejectWALWithoutCreatingSHM(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "wal.tasks")
	createDataCommandStore(t, project, "Antes de WAL")
	database, err := sql.Open("sqlite", project)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err = database.Exec("PRAGMA journal_mode=WAL; INSERT INTO tasks(title,status_id,priority,markdown,version,created_at,updated_at) SELECT 'Commit WAL',id,2,'',1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP FROM statuses WHERE is_initial=1"); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(project + "-shm"); err != nil {
		t.Fatal(err)
	}
	if err = exportTasks(context.Background(), root, invocation{kind: commandExport, format: "json"}, &bytes.Buffer{}); !errors.Is(err, db.ErrActiveSidecars) {
		t.Fatalf("WAL export error=%v", err)
	}
	var output bytes.Buffer
	if err = doctorTasks(context.Background(), root, invocation{kind: commandDoctor, structured: true}, &output); !errors.Is(err, errDoctorIssues) || !strings.Contains(output.String(), "repairable") {
		t.Fatalf("WAL doctor err=%v output=%s", err, output.String())
	}
	if _, err = os.Lstat(project + "-shm"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read-only commands created SHM: %v", err)
	}
}

func TestRestoreRejectsWALSourceAndDestinationWithoutChangingTarget(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.tasks")
	createDataCommandStore(t, source, "Fuente WAL")
	sourceDB, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = sourceDB.Exec("PRAGMA journal_mode=WAL; INSERT INTO tasks(title,status_id,priority,markdown,version,created_at,updated_at) SELECT 'Sólo WAL',id,2,'',1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP FROM statuses WHERE is_initial=1"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "target.tasks")
	if err = restoreDatabase(context.Background(), source, target, false); !errors.Is(err, db.ErrActiveSidecars) {
		t.Fatalf("WAL source restore error=%v", err)
	}
	if _, err = os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("WAL source published target: %v", err)
	}
	if err = sourceDB.Close(); err != nil {
		t.Fatal(err)
	}

	createDataCommandStore(t, target, "Destino original")
	targetDB, err := sql.Open("sqlite", target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = targetDB.Exec("PRAGMA journal_mode=WAL; INSERT INTO tasks(title,status_id,priority,markdown,version,created_at,updated_at) SELECT 'Destino WAL',id,2,'',1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP FROM statuses WHERE is_initial=1"); err != nil {
		t.Fatal(err)
	}
	backupSource := filepath.Join(root, "backup-source.tasks")
	createDataCommandStore(t, backupSource, "Restauración")
	backupStore, err := db.OpenSnapshotSource(backupSource)
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(root, "source.tasks.bak")
	if err = backupStore.Backup(context.Background(), backup); err != nil {
		t.Fatal(err)
	}
	if err = backupStore.Close(); err != nil {
		t.Fatal(err)
	}
	if err = restoreDatabase(context.Background(), backup, target, true); !errors.Is(err, db.ErrActiveSidecars) {
		t.Fatalf("WAL destination restore error=%v", err)
	}
	var count int
	if err = targetDB.QueryRow("SELECT count(*) FROM tasks WHERE title='Destino WAL'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("active destination changed: count=%d err=%v", count, err)
	}
	if err = targetDB.Close(); err != nil {
		t.Fatal(err)
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestRestoreRollsBackDatabaseAndRegistryWhenConfirmationFails(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "config")
	t.Setenv("XDG_CONFIG_HOME", config)
	target := filepath.Join(root, "target.tasks")
	createDataCommandStore(t, target, "Conservar")
	source := filepath.Join(root, "source.tasks")
	createDataCommandStore(t, source, "No conservar")
	store, err := db.OpenSnapshotSource(source)
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(root, "source.tasks.bak")
	if err = store.Backup(context.Background(), backup); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("salida cerrada")
	err = restoreTasks(context.Background(), root, invocation{kind: commandRestore, source: backup, project: target, projectSet: true, force: true}, failingWriter{err: sentinel})
	if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("confirmation error=%v", err)
	}
	restored, err := db.OpenReadOnly(target)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := restored.ListTasks(context.Background(), ports.TaskFilter{IncludeDone: true, IncludeCancelled: true})
	closeErr := restored.Close()
	if err != nil || closeErr != nil || !hasTaskTitle(tasks, "Conservar") || hasTaskTitle(tasks, "No conservar") {
		t.Fatalf("rollback tasks=%#v err=%v close=%v", tasks, err, closeErr)
	}
	paths, err := registrydb.ProjectsReadOnly(context.Background(), filepath.Join(config, "tasks", "registry.sqlite"))
	if err != nil || len(paths) != 0 {
		t.Fatalf("rollback registry paths=%v err=%v", paths, err)
	}
}

func TestBackupDoesNotMigrateCompatibleSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "old.tasks")
	createDataCommandStore(t, source, "Antigua")
	database, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec("DROP TABLE project_config; PRAGMA user_version=1"); err != nil {
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(root, "old.tasks.bak")
	if err = backupTasks(context.Background(), root, invocation{kind: commandBackup, project: source, projectSet: true, source: backup}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{"source": source, "backup": backup} {
		inspection, inspectErr := db.Inspect(context.Background(), path)
		if inspectErr != nil || inspection.SchemaVersion != 1 {
			t.Fatalf("%s inspection=%#v err=%v", name, inspection, inspectErr)
		}
	}
}

func TestGlobalDoctorFullyClassifiesRegisteredProjects(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "config")
	t.Setenv("XDG_CONFIG_HOME", config)
	global := filepath.Join(config, "tasks", "global.sqlite")
	createDataCommandStore(t, global, "Global")
	old := filepath.Join(root, "old.tasks")
	createDataCommandStore(t, old, "Antigua")
	oldDB, err := sql.Open("sqlite", old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = oldDB.Exec("DROP TABLE project_config; PRAGMA user_version=1"); err != nil {
		t.Fatal(err)
	}
	if err = oldDB.Close(); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(root, "broken.tasks")
	createDataCommandStore(t, broken, "Rota")
	brokenDB, err := sql.Open("sqlite", broken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = brokenDB.Exec("PRAGMA foreign_keys=OFF; INSERT INTO dependencies(task_id,depends_on_id) VALUES(1,999999)"); err != nil {
		t.Fatal(err)
	}
	if err = brokenDB.Close(); err != nil {
		t.Fatal(err)
	}
	index, err := registrydb.Open(filepath.Join(config, "tasks", "registry.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{old, broken} {
		if err = index.Register(context.Background(), path); err != nil {
			t.Fatal(err)
		}
	}
	if err = index.Close(); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err = doctorTasks(context.Background(), root, invocation{kind: commandDoctor, global: true, structured: true}, &output)
	if !errors.Is(err, errDoctorIssues) || !strings.Contains(output.String(), "se migrará") || !strings.Contains(output.String(), "claves_foráneas=1") {
		t.Fatalf("registered projects doctor err=%v output=%s", err, output.String())
	}
}

func TestDoctorWarnsWhenOwnerCannotReadAndWrite(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "permissions.tasks")
	createDataCommandStore(t, project, "Permisos")
	if err := os.Chmod(project, 0400); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err := doctorTasks(context.Background(), root, invocation{kind: commandDoctor, project: project, projectSet: true, structured: true}, &output)
	if !errors.Is(err, errDoctorIssues) || !strings.Contains(output.String(), "se recomienda 0600") {
		t.Fatalf("0400 doctor err=%v output=%s", err, output.String())
	}
}
