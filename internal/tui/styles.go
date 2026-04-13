package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("13"))
	descStyle    = lipgloss.NewStyle().Faint(true)
	pendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	runningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	doneStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	failStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	skipStyle    = lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("8"))
	outStyle     = lipgloss.NewStyle().Faint(true)
	timeStyle    = lipgloss.NewStyle().Faint(true)
)
