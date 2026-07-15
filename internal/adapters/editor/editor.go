package editor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func Edit(ctx context.Context, content string) (string, error) {
	command := os.Getenv("VISUAL")
	if command == "" {
		command = os.Getenv("EDITOR")
	}
	if command == "" {
		return "", fmt.Errorf("configure $VISUAL or $EDITOR")
	}
	f, e := os.CreateTemp("", "tasks-*.md")
	if e != nil {
		return "", e
	}
	name := f.Name()
	defer os.Remove(name)
	if _, e = f.WriteString(content); e != nil {
		f.Close()
		return "", e
	}
	if e = f.Close(); e != nil {
		return "", e
	}
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty editor command")
	}
	cmd := exec.CommandContext(ctx, parts[0], append(parts[1:], name)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if e = cmd.Run(); e != nil {
		return "", fmt.Errorf("editor: %w", e)
	}
	b, e := os.ReadFile(name)
	return string(b), e
}
