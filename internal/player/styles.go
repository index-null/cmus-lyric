package player

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	artistStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AD8EE6")).
			Padding(0, 1)

	currentStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4"))

	currentTransStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#AD8EE6"))

	passedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	passedTransStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#555555"))

	upcomingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CCCCCC"))

	upcomingTransStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Bold(true)

	fetchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F1FA8C"))

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA"))

	noLyricStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Italic(true)

	statusPausedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F1FA8C")).
				Bold(true)
)
