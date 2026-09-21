package lyric

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func mockLrclib(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	old := lrcLibBaseURL
	lrcLibBaseURL = ""
	srv := httptest.NewServer(handler)
	lrcLibBaseURL = srv.URL
	t.Cleanup(func() {
		srv.Close()
		lrcLibBaseURL = old
	})
}

func TestPickLrcLibLyric_PreferSynced(t *testing.T) {
	r := &lrcLibRecord{
		SyncedLyrics: "[00:01.00]synced",
		PlainLyrics:  "plain",
	}
	if got := pickLrcLibLyric(r); got != "[00:01.00]synced" {
		t.Errorf("expected synced lyrics, got %q", got)
	}
}

func TestPickLrcLibLyric_FallbackPlain(t *testing.T) {
	r := &lrcLibRecord{PlainLyrics: "plain lyrics"}
	if got := pickLrcLibLyric(r); got != "plain lyrics" {
		t.Errorf("expected plain lyrics, got %q", got)
	}
}

func TestLrclibGet_ParseResponse(t *testing.T) {
	record := lrcLibRecord{
		ID:           42,
		TrackName:    "Test",
		ArtistName:   "Artist",
		SyncedLyrics: "[00:01.00]hello",
	}
	data, _ := json.Marshal(record)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(data)
	}))
	defer server.Close()

	body, err := httpGet(server.URL, userAgent, "")
	if err != nil {
		t.Fatalf("httpGet error: %v", err)
	}
	var parsed lrcLibRecord
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if parsed.ID != 42 || parsed.SyncedLyrics != "[00:01.00]hello" {
		t.Errorf("unexpected record: %+v", parsed)
	}
}

func TestLrclibSearch_ReturnsScoredCandidates(t *testing.T) {
	mockLrclib(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode([]lrcLibRecord{
			{
				ID: 1, TrackName: "东南苦山行", ArtistName: "殷正洋", Duration: 238,
				SyncedLyrics: "[00:24.92]来自中原一群伙伴结庐东南山",
			},
			{
				ID: 2, TrackName: "东南苦山行(伴奏)", ArtistName: "殷正洋", Duration: 249,
				SyncedLyrics: "[00:01.00]x",
			},
		})
	})

	cs, err := lrclibSource{}.Search(context.Background(), Query{Title: "东南苦山行", Artist: "殷正洋", Duration: 238})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cs) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(cs))
	}
	for _, c := range cs {
		if c.Source != "lrclib" {
			t.Errorf("source should be tagged lrclib, got %q", c.Source)
		}
		if c.LRC == "" {
			t.Errorf("candidate %s should carry lyrics", c.Title)
		}
	}
	if cs[0].Score < cs[1].Score {
		t.Errorf("exact match should score higher: %+v vs %+v", cs[0], cs[1])
	}
	if !cs[0].Synced {
		t.Error("candidate with synced lyrics should be marked Synced")
	}
}

// TestLrclibSearch_Instrumental 是 Elden Ring OST 这类纯音乐的回归：
// lrclib 显式返回 instrumental=true 且歌词为 null，必须被标记而不是当成有歌词。
func TestLrclibSearch_Instrumental(t *testing.T) {
	mockLrclib(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]lrcLibRecord{
			{
				ID: 36551973, TrackName: "Erdtree Knights", ArtistName: "Tai Tomisawa",
				Duration: 167, Instrumental: true,
			},
		})
	})

	cs, err := lrclibSource{}.Search(context.Background(), Query{Title: "Erdtree Knights", Artist: "Tai Tomisawa", Duration: 166})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cs) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(cs))
	}
	if !cs[0].Instrumental {
		t.Error("instrumental record must be flagged")
	}
	if cs[0].LRC != "" {
		t.Errorf("instrumental record must not carry lyrics, got %q", cs[0].LRC)
	}
}

// TestLrclibSearch_PlaceholderRejected 覆盖「纯音乐，请欣赏」这类占位歌词。
func TestLrclibSearch_PlaceholderRejected(t *testing.T) {
	mockLrclib(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]lrcLibRecord{
			{
				ID: 9, TrackName: "Formidable Foe II", ArtistName: "Tai Tomisawa", Duration: 85,
				SyncedLyrics: "［00:05.00］纯音乐，请欣赏",
			},
		})
	})

	cs, err := lrclibSource{}.Search(context.Background(), Query{Title: "Formidable Foe II", Artist: "Tai Tomisawa", Duration: 85})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cs) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(cs))
	}
	if !cs[0].Instrumental {
		t.Errorf("placeholder lyrics must be flagged as instrument/no-lyric: %+v", cs[0])
	}
}

func TestLrclibSearch_EmptyResults(t *testing.T) {
	mockLrclib(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/get" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode([]lrcLibRecord{})
	})

	cs, err := lrclibSource{}.Search(context.Background(), Query{Title: "东南苦山行", Artist: "殷正洋", Duration: 238})
	if err == nil {
		t.Fatal("expected an error when lrclib has nothing")
	}
	if len(cs) != 0 {
		t.Errorf("expected no candidates, got %d", len(cs))
	}
}

func TestLrclibGet_NotFoundIsAnError(t *testing.T) {
	mockLrclib(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	if _, err := lrclibGet(context.Background(), "x", "y", 1); err == nil {
		t.Error("expected error for 404")
	}
}
