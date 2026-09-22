package player

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/index-null/cmus-lyric/internal/lyric"
)

// sourceStatus 记录每个歌词来源的检索状态，用于在选择器里实时展示。
type sourceStatus struct {
	name  string
	state string // pending / ok / empty / error
	count int
	err   string
}

// pickerState 是「r」键打开的歌词候选选择器。
type pickerState struct {
	active        bool
	query         lyric.Query
	status        []sourceStatus
	candidates    []lyric.Candidate
	cursor        int
	focusPreview  bool
	previewScroll int
}

func (p pickerState) selected() (lyric.Candidate, bool) {
	if len(p.candidates) == 0 || p.cursor < 0 || p.cursor >= len(p.candidates) {
		return lyric.Candidate{}, false
	}
	return p.candidates[p.cursor], true
}

func (m Model) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.confirmQuit = true
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.picker = pickerState{}
		return m, nil
	case "up", "k":
		return m.movePicker(-1), nil
	case "down", "j":
		return m.movePicker(1), nil
	case "pgup":
		return m.movePicker(-10), nil
	case "pgdn":
		return m.movePicker(10), nil
	case "tab":
		return m.togglePickerFocus(), nil
	case "enter":
		return m.applySelected(false), nil
	case "s":
		return m.applySelected(true), nil
	case "x", "X":
		return m.deleteLocalLyric(), nil
	}
	return m, nil
}

// deleteLocalLyric 删除音乐目录里残留的错误歌词文件（旧版本可能写入过）。
func (m Model) deleteLocalLyric() Model {
	lyric.DeleteLocalLyrics(m.track.File, m.track.Title)
	m.picker = pickerState{}
	if _, ok := lyric.FindLocalLyric(m.track.File, m.track.Title); !ok {
		m.lyricFile = ""
	}
	return m
}

// startSearch 并行检索所有歌词来源并打开选择器。
func (m Model) startSearch() (Model, tea.Cmd) {
	q := queryFor(m.track)

	status := make([]sourceStatus, 0, len(lyric.Sources()))
	for _, s := range lyric.Sources() {
		status = append(status, sourceStatus{name: s.Name(), state: "pending"})
	}

	m.picker = pickerState{active: true, query: q, status: status}
	m.errMsg = ""

	return m, tea.Batch(searchCmds(q)...)
}

// searchCmds 为每个来源生成一个并发命令。
func searchCmds(q lyric.Query) []tea.Cmd {
	srcs := lyric.Sources()
	cmds := make([]tea.Cmd, 0, len(srcs))
	for _, s := range srcs {
		cmds = append(cmds, searchSourceCmd(s, q))
	}
	return cmds
}

func searchSourceCmd(s lyric.Source, q lyric.Query) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), searchTimeout)
		defer cancel()

		candidates, err := s.Search(ctx, q)
		return sourceResultMsg{source: s.Name(), candidates: candidates, err: err}
	}
}

// applySourceResult 合并某个来源的检索结果，并重排候选列表。
func (m Model) applySourceResult(msg sourceResultMsg) Model {
	for i := range m.picker.status {
		if m.picker.status[i].name != msg.source {
			continue
		}
		switch {
		case msg.err != nil:
			m.picker.status[i].state = "error"
			m.picker.status[i].err = msg.err.Error()
		case len(msg.candidates) == 0:
			m.picker.status[i].state = "empty"
		default:
			m.picker.status[i].state = "ok"
			m.picker.status[i].count = len(msg.candidates)
		}
	}

	m.picker.candidates = append(m.picker.candidates, msg.candidates...)
	lyric.Rank(m.picker.candidates)
	m.picker.cursor = min(max(m.picker.cursor, 0), max(len(m.picker.candidates)-1, 0))
	m.picker.previewScroll = 0

	return m
}

// movePicker 移动选择光标；焦点在预览区时改为滚动预览。
func (m Model) movePicker(delta int) Model {
	if m.picker.focusPreview {
		m.picker.previewScroll = max(m.picker.previewScroll+delta, 0)
		return m
	}
	if len(m.picker.candidates) == 0 {
		return m
	}
	m.picker.cursor = min(max(m.picker.cursor+delta, 0), len(m.picker.candidates)-1)
	m.picker.previewScroll = 0
	return m
}

func (m Model) togglePickerFocus() Model {
	m.picker.focusPreview = !m.picker.focusPreview
	return m
}

// applySelected 应用当前高亮的候选：写入缓存，必要时同时落盘到本地。
func (m Model) applySelected(saveLocal bool) Model {
	candidate, ok := m.picker.selected()
	m.picker = pickerState{}
	if !ok {
		return m
	}

	m.curLineIdx = -1
	m.errMsg = ""
	m.unsynced = !candidate.Synced
	m.noLyric = candidate.Instrumental || lyric.IsPlaceholder(candidate.LRC)

	_ = lyric.SaveCache(lyric.Entry{
		Artist:       m.track.Artist,
		Title:        m.track.Title,
		Album:        candidate.Album,
		Duration:     m.track.Duration,
		Source:       candidate.Source,
		SavedAt:      time.Now(),
		Instrumental: candidate.Instrumental,
		LRC:          candidate.LRC,
		Trans:        candidate.Trans,
	})

	m.lyrics = candidate.Lines()
	m.lyricSource = candidate.Source
	m.lyricFile = lyric.CachePath(m.track.Artist, m.track.Title, m.track.Duration)

	if saveLocal {
		// 落盘后把展示的歌词文件切到本地路径，来源名保持纯粹。
		if err := lyric.SaveToLocal(m.track.File, m.track.Title, candidate.LRC, candidate.Trans); err == nil {
			if path, found := lyric.FindLocalLyric(m.track.File, m.track.Title); found {
				m.lyricFile = path
			}
		}
	}

	return m
}

// previewLines 返回当前候选的可预览文本（去掉时间轴）。
func (m Model) previewLines() []string {
	candidate, ok := m.picker.selected()
	if !ok {
		return nil
	}

	lines := candidate.Lines()
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, l.Text)
	}
	return out
}
