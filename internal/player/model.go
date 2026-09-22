package player

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/index-null/cmus-lyric/internal/cmus"
	"github.com/index-null/cmus-lyric/internal/lyric"
	"github.com/index-null/cmus-lyric/internal/util"
)

// searchTimeout 是单次歌词检索的超时时间。
const searchTimeout = 12 * time.Second

type Model struct {
	track      cmus.Track
	lyrics     []lyric.Line
	curFile    string
	curLineIdx int
	progress   progress.Model
	pal        palette
	width      int
	height     int
	showHelp   bool
	showDebug  bool
	showInfo   bool
	// infoDismissed 让用户可以把「无歌词」时自动弹出的信息面板关掉。
	infoDismissed bool
	errMsg        string
	fetchingMsg   string
	fetching      bool
	lyricSource   string
	lyricFile     string
	unsynced      bool
	noLyric       bool
	info          lyric.AudioInfo
	infoErr       error
	picker        pickerState
	// confirmQuit 表示「退出确认框」正在等待 y/N。
	confirmQuit bool
}

type tickMsg struct{}

// autoFetchMsg 是切歌时自动检索的结果。
type autoFetchMsg struct {
	file      string
	candidate lyric.Candidate
	found     bool
}

// sourceResultMsg 是单个歌词来源的检索结果。
type sourceResultMsg struct {
	source     string
	candidates []lyric.Candidate
	err        error
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
		if m.confirmQuit {
			return m.updateQuitConfirm(msg)
		}
		if m.picker.active {
			return m.updatePicker(msg)
		}
		switch msg.String() {
		case "q":
			m.confirmQuit = true
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		case "d":
			m.showDebug = !m.showDebug
			return m, nil
		case "i":
			if m.noLyric {
				m.infoDismissed = !m.infoDismissed
			} else {
				m.showInfo = !m.showInfo
			}
			return m, nil
		case "r":
			return m.startSearch()
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

	case autoFetchMsg:
		return m.handleAutoFetch(msg), nil

	case sourceResultMsg:
		return m.applySourceResult(msg), nil
	}

	return m, nil
}

// updateQuitConfirm 处理退出确认框的按键：y 退出，其余任何键取消。
func (m Model) updateQuitConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "ctrl+c":
		return m, tea.Quit
	default:
		m.confirmQuit = false
		return m, nil
	}
}

func (m Model) poll() (Model, tea.Cmd) {
	return m.applyTrack(cmus.Remote())
}

// applyTrack 把 cmus 的当前状态应用到模型上。
func (m Model) applyTrack(track cmus.Track) (Model, tea.Cmd) {
	m.track = track

	if track.Status != "playing" {
		m.errMsg = ""
		m.fetchingMsg = ""
		m.updateCurrentLine()
		return m, nil
	}

	if track.File == m.curFile {
		m.updateCurrentLine()
		return m, nil
	}

	return m.switchTrack(track)
}

func (m *Model) updateCurrentLine() {
	if m.lyrics == nil {
		return
	}
	m.curLineIdx = lyric.FindCurrentLine(m.lyrics, m.track.Position*100+180)
}

// switchTrack 切换曲目：清空上一首歌的全部残留状态，再按
// 本地歌词 → 缓存 → 联网检索 的顺序取歌词。
func (m Model) switchTrack(track cmus.Track) (Model, tea.Cmd) {
	m.curFile = track.File
	m.lyrics = nil
	m.curLineIdx = -1
	m.lyricFile = ""
	m.lyricSource = ""
	m.unsynced = false
	m.noLyric = false
	m.infoDismissed = false
	m.errMsg = ""
	m.fetchingMsg = ""
	m.fetching = false
	m.picker = pickerState{}
	m.info, m.infoErr = lyric.Probe(track.File, track.Duration)

	seed := track.Artist + " - " + track.Title
	if seed == " - " {
		seed = track.File
	}
	m.pal = generatePalette(seed)
	m.progress = progress.New(m.pal.progressOpts()...)
	m.progress.Width = max(m.width-6, 0)

	if res := lyric.LoadLocal(track.File, track.Title); len(res.Lines) > 0 {
		m.lyrics = res.Lines
		m.lyricSource = res.Source
		m.lyricFile = res.Path
		return m, nil
	}

	if entry, ok := lyric.LoadCache(track.Artist, track.Title, track.Duration); ok {
		if entry.Instrumental || lyric.IsPlaceholder(entry.LRC) {
			m.noLyric = true
			m.lyricSource = "cache · instrumental"
			return m, nil
		}
		if lines := entry.Lines(); len(lines) > 0 {
			m.lyrics = lines
			m.unsynced = !lyric.HasTimestamps(entry.LRC)
			m.lyricSource = "cache · " + entry.Source
			m.lyricFile = lyric.CachePath(track.Artist, track.Title, track.Duration)
			return m, nil
		}
	}

	m.fetching = true
	m.fetchingMsg = "searching lyrics..."
	m.lyricSource = "fetching"
	return m, autoFetchCmd(track)
}

// queryFor 构造检索参数；标签缺失时退回文件名。
func queryFor(track cmus.Track) lyric.Query {
	title := track.Title
	if title == "" {
		if _, name, ok := util.SplitPath(track.File); ok {
			title = name
		}
	}
	return lyric.Query{
		Title:    title,
		Artist:   track.Artist,
		Album:    track.Album,
		Duration: track.Duration,
	}
}

func autoFetchCmd(track cmus.Track) tea.Cmd {
	q := queryFor(track)
	file := track.File
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), searchTimeout)
		defer cancel()

		best, ok := lyric.FetchBest(ctx, q)
		return autoFetchMsg{file: file, candidate: best, found: ok}
	}
}

// handleAutoFetch 应用自动检索的结果；没找到时进入「无歌词」状态而不是报错。
func (m Model) handleAutoFetch(msg autoFetchMsg) Model {
	m.fetching = false
	m.fetchingMsg = ""

	if msg.file != m.curFile {
		return m
	}

	if !msg.found {
		m.lyrics = nil
		m.noLyric = true
		m.lyricSource = "no lyrics"
		return m
	}

	_ = lyric.SaveCache(lyric.Entry{
		Artist:       m.track.Artist,
		Title:        m.track.Title,
		Album:        msg.candidate.Album,
		Duration:     m.track.Duration,
		Source:       msg.candidate.Source,
		SavedAt:      time.Now(),
		Instrumental: msg.candidate.Instrumental,
		LRC:          msg.candidate.LRC,
		Trans:        msg.candidate.Trans,
	})

	m.lyrics = msg.candidate.Lines()
	m.unsynced = !msg.candidate.Synced
	m.noLyric = len(m.lyrics) == 0
	m.lyricSource = msg.candidate.Source
	m.lyricFile = lyric.CachePath(m.track.Artist, m.track.Title, m.track.Duration)
	m.curLineIdx = -1

	return m
}
