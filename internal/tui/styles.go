package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99"))

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	activeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	scoreStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	pathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	headingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("141"))

	snippetStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))

	inputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)
)
