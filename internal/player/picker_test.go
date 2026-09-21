package player

import (
	"path/filepath"
	"testing"

	"github.com/index-null/cmus-lyric/internal/lyric"
)

func TestSearchCmds_OnePerSource(t *testing.T) {
	if got, want := len(searchCmds(lyric.Query{Title: "x"})), len(lyric.Sources()); got != want {
		t.Errorf("expected one command per source (%d), got %d", want, got)
	}
}

func TestStartSearch_InitialisesStatuses(t *testing.T) {
	useTempCache(t)
	m := NewModel()
	m.track = playingTrack("/tmp/song.mp3", "A", "T", 100)

	got, _ := m.startSearch()
	if !got.picker.active {
		t.Fatal("expected the picker to be active")
	}
	if len(got.picker.status) != len(lyric.Sources()) {
		t.Errorf("expected %d source statuses, got %d", len(lyric.Sources()), len(got.picker.status))
	}
	for _, s := range got.picker.status {
		if s.state != "pending" {
			t.Errorf("source %s should start as pending, got %q", s.name, s.state)
		}
	}
	if got.picker.query.Title != "T" || got.picker.query.Artist != "A" {
		t.Errorf("query not populated: %+v", got.picker.query)
	}
}

func TestApplySourceResult_CollectsAndRanks(t *testing.T) {
	m := NewModel()
	m.picker = pickerState{
		active: true,
		status: []sourceStatus{{name: "lrclib", state: "pending"}, {name: "netease", state: "pending"}},
	}

	got := m.applySourceResult(sourceResultMsg{
		source: "lrclib",
		candidates: []lyric.Candidate{
			{Source: "lrclib", Title: "T", Score: 0.40, LRC: "[00:01.00]weak"},
			{Source: "lrclib", Title: "T", Score: 0.90, LRC: "[00:01.00]strong", Synced: true},
		},
	})

	if len(got.picker.candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(got.picker.candidates))
	}
	if got.picker.candidates[0].LRC != "[00:01.00]strong" {
		t.Errorf("best candidate should be first, got %q", got.picker.candidates[0].LRC)
	}
	if got.picker.status[0].state != "ok" || got.picker.status[0].count != 2 {
		t.Errorf("status not updated: %+v", got.picker.status[0])
	}
	if got.picker.status[1].state != "pending" {
		t.Errorf("other sources must stay pending: %+v", got.picker.status[1])
	}
}

func TestApplySourceResult_ErrorState(t *testing.T) {
	m := NewModel()
	m.picker = pickerState{active: true, status: []sourceStatus{{name: "kugou"}}}

	got := m.applySourceResult(sourceResultMsg{source: "kugou", err: errBoom})
	if got.picker.status[0].state != "error" {
		t.Errorf("expected error state, got %+v", got.picker.status[0])
	}
	if len(got.picker.candidates) != 0 {
		t.Errorf("expected no candidates, got %d", len(got.picker.candidates))
	}
}

func TestApplySourceResult_EmptyState(t *testing.T) {
	m := NewModel()
	m.picker = pickerState{active: true, status: []sourceStatus{{name: "lyrics.ovh"}}}

	got := m.applySourceResult(sourceResultMsg{source: "lyrics.ovh"})
	if got.picker.status[0].state != "empty" {
		t.Errorf("expected empty state, got %q", got.picker.status[0].state)
	}
}

func TestPickerNavigation_Clamps(t *testing.T) {
	m := NewModel()
	m.picker = pickerState{active: true, candidates: []lyric.Candidate{{Title: "a"}, {Title: "b"}, {Title: "c"}}}

	if got := m.movePicker(-1); got.picker.cursor != 0 {
		t.Errorf("cursor should stay at 0, got %d", got.picker.cursor)
	}
	if got := m.movePicker(1); got.picker.cursor != 1 {
		t.Errorf("cursor should be 1, got %d", got.picker.cursor)
	}
	if got := m.movePicker(10); got.picker.cursor != 2 {
		t.Errorf("cursor should clamp to the last index, got %d", got.picker.cursor)
	}
}

func TestPickerNavigation_NoCandidates(t *testing.T) {
	m := NewModel()
	m.picker = pickerState{active: true}
	if got := m.movePicker(1); got.picker.cursor != 0 {
		t.Errorf("cursor must stay 0 without candidates, got %d", got.picker.cursor)
	}
}

func TestPickerTogglePreviewFocus(t *testing.T) {
	m := NewModel()
	m.picker = pickerState{active: true}
	if m.togglePickerFocus().picker.focusPreview != true {
		t.Error("expected focus to move to the preview")
	}
	if m.togglePickerFocus().togglePickerFocus().picker.focusPreview != false {
		t.Error("expected focus to move back to the list")
	}
}

func TestApplySelected_UsesCandidateAndCachesIt(t *testing.T) {
	useTempCache(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "song.mp3")
	writeFile(t, file, "fake")

	m := NewModel()
	m.track = playingTrack(file, "殷正洋", "东南苦山行", 238)
	m.picker = pickerState{
		active: true,
		query:  lyric.Query{Title: "东南苦山行", Artist: "殷正洋", Duration: 238},
		candidates: []lyric.Candidate{
			{Source: "netease", Title: "东南苦山行", Artist: "殷正洋", Album: "雨中的歉意",
				LRC: "[00:24.92]来自中原一群伙伴结庐东南山\n", Synced: true},
			{Source: "kugou", Title: "东南苦山行", LRC: "[00:24.92]酷狗版本\n", Synced: true},
		},
		cursor: 1,
	}

	got := m.applySelected(false)
	if got.picker.active {
		t.Error("the picker must close after a selection")
	}
	if len(got.lyrics) != 1 || got.lyrics[0].Text != "酷狗版本" {
		t.Fatalf("the highlighted candidate should be applied, got %+v", got.lyrics)
	}
	if got.lyricSource != "kugou" {
		t.Errorf("lyricSource = %q", got.lyricSource)
	}
	entry, ok := lyric.LoadCache("殷正洋", "东南苦山行", 238)
	if !ok {
		t.Fatal("selection must be cached")
	}
	if entry.Source != "kugou" {
		t.Errorf("cached source = %q", entry.Source)
	}
}

func TestApplySelected_SavesLocalFile(t *testing.T) {
	useTempCache(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "song.mp3")
	writeFile(t, file, "fake")

	m := NewModel()
	m.track = playingTrack(file, "A", "T", 100)
	m.picker = pickerState{
		active: true,
		query:  lyric.Query{Title: "T", Artist: "A", Duration: 100},
		candidates: []lyric.Candidate{
			{Source: "lrclib", Title: "T", LRC: "[00:01.00]saved locally\n", Synced: true},
		},
	}

	got := m.applySelected(true)
	if got.lyricSource != "lrclib" {
		t.Errorf("lyricSource = %q", got.lyricSource)
	}
	localPath := filepath.Join(dir, "T.lrc")
	if _, err := readFileStat(localPath); err != nil {
		t.Errorf("expected the lyric to be written next to the audio file: %v", err)
	}
}

func TestApplySelected_NoCandidatesClosesPicker(t *testing.T) {
	m := NewModel()
	m.picker = pickerState{active: true}
	got := m.applySelected(false)
	if got.picker.active {
		t.Error("picker should close even without candidates")
	}
}

// TestDeleteLocalLyric_RemovesStaleFile：旧版本可能在音乐目录里写入过错误歌词，
// 选择器里按 x 应该能清掉。
func TestDeleteLocalLyric_RemovesStaleFile(t *testing.T) {
	useTempCache(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "song.mp3")
	local := filepath.Join(dir, "T.lrc")
	writeFile(t, file, "fake")
	writeFile(t, local, "[00:01.00]wrong\n")

	m := NewModel()
	m.track = playingTrack(file, "A", "T", 100)
	m.lyricFile = local

	got := m.deleteLocalLyric()
	if _, err := readFileStat(local); err == nil {
		t.Error("expected the local lyric file to be deleted")
	}
	if got.lyricFile != "" {
		t.Errorf("lyricFile should be cleared, got %q", got.lyricFile)
	}
}

func TestPreviewLines(t *testing.T) {
	m := NewModel()
	m.picker = pickerState{
		active: true,
		candidates: []lyric.Candidate{
			{Source: "netease", Title: "T", LRC: "[00:01.00]first\n[00:02.00]second\n"},
		},
	}
	lines := m.previewLines()
	if len(lines) != 2 {
		t.Fatalf("expected 2 preview lines, got %d", len(lines))
	}
	if lines[0] != "first" {
		t.Errorf("preview lines must drop timestamps, got %q", lines[0])
	}
}
