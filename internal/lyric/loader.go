package lyric

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dhowden/tag"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	"github.com/index-null/cmus-lyric/internal/util"
)

// lyricExtensions 本地歌词文件的候选后缀，顺序即优先级。
var lyricExtensions = []string{".lyric", ".lrc"}

// Line 是一行歌词。TimeCS 为 -1 表示没有时间轴。
type Line struct {
	TimeCS int
	Text   string
	Trans  string
}

// LoadResult 描述本地歌词的加载结果。
type LoadResult struct {
	Lines  []Line
	Path   string // 本地歌词文件路径；内嵌歌词时为空
	Source string // "embedded" 或 "local"
}

var lrcTimeRe = regexp.MustCompile(`^\[([0-9]+):([0-9]+)\.?([0-9]*)]\s*(.*)`)

// Load 返回本地歌词（优先内嵌标签，其次同目录歌词文件）。
func Load(path, title string) []Line {
	return LoadLocal(path, title).Lines
}

// LoadLocal 同上，但额外报告歌词来自哪里，便于调试面板如实展示。
func LoadLocal(file, title string) LoadResult {
	if lines := loadEmbedded(file); lines != nil {
		return LoadResult{Lines: lines, Source: "embedded"}
	}

	path, ok := FindLocalLyric(file, title)
	if !ok {
		return LoadResult{}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return LoadResult{}
	}

	body := NormalizeLRC(string(ToUTF8(content)))
	// 「纯音乐，请欣赏」这类占位内容不算歌词。
	if IsPlaceholder(body) {
		return LoadResult{}
	}

	var tlines []string
	if tc, err := os.ReadFile(transPathFor(path)); err == nil {
		tlines = splitLinesStr(NormalizeLRC(string(ToUTF8(tc))))
	}

	lines := BuildLines(splitLinesStr(body), tlines)
	if len(lines) == 0 {
		return LoadResult{}
	}
	return LoadResult{Lines: lines, Path: path, Source: "local"}
}

// LocalLyricPaths 按优先级列出可能的本地歌词路径。
func LocalLyricPaths(file, title string) []string {
	dir, name, ok := util.SplitPath(file)
	if !ok {
		return nil
	}

	stems := []string{name}
	// 标题可能来自标签，与文件名不一致，也值得找一找。
	if title != "" && title != name && !strings.ContainsAny(title, `/\:`) {
		stems = append(stems, title)
	}

	paths := make([]string, 0, len(stems)*len(lyricExtensions))
	for _, stem := range stems {
		for _, ext := range lyricExtensions {
			paths = append(paths, filepath.Join(dir, stem+ext))
		}
	}
	return paths
}

// FindLocalLyric 找到第一个真实存在的本地歌词文件。
func FindLocalLyric(file, title string) (string, bool) {
	for _, p := range LocalLyricPaths(file, title) {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
	}
	return "", false
}

func transPathFor(path string) string {
	ext := filepath.Ext(path)
	return strings.TrimSuffix(path, ext) + ".t" + ext
}

// BuildLines 把主歌词与翻译按时间轴合并成有序的歌词行。
func BuildLines(lines, tlines []string) []Line {
	mainOffset := parseOffsetLines(lines)
	transOffset := parseOffsetLines(tlines)

	lyricMap := BuildLyricMapWithOffset(lines, mainOffset)
	tlyricMap := BuildLyricMapWithOffset(tlines, transOffset)

	type entry struct {
		timeCS int
		text   string
		trans  string
	}

	entries := make([]entry, 0, len(lyricMap))
	for k, v := range lyricMap {
		entries = append(entries, entry{k, v, tlyricMap[k]})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].timeCS < entries[j].timeCS
	})

	if len(entries) == 0 {
		return nil
	}

	result := make([]Line, 0, len(entries))
	for _, e := range entries {
		result = append(result, Line{
			TimeCS: e.timeCS,
			Text:   e.text,
			Trans:  e.trans,
		})
	}

	return result
}

// BuildUnsyncedLines 将不带时间轴的纯文本转换为 Line 切片（TimeCS = -1），
// 渲染层应全部展示，不做进度高亮。
func BuildUnsyncedLines(content string) []Line {
	var result []Line
	for _, l := range splitLinesStr(content) {
		l = strings.TrimSpace(l)
		if l != "" {
			result = append(result, Line{TimeCS: -1, Text: l})
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// BuildLyricMap 解析带时间轴的歌词行。
func BuildLyricMap(lines []string) map[int]string {
	return BuildLyricMapWithOffset(lines, 0)
}

// BuildLyricMapWithOffset 在解析时叠加全局偏移（厘秒）。
func BuildLyricMapWithOffset(lines []string, offset int) map[int]string {
	m := make(map[int]string)
	for _, v := range lines {
		ar := lrcTimeRe.FindStringSubmatch(v)
		if len(ar) <= 4 {
			continue
		}
		mi, _ := strconv.Atoi(ar[1])
		sec, _ := strconv.Atoi(ar[2])
		cs := 0
		if csStr := ar[3]; csStr != "" {
			cs, _ = strconv.Atoi(csStr)
			switch len(csStr) {
			case 1:
				cs *= 10
			case 3:
				cs /= 10
			}
		}
		pos := (60*mi+sec)*100 + cs + offset
		if pos < 0 {
			pos = 0
		}
		text := strings.TrimSpace(ar[4])
		if text != "" {
			m[pos] = text
		}
	}
	return m
}

// FindCurrentLine 返回当前播放位置对应的歌词行下标。
func FindCurrentLine(lyrics []Line, posCS int) int {
	idx := -1
	for i, l := range lyrics {
		if posCS >= l.TimeCS {
			idx = i
		} else {
			break
		}
	}
	return idx
}

// ToUTF8 把 GBK 等编码的内容转换为 UTF-8。
func ToUTF8(data []byte) []byte {
	if utf8.Valid(data) {
		return data
	}
	reader := transform.NewReader(bytes.NewReader(data), simplifiedchinese.GBK.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return data
	}
	return decoded
}

// LoadEmbeddedCover 读取音频文件内嵌封面。
func LoadEmbeddedCover(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return nil
	}

	pic := m.Picture()
	if pic == nil || len(pic.Data) == 0 {
		return nil
	}
	return pic.Data
}

// loadEmbedded 读取音频文件内嵌的歌词标签。
func loadEmbedded(path string) []Line {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return nil
	}

	lrc := m.Lyrics()
	if len(strings.TrimSpace(lrc)) == 0 {
		return nil
	}

	lines := strings.Split(lrc, "\n")
	if len(BuildLyricMap(lines)) == 0 {
		return nil
	}

	return BuildLines(lines, nil)
}

// DeleteLocalLyrics 删除与音频文件同目录的本地歌词（含翻译）。
func DeleteLocalLyrics(file, title string) {
	dir, name, ok := util.SplitPath(file)
	if !ok {
		return
	}

	stems := []string{name}
	if title != "" && title != name {
		stems = append(stems, title)
	}

	for _, stem := range stems {
		for _, ext := range lyricExtensions {
			os.Remove(filepath.Join(dir, stem+ext))
			os.Remove(filepath.Join(dir, stem+".t"+ext))
		}
	}
}

// SaveToLocal 把歌词写到音频文件同目录。
func SaveToLocal(file, title, lrc, tlyric string) error {
	dir, name, ok := util.SplitPath(file)
	if !ok {
		return nil
	}

	if title != "" {
		name = title
	}

	path := filepath.Join(dir, name+".lrc")
	if err := save(path, strings.NewReader(lrc)); err != nil {
		return err
	}

	if len(tlyric) > 0 {
		_ = save(transPathFor(path), strings.NewReader(tlyric))
	}

	return nil
}
