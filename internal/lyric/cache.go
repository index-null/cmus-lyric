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

func LoadFromCache(artist, title string) ([]Line, string, string) {
	path := CachePath(artist, title)
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, "", ""
	}

	content = ToUTF8(content)
	mainLines := splitLines(content)

	var tlines []string
	var tcontent string
	tpath := CacheTransPath(artist, title)
	if tc, err := os.ReadFile(tpath); err == nil {
		tc = ToUTF8(tc)
		tcontent = string(tc)
		tlines = splitLines(tc)
	}

	if !hasTimestampLines(string(content)) {
		// 无时间戳 → 返回原始内容让调用方处理未同步歌词
		return nil, string(content), tcontent
	}

	return BuildLines(mainLines, tlines), string(content), tcontent
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

func CacheCoverPath(artist, title string) string {
	return filepath.Join(CacheDir(), CacheKey(artist, title)+".cover")
}

func LoadCoverFromCache(artist, title string) []byte {
	data, err := os.ReadFile(CacheCoverPath(artist, title))
	if err != nil {
		return nil
	}
	return data
}

func SaveCoverToCache(artist, title string, data []byte) error {
	dir := CacheDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(CacheCoverPath(artist, title), data, 0644)
}

func DeleteCache(artist, title string) {
	os.Remove(CachePath(artist, title))
	os.Remove(CacheTransPath(artist, title))
	os.Remove(CacheCoverPath(artist, title))
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
