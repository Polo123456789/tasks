package trash

import (
	"strings"
	"testing"

	"github.com/Polo123456789/tasks/internal/tui/presenter"
)

func TestViewShowsProjectAndExpiration(t *testing.T) {
	view := View([]presenter.Task{{Title: "Deleted", Project: "alpha", DeletedAt: "2026-07-01"}}, 0, 20)
	for _, expected := range []string{"Deleted [alpha]", "eliminada 2026-07-01", "vence 2026-07-31"} {
		if !strings.Contains(view, expected) {
			t.Errorf("missing %q:\n%s", expected, view)
		}
	}
}

func TestViewMarksSelectedTrashItem(t *testing.T) {
	view := View([]presenter.Task{{Title: "First"}, {Title: "Second"}}, 1, 20)
	if !strings.Contains(view, "› Second") {
		t.Fatalf("selected trash item is not visible:\n%s", view)
	}
}
