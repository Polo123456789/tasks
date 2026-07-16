package theme

import "github.com/charmbracelet/lipgloss"

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
