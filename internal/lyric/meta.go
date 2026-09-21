package lyric

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dhowden/tag"
)

// AudioInfo 是音频文件的可展示元信息，用于没有歌词时展示「歌曲信息」。
type AudioInfo struct {
	Path          string
	Size          int64
	Modified      time.Time
	FileType      string // MP3 / FLAC / M4A ...
	TagFormat     string // ID3v2.3 / VORBIS ...
	Bitrate       int    // kbps（平均）
	Duration      int    // 秒
	Title         string
	Artist        string
	AlbumArtist   string
	Album         string
	Genre         string
	Composer      string
	Comment       string
	Year          int
	Track         int
	TrackTotal    int
	Disc          int
	DiscTotal     int
	EmbeddedLyric bool
	EmbeddedCover bool
}

// Probe 读取文件大小/码率与标签元信息；标签缺失不会导致失败。
func Probe(path string, duration int) (AudioInfo, error) {
	st, err := os.Stat(path)
	if err != nil {
		return AudioInfo{}, err
	}

	info := AudioInfo{
		Path:     path,
		Size:     st.Size(),
		Modified: st.ModTime(),
		Duration: duration,
		Bitrate:  BitrateOf(st.Size(), duration),
	}

	f, err := os.Open(path)
	if err != nil {
		return info, nil
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return info, nil
	}

	info.FileType = string(m.FileType())
	info.TagFormat = string(m.Format())
	info.Title = m.Title()
	info.Artist = m.Artist()
	info.AlbumArtist = m.AlbumArtist()
	info.Album = m.Album()
	info.Genre = m.Genre()
	info.Composer = m.Composer()
	info.Comment = m.Comment()
	info.Year = m.Year()
	info.Track, info.TrackTotal = m.Track()
	info.Disc, info.DiscTotal = m.Disc()
	info.EmbeddedLyric = strings.TrimSpace(m.Lyrics()) != ""
	if pic := m.Picture(); pic != nil {
		info.EmbeddedCover = len(pic.Data) > 0
	}

	return info, nil
}

// FormatLabel 返回「MP3 · ID3v2.3」这样的格式描述。
func (a AudioInfo) FormatLabel() string {
	switch {
	case a.FileType != "" && a.TagFormat != "":
		return a.FileType + " · " + a.TagFormat
	case a.FileType != "":
		return a.FileType
	case a.TagFormat != "":
		return a.TagFormat
	default:
		return "-"
	}
}

// BitrateLabel 返回可读的码率，未知时返回 "-"。
func (a AudioInfo) BitrateLabel() string {
	if a.Bitrate <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d kbps", a.Bitrate)
}

// BitrateOf 由文件大小和时长估算平均码率（kbps）。
func BitrateOf(size int64, duration int) int {
	if size <= 0 || duration <= 0 {
		return 0
	}
	return int(size * 8 / int64(duration) / 1000)
}

// HumanSize 把字节数格式化成可读字符串。
func HumanSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size) / unit
	units := []string{"KB", "MB", "GB", "TB"}
	i := 0
	for value >= unit && i < len(units)-1 {
		value /= unit
		i++
	}
	return fmt.Sprintf("%.1f %s", value, units[i])
}
