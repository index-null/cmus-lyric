package player

import (
	"fmt"
	"os"
	"strings"
	"time"

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
	sections = append(sections, m.renderBody())
	sections = append(sections, m.renderFooter())

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return borderStyle.Render(content)
}

// renderBody 决定中间区域展示什么：候选选择器 / 歌曲信息 / 歌词。
func (m Model) renderBody() string {
	if m.picker.active {
		return m.renderPicker()
	}
	if m.showInfo || (m.noLyric && !m.infoDismissed) {
		return m.renderInfo()
	}
	return m.renderLyrics()
}

// bodyHeight 是中间区域可用的行数。
func (m Model) bodyHeight() int {
	return max(m.height-3-3-2, 1)
}

func (m Model) renderHeader() string {
	w := m.innerWidth()
	p := m.pal

	var title, artist, album string
	if m.track.Title != "" {
		title = m.track.Title
	} else if m.track.File != "" {
		if _, name, ok := util.SplitPath(m.track.File); ok {
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
	availH := m.bodyHeight()
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
		return centerMsg(noLyricStyle.Render("no lyrics — press r to search"))
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
	sourceStr := ""
	if m.lyricSource != "" {
		sourceStr = lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.Secondary.Hex())).
			Render("[" + m.lyricSource + "] ")
	}
	helpHint := footerStyle.Render("q: quit  r: lyrics  i: info  ?: help")
	spacer := strings.Repeat(" ", max(0, w-lipgloss.Width(timeStr)-lipgloss.Width(sourceStr)-lipgloss.Width(helpHint)))
	statusLine := timeStr + spacer + sourceStr + helpHint

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
		{"i", "toggle track info (bitrate / tags)"},
		{"d", "toggle debug"},
		{"r", "search every lyric source and pick one"},
		{"↑/↓  j/k", "move through candidates (or scroll preview)"},
		{"Tab", "switch focus between list and preview"},
		{"Enter", "use the highlighted lyric"},
		{"s", "use it and save next to the audio file"},
		{"x", "delete the local lyric file"},
		{"Esc", "close the picker"},
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
		debugKeyStyle.Render("Duration:") + " " + debugValStyle.Render(fmtDuration(m.track.Duration)),
		debugKeyStyle.Render("Position:") + " " + debugValStyle.Render(fmtDuration(m.track.Position)),
		debugKeyStyle.Render("Status:") + " " + debugValStyle.Render(m.track.Status),
	}

	// 2. 音频文件信息（码率 / 格式 / 标签）
	audioLines := []string{
		debugKeyStyle.Render("Format:") + " " + debugValStyle.Render(m.info.FormatLabel()),
		debugKeyStyle.Render("Bitrate:") + " " + debugValStyle.Render(m.info.BitrateLabel()),
		debugKeyStyle.Render("Size:") + " " + debugValStyle.Render(lyric.HumanSize(m.info.Size)),
	}

	// 3. 歌词来源
	sourceStr := m.lyricSource
	if sourceStr == "" {
		sourceStr = "none"
	}
	if m.unsynced && !strings.Contains(sourceStr, "unsynced") {
		sourceStr += " (unsynced)"
	}
	sourceLine := debugKeyStyle.Render("Lyric Source:") + " " + debugValStyle.Render(sourceStr)

	// 4. 本地歌词文件（每次都重新查找，绝不沿用上一首歌的路径）
	localLines := []string{debugKeyStyle.Render("Local File:")}
	if m.lyricSource == "embedded" {
		localLines[0] += " " + debugValStyle.Render("(embedded in the audio file)")
	} else if path, ok := lyric.FindLocalLyric(m.track.File, m.track.Title); ok {
		localLines[0] += " " + debugValStyle.Render(path+" (exists)")
		localLines = append(localLines, previewFile(path, debugValStyle)...)
	} else {
		localLines[0] += " " + debugValStyle.Render("(not found)")
	}

	// 5. 缓存文件（键包含时长，避免同名不同版本串味）
	cachePath := lyric.CachePath(m.track.Artist, m.track.Title, m.track.Duration)
	cacheLines := []string{debugKeyStyle.Render("Cache File:")}
	if _, err := os.Stat(cachePath); err == nil {
		cacheLines[0] += " " + debugValStyle.Render(cachePath+" (exists)")
		if entry, ok := lyric.LoadCache(m.track.Artist, m.track.Title, m.track.Duration); ok {
			if entry.Source != "" {
				cacheLines = append(cacheLines, "  "+debugValStyle.Render("from "+entry.Source))
			}
			if !entry.SavedAt.IsZero() {
				cacheLines = append(cacheLines, "  "+debugValStyle.Render("saved "+entry.SavedAt.Format(time.DateTime)))
			}
			if entry.Instrumental {
				cacheLines = append(cacheLines, "  "+debugValStyle.Render("marked instrumental"))
			}
		}
		cacheLines = append(cacheLines, previewFile(cachePath, debugValStyle)...)
	} else {
		cacheLines[0] += " " + debugValStyle.Render(cachePath+" (not found)")
	}

	all := make([]string, 0, 16)
	all = append(all, title, divider, "")
	all = append(all, metaLines...)
	all = append(all, "", divider, "")
	all = append(all, audioLines...)
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

// previewFile 预览歌词文件的前若干行。
func previewFile(path string, style lipgloss.Style) []string {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := make([]string, 0, 5)
	for i, l := range splitContentLines(string(content)) {
		if i >= 5 {
			break
		}
		lines = append(lines, "  "+style.Render(l))
	}
	return lines
}

func splitContentLines(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	return strings.SplitN(content, "\n", 6)
}

func fmtDuration(seconds int) string {
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}
