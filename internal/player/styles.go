package player

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

var (
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Bold(true)

	fetchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F1FA8C"))

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	noLyricStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Italic(true)

	statusPausedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F1FA8C")).
				Bold(true)
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
		s := lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex()))
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

func lyricFadedStyle(distance, maxDist int, bright, dim colorful.Color) lipgloss.Style {
	c := fadedColor(distance, maxDist, bright, dim)
	return lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex()))
}

func gradientDivider(w int, from, to colorful.Color) string {
	if w <= 0 {
		return ""
	}
	var sb strings.Builder
	for i := range w {
		t := 0.0
		if w > 1 {
			t = float64(i) / float64(w-1)
		}
		c := from.BlendLab(to, t)
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex())).Render("─"))
	}
	return sb.String()
}
