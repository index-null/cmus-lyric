package lyric

import (
	"regexp"
	"strings"
)

var (
	// 某些歌词源会输出全角方括号，导致时间戳完全无法被解析。
	fullWidthBrackets = strings.NewReplacer("［", "[", "］", "]", "【", "[", "】", "]")
	// 时间戳里的全角冒号（如 00：05.00）。
	fullWidthColonRe = regexp.MustCompile(`([0-9])：([0-9])`)
	// 任意位置的时间戳标记，用于把文本剥离出来。
	anyTimeRe = regexp.MustCompile(`\[[0-9]{1,3}:[0-9]{1,2}(?:\.[0-9]{1,3})?]`)
	offsetRe  = regexp.MustCompile(`(?i)\[offset:\s*([+-]?[0-9]+)\s*]`)
	// 歌词里常见的「制作信息行」，只剩这些行时说明并没有真正的歌词。
	creditLineRe = regexp.MustCompile(`^(作词|作曲|编曲|制作人|词|曲|录音|混音|母带|监制|出品|和声|吉他|贝斯|鼓)\s*[:：]`)
)

// placeholderHints 命中即视为「没有歌词」的占位内容。
var placeholderHints = []string{
	"纯音乐",
	"此歌曲为没有填词的纯音乐",
	"暂无歌词",
	"歌词暂未提供",
	"该歌曲暂无歌词",
	"instrumental",
	"no lyrics",
	"there are no lyrics",
	"lyrics not available",
}

// NormalizeLRC 统一换行、去掉 BOM，并把全角方括号/冒号修正为半角，
// 让下游的正则解析能稳定工作。
func NormalizeLRC(content string) string {
	if content == "" {
		return ""
	}
	s := strings.TrimPrefix(content, "\ufeff")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = fullWidthBrackets.Replace(s)
	return fullWidthColonRe.ReplaceAllString(s, "${1}:${2}")
}

// StripTimestamps 去掉一行里的所有时间标记，只留下文本部分。
func StripTimestamps(line string) string {
	return strings.TrimSpace(anyTimeRe.ReplaceAllString(line, ""))
}

// TimestampLines 返回带时间标记的行数。
func TimestampLines(content string) int {
	n := 0
	for _, line := range splitLinesStr(NormalizeLRC(content)) {
		if lrcTimeRe.MatchString(strings.TrimSpace(line)) {
			n++
		}
	}
	return n
}

// HasTimestamps 判断内容是否是带时间轴的 LRC。
func HasTimestamps(content string) bool {
	return TimestampLines(content) > 0
}

// IsPlaceholder 判断歌词内容是否只是「纯音乐，请欣赏」之类的占位文本。
// 这类内容曾经被当成真歌词展示，是 bad-case 里最刺眼的问题之一。
func IsPlaceholder(content string) bool {
	body := NormalizeLRC(content)

	var kept []string
	for _, line := range splitLinesStr(body) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && !anyTimeRe.MatchString(line) {
			// [ti:xxx] 这类头部元信息不参与判断
			continue
		}
		text := strings.TrimSpace(StripTimestamps(line))
		if text == "" {
			continue
		}
		kept = append(kept, text)
	}

	if len(kept) == 0 {
		return true
	}

	meaningful := 0
	for _, text := range kept {
		if creditLineRe.MatchString(text) {
			continue
		}
		meaningful++
	}
	if meaningful == 0 {
		return true
	}

	joined := strings.ToLower(strings.Join(kept, "\n"))
	for _, hint := range placeholderHints {
		if strings.Contains(joined, hint) {
			return true
		}
	}
	return false
}

// ParseOffset 读取 [offset:±ms] 声明，返回厘秒（正偏移表示整体延后）。
func ParseOffset(content string) int {
	return parseOffsetLines(splitLinesStr(NormalizeLRC(content)))
}

func parseOffsetLines(lines []string) int {
	for _, line := range lines {
		m := offsetRe.FindStringSubmatch(strings.TrimSpace(line))
		if len(m) != 2 {
			continue
		}
		ms := atoiOrZero(m[1])
		if ms == 0 {
			return 0
		}
		return ms / 10
	}
	return 0
}

func atoiOrZero(s string) int {
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	} else if strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	if neg {
		return -n
	}
	return n
}
