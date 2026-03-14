package components

import "github.com/charmbracelet/lipgloss"

var (
	Primary  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	Secondary = lipgloss.NewStyle()
	Muted    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	Error    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	Border   = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("8"))
)
