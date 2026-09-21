package player

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/index-null/cmus-lyric/internal/lyric"
)

// infoRow 是歌曲信息面板里的一行。
type infoRow struct {
	label string
	value string
}

// renderInfo 在没有歌词（或用户按 i）时展示专业排版的文件/标签信息。
func (m Model) renderInfo() string {
	availH := m.bodyHeight()
	w := m.innerWidth()
	p := m.pal

	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(p.Primary.Hex())).Bold(true)
	valStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#C8C8C8"))
	sectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(p.Secondary.Hex())).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(p.Dim.Hex()))

	divider := gradientDivider(w, p.Primary, p.Secondary)

	rows := make([]string, 0, availH)
	rows = append(rows, sectionStyle.Render("AUDIO"), divider)
	rows = append(rows, m.kvRows(append(m.audioRows(), m.tagRows()...), w, keyStyle, valStyle)...)

	if m.infoErr == nil && m.info.Path != "" {
		rows = append(rows, "", sectionStyle.Render("FILE"), divider)
		rows = append(rows, wrapText(m.info.Path, w, valStyle)...)
	}

	rows = append(rows, "", divider)

	if m.noLyric {
		rows = append(rows,
			lipgloss.NewStyle().Foreground(lipgloss.Color(p.Accent.Hex())).Bold(true).
				Render("no lyrics found — this track is probably instrumental"),
			dimStyle.Render("press r to search all sources, i to close this panel"),
		)
	} else {
		rows = append(rows, dimStyle.Render("press r to search lyrics · i to close this panel"))
	}

	if len(rows) > availH {
		rows = rows[:availH]
	}
	for len(rows) < availH {
		rows = append(rows, "")
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m Model) audioRows() []infoRow {
	rows := []infoRow{
		{"Format", m.info.FormatLabel()},
		{"Bitrate", m.info.BitrateLabel()},
		{"Size", lyric.HumanSize(m.info.Size)},
		{"Duration", fmtDuration(m.track.Duration)},
		{"Position", fmtDuration(m.track.Position)},
	}
	if !m.info.Modified.IsZero() {
		rows = append(rows, infoRow{"Modified", m.info.Modified.Format("2006-01-02 15:04")})
	}
	if m.info.EmbeddedLyric || m.info.EmbeddedCover {
		embedded := make([]string, 0, 2)
		if m.info.EmbeddedLyric {
			embedded = append(embedded, "lyrics")
		}
		if m.info.EmbeddedCover {
			embedded = append(embedded, "cover")
		}
		rows = append(rows, infoRow{"Embedded", strings.Join(embedded, " · ")})
	}
	return rows
}

func (m Model) tagRows() []infoRow {
	var rows []infoRow
	add := func(label, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		rows = append(rows, infoRow{label, value})
	}

	if m.info.AlbumArtist != "" && m.info.AlbumArtist != m.track.Artist {
		add("Album artist", m.info.AlbumArtist)
	}
	if m.info.Year > 0 {
		add("Year", fmt.Sprintf("%d", m.info.Year))
	}
	add("Genre", m.info.Genre)
	add("Composer", m.info.Composer)
	if m.info.Track > 0 {
		total := ""
		if m.info.TrackTotal > 0 {
			total = fmt.Sprintf(" / %d", m.info.TrackTotal)
		}
		add("Track", fmt.Sprintf("%d%s", m.info.Track, total))
	}
	if m.info.Disc > 0 {
		total := ""
		if m.info.DiscTotal > 0 {
			total = fmt.Sprintf(" / %d", m.info.DiscTotal)
		}
		add("Disc", fmt.Sprintf("%d%s", m.info.Disc, total))
	}
	add("Comment", m.info.Comment)
	add("Lyric source", m.lyricSource)
	add("Lyric file", m.lyricFile)
	return rows
}

// kvRows 把信息渲染成对齐的两列。
func (m Model) kvRows(rows []infoRow, w int, keyStyle, valStyle lipgloss.Style) []string {
	labelWidth := 13
	valueWidth := max(w-labelWidth-2, 8)

	out := make([]string, 0, len(rows))
	for _, r := range rows {
		label := keyStyle.Render(padRight(r.label, labelWidth))
		values := wrapText(r.value, valueWidth, valStyle)
		for i, v := range values {
			if i == 0 {
				out = append(out, "  "+label+" "+v)
				continue
			}
			out = append(out, "  "+strings.Repeat(" ", labelWidth+1)+v)
		}
	}
	return out
}

func padRight(s string, w int) string {
	runes := []rune(s)
	if len(runes) >= w {
		return string(runes[:w])
	}
	return s + strings.Repeat(" ", w-len(runes))
}

// wrapText 按显示宽度折行（兼容 CJK 双宽字符）。
func wrapText(s string, w int, style lipgloss.Style) []string {
	if w <= 0 {
		return nil
	}
	if s == "" {
		return []string{""}
	}

	var (
		lines []string
		cur   []rune
		used  int
	)
	for _, r := range s {
		cw := lipgloss.Width(string(r))
		if used+cw > w {
			lines = append(lines, style.Render(strings.TrimSpace(string(cur))))
			cur = cur[:0]
			used = 0
		}
		cur = append(cur, r)
		used += cw
	}
	lines = append(lines, style.Render(strings.TrimSpace(string(cur))))
	return lines
}
