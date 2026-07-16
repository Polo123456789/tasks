package theme

import (
	"hash/fnv"
	"strings"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/charmbracelet/lipgloss"
)

var (
	Primary  = lipgloss.Color("#7D56F4")
	Muted    = lipgloss.Color("#6C7086")
	Danger   = lipgloss.Color("#F38BA8")
	Success  = lipgloss.Color("#A6E3A1")
	Title    = lipgloss.NewStyle().Bold(true).Foreground(Primary)
	Border   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Muted).Padding(0, 1)
	Selected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#11111B")).Background(Primary)
	Help     = lipgloss.NewStyle().Foreground(Muted)
)

var statusPalette = [...]lipgloss.Color{
	"#89B4FA", // blue
	"#CBA6F7", // mauve
	"#94E2D5", // teal
	"#FAB387", // peach
	"#F9E2AF", // yellow
	"#74C7EC", // sapphire
	"#B4BEFE", // lavender
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
