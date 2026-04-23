package lyric

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dhowden/tag"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

type Line struct {
	TimeCS int
	Text   string
	Trans  string
}

func Load(path, title string) []Line {
	dotIdx := strings.LastIndex(path, ".")
	slashIdx := strings.LastIndex(path, "/")
	if dotIdx < 0 || slashIdx < 0 {
		return nil
	}

	if lines := loadEmbedded(path); lines != nil {
		return lines
	}

	base := path[:dotIdx]
	dir := path[:slashIdx]

	extensions := []string{".lyric", ".lrc"}

	bases := []string{base}
	if len(title) > 0 {
		titleBase := dir + "/" + title
		if titleBase != base {
			bases = append(bases, titleBase)
		}
	}

	var content []byte
	var tlines []string
	found := false

	for _, b := range bases {
		for _, ext := range extensions {
			lpath := b + ext
			c, e := os.ReadFile(lpath)
			if e != nil {
				continue
			}
			content = c
			found = true

			tlpath := b + ".t" + ext
			tc, te := os.ReadFile(tlpath)
			if te == nil {
				tc = ToUTF8(tc)
				tlines = strings.Split(string(tc), "\n")
			}
			break
		}
		if found {
			break
		}
	}

	if !found {
		return nil
	}

	content = ToUTF8(content)
	lines := strings.Split(string(content), "\n")

	return BuildLines(lines, tlines)
}

func BuildLines(lines, tlines []string) []Line {
	lyricMap := BuildLyricMap(lines)
	tlyricMap := BuildLyricMap(tlines)

	type entry struct {
		timeCS int
		text   string
		trans  string
	}

	entries := make([]entry, 0, len(lyricMap))
	for k, v := range lyricMap {
		t := tlyricMap[k]
		entries = append(entries, entry{k, v, t})
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

var lrcTimeRe = regexp.MustCompile(`^\[([0-9]+):([0-9]+)\.?([0-9]*)]\s*(.*)`)

func BuildLyricMap(lines []string) map[int]string {
	m := make(map[int]string)
	for _, v := range lines {
		ar := lrcTimeRe.FindStringSubmatch(v)
		if len(ar) > 4 {
			mi, _ := strconv.Atoi(ar[1])
			sec, _ := strconv.Atoi(ar[2])
			csStr := ar[3]
			cs := 0
			if csStr != "" {
				cs, _ = strconv.Atoi(csStr)
				switch len(csStr) {
				case 1:
					cs *= 10
				case 3:
					cs /= 10
				}
			}
			pos := (60*mi+sec)*100 + cs
			text := strings.TrimSpace(ar[4])
			if text != "" {
				m[pos] = text
			}
		}
	}
	return m
}

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
	lyricMap := BuildLyricMap(lines)
	if len(lyricMap) == 0 {
		return nil
	}

	return BuildLines(lines, nil)
}
