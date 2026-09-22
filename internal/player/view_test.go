package player

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/index-null/cmus-lyric/internal/lyric"
)

// TestViewDebug_ReportsTheCurrentLocalLyric：调试面板必须显示当前这首歌的
// 本地歌词文件，而不是上一首歌残留的路径。
func TestViewDebug_ReportsTheCurrentLocalLyric(t *testing.T) {
	useTempCache(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "song.mp3")
	lrc := filepath.Join(dir, "song.lrc")
	writeFile(t, file, "fake")
	writeFile(t, lrc, "[00:01.00]hello\n")

	const stale = "/Users/someone/Music/Amaranth Cove/Amaranth Cove-Infinite Sustain.lrc"

	m := NewModel()
	m.width, m.height = 120, 32
	m.lyricFile = stale
	m, _ = m.applyTrack(playingTrack(file, "A", "song", 100))
	m.showDebug = true

	out := m.View()
	if !strings.Contains(out, lrc) {
		t.Errorf("debug panel should show %q:\n%s", lrc, out)
	}
	if strings.Contains(out, stale) {
		t.Errorf("debug panel must not show the stale path %q", stale)
	}
}

func TestViewDebug_ReportsCachePathWithDuration(t *testing.T) {
	useTempCache(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "song.mp3")
	writeFile(t, file, "fake")

	if err := lyric.SaveCache(lyric.Entry{
		Artist: "A", Title: "T", Duration: 100, Source: "netease", LRC: "[00:01.00]hi",
	}); err != nil {
		t.Fatal(err)
	}

	m := NewModel()
	m.width, m.height = 120, 32
	m, _ = m.applyTrack(playingTrack(file, "A", "T", 100))
	m.showDebug = true

	out := m.View()
	// 缓存路径很长，会被边框自动换行，因此只断言缓存区段的关键信息。
	if !strings.Contains(out, "Cache File:") {
		t.Errorf("debug panel should have a cache section:\n%s", out)
	}
	if !strings.Contains(out, "(exists)") {
		t.Errorf("cache should be reported as existing:\n%s", out)
	}
	if !strings.Contains(out, "from netease") {
		t.Errorf("debug panel should report where the cached lyric came from:\n%s", out)
	}
}

// TestViewInfo_ShowsBitrate：没有歌词时展示文件信息，而不是空白或报错。
func TestViewInfo_ShowsBitrate(t *testing.T) {
	useTempCache(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "song.mp3")
	writeFile(t, file, "fake audio")

	m := NewModel()
	m.width, m.height = 100, 30
	m, _ = m.applyTrack(playingTrack(file, "Tai Tomisawa", "Erdtree Knights", 166))
	m.showInfo = true

	out := m.View()
	for _, want := range []string{"Bitrate", "Format", "Size", "Duration"} {
		if !strings.Contains(out, want) {
			t.Errorf("info panel should contain %q:\n%s", want, out)
		}
	}
}

// TestToggleInfo_DismissesAutoPanel：无歌词时信息面板会自动出现，按 i 应该能关掉。
func TestToggleInfo_DismissesAutoPanel(t *testing.T) {
	useTempCache(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "song.mp3")
	writeFile(t, file, "fake")

	m := NewModel()
	m.width, m.height = 100, 30
	m, _ = m.applyTrack(playingTrack(file, "Tai Tomisawa", "Erdtree Knights", 166))
	m = m.handleAutoFetch(autoFetchMsg{file: file, found: false})
	if !m.noLyric {
		t.Fatal("expected the no-lyric state")
	}
	if !strings.Contains(m.View(), "Bitrate") {
		t.Fatal("the info panel should appear automatically")
	}

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	got, ok := model.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", model)
	}
	if strings.Contains(got.View(), "Bitrate") {
		t.Error("pressing i should dismiss the auto info panel")
	}
}

func TestViewPicker_RendersCandidatesAndPreview(t *testing.T) {
	useTempCache(t)
	m := NewModel()
	m.width, m.height = 120, 30
	m.track = playingTrack("/tmp/song.mp3", "殷正洋", "东南苦山行", 238)

	m, _ = m.startSearch()
	m = m.applySourceResult(sourceResultMsg{
		source: "netease",
		candidates: []lyric.Candidate{{
			Source: "netease", Title: "东南苦山行", Artist: "殷正洋",
			LRC: "[00:24.92]来自中原一群伙伴结庐东南山\n", Synced: true, Score: 1,
		}},
	})

	out := m.View()
	for _, want := range []string{"netease", "东南苦山行", "Preview", "来自中原一群伙伴结庐东南山"} {
		if !strings.Contains(out, want) {
			t.Errorf("picker should contain %q:\n%s", want, out)
		}
	}
}

// TestViewPicker_EmptyStateDoesNotPanic：还没有任何结果时也要能渲染。
func TestViewPicker_EmptyStateDoesNotPanic(t *testing.T) {
	useTempCache(t)
	m := NewModel()
	m.width, m.height = 80, 24
	m.track = playingTrack("/tmp/song.mp3", "A", "T", 100)
	m, _ = m.startSearch()

	if out := m.View(); out == "" {
		t.Error("expected the picker to render while searching")
	}
}

func TestUpdatePickerKeys(t *testing.T) {
	useTempCache(t)
	m := NewModel()
	m.width, m.height = 100, 30
	m.track = playingTrack("/tmp/song.mp3", "A", "T", 100)
	m, _ = m.startSearch()
	m = m.applySourceResult(sourceResultMsg{
		source: "lrclib",
		candidates: []lyric.Candidate{
			{Source: "lrclib", Title: "T", LRC: "[00:01.00]one", Synced: true, Score: 0.9},
			{Source: "lrclib", Title: "T", LRC: "[00:01.00]two", Synced: true, Score: 0.5},
		},
	})

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got, ok := model.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", model)
	}
	if got.picker.cursor != 1 {
		t.Errorf("cursor should move down, got %d", got.picker.cursor)
	}

	model, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got, ok = model.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", model)
	}
	if got.picker.active {
		t.Error("Enter should close the picker")
	}
	if len(got.lyrics) != 1 || got.lyrics[0].Text != "two" {
		t.Errorf("Enter should apply the highlighted candidate, got %+v", got.lyrics)
	}
}

// TestUpdateQuitConfirm：q 不再直接退出，必须先确认（默认否）。
func TestUpdateQuitConfirm(t *testing.T) {
	m := NewModel()
	m.width, m.height = 100, 30

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got, ok := model.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", model)
	}
	if cmd != nil {
		t.Fatal("q must not quit without confirmation")
	}
	if !got.confirmQuit {
		t.Fatal("q should open the confirm dialog")
	}
	if out := got.View(); !strings.Contains(out, "Quit cmus-lyric?") {
		t.Errorf("confirm dialog should be rendered:\n%s", out)
	}

	// 默认是否：随便按一个键都会取消。
	model, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	got, ok = model.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", model)
	}
	if got.confirmQuit {
		t.Error("n should dismiss the confirm dialog")
	}

	// Ctrl+C 仍然立即退出。
	if _, cmd = got.Update(tea.KeyMsg{Type: tea.KeyCtrlC}); cmd == nil {
		t.Error("ctrl+c should quit immediately")
	}

	// y 才真正退出。
	model, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got = model.(Model)
	if _, cmd = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}); cmd == nil {
		t.Error("y should quit")
	}
}

// TestUpdateQuitConfirmFromPicker：选择器里按 q 也要确认。
func TestUpdateQuitConfirmFromPicker(t *testing.T) {
	useTempCache(t)
	m := NewModel()
	m.width, m.height = 100, 30
	m.track = playingTrack("/tmp/song.mp3", "A", "T", 100)
	m, _ = m.startSearch()

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got, ok := model.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", model)
	}
	if cmd != nil {
		t.Fatal("q must not quit without confirmation")
	}
	if !got.confirmQuit {
		t.Fatal("q should open the confirm dialog")
	}

	// Esc 取消后应回到选择器，而不是退出。
	model, _ = got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got = model.(Model)
	if got.confirmQuit || !got.picker.active {
		t.Errorf("Esc should cancel and return to the picker, confirm=%v picker=%v", got.confirmQuit, got.picker.active)
	}
}

func TestUpdatePickerEscapeCloses(t *testing.T) {
	useTempCache(t)
	m := NewModel()
	m.width, m.height = 100, 30
	m.track = playingTrack("/tmp/song.mp3", "A", "T", 100)
	m, _ = m.startSearch()

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got, ok := model.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", model)
	}
	if got.picker.active {
		t.Error("Esc should close the picker")
	}
}
