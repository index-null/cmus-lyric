package player

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	artistStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AD8EE6")).
			Padding(0, 1)

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

	gradientStart, _ = colorful.Hex("#7D56F4")
	gradientEnd, _   = colorful.Hex("#00D4AA")
)

func gradientText(text string, from, to colorful.Color, bold bool) string {
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return ""
	}
	var sb strings.Builder
	for i, r := range runes {
		t := 0.0
		if n > 1 {
			t = float64(i) / float64(n-1)
		}
		c := from.BlendLab(to, t)
		hex := c.Hex()
		s := lipgloss.NewStyle().Foreground(lipgloss.Color(hex))
		if bold {
			s = s.Bold(true)
		}
		sb.WriteString(s.Render(string(r)))
	}
	return sb.String()
}

func fadedColor(distance, maxDist int, bright, dim colorful.Color) colorful.Color {
	if maxDist <= 0 {
		return bright
	}
	t := float64(distance) / float64(maxDist)
	if t > 1 {
		t = 1
	}
	return bright.BlendLab(dim, t)
}

func lyricStyle(distance, maxDist int, bright, dim colorful.Color) lipgloss.Style {
	c := fadedColor(distance, maxDist, bright, dim)
	return lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex()))
}
