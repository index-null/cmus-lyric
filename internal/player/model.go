package player

import (
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/index-null/cmus-lyric/internal/cmus"
	"github.com/index-null/cmus-lyric/internal/lyric"
)

type Model struct {
	track        cmus.Track
	lyrics       []lyric.Line
	curFile      string
	curLineIdx   int
	progress     progress.Model
	pal          palette
	width        int
	height       int
	showHelp     bool
	showDebug    bool
	errMsg       string
	fetchingMsg  string
	fetching     bool
	lyricSource  string
	lyricFile    string
}

type tickMsg struct{}

type fetchDoneMsg struct {
	file   string
	artist string
	title  string
	lrc    string
	tlyric string
	err    error
}

func NewModel() Model {
	pal := generatePalette("cmus-lyric")
	p := progress.New(pal.progressOpts()...)
	return Model{
		progress:   p,
		pal:        pal,
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
		case "d":
			m.showDebug = !m.showDebug
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progress.Width = max(msg.Width-6, 0)
		return m, nil

	case tickMsg:
		var cmd tea.Cmd
		m, cmd = m.poll()
		return m, tea.Batch(cmd, tea.Tick(200*time.Millisecond, func(_ time.Time) tea.Msg {
			return tickMsg{}
		}))

	case fetchDoneMsg:
		m.fetching = false
		m.fetchingMsg = ""
		if msg.file != m.curFile {
			return m, nil
		}
		if msg.err != nil {
			m.errMsg = "fetch failed: " + msg.err.Error()
			m.lyrics = nil
			m.lyricSource = "fetch failed"
			return m, nil
		}
		if len(msg.lrc) > 0 {
			_ = lyric.SaveToCache(msg.artist, msg.title, msg.lrc, msg.tlyric)
		}
		m.lyrics = lyric.Load(m.track.File, m.track.Title)
		if m.lyrics == nil {
			m.lyrics, _, _ = lyric.LoadFromCache(m.track.Artist, m.track.Title)
		}
		m.lyricSource = "fetched"
		m.lyricFile = lyric.CachePath(msg.artist, msg.title)
		m.curLineIdx = -1
		return m, nil
	}

	return m, nil
}

func (m Model) poll() (Model, tea.Cmd) {
	track := cmus.Remote()
	m.track = track

	if track.Status != "playing" {
		m.errMsg = ""
		m.fetchingMsg = ""
		return m, nil
	}

	if track.File != m.curFile {
		m.curFile = track.File
		m.errMsg = ""
		m.fetchingMsg = ""
		m.fetching = false

		seed := track.Artist + " - " + track.Title
		if seed == " - " {
			seed = track.File
		}
		m.pal = generatePalette(seed)
		m.progress = progress.New(m.pal.progressOpts()...)
		m.progress.Width = max(m.width-6, 0)

		lyrics := lyric.Load(track.File, track.Title)
		if lyrics != nil {
			m.lyricSource = "local"
			dotIdx := strings.LastIndex(track.File, ".")
			slashIdx := strings.LastIndex(track.File, "/")
			if dotIdx >= 0 && slashIdx >= 0 {
				base := track.File[:dotIdx]
				dir := track.File[:slashIdx]
				bases := []string{base}
				if len(track.Title) > 0 {
					titleBase := dir + "/" + track.Title
					if titleBase != base {
						bases = append(bases, titleBase)
					}
				}
				for _, b := range bases {
					for _, ext := range []string{".lyric", ".lrc"} {
						if _, err := os.Stat(b + ext); err == nil {
							m.lyricFile = b + ext
							break
						}
					}
					if m.lyricFile != "" {
						break
					}
				}
			}
		} else {
			cachedLyrics, cachedLrc, cachedTLyric := lyric.LoadFromCache(track.Artist, track.Title)
			if cachedLyrics != nil {
				lyrics = cachedLyrics
				m.lyricSource = "cache"
				m.lyricFile = lyric.CachePath(track.Artist, track.Title)
				// 回写缓存内容到本地文件
				_ = lyric.SaveToLocal(track.File, track.Title, cachedLrc, cachedTLyric)
			}
		}
		if lyrics == nil && !m.fetching {
			m.fetching = true
			m.fetchingMsg = "fetching lyrics..."
			m.lyricSource = "fetching"
			file := track.File
			dt := track.Duration
			artist := track.Artist
			title := track.Title
			cmd := func() tea.Msg {
				lrc, tlyric, err := lyric.FetchContent(file, dt, artist, title)
				return fetchDoneMsg{
					file:   file,
					artist: artist,
					title:  title,
					lrc:    lrc,
					tlyric: tlyric,
					err:    err,
				}
			}
			return m, cmd
		}
		m.lyrics = lyrics
		m.curLineIdx = -1
	}

	if m.lyrics != nil {
		posCS := track.Position*100 + 180
		m.curLineIdx = lyric.FindCurrentLine(m.lyrics, posCS)
	}

	return m, nil
}
