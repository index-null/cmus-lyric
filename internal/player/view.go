package player

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/index-null/cmus-lyric/internal/lyric"
	"github.com/index-null/cmus-lyric/internal/util"
)

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	if m.showDebug {
		return m.renderDebug()
	}

	if m.showHelp {
		return m.renderHelp()
	}

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.pal.Border.Hex())).
		Width(m.width - 2).
		Height(m.height - 2)

	sections := make([]string, 0, 3)
	sections = append(sections, m.renderHeader())
	sections = append(sections, m.renderLyrics())
	sections = append(sections, m.renderFooter())

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return borderStyle.Render(content)
}

func (m Model) renderHeader() string {
	w := m.innerWidth()
	p := m.pal

	var title, artist, album string
	if m.track.Title != "" {
		title = m.track.Title
	} else if m.track.File != "" {
		_, name, ok := util.SplitPath(m.track.File)
		if ok {
			title = name
		}
	}
	artist = m.track.Artist
	album = m.track.Album

	var statusIcon string
	switch m.track.Status {
	case "playing":
		statusIcon = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Accent.Hex())).Bold(true).Render(">>")
	case "paused":
		statusIcon = statusPausedStyle.Render("||")
	default:
		statusIcon = footerStyle.Render("--")
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(p.Title.Hex())).
		Background(lipgloss.Color(p.TitleBg.Hex())).
		Padding(0, 1)

	titleLine := statusIcon + " " + titleStyle.Render(title)

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
	artistStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(p.Artist.Hex())).
		Padding(0, 1)
	infoLine := "   " + artistStyle.Render(info)

	divider := gradientDivider(w, p.Primary, p.Secondary)

	return lipgloss.JoinVertical(lipgloss.Left, titleLine, infoLine, divider)
}

func (m Model) renderLyrics() string {
	headerH := 3
	footerH := 3
	borderH := 2
	availH := max(m.height-headerH-footerH-borderH, 1)
	w := m.innerWidth()
	p := m.pal

	centerMsg := func(msg string) string {
		pad := max(availH/2-1, 0)
		lines := make([]string, 0, availH)
		for range pad {
			lines = append(lines, "")
		}
		centered := lipgloss.NewStyle().Width(w).Align(lipgloss.Center).Render(msg)
		lines = append(lines, centered)
		for len(lines) < availH {
			lines = append(lines, "")
		}
		return lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	if m.track.Status != "playing" {
		return centerMsg(statusPausedStyle.Render("paused / stopped"))
	}
	if m.errMsg != "" {
		return centerMsg(errorStyle.Render(m.errMsg))
	}
	if m.fetchingMsg != "" {
		return centerMsg(fetchStyle.Render(m.fetchingMsg))
	}
	if len(m.lyrics) == 0 {
		return centerMsg(noLyricStyle.Render("no lyrics"))
	}

	if m.unsynced {
		// 未同步歌词：根据播放进度估算当前位置，自然向下滚动
		totalLines := len(m.lyrics)
		estimatedLine := 0
		if m.track.Duration > 0 && totalLines > 0 {
			estimatedLine = m.track.Position * totalLines / m.track.Duration
		}
		estimatedLine = max(min(estimatedLine, totalLines-1), 0)

		halfWin := availH / 2
		start := max(estimatedLine-halfWin, 0)
		end := min(start+availH, totalLines)
		if end-start < availH {
			start = max(end-availH, 0)
		}

		align := lipgloss.NewStyle().Width(w).Align(lipgloss.Center)
		lines := make([]string, 0, availH)
		for i := start; i < end; i++ {
			txt := m.lyrics[i].Text
			if txt == "" {
				txt = "..."
			}
			s := lipgloss.NewStyle().Foreground(lipgloss.Color(p.Upcoming.Hex()))
			lines = append(lines, align.Render(s.Render(txt)))
		}
		for len(lines) < availH {
			lines = append(lines, "")
		}
		return lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	type renderedLine struct {
		idx  int
		text string
	}

	const fadeRadius = 8
	align := lipgloss.NewStyle().Width(w).Align(lipgloss.Center)

	var rendered []renderedLine
	for i, l := range m.lyrics {
		txt := l.Text
		if txt == "" {
			txt = "..."
		}

		var styledMain, styledTrans string
		switch {
		case i == m.curLineIdx:
			styledMain = align.Render(gradientText("♪ "+txt, p.Primary, p.Secondary, true))
			if l.Trans != "" {
				styledTrans = align.Render(gradientText(l.Trans, p.Trans, p.Secondary, false))
			}
		case i < m.curLineIdx:
			dist := m.curLineIdx - i
			s := lyricFadedStyle(dist, fadeRadius, p.Passed, p.Dim)
			styledMain = align.Render(s.Render(txt))
			if l.Trans != "" {
				ts := lyricFadedStyle(dist, fadeRadius, p.Trans, p.Dim)
				styledTrans = align.Render(ts.Render(l.Trans))
			}
		default:
			dist := i - m.curLineIdx
			s := lyricFadedStyle(dist, fadeRadius, p.Upcoming, p.Dim)
			styledMain = align.Render(s.Render(txt))
			if l.Trans != "" {
				ts := lyricFadedStyle(dist, fadeRadius, p.Trans, p.Dim)
				styledTrans = align.Render(ts.Render(l.Trans))
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
	w := m.innerWidth()
	p := m.pal

	divider := gradientDivider(w, p.Primary, p.Secondary)

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
	w := m.innerWidth()
	p := m.pal

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(p.Border.Hex())).
		Width(m.width - 2).
		Height(m.height - 2)

	helpTitleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(p.Title.Hex())).
		Background(lipgloss.Color(p.TitleBg.Hex())).
		Padding(0, 1)

	helpKeyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(p.Primary.Hex())).
		Bold(true)

	helpDescStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#AAAAAA"))

	title := helpTitleStyle.Render("Help")
	divider := gradientDivider(w, p.Primary, p.Secondary)

	keys := []struct{ key, desc string }{
		{"q / Ctrl+C", "quit"},
		{"?", "toggle help"},
		{"d", "toggle debug"},
		{"r", "refetch lyrics"},
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

	content := lipgloss.JoinVertical(lipgloss.Left, all...)
	return borderStyle.Render(content)
}

func (m Model) innerWidth() int {
	return max(m.width-4, 0)
}

func (m Model) renderDebug() string {
	w := m.innerWidth()
	p := m.pal

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(p.Border.Hex())).
		Width(m.width - 2).
		Height(m.height - 2)

	debugTitleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(p.Title.Hex())).
		Background(lipgloss.Color(p.TitleBg.Hex())).
		Padding(0, 1)

	debugKeyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(p.Primary.Hex())).
		Bold(true)

	debugValStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#AAAAAA"))

	title := debugTitleStyle.Render("Debug")
	divider := gradientDivider(w, p.Primary, p.Secondary)

	// 1. 歌曲元信息
	metaLines := []string{
		debugKeyStyle.Render("File:") + " " + debugValStyle.Render(m.track.File),
		debugKeyStyle.Render("Artist:") + " " + debugValStyle.Render(m.track.Artist),
		debugKeyStyle.Render("Title:") + " " + debugValStyle.Render(m.track.Title),
		debugKeyStyle.Render("Album:") + " " + debugValStyle.Render(m.track.Album),
		debugKeyStyle.Render("Duration:") + " " + debugValStyle.Render(fmt.Sprintf("%d:%02d", m.track.Duration/60, m.track.Duration%60)),
		debugKeyStyle.Render("Position:") + " " + debugValStyle.Render(fmt.Sprintf("%d:%02d", m.track.Position/60, m.track.Position%60)),
		debugKeyStyle.Render("Status:") + " " + debugValStyle.Render(m.track.Status),
	}

	// 2. 歌词来源
	sourceStr := m.lyricSource
	if sourceStr == "" {
		sourceStr = "none"
	}
	if m.unsynced && !strings.Contains(sourceStr, "unsynced") {
		sourceStr += " (unsynced)"
	}
	sourceLine := debugKeyStyle.Render("Lyric Source:") + " " + debugValStyle.Render(sourceStr)

	// 3. 本地文件和缓存文件信息
	localLines := []string{debugKeyStyle.Render("Local File:")}
	if m.lyricFile != "" && m.lyricSource == "local" {
		if content, err := os.ReadFile(m.lyricFile); err == nil {
			localLines[0] += " " + debugValStyle.Render(m.lyricFile+" (exists)")
			// 预览前5行
			lines := strings.SplitN(string(content), "\n", 6)
			for i, l := range lines {
				if i >= 5 {
					break
				}
				localLines = append(localLines, "  "+debugValStyle.Render(l))
			}
		} else {
			localLines[0] += " " + debugValStyle.Render(m.lyricFile+" (not found)")
		}
	} else {
		// 检查可能的本地文件路径
		if dir, name, ok := util.SplitPath(m.track.File); ok {
			base := dir + "/" + name
			found := false
			for _, ext := range []string{".lyric", ".lrc"} {
				for _, b := range []string{base, dir + "/" + m.track.Title} {
					if _, err := os.Stat(b + ext); err == nil {
						localLines[0] += " " + debugValStyle.Render(b+ext+" (exists)")
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				localLines[0] += " " + debugValStyle.Render("(not found)")
			}
		}
	}

	cacheLines := []string{debugKeyStyle.Render("Cache File:")}
	cachePath := lyric.CachePath(m.track.Artist, m.track.Title)
	if content, err := os.ReadFile(cachePath); err == nil {
		cacheLines[0] += " " + debugValStyle.Render(cachePath+" (exists)")
		// 预览前5行
		lines := strings.SplitN(string(content), "\n", 6)
		for i, l := range lines {
			if i >= 5 {
				break
			}
			cacheLines = append(cacheLines, "  "+debugValStyle.Render(l))
		}
	} else {
		cacheLines[0] += " " + debugValStyle.Render(cachePath+" (not found)")
	}

	// 组装所有内容
	all := make([]string, 0)
	all = append(all, title, divider, "")
	all = append(all, metaLines...)
	all = append(all, "", divider, "")
	all = append(all, sourceLine)
	all = append(all, "", divider, "")
	all = append(all, localLines...)
	all = append(all, "", divider, "")
	all = append(all, cacheLines...)
	all = append(all, "", divider)
	all = append(all, footerStyle.Render("  press d to go back"))

	content := lipgloss.JoinVertical(lipgloss.Left, all...)
	return borderStyle.Render(content)
}
