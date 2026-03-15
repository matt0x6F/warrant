package components

import "github.com/charmbracelet/lipgloss"

var (
	Primary  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	Secondary = lipgloss.NewStyle()
	Muted    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	Error    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	Success  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	Border   = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("8"))
)

// StateStyle returns a style for ticket state. pending=yellow, done=green, failed=red, etc.
func StateStyle(state string) lipgloss.Style {
	switch state {
	case "pending":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	case "claimed", "executing":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	case "awaiting_review":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	case "done":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	case "blocked", "needs_human":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	case "failed":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	default:
		return Muted
	}
}
