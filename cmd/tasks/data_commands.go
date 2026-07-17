package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Polo123456789/tasks/internal/adapters/filesystem"
	"github.com/Polo123456789/tasks/internal/adapters/registry"
	db "github.com/Polo123456789/tasks/internal/adapters/sqlite"
	"github.com/Polo123456789/tasks/internal/projectimport"
)

var errDoctorIssues = errors.New("doctor found issues")

func selectedStorePath(cwd string, invocation invocation, allowMissing bool) (addDestination, string, error) {
	if invocation.global {
		config, err := os.UserConfigDir()
		if err != nil {
			return addDestination{}, "", err
		}
		path := filepath.Join(config, "tasks", "global.sqlite")
		if !allowMissing {
			if err = requireRegularFile(path); err != nil {
				return addDestination{}, "", fmt.Errorf("open global store %s: %w", path, err)
			}
		}
		return addDestination{Kind: "global"}, path, nil
	}
	if invocation.projectSet {
		if allowMissing {
			path, err := projectDestinationPath(cwd, invocation.project)
			return addDestination{Kind: "project", Path: path}, path, err
		}
		path, err := existingProjectPath(cwd, invocation.project)
		return addDestination{Kind: "project", Path: path}, path, err
	}
	project, err := filesystem.Discover(cwd)
	if err != nil {
		return addDestination{}, "", err
	}
	if project == "" {
		return addDestination{}, "", fmt.Errorf("no project detected; select the global store explicitly with --global or provide --project")
	}
	return addDestination{Kind: "project", Path: project}, project, nil
}

func projectDestinationPath(cwd, path string) (string, error) {
	if filepath.Ext(path) != ".tasks" {
		return "", fmt.Errorf("project path must end in .tasks")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err == nil {
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("project path must reference a regular file")
		}
		return filepath.EvalSymlinks(absolute)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return absolute, nil
}

func requireRegularFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file")
	}
	return nil
}

func exportTasks(ctx context.Context, cwd string, invocation invocation, output io.Writer) (resultErr error) {
	_, path, err := selectedStorePath(cwd, invocation, false)
	if err != nil {
		return err
	}
	store, err := db.OpenReadOnly(path)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, store.Close()) }()
	document, err := store.ExportProject(ctx)
	if err != nil {
		return err
	}
	switch invocation.format {
	case "json":
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(document)
	case "markdown":
		_, err = io.WriteString(output, renderExportMarkdown(document))
		return err
	case "csv":
		return writeExportCSV(output, document)
	default:
		return fmt.Errorf("unsupported export format %q", invocation.format)
	}
}

func renderExportMarkdown(document projectimport.Document) string {
	statusNames := make(map[string]string, len(document.Statuses)+2)
	for _, status := range document.Statuses {
		statusNames[status.Key] = status.Name
	}
	statusNames[projectimport.StatusDone] = "Finalizada"
	statusNames[projectimport.StatusCancelled] = "Cancelada"
	var output strings.Builder
	output.WriteString("# Tareas exportadas\n\n")
	if len(document.Tasks) == 0 {
		output.WriteString("_No hay tareas activas._\n")
		return output.String()
	}
	for _, task := range document.Tasks {
		status := statusNames[task.Status]
		if status == "" {
			status = task.Status
		}
		fmt.Fprintf(&output, "- **%s** · %s · prioridad %s\n", cleanExportLine(task.Title), cleanExportLine(status), task.Priority)
		if task.Start != nil || task.Due != nil || task.Recurrence != nil {
			var planning []string
			if task.Start != nil {
				planning = append(planning, "inicio "+*task.Start)
			}
			if task.Due != nil {
				planning = append(planning, "vence "+*task.Due)
			}
			if task.Recurrence != nil {
				planning = append(planning, "recurrencia "+*task.Recurrence)
			}
			fmt.Fprintf(&output, "  - Planificación: %s\n", strings.Join(planning, "; "))
		}
		for _, subtask := range task.Subtasks {
			fmt.Fprintf(&output, "  - Subtarea: %s (%s)\n", cleanExportLine(subtask.Title), cleanExportLine(statusNames[subtask.Status]))
		}
		if len(task.DependsOn) > 0 {
			fmt.Fprintf(&output, "  - Depende de: %s\n", strings.Join(task.DependsOn, ", "))
		}
		if strings.TrimSpace(task.Markdown) != "" {
			for _, line := range strings.Split(task.Markdown, "\n") {
				fmt.Fprintf(&output, "  > %s\n", cleanExportLine(line))
			}
		}
	}
	return output.String()
}

func cleanExportLine(value string) string {
	return strings.TrimSpace(strings.NewReplacer("\r", " ", "\n", " ").Replace(value))
}

func writeExportCSV(output io.Writer, document projectimport.Document) error {
	writer := csv.NewWriter(output)
	if err := writer.Write([]string{"key", "title", "status", "priority", "start", "due", "recurrence", "subtasks", "depends_on", "markdown"}); err != nil {
		return err
	}
	for _, task := range document.Tasks {
		start, due, recurrence := "", "", ""
		if task.Start != nil {
			start = *task.Start
		}
		if task.Due != nil {
			due = *task.Due
		}
		if task.Recurrence != nil {
			recurrence = *task.Recurrence
		}
		subtasks := make([]string, 0, len(task.Subtasks))
		for _, subtask := range task.Subtasks {
			subtasks = append(subtasks, subtask.Title+" ["+subtask.Status+"]")
		}
		if err := writer.Write([]string{task.Key, task.Title, task.Status, task.Priority, start, due, recurrence, strings.Join(subtasks, "; "), strings.Join(task.DependsOn, "; "), task.Markdown}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func backupTasks(ctx context.Context, cwd string, invocation invocation, output io.Writer) (resultErr error) {
	_, source, err := selectedStorePath(cwd, invocation, false)
	if err != nil {
		return err
	}
	target, err := outputDatabasePath(cwd, invocation.source)
	if err != nil {
		return err
	}
	if samePath(source, target) {
		return fmt.Errorf("backup destination must differ from the selected store")
	}
	store, err := db.OpenSnapshotSource(source)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, store.Close()) }()
	if err = store.Backup(ctx, target); err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "Respaldo creado: %s\n", target)
	return err
}

func outputDatabasePath(cwd, value string) (string, error) {
	if !filepath.IsAbs(value) {
		value = filepath.Join(cwd, value)
	}
	return filepath.Abs(value)
}

func samePath(left, right string) bool {
	leftAbsolute, leftErr := filepath.Abs(left)
	rightAbsolute, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && leftAbsolute == rightAbsolute
}

func restoreTasks(ctx context.Context, cwd string, invocation invocation, output io.Writer) error {
	destination, target, err := selectedStorePath(cwd, invocation, true)
	if err != nil {
		return err
	}
	source, err := existingRestoreSource(cwd, invocation.source)
	if err != nil {
		return err
	}
	if samePath(source, target) {
		return fmt.Errorf("restore source and destination must differ")
	}
	if existing, statErr := os.Lstat(target); statErr == nil && existing.Mode().IsRegular() && !invocation.force {
		return fmt.Errorf("restore destination %s already exists; use --force to replace it", target)
	}
	sourceCheck, err := db.OpenSnapshotSource(source)
	if err != nil {
		return fmt.Errorf("restore source is corrupt, incompatible, active, or unreadable: %w", err)
	}
	if err = sourceCheck.Close(); err != nil {
		return err
	}
	var index *registry.SQLite
	var indexPath string
	registeredBefore := false
	if destination.Kind == "project" {
		config, configErr := os.UserConfigDir()
		if configErr != nil {
			return fmt.Errorf("locate registry: %w", configErr)
		}
		indexPath = filepath.Join(config, "tasks", "registry.sqlite")
		index, err = registry.Open(indexPath)
		if err != nil {
			return fmt.Errorf("open registry: %w", err)
		}
		if err = index.CheckWritable(ctx); err != nil {
			return errors.Join(fmt.Errorf("registry is not writable: %w", err), index.Close())
		}
		projects, projectsErr := index.Projects(ctx)
		if projectsErr != nil {
			return errors.Join(projectsErr, index.Close())
		}
		for _, project := range projects {
			registeredBefore = registeredBefore || samePath(project, target)
		}
	}
	undo, err := prepareRestoreUndo(target)
	if err != nil {
		if index != nil {
			err = errors.Join(err, index.Close())
		}
		return err
	}
	defer undo.cleanup()
	if err = restoreDatabase(ctx, source, target, invocation.force); err != nil {
		if index != nil {
			err = errors.Join(err, index.Close())
		}
		return err
	}
	if index != nil {
		registerErr := index.Register(ctx, target)
		closeErr := index.Close()
		if registerErr != nil || closeErr != nil {
			rollbackErr := undo.rollback()
			if !registeredBefore {
				rollbackErr = errors.Join(rollbackErr, undoRegistryRegistration(ctx, indexPath, target))
			}
			return fmt.Errorf("register restored project; destination rolled back: %w", errors.Join(registerErr, closeErr, rollbackErr))
		}
	}
	_, err = fmt.Fprintf(output, "Restaurado: %s (%s)\n", target, destination.Kind)
	if err == nil {
		return nil
	}
	rollbackErr := undo.rollback()
	if indexPath != "" && !registeredBefore {
		rollbackErr = errors.Join(rollbackErr, undoRegistryRegistration(ctx, indexPath, target))
	}
	return fmt.Errorf("write restore confirmation; destination rolled back: %w", errors.Join(err, rollbackErr))
}

type restoreUndo struct {
	target   string
	snapshot string
	existed  bool
}

func prepareRestoreUndo(target string) (undo restoreUndo, resultErr error) {
	undo.target = target
	info, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return undo, nil
	}
	if err != nil {
		return undo, err
	}
	if !info.Mode().IsRegular() {
		return undo, fmt.Errorf("restore destination is not a regular file")
	}
	if err = db.EnsureQuiescent(target); err != nil {
		return undo, fmt.Errorf("prepare destination rollback: %w", err)
	}
	input, err := os.Open(target)
	if err != nil {
		return undo, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(target), ".tasks-rollback-")
	if err != nil {
		return undo, errors.Join(err, input.Close())
	}
	undo.snapshot = temporary.Name()
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, input.Close(), temporary.Close(), os.Remove(undo.snapshot))
		}
	}()
	if err = temporary.Chmod(0600); err != nil {
		return undo, err
	}
	_, copyErr := io.Copy(temporary, input)
	resultErr = errors.Join(copyErr, temporary.Sync(), temporary.Close(), input.Close())
	if resultErr != nil {
		return undo, resultErr
	}
	undo.existed = true
	return undo, nil
}

func (u *restoreUndo) rollback() error {
	if u.existed {
		if u.snapshot == "" {
			return fmt.Errorf("restore rollback snapshot is unavailable")
		}
		if err := os.Rename(u.snapshot, u.target); err != nil {
			return fmt.Errorf("restore previous destination: %w", err)
		}
		u.snapshot = ""
		return nil
	}
	if err := os.Remove(u.target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove newly restored destination: %w", err)
	}
	return nil
}

func (u *restoreUndo) cleanup() {
	if u.snapshot != "" {
		_ = os.Remove(u.snapshot)
		u.snapshot = ""
	}
}

func undoRegistryRegistration(ctx context.Context, path, target string) (resultErr error) {
	index, err := registry.Open(path)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, index.Close()) }()
	return index.Unregister(ctx, target)
}

func existingRestoreSource(cwd, value string) (string, error) {
	if !filepath.IsAbs(value) {
		value = filepath.Join(cwd, value)
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	if err = requireRegularFile(absolute); err != nil {
		return "", fmt.Errorf("open restore source %s: %w", absolute, err)
	}
	return filepath.EvalSymlinks(absolute)
}

func restoreDatabase(ctx context.Context, source, target string, force bool) (resultErr error) {
	existing, statErr := os.Lstat(target)
	if statErr == nil {
		if !existing.Mode().IsRegular() {
			return fmt.Errorf("restore destination is not a regular file")
		}
		if !force {
			return fmt.Errorf("restore destination %s already exists; use --force to replace it", target)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	if err := db.EnsureQuiescent(target); err != nil && !(errors.Is(statErr, os.ErrNotExist) && errors.Is(err, os.ErrNotExist)) {
		return fmt.Errorf("restore destination is active or requires recovery: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	sourceStore, err := db.OpenSnapshotSource(source)
	if err != nil {
		return fmt.Errorf("restore source is corrupt, incompatible, active, or unreadable: %w", err)
	}
	temporaryPath, snapshotErr := sourceStore.Snapshot(ctx, filepath.Dir(target))
	closeSourceErr := sourceStore.Close()
	if err = errors.Join(snapshotErr, closeSourceErr); err != nil {
		return err
	}
	defer func() { _ = os.Remove(temporaryPath) }()
	staged, err := db.Open(temporaryPath)
	if err != nil {
		return fmt.Errorf("validate staged restore: %w", err)
	}
	if err = staged.Close(); err != nil {
		return err
	}
	stagedInspection, err := db.Inspect(ctx, temporaryPath)
	if err != nil || stagedInspection.SchemaVersion != db.SchemaVersion || stagedInspection.Integrity != "ok" || stagedInspection.ForeignKeyViolations != 0 || len(stagedInspection.MissingTables) != 0 {
		return fmt.Errorf("staged restore failed validation: inspection=%#v error=%v", stagedInspection, err)
	}
	if err = db.EnsureQuiescent(target); err != nil && !(errors.Is(statErr, os.ErrNotExist) && errors.Is(err, os.ErrNotExist)) {
		return fmt.Errorf("restore destination became active before publication: %w", err)
	}
	if force && statErr == nil {
		if err = os.Rename(temporaryPath, target); err != nil {
			return fmt.Errorf("publish restored database: %w", err)
		}
	} else {
		if err = os.Link(temporaryPath, target); err != nil {
			return fmt.Errorf("publish restored database: %w", err)
		}
	}
	return nil
}

type doctorCheck struct {
	Name    string `json:"name"`
	Level   string `json:"level"`
	Kind    string `json:"kind,omitempty"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

type doctorReport struct {
	OK          bool           `json:"ok"`
	Destination addDestination `json:"destination"`
	Checks      []doctorCheck  `json:"checks"`
}

func doctorTasks(ctx context.Context, cwd string, invocation invocation, output io.Writer) error {
	destination, path, err := selectedStorePath(cwd, invocation, true)
	if err != nil {
		return err
	}
	report := doctorReport{OK: true, Destination: destination}
	if destination.Kind == "global" {
		report.Destination.Path = path
	}
	inspectDoctorStore(ctx, &report, "almacén", path)
	if destination.Kind == "global" {
		config, configErr := os.UserConfigDir()
		if configErr != nil {
			addDoctorCheck(&report, doctorCheck{Name: "registro", Level: "error", Kind: "repairable", Message: configErr.Error()})
		} else {
			inspectDoctorRegistry(ctx, &report, filepath.Join(config, "tasks", "registry.sqlite"))
		}
	}
	if invocation.structured {
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		if err = encoder.Encode(report); err != nil {
			return err
		}
	} else if err = writeDoctorHuman(output, report); err != nil {
		return err
	}
	if !report.OK {
		return errDoctorIssues
	}
	return nil
}

func inspectDoctorStore(ctx context.Context, report *doctorReport, name, path string) {
	info, err := os.Stat(path)
	if err != nil {
		kind := "repairable"
		message := "no existe; puede crearse o restaurarse"
		if !errors.Is(err, os.ErrNotExist) {
			message = err.Error()
		}
		if name == "proyecto-registrado" {
			message = "no disponible: " + message
		}
		addDoctorCheck(report, doctorCheck{Name: name, Level: "error", Kind: kind, Path: path, Message: message})
		return
	}
	if !info.Mode().IsRegular() {
		addDoctorCheck(report, doctorCheck{Name: name, Level: "error", Kind: "repairable", Path: path, Message: "no es un archivo regular"})
		return
	}
	inspectDoctorDirectory(report, name+".directorio", filepath.Dir(path))
	if info.Mode().Perm()&0600 != 0600 || info.Mode().Perm()&0077 != 0 {
		addDoctorCheck(report, doctorCheck{Name: name + ".permisos", Level: "warning", Kind: "repairable", Path: path, Message: fmt.Sprintf("permisos %04o; se recomienda 0600", info.Mode().Perm())})
	} else {
		addDoctorCheck(report, doctorCheck{Name: name + ".permisos", Level: "ok", Path: path, Message: fmt.Sprintf("permisos privados %04o", info.Mode().Perm())})
	}
	inspection, err := db.Inspect(ctx, path)
	if err != nil {
		addDoctorCheck(report, doctorCheck{Name: name + ".sqlite", Level: "error", Kind: diagnosticErrorKind(err), Path: path, Message: err.Error()})
		return
	}
	switch {
	case inspection.SchemaVersion > inspection.SupportedVersion:
		addDoctorCheck(report, doctorCheck{Name: name + ".esquema", Level: "error", Kind: "incompatible", Path: path, Message: fmt.Sprintf("versión %d, máximo compatible %d", inspection.SchemaVersion, inspection.SupportedVersion)})
	case inspection.SchemaVersion < inspection.SupportedVersion:
		addDoctorCheck(report, doctorCheck{Name: name + ".esquema", Level: "warning", Kind: "repairable", Path: path, Message: fmt.Sprintf("versión %d; se migrará a %d al abrir en modo escritura", inspection.SchemaVersion, inspection.SupportedVersion)})
	default:
		addDoctorCheck(report, doctorCheck{Name: name + ".esquema", Level: "ok", Path: path, Message: fmt.Sprintf("versión %d compatible", inspection.SchemaVersion)})
	}
	if inspection.Integrity != "ok" || inspection.ForeignKeyViolations != 0 || len(inspection.MissingTables) != 0 {
		addDoctorCheck(report, doctorCheck{Name: name + ".integridad", Level: "error", Kind: "corruption", Path: path, Message: fmt.Sprintf("integrity=%q; claves_foráneas=%d; tablas_faltantes=%v", inspection.Integrity, inspection.ForeignKeyViolations, inspection.MissingTables)})
	} else {
		addDoctorCheck(report, doctorCheck{Name: name + ".integridad", Level: "ok", Path: path, Message: "integrity_check y foreign_key_check correctos"})
	}
}

func inspectDoctorRegistry(ctx context.Context, report *doctorReport, path string) {
	info, err := os.Stat(path)
	if err != nil {
		message := "no existe; se creará cuando se registre un proyecto"
		if !errors.Is(err, os.ErrNotExist) {
			message = err.Error()
		}
		addDoctorCheck(report, doctorCheck{Name: "registro", Level: "warning", Kind: "repairable", Path: path, Message: message})
		return
	}
	if !info.Mode().IsRegular() {
		addDoctorCheck(report, doctorCheck{Name: "registro", Level: "error", Kind: "repairable", Path: path, Message: "no es un archivo regular"})
		return
	}
	inspectDoctorDirectory(report, "registro.directorio", filepath.Dir(path))
	if info.Mode().Perm()&0600 != 0600 || info.Mode().Perm()&0077 != 0 {
		addDoctorCheck(report, doctorCheck{Name: "registro.permisos", Level: "warning", Kind: "repairable", Path: path, Message: fmt.Sprintf("permisos %04o; se recomienda 0600", info.Mode().Perm())})
	} else {
		addDoctorCheck(report, doctorCheck{Name: "registro.permisos", Level: "ok", Path: path, Message: "permisos privados"})
	}
	inspection, err := registry.InspectReadOnly(ctx, path)
	if err != nil {
		addDoctorCheck(report, doctorCheck{Name: "registro", Level: "error", Kind: diagnosticErrorKind(err), Path: path, Message: err.Error()})
		return
	}
	if inspection.Integrity != "ok" {
		addDoctorCheck(report, doctorCheck{Name: "registro.integridad", Level: "error", Kind: "corruption", Path: path, Message: fmt.Sprintf("integrity_check=%q", inspection.Integrity)})
	} else {
		addDoctorCheck(report, doctorCheck{Name: "registro.integridad", Level: "ok", Path: path, Message: "integrity_check correcto"})
	}
	addDoctorCheck(report, doctorCheck{Name: "registro", Level: "ok", Path: path, Message: fmt.Sprintf("%d proyecto(s) registrado(s)", len(inspection.Paths))})
	for _, project := range inspection.Paths {
		inspectDoctorStore(ctx, report, "proyecto-registrado", project)
	}
}

func inspectDoctorDirectory(report *doctorReport, name, path string) {
	info, err := os.Stat(path)
	if err != nil {
		addDoctorCheck(report, doctorCheck{Name: name, Level: "warning", Kind: "repairable", Path: path, Message: err.Error()})
		return
	}
	if !info.IsDir() || info.Mode().Perm()&0700 != 0700 {
		addDoctorCheck(report, doctorCheck{Name: name, Level: "warning", Kind: "repairable", Path: path, Message: fmt.Sprintf("permisos %04o; el propietario necesita lectura, escritura y acceso", info.Mode().Perm())})
		return
	}
	addDoctorCheck(report, doctorCheck{Name: name, Level: "ok", Path: path, Message: fmt.Sprintf("directorio accesible %04o", info.Mode().Perm())})
}

func diagnosticErrorKind(err error) string {
	message := strings.ToLower(err.Error())
	if errors.Is(err, os.ErrPermission) || errors.Is(err, db.ErrActiveSidecars) || strings.Contains(message, "locked") || strings.Contains(message, "busy") || strings.Contains(message, "temporar") {
		return "repairable"
	}
	return "corruption"
}

func addDoctorCheck(report *doctorReport, check doctorCheck) {
	report.Checks = append(report.Checks, check)
	if check.Level != "ok" {
		report.OK = false
	}
}

func writeDoctorHuman(output io.Writer, report doctorReport) error {
	label := report.Destination.Kind
	if report.Destination.Path != "" {
		label += " " + report.Destination.Path
	}
	if _, err := fmt.Fprintf(output, "Doctor · %s\n", label); err != nil {
		return err
	}
	levels := map[string]string{"ok": "OK", "warning": "ADVERTENCIA", "error": "ERROR"}
	for _, check := range report.Checks {
		kind := ""
		if check.Kind != "" {
			kind = " · " + check.Kind
		}
		path := ""
		if check.Path != "" {
			path = " · " + check.Path
		}
		if _, err := fmt.Fprintf(output, "[%s%s] %s%s: %s\n", levels[check.Level], kind, check.Name, path, check.Message); err != nil {
			return err
		}
	}
	result := "correcto"
	if !report.OK {
		result = "se encontraron problemas"
	}
	_, err := fmt.Fprintf(output, "Resultado: %s\n", result)
	return err
}
