package lyric

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// useTempCacheDir 把缓存目录重定向到临时目录，避免测试污染真实缓存。
func useTempCacheDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := cacheDirOverride
	cacheDirOverride = dir
	t.Cleanup(func() { cacheDirOverride = old })
	return dir
}

func TestCacheKey_Deterministic(t *testing.T) {
	k1 := CacheKey("Artist", "Title", 200)
	k2 := CacheKey("Artist", "Title", 200)
	if k1 != k2 {
		t.Errorf("cache key should be deterministic: %q != %q", k1, k2)
	}
}

func TestCacheKey_TitleMatters(t *testing.T) {
	if CacheKey("Artist", "Title A", 200) == CacheKey("Artist", "Title B", 200) {
		t.Error("different titles should produce different keys")
	}
}

func TestCacheKey_ArtistMatters(t *testing.T) {
	if CacheKey("Artist A", "Title", 200) == CacheKey("Artist B", "Title", 200) {
		t.Error("different artists should produce different keys")
	}
}

// TestCacheKey_DurationMatters 是 bad-case 的核心回归：
// 同一歌手/曲名的不同版本（原曲 / 伴奏 / 钢琴版）时长不同，不能共用一份缓存。
func TestCacheKey_DurationMatters(t *testing.T) {
	if CacheKey("Tai Tomisawa", "Erdtree Knights", 166) == CacheKey("Tai Tomisawa", "Erdtree Knights", 170) {
		t.Error("different durations should produce different keys (covers/remixes must not share cache)")
	}
}

func TestSaveAndLoadCache_Roundtrip(t *testing.T) {
	useTempCacheDir(t)

	e := Entry{
		Artist:   "RoundtripArtist",
		Title:    "RoundtripTitle",
		Album:    "RoundtripAlbum",
		Duration: 238,
		Source:   "netease",
		SavedAt:  time.Now().Truncate(time.Second),
		LRC:      "[00:05.00]Hello\n[00:10.00]World\n",
		Trans:    "[00:05.00]你好\n[00:10.00]世界\n",
	}
	if err := SaveCache(e); err != nil {
		t.Fatalf("SaveCache failed: %v", err)
	}

	got, ok := LoadCache(e.Artist, e.Title, e.Duration)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.Source != e.Source {
		t.Errorf("source: got %q, want %q", got.Source, e.Source)
	}
	if got.Album != e.Album {
		t.Errorf("album: got %q, want %q", got.Album, e.Album)
	}
	if got.LRC != e.LRC {
		t.Errorf("lrc: got %q, want %q", got.LRC, e.LRC)
	}
	if got.Trans != e.Trans {
		t.Errorf("trans: got %q, want %q", got.Trans, e.Trans)
	}
}

func TestLoadCache_NotFound(t *testing.T) {
	useTempCacheDir(t)
	if _, ok := LoadCache("nobody", "nothing", 1); ok {
		t.Error("expected cache miss")
	}
}

// TestLoadCache_InstrumentsNotSharedAcrossDurations 验证不同时长不会串味。
func TestLoadCache_InstrumentsNotSharedAcrossDurations(t *testing.T) {
	useTempCacheDir(t)

	if err := SaveCache(Entry{Artist: "A", Title: "T", Duration: 100, LRC: "[00:01.00]short"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveCache(Entry{Artist: "A", Title: "T", Duration: 300, LRC: "[00:01.00]long"}); err != nil {
		t.Fatal(err)
	}

	a, ok := LoadCache("A", "T", 100)
	if !ok || a.LRC != "[00:01.00]short" {
		t.Errorf("duration 100 entry wrong: %+v ok=%v", a, ok)
	}
	b, ok := LoadCache("A", "T", 300)
	if !ok || b.LRC != "[00:01.00]long" {
		t.Errorf("duration 300 entry wrong: %+v ok=%v", b, ok)
	}
}

func TestSaveCache_UnsyncedContent(t *testing.T) {
	useTempCacheDir(t)

	e := Entry{Artist: "A", Title: "T", Duration: 10, LRC: "line one\nline two\n", Source: "lyrics.ovh"}
	if err := SaveCache(e); err != nil {
		t.Fatal(err)
	}
	got, ok := LoadCache("A", "T", 10)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.LRC != e.LRC {
		t.Errorf("unsynced content must be preserved verbatim: %q", got.LRC)
	}
	if HasTimestamps(got.LRC) {
		t.Error("content should stay unsynced")
	}
}

func TestSaveCache_InstrumentalMarker(t *testing.T) {
	useTempCacheDir(t)

	e := Entry{Artist: "A", Title: "T", Duration: 85, Instrumental: true, Source: "lrclib"}
	if err := SaveCache(e); err != nil {
		t.Fatal(err)
	}
	got, ok := LoadCache("A", "T", 85)
	if !ok {
		t.Fatal("expected cache hit for instrumental marker")
	}
	if !got.Instrumental {
		t.Error("instrumental flag must survive the roundtrip")
	}
}

func TestDeleteCache_RemovesEveryArtifact(t *testing.T) {
	dir := useTempCacheDir(t)

	e := Entry{Artist: "Del", Title: "Me", Duration: 42, LRC: "[00:01.00]x", Trans: "[00:01.00]y", Source: "lrclib"}
	if err := SaveCache(e); err != nil {
		t.Fatal(err)
	}
	if err := SaveCoverToCache("Del", "Me", 42, []byte("img")); err != nil {
		t.Fatal(err)
	}

	DeleteCache("Del", "Me", 42)

	for _, p := range []string{
		CachePath("Del", "Me", 42),
		CacheTransPath("Del", "Me", 42),
		CacheMetaPath("Del", "Me", 42),
		CacheCoverPath("Del", "Me", 42),
	} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed", p)
		}
	}
	_ = dir
}

func TestCoverCache(t *testing.T) {
	useTempCacheDir(t)

	if got := LoadCoverFromCache("cov", "er", 1); got != nil {
		t.Errorf("expected nil for missing cover, got %v", got)
	}
	if err := SaveCoverToCache("cov", "er", 1, []byte("cover-bytes")); err != nil {
		t.Fatal(err)
	}
	got := LoadCoverFromCache("cov", "er", 1)
	if string(got) != "cover-bytes" {
		t.Errorf("unexpected cover data: %q", got)
	}
}

func TestCacheDir_HonoursOverride(t *testing.T) {
	dir := useTempCacheDir(t)
	if CacheDir() != dir {
		t.Errorf("CacheDir() = %q, want %q", CacheDir(), dir)
	}
	if _, err := os.Stat(filepath.Join(dir, "x")); err == nil {
		t.Error("sanity: temp dir should not contain unexpected files")
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"a\nb\nc", 3},
		{"a\nb\nc\n", 3},
		{"single", 1},
	}

	for _, tt := range tests {
		result := splitLinesStr(tt.input)
		if len(result) != tt.expected {
			t.Errorf("splitLinesStr(%q): expected %d lines, got %d: %v", tt.input, tt.expected, len(result), result)
		}
	}
}
