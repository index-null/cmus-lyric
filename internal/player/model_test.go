package player

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/index-null/cmus-lyric/internal/cmus"
	"github.com/index-null/cmus-lyric/internal/lyric"
)

var errBoom = errors.New("boom")

func readFileStat(path string) (os.FileInfo, error) { return os.Stat(path) }

func useTempCache(t *testing.T) {
	t.Helper()
	lyric.SetCacheDir(t.TempDir())
	t.Cleanup(func() { lyric.SetCacheDir("") })
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func playingTrack(file, artist, title string, duration int) cmus.Track {
	return cmus.Track{
		File:     file,
		Artist:   artist,
		Title:    title,
		Album:    "Some Album",
		Duration: duration,
		Position: 10,
		Status:   "playing",
	}
}

// TestApplyTrack_ResetsStaleLyricFile 是 bad-case 的核心回归：
// 切歌后调试面板里的 Local File 仍指向上一首歌的歌词文件。
func TestApplyTrack_ResetsStaleLyricFile(t *testing.T) {
	useTempCache(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "Tai Tomisawa-Formidable Foe II.mp3")
	writeFile(t, file, "fake")

	m := NewModel()
	m.lyricFile = "/Users/someone/Music/Amaranth Cove/Amaranth Cove-Infinite Sustain.lrc"

	got, _ := m.applyTrack(playingTrack(file, "Tai Tomisawa", "Formidable Foe II", 85))
	if got.lyricFile != "" {
		t.Errorf("stale lyric file must be cleared on track change, got %q", got.lyricFile)
	}
	if len(got.lyrics) != 0 {
		t.Errorf("expected no lyrics, got %d", len(got.lyrics))
	}
}

// TestApplyTrack_SameTrackKeepsState：同一首歌的轮询不应清空已有歌词。
func TestApplyTrack_SameTrackKeepsState(t *testing.T) {
	useTempCache(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "song.mp3")
	writeFile(t, file, "fake")

	m := NewModel()
	m, _ = m.applyTrack(playingTrack(file, "A", "Same", 100))
	m.lyrics = []lyric.Line{{TimeCS: 0, Text: "keep me"}}
	m.lyricFile = filepath.Join(dir, "Same.lrc")

	got, _ := m.applyTrack(playingTrack(file, "A", "Same", 100))
	if len(got.lyrics) != 1 {
		t.Errorf("polling the same track must keep lyrics, got %d", len(got.lyrics))
	}
}

func TestApplyTrack_LocalLyricGetsCorrectPath(t *testing.T) {
	useTempCache(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "01-track.mp3")
	lrc := filepath.Join(dir, "Real Title.lrc")
	writeFile(t, file, "fake")
	writeFile(t, lrc, "[00:01.00]found me\n")

	got, _ := NewModel().applyTrack(playingTrack(file, "A", "Real Title", 100))
	if got.lyricFile != lrc {
		t.Errorf("lyricFile = %q, want %q", got.lyricFile, lrc)
	}
	if got.lyricSource != "local" {
		t.Errorf("lyricSource = %q, want local", got.lyricSource)
	}
	if len(got.lyrics) != 1 {
		t.Errorf("expected 1 lyric line, got %d", len(got.lyrics))
	}
	if got.noLyric {
		t.Error("noLyric must be false when a local lyric exists")
	}
}

// TestApplyTrack_PlaceholderLocalIsNoLyric：残留在音乐目录里的
// 「纯音乐，请欣赏」不能被当成歌词。
func TestApplyTrack_PlaceholderLocalIsNoLyric(t *testing.T) {
	useTempCache(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "song.mp3")
	writeFile(t, file, "fake")
	writeFile(t, filepath.Join(dir, "song.lrc"), "［00:05.00］纯音乐，请欣赏\n")

	got, _ := NewModel().applyTrack(playingTrack(file, "A", "song", 100))
	if len(got.lyrics) != 0 {
		t.Errorf("placeholder must not be loaded, got %+v", got.lyrics)
	}
	if got.lyricFile != "" {
		t.Errorf("placeholder file must not be reported as the lyric source, got %q", got.lyricFile)
	}
	// 占位文件被忽略后应当继续联网找真歌词，而不是就此放弃。
	if !got.fetching {
		t.Error("expected the player to keep searching for real lyrics")
	}
}

func TestApplyTrack_CacheHitUsesDurationAwareKey(t *testing.T) {
	useTempCache(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "song.mp3")
	writeFile(t, file, "fake")

	if err := lyric.SaveCache(lyric.Entry{
		Artist: "殷正洋", Title: "东南苦山行", Duration: 238, Source: "netease",
		LRC: "[00:24.92]来自中原一群伙伴结庐东南山\n",
	}); err != nil {
		t.Fatal(err)
	}

	got, _ := NewModel().applyTrack(playingTrack(file, "殷正洋", "东南苦山行", 238))
	if len(got.lyrics) != 1 {
		t.Fatalf("expected the cached lyric to be used, got %d lines", len(got.lyrics))
	}
	if got.lyricSource != "cache · netease" {
		t.Errorf("lyricSource = %q", got.lyricSource)
	}
	if got.fetching {
		t.Error("a cache hit must not trigger a fetch")
	}
}

// TestApplyTrack_CacheFromAnotherDurationIsIgnored
// 同一曲名的另一个版本（伴奏 / 现场版）的缓存不能串味。
func TestApplyTrack_CacheFromAnotherDurationIsIgnored(t *testing.T) {
	useTempCache(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "song.mp3")
	writeFile(t, file, "fake")

	if err := lyric.SaveCache(lyric.Entry{
		Artist: "A", Title: "T", Duration: 999, Source: "netease", LRC: "[00:01.00]other version",
	}); err != nil {
		t.Fatal(err)
	}

	got, _ := NewModel().applyTrack(playingTrack(file, "A", "T", 100))
	if len(got.lyrics) != 0 {
		t.Errorf("cache from a different duration must not be reused, got %+v", got.lyrics)
	}
	if !got.fetching {
		t.Error("expected the player to fall back to fetching")
	}
}

func TestApplyTrack_InstrumentalCacheIsReported(t *testing.T) {
	useTempCache(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "song.mp3")
	writeFile(t, file, "fake")

	if err := lyric.SaveCache(lyric.Entry{
		Artist: "Tai Tomisawa", Title: "Erdtree Knights", Duration: 166,
		Source: "lrclib", Instrumental: true,
	}); err != nil {
		t.Fatal(err)
	}

	got, _ := NewModel().applyTrack(playingTrack(file, "Tai Tomisawa", "Erdtree Knights", 166))
	if !got.noLyric {
		t.Error("instrumental cache entry should put the player into the no-lyric state")
	}
	if len(got.lyrics) != 0 {
		t.Errorf("expected no lyrics, got %d", len(got.lyrics))
	}
}

func TestApplyTrack_ProbesAudioInfo(t *testing.T) {
	useTempCache(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "song.mp3")
	writeFile(t, file, "fake audio")

	got, _ := NewModel().applyTrack(playingTrack(file, "A", "T", 100))
	if got.info.Size != int64(len("fake audio")) {
		t.Errorf("expected probed size %d, got %d", len("fake audio"), got.info.Size)
	}
	if got.info.Path != file {
		t.Errorf("info.Path = %q", got.info.Path)
	}
	if got.infoErr != nil {
		t.Errorf("unexpected probe error: %v", got.infoErr)
	}
}

func TestAutoFetchMsg_AppliesAndCaches(t *testing.T) {
	useTempCache(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "song.mp3")
	writeFile(t, file, "fake")

	m := NewModel()
	m, _ = m.applyTrack(playingTrack(file, "殷正洋", "东南苦山行", 238))

	msg := autoFetchMsg{
		file: file,
		candidate: lyric.Candidate{
			Source: "netease", Title: "东南苦山行", Artist: "殷正洋",
			LRC: "[00:24.92]来自中原一群伙伴结庐东南山\n", Synced: true,
		},
		found: true,
	}
	got := m.handleAutoFetch(msg)

	if len(got.lyrics) != 1 {
		t.Fatalf("expected the fetched lyric to be applied, got %d lines", len(got.lyrics))
	}
	if got.noLyric {
		t.Error("noLyric must be false after a successful fetch")
	}
	if got.lyricSource != "netease" {
		t.Errorf("lyricSource = %q, want netease", got.lyricSource)
	}
	entry, ok := lyric.LoadCache("殷正洋", "东南苦山行", 238)
	if !ok {
		t.Fatal("the fetched lyric must be cached")
	}
	if entry.Source != "netease" {
		t.Errorf("cached source = %q", entry.Source)
	}
}

func TestAutoFetchMsg_NotFoundShowsInfo(t *testing.T) {
	useTempCache(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "song.mp3")
	writeFile(t, file, "fake")

	m := NewModel()
	m, _ = m.applyTrack(playingTrack(file, "Tai Tomisawa", "Erdtree Knights", 166))

	got := m.handleAutoFetch(autoFetchMsg{file: file, found: false})
	if !got.noLyric {
		t.Error("expected noLyric when every source comes back empty")
	}
	if got.errMsg != "" {
		t.Errorf("an empty result must not be rendered as an error, got %q", got.errMsg)
	}
	if got.fetching {
		t.Error("fetching flag must be cleared")
	}
}

func TestAutoFetchMsg_IgnoresStaleTrack(t *testing.T) {
	useTempCache(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "song.mp3")
	writeFile(t, file, "fake")

	m := NewModel()
	m, _ = m.applyTrack(playingTrack(file, "A", "T", 100))
	m.curFile = filepath.Join(dir, "other.mp3")

	got := m.handleAutoFetch(autoFetchMsg{
		file:      file,
		candidate: lyric.Candidate{Source: "netease", LRC: "[00:01.00]stale"},
		found:     true,
	})
	if len(got.lyrics) != 0 {
		t.Errorf("results for a previous track must be discarded, got %+v", got.lyrics)
	}
}

func TestApplyTrack_StoppedKeepsLyrics(t *testing.T) {
	useTempCache(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "song.mp3")
	writeFile(t, file, "fake")

	m := NewModel()
	m.lyrics = []lyric.Line{{TimeCS: 0, Text: "keep"}}

	track := playingTrack(file, "A", "T", 100)
	track.Status = "paused"
	got, _ := m.applyTrack(track)
	if len(got.lyrics) != 1 {
		t.Errorf("paused playback must not drop the loaded lyrics, got %d", len(got.lyrics))
	}
}
