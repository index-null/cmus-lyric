package lyric

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// cacheDirOverride 让测试可以把缓存重定向到临时目录，避免污染真实缓存。
var cacheDirOverride string

// SetCacheDir 覆盖缓存目录，传空字符串恢复默认位置。
func SetCacheDir(dir string) { cacheDirOverride = dir }

// CacheDir 返回歌词缓存目录。
func CacheDir() string {
	if cacheDirOverride != "" {
		return cacheDirOverride
	}
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "cmus-lyric")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "cmus-lyric")
}

// Entry 是一份缓存歌词及其来源信息。
type Entry struct {
	Artist       string    `json:"artist"`
	Title        string    `json:"title"`
	Album        string    `json:"album"`
	Duration     int       `json:"duration"`
	Source       string    `json:"source"`
	SavedAt      time.Time `json:"saved_at"`
	Instrumental bool      `json:"instrumental"`
	LRC          string    `json:"-"`
	Trans        string    `json:"-"`
}

// CacheKey 计算缓存键。时长参与计算，避免同一曲名的不同版本共用一份缓存。
func CacheKey(artist, title string, duration int) string {
	h := sha256.Sum256([]byte(artist + "\x00" + title + "\x00" + strconv.Itoa(duration)))
	return fmt.Sprintf("%x", h[:12])
}

// CachePath 返回主歌词缓存文件。
func CachePath(artist, title string, duration int) string {
	return filepath.Join(CacheDir(), CacheKey(artist, title, duration)+".lrc")
}

// CacheTransPath 返回翻译歌词缓存文件。
func CacheTransPath(artist, title string, duration int) string {
	return filepath.Join(CacheDir(), CacheKey(artist, title, duration)+".t.lrc")
}

// CacheMetaPath 返回缓存元信息文件。
func CacheMetaPath(artist, title string, duration int) string {
	return filepath.Join(CacheDir(), CacheKey(artist, title, duration)+".json")
}

// CacheCoverPath 返回封面缓存文件。
func CacheCoverPath(artist, title string, duration int) string {
	return filepath.Join(CacheDir(), CacheKey(artist, title, duration)+".cover")
}

// Lines 把缓存内容解析成可渲染的歌词行。
func (e Entry) Lines() []Line {
	if e.LRC == "" {
		return nil
	}
	return Candidate{LRC: e.LRC, Trans: e.Trans}.Lines()
}

// SaveCache 写入歌词、翻译与来源元信息。
func SaveCache(e Entry) error {
	dir := CacheDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if err := Save(CachePath(e.Artist, e.Title, e.Duration), e.LRC); err != nil {
		return err
	}

	if len(e.Trans) > 0 {
		if err := Save(CacheTransPath(e.Artist, e.Title, e.Duration), e.Trans); err != nil {
			return err
		}
	}

	if e.SavedAt.IsZero() {
		e.SavedAt = time.Now()
	}
	meta, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return os.WriteFile(CacheMetaPath(e.Artist, e.Title, e.Duration), meta, 0644)
}

// LoadCache 读取缓存；不存在则返回 false。
func LoadCache(artist, title string, duration int) (Entry, bool) {
	content, err := os.ReadFile(CachePath(artist, title, duration))
	if err != nil {
		return Entry{}, false
	}

	e := Entry{
		Artist:   artist,
		Title:    title,
		Duration: duration,
		LRC:      string(ToUTF8(content)),
	}

	if meta, err := os.ReadFile(CacheMetaPath(artist, title, duration)); err == nil {
		var parsed Entry
		if json.Unmarshal(meta, &parsed) == nil {
			e.Album = parsed.Album
			e.Source = parsed.Source
			e.SavedAt = parsed.SavedAt
			e.Instrumental = parsed.Instrumental
		}
	}

	if trans, err := os.ReadFile(CacheTransPath(artist, title, duration)); err == nil {
		e.Trans = string(ToUTF8(trans))
	}

	return e, true
}

// DeleteCache 删除某个条目产生的所有文件。
func DeleteCache(artist, title string, duration int) {
	os.Remove(CachePath(artist, title, duration))
	os.Remove(CacheTransPath(artist, title, duration))
	os.Remove(CacheMetaPath(artist, title, duration))
	os.Remove(CacheCoverPath(artist, title, duration))
}

// LoadCoverFromCache 读取缓存封面。
func LoadCoverFromCache(artist, title string, duration int) []byte {
	data, err := os.ReadFile(CacheCoverPath(artist, title, duration))
	if err != nil {
		return nil
	}
	return data
}

// SaveCoverToCache 写入缓存封面。
func SaveCoverToCache(artist, title string, duration int, data []byte) error {
	if err := os.MkdirAll(CacheDir(), 0755); err != nil {
		return err
	}
	return os.WriteFile(CacheCoverPath(artist, title, duration), data, 0644)
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
