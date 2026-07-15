package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ConflictError struct {
	Directory string
	Files     []string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("multiple .tasks files in %s: %s", e.Directory, strings.Join(e.Files, ", "))
}
func Discover(start string) (string, error) {
	dir, e := filepath.Abs(start)
	if e != nil {
		return "", e
	}
	if info, e := os.Stat(dir); e == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		entries, e := os.ReadDir(dir)
		if e != nil {
			return "", e
		}
		var found []string
		for _, v := range entries {
			if !v.IsDir() && strings.EqualFold(filepath.Ext(v.Name()), ".tasks") {
				found = append(found, filepath.Join(dir, v.Name()))
			}
		}
		sort.Strings(found)
		if len(found) > 1 {
			return "", &ConflictError{dir, found}
		}
		if len(found) == 1 {
			return filepath.EvalSymlinks(found[0])
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}
func ValidateProjectName(name string) error {
	if filepath.Base(name) != name || filepath.Ext(name) != ".tasks" || strings.TrimSuffix(name, ".tasks") == "" {
		return fmt.Errorf("project name must be a filename ending in .tasks")
	}
	return nil
}
