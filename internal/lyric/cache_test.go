package lyric

import (
	"os"
	"testing"
)

func TestCacheKey_Deterministic(t *testing.T) {
	k1 := CacheKey("Artist", "Title")
	k2 := CacheKey("Artist", "Title")
	if k1 != k2 {
		t.Errorf("cache key should be deterministic: %q != %q", k1, k2)
	}
}

func TestCacheKey_Unique(t *testing.T) {
	k1 := CacheKey("Artist", "Title A")
	k2 := CacheKey("Artist", "Title B")
	if k1 == k2 {
		t.Errorf("different titles should produce different keys")
	}
}

func TestCacheKey_ArtistMatters(t *testing.T) {
	k1 := CacheKey("Artist A", "Title")
	k2 := CacheKey("Artist B", "Title")
	if k1 == k2 {
		t.Errorf("different artists should produce different keys")
	}
}

func TestSaveAndLoadFromCache(t *testing.T) {
	artist := "RoundtripArtist_" + t.Name()
	title := "RoundtripTitle_" + t.Name()
	lrc := "[00:05.00]Hello\n[00:10.00]World\n"
	tlyric := "[00:05.00]你好\n[00:10.00]世界\n"

	err := SaveToCache(artist, title, lrc, tlyric)
	if err != nil {
		t.Fatalf("SaveToCache failed: %v", err)
	}
	t.Cleanup(func() {
		os.Remove(CachePath(artist, title))
		os.Remove(CacheTransPath(artist, title))
	})

	result := LoadFromCache(artist, title)
	if result == nil {
		t.Fatal("expected non-nil result from cache")
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(result))
	}
	if result[0].Text != "Hello" {
		t.Errorf("expected 'Hello', got %q", result[0].Text)
	}
	if result[0].Trans != "你好" {
		t.Errorf("expected translation '你好', got %q", result[0].Trans)
	}
}

func TestSaveToCache_CreatesDir(t *testing.T) {
	err := SaveToCache("CacheTestArtist", "CacheTestTitle", "[00:01.00]test", "")
	if err != nil {
		t.Fatalf("SaveToCache failed: %v", err)
	}

	path := CachePath("CacheTestArtist", "CacheTestTitle")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected cache file at %s", path)
	}

	os.Remove(path)
	os.Remove(CacheTransPath("CacheTestArtist", "CacheTestTitle"))
}

func TestLoadFromCache_NotFound(t *testing.T) {
	result := LoadFromCache("NonExistent_"+t.Name(), "Song_"+t.Name())
	if result != nil {
		t.Errorf("expected nil for missing cache, got %v", result)
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
