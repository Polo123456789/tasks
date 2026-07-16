package settings

import (
	"strings"
	"testing"

	"github.com/Polo123456789/tasks/internal/domain"
)

func TestViewMarksInitialAndSpecialStatuses(t *testing.T) {
	view := View([]domain.Status{
		{ID: 1, Name: "Pending", Kind: domain.StatusNormal, Position: 1, Initial: true},
		{ID: 2, Name: "Cancelled", Kind: domain.StatusCancelled, Position: 2},
		{ID: 3, Name: "Done", Kind: domain.StatusDone, Position: 3},
	}, 0, 20)
	for _, expected := range []string{"Pending", "inicial", "especial · cancelada", "especial · finalizada"} {
		if !strings.Contains(view, expected) {
			t.Errorf("missing %q:\n%s", expected, view)
		}
	}
}
