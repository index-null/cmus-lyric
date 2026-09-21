package lyric

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func mockKugou(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	oldSearch, oldKrc, oldDown := kugouSearchURL, kugouKrcURL, kugouDownloadURL
	kugouSearchURL = srv.URL + "/api/v3/search/song"
	kugouKrcURL = srv.URL + "/krc/search"
	kugouDownloadURL = srv.URL + "/download"
	t.Cleanup(func() {
		srv.Close()
		kugouSearchURL, kugouKrcURL, kugouDownloadURL = oldSearch, oldKrc, oldDown
	})
}

func TestKugouSource_DecodesBase64Lyrics(t *testing.T) {
	const lrc = "[ti:东南苦山行]\n[00:24.92]来自中原一群伙伴结庐东南山\n"
	mockKugou(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/search/song":
			json.NewEncoder(w).Encode(kugouSearchResult{
				Status: 1,
				Data: struct {
					Info []kugouSong `json:"info"`
				}{Info: []kugouSong{{
					Hash: "abc", SongName: "东南苦山行", SingerName: "殷正洋",
					AlbumName: "雨中的歉意", Duration: 238,
				}}},
			})
		case "/krc/search":
			json.NewEncoder(w).Encode(kugouKrcResult{
				Status: 200,
				Candidates: []kugouKrcCandidate{{
					ID: "1", AccessKey: "key", Song: "东南苦山行", Singer: "殷正洋", Duration: 238994,
				}},
			})
		case "/download":
			json.NewEncoder(w).Encode(kugouDownloadResult{
				Status:  200,
				Charset: "utf8",
				Content: base64.StdEncoding.EncodeToString([]byte(lrc)),
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cs, err := kugouSource{}.Search(context.Background(), Query{Title: "东南苦山行", Artist: "殷正洋", Duration: 238})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cs) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(cs))
	}
	if cs[0].LRC != lrc {
		t.Errorf("lrc mismatch:\n got=%q\nwant=%q", cs[0].LRC, lrc)
	}
	if !cs[0].Synced {
		t.Error("kugou lrc should be detected as synced")
	}
	if cs[0].Source != "kugou" {
		t.Errorf("source: got %q", cs[0].Source)
	}
	if cs[0].Artist != "殷正洋" || cs[0].Album != "雨中的歉意" {
		t.Errorf("metadata mismatch: %+v", cs[0])
	}
}

func TestKugouSource_NoSearchHit(t *testing.T) {
	mockKugou(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(kugouSearchResult{
			Status: 1,
			Data: struct {
				Info []kugouSong `json:"info"`
			}{Info: []kugouSong{}},
		})
	})

	cs, err := kugouSource{}.Search(context.Background(), Query{Title: "nothing"})
	if err == nil {
		t.Error("expected an error when kugou has no match")
	}
	if len(cs) != 0 {
		t.Errorf("expected no candidates, got %d", len(cs))
	}
}

func TestKugouSource_BadAccessKeyIsReported(t *testing.T) {
	mockKugou(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/search/song":
			json.NewEncoder(w).Encode(kugouSearchResult{
				Status: 1,
				Data: struct {
					Info []kugouSong `json:"info"`
				}{Info: []kugouSong{{Hash: "h", SongName: "s", Duration: 10}}},
			})
		case "/krc/search":
			json.NewEncoder(w).Encode(kugouKrcResult{
				Status:     200,
				Candidates: []kugouKrcCandidate{{ID: "1", AccessKey: "bad"}},
			})
		case "/download":
			json.NewEncoder(w).Encode(kugouDownloadResult{Status: 403, Info: "Bad Accesskey"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cs, err := kugouSource{}.Search(context.Background(), Query{Title: "s"})
	if err == nil {
		t.Fatal("expected an error for a rejected download")
	}
	if len(cs) != 0 {
		t.Errorf("expected no candidates, got %d", len(cs))
	}
}

func TestKugouSource_SkipsPlaceholderLyrics(t *testing.T) {
	mockKugou(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/search/song":
			json.NewEncoder(w).Encode(kugouSearchResult{
				Status: 1,
				Data: struct {
					Info []kugouSong `json:"info"`
				}{Info: []kugouSong{{Hash: "h", SongName: "s", Duration: 10}}},
			})
		case "/krc/search":
			json.NewEncoder(w).Encode(kugouKrcResult{
				Status:     200,
				Candidates: []kugouKrcCandidate{{ID: "1", AccessKey: "k"}},
			})
		case "/download":
			content := base64.StdEncoding.EncodeToString([]byte("[00:05.00]纯音乐，请欣赏"))
			json.NewEncoder(w).Encode(kugouDownloadResult{Status: 200, Charset: "utf8", Content: content})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cs, err := kugouSource{}.Search(context.Background(), Query{Title: "s", Duration: 10})
	if err != nil {
		t.Fatalf("placeholder lyrics should not be an error: %v", err)
	}
	if len(cs) != 0 {
		t.Errorf("placeholder lyrics must be dropped, got %+v", cs)
	}
}
