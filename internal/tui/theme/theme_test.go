package theme

import (
	"testing"

	"github.com/Polo123456789/tasks/internal/domain"
)

func TestStatusColorsAreSemanticAndStable(t *testing.T) {
	done := StatusColor(domain.StatusDone, "Finalizada")
	cancelled := StatusColor(domain.StatusCancelled, "Cancelada")
	if done != Success {
		t.Fatalf("done color=%q, want success=%q", done, Success)
	}
	if cancelled != Muted || done == cancelled {
		t.Fatalf("cancelled=%q done=%q", cancelled, done)
	}
	first := StatusColor(domain.StatusNormal, "En progreso")
	if first != StatusColor(domain.StatusNormal, "En progreso") {
		t.Fatal("normal status color is not stable")
	}
}
