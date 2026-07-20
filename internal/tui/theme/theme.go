package theme

import (
	"hash/fnv"
	"strings"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/charmbracelet/lipgloss"
)

var (
	Primary  = lipgloss.Color("#88B8F6")
	Muted    = lipgloss.Color("#857B6F")
	Danger   = lipgloss.Color("#E5786D")
	Warning  = lipgloss.Color("#EADEAD")
	Success  = lipgloss.Color("#95E454")
	Title    = lipgloss.NewStyle().Bold(true).Foreground(Primary)
	Border   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Muted).Padding(0, 1)
	Selected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#080808")).Background(lipgloss.Color("#CAE982"))
	Help     = lipgloss.NewStyle().Foreground(Muted)
)

var statusPalette = [...]lipgloss.Color{
	"#88B8F6", // blue
	"#D787FF", // purple
	"#5FAFD7", // cyan
	"#E5786D", // red
	"#D4D987", // yellow
	"#EADEAD", // cream
	"#CAE982", // green
}

func StatusColor(kind domain.StatusKind, name string) lipgloss.Color {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if kind == domain.StatusDone || normalized == "finalizada" || normalized == "done" {
		return Success
	}
	if kind == domain.StatusCancelled || normalized == "cancelada" || normalized == "cancelled" {
		return Muted
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(normalized))
	return statusPalette[int(hash.Sum32())%len(statusPalette)]
}

func Status(kind domain.StatusKind, name string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(StatusColor(kind, name))
}

func Blocked() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Danger).Bold(true)
}
