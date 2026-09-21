package lyric

import (
	"context"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// maxCandidates 限制每个来源返回的候选数量，避免选择列表过长。
const maxCandidates = 5

// Query 描述一次歌词检索所需的歌曲信息。
type Query struct {
	Title    string
	Artist   string
	Album    string
	Duration int // 秒
}

// Candidate 是某个来源给出的一份歌词候选。
type Candidate struct {
	Source       string
	SourceID     string
	Title        string
	Artist       string
	Album        string
	Duration     int
	LRC          string
	Trans        string
	Synced       bool
	Instrumental bool
	Score        float64
}

// Lines 把候选歌词解析成可渲染的行；无时间轴时按纯文本处理。
func (c Candidate) Lines() []Line {
	if c.LRC == "" {
		return nil
	}
	normalized := NormalizeLRC(c.LRC)
	if HasTimestamps(normalized) {
		var trans []string
		if c.Trans != "" {
			trans = splitLinesStr(NormalizeLRC(c.Trans))
		}
		if lines := BuildLines(splitLinesStr(normalized), trans); len(lines) > 0 {
			return lines
		}
	}
	return BuildUnsyncedLines(normalized)
}

// Source 是一个歌词来源。
type Source interface {
	Name() string
	Search(ctx context.Context, q Query) ([]Candidate, error)
}

// registry 保存全部可用来源，顺序即并发检索的启动顺序。
var registry = []Source{
	lrclibSource{},
	neteaseSource{},
	kugouSource{},
	ovhSource{},
}

// Sources 返回已注册的来源。
func Sources() []Source { return registry }

// Register 追加一个来源（供扩展使用）。
func Register(s Source) { registry = append(registry, s) }

// SourceResult 是单个来源的检索结果。
type SourceResult struct {
	Source     string
	Candidates []Candidate
	Err        error
}

// SearchAll 并行检索所有来源，返回与 Sources() 顺序一致的结果。
// 单个来源失败不会影响其他来源。
func SearchAll(ctx context.Context, q Query) []SourceResult {
	srcs := Sources()
	results := make([]SourceResult, len(srcs))

	var wg sync.WaitGroup
	wg.Add(len(srcs))
	for i, s := range srcs {
		go func() {
			defer wg.Done()
			candidates, err := s.Search(ctx, q)
			results[i] = SourceResult{Source: s.Name(), Candidates: candidates, Err: err}
		}()
	}
	wg.Wait()

	return results
}

// Best 在所有结果里挑出可用且得分最高的候选。
func Best(results []SourceResult, q Query) (Candidate, bool) {
	var all []Candidate
	for _, r := range results {
		all = append(all, r.Candidates...)
	}
	Rank(all)

	for _, c := range all {
		if c.Instrumental || c.LRC == "" || IsPlaceholder(c.LRC) {
			continue
		}
		return c, true
	}
	return Candidate{}, false
}

// FetchBest 并行检索并返回最佳候选，用于切歌时的自动匹配。
func FetchBest(ctx context.Context, q Query) (Candidate, bool) {
	return Best(SearchAll(ctx, q), q)
}

// Rank 原地排序：带时间轴的优先，其次按得分。
func Rank(candidates []Candidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Synced != candidates[j].Synced {
			return candidates[i].Synced
		}
		return candidates[i].Score > candidates[j].Score
	})
}

// penaltyWords 出现在候选标题里、但查询里没有时，说明是伴奏/翻唱之类的版本。
var penaltyWords = []string{
	"伴奏", "纯音乐", "instrumental", "off vocal", "offvocal",
	"remix", "cover", "karaoke", "inst.", " instrumental",
}

func hasPenaltyWord(s string) bool {
	lower := strings.ToLower(s)
	for _, w := range penaltyWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

// Score 给一份候选打分（0~1）：标题权重最高，其次是艺人和时长。
func Score(q Query, title, artist, album string, duration int) float64 {
	titleScore := Similarity(q.Title, title)

	artistScore := 0.5
	if q.Artist != "" && artist != "" {
		artistScore = Similarity(q.Artist, artist)
	}

	durationScore := 0.5
	if q.Duration > 0 && duration > 0 {
		diff := q.Duration - duration
		if diff < 0 {
			diff = -diff
		}
		if diff > 30 {
			diff = 30
		}
		durationScore = 1 - float64(diff)/30
	}

	total := 0.6*titleScore + 0.25*artistScore + 0.15*durationScore
	if hasPenaltyWord(title) && !hasPenaltyWord(q.Title) {
		total *= 0.5
	}
	return total
}

// Similarity 返回两个字符串归一化后的相似度（0~1）。
func Similarity(a, b string) float64 {
	ra := []rune(normalizeForCompare(a))
	rb := []rune(normalizeForCompare(b))

	if len(ra) == 0 && len(rb) == 0 {
		return 1
	}
	if len(ra) == 0 || len(rb) == 0 {
		return 0
	}

	best := 1 - float64(levenshtein(ra, rb))/float64(max(len(ra), len(rb)))

	// 完全包含的情况给一点加成（"Song" vs "Song (Live)"）
	sa, sb := string(ra), string(rb)
	if strings.Contains(sa, sb) || strings.Contains(sb, sa) {
		containment := float64(min(len(ra), len(rb))) / float64(max(len(ra), len(rb)))
		if boosted := containment * 0.95; boosted > best {
			best = boosted
		}
	}
	return best
}

func normalizeForCompare(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func levenshtein(a, b []rune) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		copy(prev, curr)
	}
	return prev[len(b)]
}
