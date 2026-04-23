package cover

import (
	"bytes"
	"image"
	"time"

	// Register image decoders for cover art.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/mosaic"

	"github.com/index-null/cmus-lyric/internal/cmus"
	"github.com/index-null/cmus-lyric/internal/lyric"
)

type Model struct {
	track            cmus.Track
	curFile          string
	coverImg         image.Image
	renderedImg      string
	renderW, renderH int
	width, height    int
	errMsg           string
	fetching         bool
}

type tickMsg struct{}
type coverDoneMsg struct {
	file string
	img  image.Image
	err  error
}

func NewModel() Model { return Model{} }

func (m Model) Init() tea.Cmd { return tick() }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.rerender()
		return m, nil
	case tickMsg:
		var cmd tea.Cmd
		m, cmd = m.poll()
		return m, tea.Batch(cmd, tick())
	case coverDoneMsg:
		m.fetching = false
		if msg.file != m.curFile {
			return m, nil
		}
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.coverImg = nil
			m.renderedImg = ""
			return m, nil
		}
		m.errMsg = ""
		m.coverImg = msg.img
		m.rerender()
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	if m.renderedImg != "" {
		return m.renderedImg
	}
	msg := "waiting for cmus..."
	if m.track.File != "" && m.fetching {
		msg = "loading cover..."
	} else if m.track.File != "" {
		msg = "no cover"
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Render(msg))
}

func (m *Model) rerender() {
	if m.coverImg == nil || m.width < 2 || m.height < 2 {
		m.renderedImg = ""
		return
	}
	w, h := m.width, m.height
	if w > h*2 {
		w = h * 2
	}
	if h > w {
		h = w
	}
	if m.renderedImg != "" && w == m.renderW && h == m.renderH {
		return
	}
	mos := mosaic.New().Width(w).Height(h)
	rendered := mos.Render(m.coverImg)
	m.renderedImg = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, rendered)
	m.renderW, m.renderH = w, h
}

func (m Model) poll() (Model, tea.Cmd) {
	track := cmus.Remote()
	m.track = track
	if track.File == m.curFile {
		return m, nil
	}
	m.curFile = track.File
	m.coverImg = nil
	m.renderedImg = ""
	m.errMsg = ""
	if track.File == "" {
		return m, nil
	}
	m.fetching = true
	file, artist, title, dur := track.File, track.Artist, track.Title, track.Duration
	return m, func() tea.Msg {
		img, err := loadCover(file, artist, title, dur)
		return coverDoneMsg{file: file, img: img, err: err}
	}
}

func loadCover(file, artist, title string, duration int) (image.Image, error) {
	if data := lyric.LoadEmbeddedCover(file); data != nil {
		if img, _, err := image.Decode(bytes.NewReader(data)); err == nil {
			return img, nil
		}
	}
	if data := lyric.LoadCoverFromCache(artist, title); data != nil {
		if img, _, err := image.Decode(bytes.NewReader(data)); err == nil {
			return img, nil
		}
	}
	coverURL, err := lyric.FetchCoverURL(title, artist, duration)
	if err != nil {
		return nil, err
	}
	data, err := lyric.FetchCoverData(coverURL)
	if err != nil {
		return nil, err
	}
	_ = lyric.SaveCoverToCache(artist, title, data)
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}

func tick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(_ time.Time) tea.Msg { return tickMsg{} })
}
