package lyric

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeSource 用于在不触网的情况下验证并行编排逻辑。
type fakeSource struct {
	name   string
	delay  time.Duration
	result []Candidate
	err    error
}

func (f fakeSource) Name() string { return f.name }

func (f fakeSource) Search(ctx context.Context, _ Query) ([]Candidate, error) {
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return f.result, f.err
}

func withSources(t *testing.T, ss ...Source) {
	t.Helper()
	old := registry
	registry = ss
	t.Cleanup(func() { registry = old })
}

func TestSearchAll_RunsSourcesInParallel(t *testing.T) {
	withSources(t,
		fakeSource{name: "a", delay: 300 * time.Millisecond, result: []Candidate{{Source: "a", Title: "A"}}},
		fakeSource{name: "b", delay: 300 * time.Millisecond, result: []Candidate{{Source: "b", Title: "B"}}},
		fakeSource{name: "c", delay: 300 * time.Millisecond, result: []Candidate{{Source: "c", Title: "C"}}},
	)

	start := time.Now()
	results := SearchAll(context.Background(), Query{Title: "T"})
	elapsed := time.Since(start)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if elapsed > 700*time.Millisecond {
		t.Errorf("sources did not run in parallel: took %v", elapsed)
	}
	for _, r := range results {
		if r.Err != nil {
			t.Errorf("source %s returned error: %v", r.Source, r.Err)
		}
		if len(r.Candidates) != 1 {
			t.Errorf("source %s: expected 1 candidate, got %d", r.Source, len(r.Candidates))
		}
	}
}

func TestSearchAll_IsolatesFailures(t *testing.T) {
	withSources(t,
		fakeSource{name: "broken", err: errors.New("boom")},
		fakeSource{name: "good", result: []Candidate{{Source: "good", Title: "ok"}}},
	)

	results := SearchAll(context.Background(), Query{Title: "T"})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Error("expected the failing source to report an error")
	}
	if results[1].Err != nil || len(results[1].Candidates) != 1 {
		t.Errorf("healthy source should still deliver: %+v", results[1])
	}
}

func TestSearchAll_RespectsContext(t *testing.T) {
	withSources(t, fakeSource{name: "slow", delay: 2 * time.Second})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	results := SearchAll(ctx, Query{Title: "T"})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Error("expected context error for the cancelled source")
	}
	if len(results[0].Candidates) != 0 {
		t.Errorf("expected no candidates, got %d", len(results[0].Candidates))
	}
}

func TestSimilarity(t *testing.T) {
	tests := []struct {
		a, b string
		min  float64
		max  float64
	}{
		{"hello", "hello", 0.99, 1.01},
		{"东南苦山行", "东南苦山行", 0.99, 1.01},
		{"Hello World", "hello world", 0.99, 1.01},
		{"abcdef", "abcdefgh", 0.6, 0.9},
		{"abc", "xyz", 0.0, 0.2},
		{"", "", 0.99, 1.01},
		{"abc", "", 0.0, 0.01},
	}
	for _, tt := range tests {
		got := Similarity(tt.a, tt.b)
		if got < tt.min || got > tt.max {
			t.Errorf("Similarity(%q,%q) = %.3f, want in [%.2f,%.2f]", tt.a, tt.b, got, tt.min, tt.max)
		}
	}
}

func TestScore_ExactMatchBeatsEverything(t *testing.T) {
	q := Query{Title: "东南苦山行", Artist: "殷正洋", Duration: 238}

	exact := Score(q, "东南苦山行", "殷正洋", "", 238)
	karaoke := Score(q, "东南苦山行(伴奏)", "殷正洋", "", 249)
	other := Score(q, "猜心", "殷正洋", "", 309)
	wrongArtist := Score(q, "东南苦山行", "Someone Else", "", 238)

	if !(exact > karaoke) {
		t.Errorf("exact (%.3f) should beat karaoke version (%.3f)", exact, karaoke)
	}
	if !(exact > other) {
		t.Errorf("exact (%.3f) should beat unrelated title (%.3f)", exact, other)
	}
	if !(exact > wrongArtist) {
		t.Errorf("exact (%.3f) should beat wrong artist (%.3f)", exact, wrongArtist)
	}
	if exact < 0.9 {
		t.Errorf("exact match should score close to 1, got %.3f", exact)
	}
}

func TestScore_UnknownArtistIsNeutral(t *testing.T) {
	q := Query{Title: "Yellow", Duration: 200}
	withArtist := Score(Query{Title: "Yellow", Artist: "Coldplay", Duration: 200}, "Yellow", "Coldplay", "", 200)
	noArtist := Score(q, "Yellow", "Coldplay", "", 200)

	if withArtist <= noArtist {
		t.Errorf("confirmed artist match (%.3f) should beat unknown artist (%.3f)", withArtist, noArtist)
	}
}

func TestScore_DurationDriftLowersScore(t *testing.T) {
	q := Query{Title: "Song", Artist: "A", Duration: 200}
	close := Score(q, "Song", "A", "", 202)
	far := Score(q, "Song", "A", "", 260)
	if close <= far {
		t.Errorf("close duration (%.3f) should beat far duration (%.3f)", close, far)
	}
}

func TestRank_PrefersSyncedThenScore(t *testing.T) {
	cs := []Candidate{
		{Source: "a", Title: "Song", Score: 0.95},
		{Source: "b", Title: "Song", Score: 0.60, Synced: true},
		{Source: "c", Title: "Song", Score: 0.90},
	}
	Rank(cs)
	if cs[0].Source != "b" {
		t.Errorf("synced candidate should rank first, got %s", cs[0].Source)
	}
	if cs[1].Source != "a" {
		t.Errorf("second should be the highest plain score, got %s", cs[1].Source)
	}
}

func TestBest_PicksHighestScoredUsableCandidate(t *testing.T) {
	q := Query{Title: "Song", Artist: "A", Duration: 200}
	results := []SourceResult{
		{Source: "a", Candidates: []Candidate{{Source: "a", Title: "Song", Score: 0.5, LRC: "[00:01.00]a"}}},
		{Source: "b", Candidates: []Candidate{{Source: "b", Title: "Song", Score: 0.95, LRC: "[00:01.00]b", Synced: true}}},
	}
	best, ok := Best(results, q)
	if !ok {
		t.Fatal("expected a candidate")
	}
	if best.Source != "b" {
		t.Errorf("expected source b, got %s", best.Source)
	}
}

func TestBest_RejectsPlaceholderAndInstrumental(t *testing.T) {
	q := Query{Title: "Foe", Artist: "Tai Tomisawa", Duration: 85}
	results := []SourceResult{
		{Source: "lrclib", Candidates: []Candidate{{Source: "lrclib", Title: "Foe", Score: 1, Instrumental: true}}},
		{Source: "netease", Candidates: []Candidate{{Source: "netease", Title: "Foe", Score: 0.9, LRC: "[00:05.00]纯音乐，请欣赏"}}},
	}
	if _, ok := Best(results, q); ok {
		t.Error("placeholder / instrumental candidates must not be selected")
	}
}

func TestBest_NoResults(t *testing.T) {
	if _, ok := Best(nil, Query{Title: "x"}); ok {
		t.Error("expected no candidate for empty results")
	}
}

func TestCandidateLines(t *testing.T) {
	c := Candidate{
		LRC:    "[00:05.00]Hello\n[00:10.00]World\n",
		Trans:  "[00:05.00]你好\n",
		Synced: true,
	}
	lines := c.Lines()
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0].Text != "Hello" || lines[0].Trans != "你好" {
		t.Errorf("unexpected first line: %+v", lines[0])
	}
}

func TestCandidateLines_Unsynced(t *testing.T) {
	c := Candidate{LRC: "line one\nline two\n"}
	lines := c.Lines()
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	for _, l := range lines {
		if l.TimeCS != -1 {
			t.Errorf("unsynced lines must use TimeCS=-1, got %d", l.TimeCS)
		}
	}
}

func TestRegistry_DefaultSources(t *testing.T) {
	names := map[string]bool{}
	for _, s := range Sources() {
		names[s.Name()] = true
	}
	for _, want := range []string{"lrclib", "netease", "kugou", "lyrics.ovh"} {
		if !names[want] {
			t.Errorf("expected source %q to be registered, got %v", want, names)
		}
	}
}
