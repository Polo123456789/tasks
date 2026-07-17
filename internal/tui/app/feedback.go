package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

type feedbackKind uint8

const (
	feedbackSuccess feedbackKind = iota
	feedbackWarning
)

type undoKind uint8

const (
	undoStatus undoKind = iota
	undoTrash
)

type undoAction struct {
	kind             undoKind
	source, title    string
	id, version      int64
	previousStatusID int64
}

type undoFinished struct {
	requestID uint64
	action    undoAction
	task      domain.Task
	err       error
}

type conflictState struct {
	requestID uint64
	source    string
	id        int64
	title     string
}

type conflictReviewed struct {
	conflict conflictState
	task     domain.Task
	err      error
}

func inferredFeedbackKind(message string) feedbackKind {
	for _, prefix := range []string{"No se ", "No hay ", "Selecciona ", "El origen ", "Algunos ", "Eliminación cancelada"} {
		if strings.HasPrefix(message, prefix) {
			return feedbackWarning
		}
	}
	return feedbackSuccess
}

func (m Model) feedbackLine() string {
	lines := make([]string, 0, 2)
	if m.err != nil {
		message := friendlyError(m.err)
		if errors.Is(m.err, domain.ErrConflict) {
			message = "conflicto: el elemento cambió en otra sesión"
		}
		line := "✗ ERROR     " + message
		if m.conflict != nil {
			line += " · r recargar · v revisar cambio · Esc conservar vista"
		}
		lines = append(lines, theme.Help.Foreground(theme.Danger).Render(line))
	}
	if m.notice != "" {
		kind := m.noticeKind
		if kind != feedbackWarning {
			kind = inferredFeedbackKind(m.notice)
		}
		if kind == feedbackWarning {
			lines = append(lines, theme.Help.Foreground(theme.Warning).Render("⚠ ADVERTENCIA "+m.notice))
		} else {
			lines = append(lines, theme.Help.Foreground(theme.Success).Render("✓ ÉXITO     "+m.notice))
		}
	}
	if m.refreshErr != nil {
		lines = append(lines, theme.Help.Foreground(theme.Warning).Render("⚠ ADVERTENCIA no se pudo actualizar la vista: "+friendlyError(m.refreshErr)))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) clearTransientFeedback() {
	m.notice = ""
	m.noticeKind = feedbackSuccess
	m.refreshErr = nil
	if m.conflict == nil && m.loadedOnce {
		m.err = nil
	}
}

func (m Model) undoCommand(requestID uint64, action undoAction) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var task domain.Task
		var err error
		switch action.kind {
		case undoStatus:
			task, err = m.backend.SetStatus(ctx, action.source, action.id, action.previousStatusID, action.version)
		case undoTrash:
			task, err = m.backend.Restore(ctx, action.source, action.id, action.version)
		default:
			err = fmt.Errorf("acción de deshacer inválida")
		}
		return undoFinished{requestID: requestID, action: action, task: task, err: err}
	}
}

func (m *Model) setConflict(source string, id int64, title string) {
	m.nextConflictRequestID++
	m.conflict = &conflictState{requestID: m.nextConflictRequestID, source: source, id: id, title: title}
}

func (m Model) reviewConflictCommand(conflict conflictState) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		task, err := m.backend.Detail(ctx, conflict.source, conflict.id)
		return conflictReviewed{conflict: conflict, task: task, err: err}
	}
}
