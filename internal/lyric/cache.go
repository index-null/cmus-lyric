package lyric

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

func CacheDir() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "cmus-lyric")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "cmus-lyric")
}

func CacheKey(artist, title string) string {
	h := sha256.Sum256([]byte(artist + "\x00" + title))
	return fmt.Sprintf("%x", h[:12])
}

func CachePath(artist, title string) string {
	return filepath.Join(CacheDir(), CacheKey(artist, title)+".lrc")
}

func CacheTransPath(artist, title string) string {
	return filepath.Join(CacheDir(), CacheKey(artist, title)+".t.lrc")
}

func LoadFromCache(artist, title string) []Line {
	path := CachePath(artist, title)
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	content = ToUTF8(content)
	lines := BuildLyricMap(splitLines(content))
	if len(lines) == 0 {
		return nil
	}

	mainLines := splitLines(content)
	var tlines []string
	tpath := CacheTransPath(artist, title)
	if tc, err := os.ReadFile(tpath); err == nil {
		tc = ToUTF8(tc)
		tlines = splitLines(tc)
	}

	return BuildLines(mainLines, tlines)
}

func SaveToCache(artist, title, lrc, tlyric string) error {
	dir := CacheDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := CachePath(artist, title)
	if err := Save(path, lrc); err != nil {
		return err
	}

	if len(tlyric) > 0 {
		tpath := CacheTransPath(artist, title)
		_ = Save(tpath, tlyric)
	}

	return nil
}

func splitLines(data []byte) []string {
	return splitLinesStr(string(data))
}

func splitLinesStr(s string) []string {
	if len(s) == 0 {
		return nil
	}
	lines := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
