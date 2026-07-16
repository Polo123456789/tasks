package editor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type Session struct {
	cmd      *exec.Cmd
	filename string
}

func NewSession(ctx context.Context, content string) (*Session, error) {
	command := os.Getenv("VISUAL")
	if command == "" {
		command = os.Getenv("EDITOR")
	}
	if command == "" {
		return nil, fmt.Errorf("configure $VISUAL or $EDITOR")
	}
	f, e := os.CreateTemp("", "tasks-*.md")
	if e != nil {
		return nil, e
	}
	name := f.Name()
	if _, e = f.WriteString(content); e != nil {
		f.Close()
		os.Remove(name)
		return nil, e
	}
	if e = f.Close(); e != nil {
		os.Remove(name)
		return nil, e
	}
	parts := strings.Fields(command)
	if len(parts) == 0 {
		os.Remove(name)
		return nil, fmt.Errorf("empty editor command")
	}
	return &Session{cmd: exec.CommandContext(ctx, parts[0], append(parts[1:], name)...), filename: name}, nil
}

func (s *Session) Run() error            { return s.cmd.Run() }
func (s *Session) SetStdin(r io.Reader)  { s.cmd.Stdin = r }
func (s *Session) SetStdout(w io.Writer) { s.cmd.Stdout = w }
func (s *Session) SetStderr(w io.Writer) { s.cmd.Stderr = w }
func (s *Session) Path() string          { return s.filename }
func (s *Session) Read() (string, error) {
	b, err := os.ReadFile(s.filename)
	return string(b), err
}
func (s *Session) Cleanup() error {
	if err := os.Remove(s.filename); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
func (s *Session) Finish(runErr error) (string, error) {
	if runErr != nil {
		return "", errors.Join(fmt.Errorf("editor: %w", runErr), s.Cleanup())
	}
	content, readErr := s.Read()
	return content, errors.Join(readErr, s.Cleanup())
}

func Edit(ctx context.Context, content string) (string, error) {
	session, err := NewSession(ctx, content)
	if err != nil {
		return "", err
	}
	session.SetStdin(os.Stdin)
	session.SetStdout(os.Stdout)
	session.SetStderr(os.Stderr)
	return session.Finish(session.Run())
}
