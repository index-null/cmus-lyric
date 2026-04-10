package player

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/index-null/cmus-lyric/internal/lyric"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

type cmusTrack struct {
	Position int
	File     string
	Duration int
	Artist   string
	Title    string
	Album    string
	Status   string
}

type lyricLine struct {
	TimeCS int
	Text   string
	Trans  string
}

type Model struct {
	track       cmusTrack
	lyrics      []lyricLine
	curFile     string
	curLineIdx  int
	progress    progress.Model
	width       int
	height      int
	showHelp    bool
	errMsg      string
	fetchingMsg string
}

type tickMsg struct{}

func NewModel() Model {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithoutPercentage(),
	)
	return Model{
		progress:   p,
		curLineIdx: -1,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(_ time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progress.Width = msg.Width - 4
		return m, nil

	case tickMsg:
		m = m.poll()
		return m, tea.Tick(200*time.Millisecond, func(_ time.Time) tea.Msg {
			return tickMsg{}
		})
	}

	return m, nil
}

func (m Model) poll() Model {
	track := cmusRemote()
	m.track = track

	if track.Status != "playing" {
		m.errMsg = ""
		m.fetchingMsg = ""
		return m
	}

	if track.File != m.curFile {
		m.curFile = track.File
		m.errMsg = ""
		m.fetchingMsg = ""

		lyrics := loadLyrics(track.File, track.Title)
		if lyrics == nil {
			m.fetchingMsg = "fetching lyrics..."
			err := lyric.FetchForCmus(track.File, track.Duration, track.Artist, track.Title)
			m.fetchingMsg = ""
			if err != nil {
				m.errMsg = "fetch failed: " + err.Error()
				m.lyrics = nil
				return m
			}
			lyrics = loadLyrics(track.File, track.Title)
		}
		m.lyrics = lyrics
		m.curLineIdx = -1
	}

	if m.lyrics != nil {
		posCS := track.Position * 100
		m.curLineIdx = findCurrentLine(m.lyrics, posCS)
	}

	return m
}

func findCurrentLine(lyrics []lyricLine, posCS int) int {
	idx := -1
	for i, l := range lyrics {
		if posCS >= l.TimeCS {
			idx = i
		} else {
			break
		}
	}
	return idx
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	if m.showHelp {
		return m.renderHelp()
	}

	var sections []string
	sections = append(sections, m.renderHeader())
	sections = append(sections, m.renderLyrics())
	sections = append(sections, m.renderFooter())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// --- Styles ---

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

// --- Render ---

func (m Model) renderHeader() string {
	w := m.width

	var title, artist, album string
	if m.track.Title != "" {
		title = m.track.Title
	} else if m.track.File != "" {
		pathIdx := strings.LastIndexAny(m.track.File, "/")
		dotIdx := strings.LastIndexAny(m.track.File, ".")
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

	var rendered []renderedLine
	for i, l := range m.lyrics {
		txt := l.Text
		if txt == "" {
			txt = "..."
		}

		var styledMain, styledTrans string
		switch {
		case i < m.curLineIdx:
			styledMain = passedStyle.Render("  " + txt)
			if l.Trans != "" {
				styledTrans = passedTransStyle.Render("  " + l.Trans)
			}
		case i == m.curLineIdx:
			styledMain = currentStyle.Render("  > " + txt)
			if l.Trans != "" {
				styledTrans = currentTransStyle.Render("    " + l.Trans)
			}
		default:
			styledMain = upcomingStyle.Render("  " + txt)
			if l.Trans != "" {
				styledTrans = upcomingTransStyle.Render("  " + l.Trans)
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

	var helpLines []string
	for _, k := range keys {
		line := "  " + helpKeyStyle.Render(k.key) + " " + helpDescStyle.Render(k.desc)
		helpLines = append(helpLines, line)
	}

	all := []string{title, divider, ""}
	all = append(all, helpLines...)
	all = append(all, "", divider)
	all = append(all, footerStyle.Render("  press ? to go back"))

	return lipgloss.JoinVertical(lipgloss.Left, all...)
}

// --- Lyric loading ---

func loadLyrics(path, title string) []lyricLine {
	pathIdx := strings.LastIndexAny(path, ".")
	base := path[:pathIdx]
	dir := path[:strings.LastIndexAny(path, "/")]

	extensions := []string{".lyric", ".lrc"}

	bases := []string{base}
	if len(title) > 0 {
		titleBase := dir + "/" + title
		if titleBase != base {
			bases = append(bases, titleBase)
		}
	}

	var content []byte
	var tlines []string
	found := false

	for _, b := range bases {
		for _, ext := range extensions {
			lpath := b + ext
			c, e := os.ReadFile(lpath)
			if e != nil {
				continue
			}
			content = c
			found = true

			tlpath := b + ".t" + ext
			tc, te := os.ReadFile(tlpath)
			if te == nil {
				tc = toUTF8(tc)
				tlines = strings.Split(string(tc), "\n")
			}
			break
		}
		if found {
			break
		}
	}

	if !found {
		return nil
	}

	content = toUTF8(content)
	lines := strings.Split(string(content), "\n")

	lyricMap := buildLyricMapCS(lines)
	tlyricMap := buildLyricMapCS(tlines)

	type entry struct {
		timeCS int
		text   string
		trans  string
	}

	var entries []entry
	for k, v := range lyricMap {
		t := tlyricMap[k]
		entries = append(entries, entry{k, v, t})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].timeCS < entries[j].timeCS
	})

	var result []lyricLine
	for _, e := range entries {
		result = append(result, lyricLine{
			TimeCS: e.timeCS,
			Text:   e.text,
			Trans:  e.trans,
		})
	}

	return result
}

func toUTF8(data []byte) []byte {
	if utf8.Valid(data) {
		return data
	}
	reader := transform.NewReader(bytes.NewReader(data), simplifiedchinese.GBK.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return data
	}
	return decoded
}

func buildLyricMapCS(lines []string) map[int]string {
	m := make(map[int]string)
	re := regexp.MustCompile(`^\[([0-9]+):([0-9]+)\.?([0-9]*)]\s*(.*)`)
	for _, v := range lines {
		ar := re.FindStringSubmatch(v)
		if len(ar) > 4 {
			mi, _ := strconv.Atoi(ar[1])
			sec, _ := strconv.Atoi(ar[2])
			csStr := ar[3]
			cs := 0
			if csStr != "" {
				cs, _ = strconv.Atoi(csStr)
				switch len(csStr) {
				case 1:
					cs *= 10
				case 3:
					cs /= 10
				}
			}
			pos := (60*mi+sec)*100 + cs
			text := strings.TrimSpace(ar[4])
			if text != "" {
				m[pos] = text
			}
		}
	}
	return m
}

// --- cmus IPC ---

func cmusRemote() cmusTrack {
	cmd := exec.Command("cmus-remote", "-Q")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return cmusTrack{Status: "stopped"}
	}

	info := strings.Split(out.String(), "\n")
	if len(info) < 1 || len(info[0]) < 1 {
		return cmusTrack{Status: "stopped"}
	}

	parts := strings.Fields(info[0])
	if len(parts) < 2 {
		return cmusTrack{Status: "stopped"}
	}

	track := cmusTrack{Status: parts[1]}
	for _, line := range info {
		switch {
		case strings.HasPrefix(line, "file "):
			track.File = line[5:]
		case strings.HasPrefix(line, "duration "):
			track.Duration, _ = strconv.Atoi(line[9:])
		case strings.HasPrefix(line, "position "):
			track.Position, _ = strconv.Atoi(line[9:])
		case strings.HasPrefix(line, "tag artist "):
			track.Artist = line[11:]
		case strings.HasPrefix(line, "tag title "):
			track.Title = line[10:]
		case strings.HasPrefix(line, "tag album "):
			track.Album = line[10:]
		}
	}
	return track
}
