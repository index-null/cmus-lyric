package player

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/index-null/cmus-lyric/internal/lyric"
)

// statusGlyph 把来源状态映射成一个带颜色的符号。
func (m Model) statusGlyph(s sourceStatus) string {
	var color, glyph string
	switch s.state {
	case "pending":
		color, glyph = m.pal.Dim.Hex(), "·"
	case "ok":
		color, glyph = m.pal.Accent.Hex(), fmt.Sprintf("✓%d", s.count)
	case "empty":
		color, glyph = m.pal.Dim.Hex(), "✗"
	default:
		color, glyph = "#FF5555", "!"
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(glyph)
}

// renderPicker 左侧候选列表 + 右侧歌词预览。
func (m Model) renderPicker() string {
	availH := m.bodyHeight()
	w := m.innerWidth()

	leftW := min(max(w*2/5, 26), 48)
	rightW := max(w-leftW, 0)

	left := lipgloss.NewStyle().
		Width(leftW).
		Height(availH).
		Render(m.renderCandidateList(leftW, availH))

	right := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderLeft(true).
		BorderForeground(lipgloss.Color(m.pal.Border.Hex())).
		Width(rightW).
		Height(availH).
		Render(m.renderPreview(max(rightW-1, 0), availH))

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m Model) renderCandidateList(w, h int) string {
	p := m.pal

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(p.Title.Hex())).
		Background(lipgloss.Color(p.TitleBg.Hex())).
		Padding(0, 1)

	title := titleStyle.Render("Lyrics")
	divider := gradientDivider(w, p.Primary, p.Secondary)

	// 来源状态行
	statusParts := make([]string, 0, len(m.picker.status))
	for _, s := range m.picker.status {
		statusParts = append(statusParts,
			lipgloss.NewStyle().Foreground(lipgloss.Color(p.Artist.Hex())).Render(s.name)+
				" "+m.statusGlyph(s))
	}
	statusLine := truncateWidth(strings.Join(statusParts, "  "), w)

	rows := []string{title, divider, statusLine, ""}

	// 候选列表：每项两行（标题 + 元信息）
	const rowHeight = 2
	bodyRows := max((h-len(rows)-1)/rowHeight, 1)

	start := 0
	if len(m.picker.candidates) > bodyRows {
		start = m.picker.cursor - bodyRows/2
		start = min(max(start, 0), len(m.picker.candidates)-bodyRows)
	}
	end := min(start+bodyRows, len(m.picker.candidates))

	for i := start; i < end; i++ {
		rows = append(rows, m.renderCandidateRow(m.picker.candidates[i], i == m.picker.cursor, w)...)
	}
	if len(m.picker.candidates) == 0 {
		empty := "searching…"
		if m.allSourcesDone() {
			empty = "nothing found — try another spelling"
		}
		rows = append(rows, lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.Dim.Hex())).
			Render(empty))
	}

	for len(rows) < h-1 {
		rows = append(rows, "")
	}

	hint := "↑↓ select · Tab preview · Enter use · s save · x delete · Esc close"
	rows = append(rows, lipgloss.NewStyle().
		Foreground(lipgloss.Color(p.Dim.Hex())).
		Render(truncateWidth(hint, w)))

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m Model) renderCandidateRow(c lyric.Candidate, selected bool, w int) []string {
	p := m.pal

	marker := "  "
	titleColor := p.Upcoming.Hex()
	metaColor := p.Dim.Hex()
	if selected {
		marker = "▶ "
		titleColor = p.Primary.Hex()
		metaColor = p.Secondary.Hex()
	}

	title := truncateWidth(c.Title, max(w-4, 4))
	if c.Instrumental || lyric.IsPlaceholder(c.LRC) {
		title += " " + lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.Dim.Hex())).Render("[no lyrics]")
	}

	badge := "unsynced"
	if c.Synced {
		badge = "synced"
	}

	meta := strings.Join(nonEmpty([]string{c.Artist, c.Album}), " · ")
	if meta != "" {
		meta += " · "
	}
	meta += fmt.Sprintf("%s · %s · %.0f%%", fmtDuration(c.Duration), badge, c.Score*100)

	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(titleColor))
	if selected {
		titleStyle = titleStyle.Bold(true)
	}
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(metaColor))

	return []string{
		marker + titleStyle.Render(title),
		"   " + metaStyle.Render(truncateWidth(c.Source+" · "+meta, max(w-5, 4))),
	}
}

func (m Model) renderPreview(w, h int) string {
	p := m.pal

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(p.Secondary.Hex())).
		Bold(true)
	header := "Preview"
	if m.picker.focusPreview {
		header += " · focused"
	}
	if c, ok := m.picker.selected(); ok {
		header += " · " + c.Source
	}
	divider := gradientDivider(w, p.Primary, p.Secondary)

	rows := []string{headerStyle.Render(header), divider}

	lines := m.previewLines()
	bodyH := max(h-len(rows), 1)

	if len(lines) == 0 {
		rows = append(rows, lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.Dim.Hex())).
			Render("select a candidate to preview it"))
		rows = padRows(rows, h)
		return lipgloss.JoinVertical(lipgloss.Left, rows...)
	}

	start := min(m.picker.previewScroll, max(len(lines)-1, 0))
	end := min(start+bodyH, len(lines))

	for i := start; i < end; i++ {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(p.Upcoming.Hex()))
		rows = append(rows, style.Render(truncateWidth(lines[i], w)))
	}

	if len(lines) > bodyH {
		rows = append(rows, lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.Dim.Hex())).
			Render(fmt.Sprintf("  %d/%d", start+1, len(lines))))
	}

	return lipgloss.JoinVertical(lipgloss.Left, padRows(rows, h)...)
}

func (m Model) allSourcesDone() bool {
	for _, s := range m.picker.status {
		if s.state == "pending" {
			return false
		}
	}
	return len(m.picker.status) > 0
}

func padRows(rows []string, h int) []string {
	if len(rows) > h {
		return rows[:h]
	}
	for len(rows) < h {
		rows = append(rows, "")
	}
	return rows
}

func nonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// truncateWidth 按显示宽度截断，兼容中文等双宽字符。
func truncateWidth(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}

	out := make([]rune, 0, len(s))
	used := 0
	for _, r := range s {
		cw := lipgloss.Width(string(r))
		if used+cw > w-1 {
			break
		}
		out = append(out, r)
		used += cw
	}
	return string(out) + "…"
}
