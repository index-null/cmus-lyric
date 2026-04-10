package player

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	if m.showHelp {
		return m.renderHelp()
	}

	sections := make([]string, 0, 3)
	sections = append(sections, m.renderHeader())
	sections = append(sections, m.renderLyrics())
	sections = append(sections, m.renderFooter())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderHeader() string {
	w := m.width

	var title, artist, album string
	if m.track.Title != "" {
		title = m.track.Title
	} else if m.track.File != "" {
		pathIdx := strings.LastIndex(m.track.File, "/")
		dotIdx := strings.LastIndex(m.track.File, ".")
		if pathIdx >= 0 && dotIdx > pathIdx {
			title = m.track.File[pathIdx+1 : dotIdx]
		}
	}
	artist = m.track.Artist
	album = m.track.Album

	titleText := titleStyle.Render(title)
	info := ""
	if artist != "" {
		info += artist
	}
	if album != "" {
		if info != "" {
			info += " - "
		}
		info += album
	}
	infoText := artistStyle.Render(info)

	header := lipgloss.JoinHorizontal(lipgloss.Center, titleText, " ", infoText)

	divider := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#444444")).
		Render(strings.Repeat("─", w))

	return lipgloss.JoinVertical(lipgloss.Left, header, divider)
}

func (m Model) renderLyrics() string {
	headerH := 2
	footerH := 3
	availH := max(m.height-headerH-footerH, 1)

	centerMsg := func(msg string) string {
		pad := max(availH/2-1, 0)
		lines := make([]string, 0, availH)
		for range pad {
			lines = append(lines, "")
		}
		lines = append(lines, msg)
		for len(lines) < availH {
			lines = append(lines, "")
		}
		return lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	if m.track.Status != "playing" {
		return centerMsg(statusPausedStyle.Render("  paused / stopped"))
	}
	if m.errMsg != "" {
		return centerMsg(errorStyle.Render("  " + m.errMsg))
	}
	if m.fetchingMsg != "" {
		return centerMsg(fetchStyle.Render("  " + m.fetchingMsg))
	}
	if len(m.lyrics) == 0 {
		return centerMsg(noLyricStyle.Render("  no lyrics"))
	}

	type renderedLine struct {
		idx  int
		text string
	}

	passedBright, _ := colorful.Hex("#888888")
	passedDim, _ := colorful.Hex("#333333")
	upcomingBright, _ := colorful.Hex("#CCCCCC")
	upcomingDim, _ := colorful.Hex("#444444")
	transBright, _ := colorful.Hex("#AD8EE6")
	transDim, _ := colorful.Hex("#444444")

	const fadeRadius = 8

	var rendered []renderedLine
	for i, l := range m.lyrics {
		txt := l.Text
		if txt == "" {
			txt = "..."
		}

		var styledMain, styledTrans string
		switch {
		case i == m.curLineIdx:
			styledMain = "  " + gradientText("> "+txt, gradientStart, gradientEnd, true)
			if l.Trans != "" {
				styledTrans = "    " + gradientText(l.Trans, transBright, gradientEnd, false)
			}
		case i < m.curLineIdx:
			dist := m.curLineIdx - i
			s := lyricStyle(dist, fadeRadius, passedBright, passedDim)
			styledMain = s.Render("  " + txt)
			if l.Trans != "" {
				ts := lyricStyle(dist, fadeRadius, transBright, transDim)
				styledTrans = ts.Render("  " + l.Trans)
			}
		default:
			dist := i - m.curLineIdx
			s := lyricStyle(dist, fadeRadius, upcomingBright, upcomingDim)
			styledMain = s.Render("  " + txt)
			if l.Trans != "" {
				ts := lyricStyle(dist, fadeRadius, transBright, transDim)
				styledTrans = ts.Render("  " + l.Trans)
			}
		}

		rendered = append(rendered, renderedLine{i, styledMain})
		if styledTrans != "" {
			rendered = append(rendered, renderedLine{i, styledTrans})
		}
	}

	curRenderIdx := 0
	for ri, r := range rendered {
		if r.idx == m.curLineIdx {
			curRenderIdx = ri
			break
		}
	}

	scrollOffset := max(curRenderIdx-availH/2, 0)
	scrollOffset = min(scrollOffset, len(rendered)-availH)
	scrollOffset = max(scrollOffset, 0)

	end := min(scrollOffset+availH, len(rendered))

	lines := make([]string, 0, availH)
	for i := scrollOffset; i < end; i++ {
		lines = append(lines, rendered[i].text)
	}
	for len(lines) < availH {
		lines = append(lines, "")
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m Model) renderFooter() string {
	w := m.width

	divider := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#444444")).
		Render(strings.Repeat("─", w))

	var pct float64
	if m.track.Duration > 0 {
		pct = float64(m.track.Position) / float64(m.track.Duration)
	}
	pct = max(min(pct, 1), 0)

	bar := m.progress.ViewAs(pct)

	posStr := fmt.Sprintf("%d:%02d", m.track.Position/60, m.track.Position%60)
	durStr := fmt.Sprintf("%d:%02d", m.track.Duration/60, m.track.Duration%60)
	timeStr := footerStyle.Render(fmt.Sprintf("  %s / %s", posStr, durStr))
	helpHint := footerStyle.Render("q: quit  ?: help")
	spacer := strings.Repeat(" ", max(0, w-lipgloss.Width(timeStr)-lipgloss.Width(helpHint)))
	statusLine := timeStr + spacer + helpHint

	return lipgloss.JoinVertical(lipgloss.Left, divider, bar, statusLine)
}

func (m Model) renderHelp() string {
	w := m.width

	title := titleStyle.Render("Help")
	divider := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#444444")).
		Render(strings.Repeat("─", w))

	keys := []struct{ key, desc string }{
		{"q / Ctrl+C", "quit"},
		{"?", "toggle help"},
	}

	helpLines := make([]string, 0, len(keys))
	for _, k := range keys {
		line := "  " + helpKeyStyle.Render(k.key) + " " + helpDescStyle.Render(k.desc)
		helpLines = append(helpLines, line)
	}

	all := make([]string, 0, 3+len(helpLines)+3)
	all = append(all, title, divider, "")
	all = append(all, helpLines...)
	all = append(all, "", divider)
	all = append(all, footerStyle.Render("  press ? to go back"))

	return lipgloss.JoinVertical(lipgloss.Left, all...)
}
