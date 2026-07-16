package history

import (
	"strings"
	"testing"
	"time"

	"github.com/Polo123456789/tasks/internal/domain"
)

func TestViewShowsChronologicalEventData(t *testing.T) {
	view := View([]domain.HistoryEvent{{Kind: "completed", Detail: "manual", CreatedAt: time.Date(2026, 7, 15, 12, 30, 0, 0, time.Local)}}, 20)
	for _, expected := range []string{"2026-07-15 12:30", "finalizada", "manual"} {
		if !strings.Contains(view, expected) {
			t.Errorf("missing %q:\n%s", expected, view)
		}
	}
}
