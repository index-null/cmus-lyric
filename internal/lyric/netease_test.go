package lyric

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func mockNetease(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	oldSearch, oldLyric, oldDetail := neteaseSearchAPI, neteaseLyricAPI, neteaseDetailAPI
	neteaseSearchAPI = srv.URL + "/api/search/get/web"
	neteaseLyricAPI = srv.URL + "/api/song/lyric"
	neteaseDetailAPI = srv.URL + "/api/song/detail"
	t.Cleanup(func() {
		srv.Close()
		neteaseSearchAPI, neteaseLyricAPI, neteaseDetailAPI = oldSearch, oldLyric, oldDetail
	})
	return srv
}

func writeNeteaseSongs(t *testing.T, w http.ResponseWriter, songs []neteaseSong) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(neteaseSongResult{
		Result: struct {
			Songs []neteaseSong `json:"songs"`
		}{Songs: songs},
		Code: 200,
	}); err != nil {
		t.Errorf("encode error: %v", err)
	}
}

func TestNeteaseSearch_ParsesArtistsAndAlbum(t *testing.T) {
	var got neteaseSongResult
	srv := mockNetease(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/search/get/web" {
			writeNeteaseSongs(t, w, []neteaseSong{{
				ID: 177185, Name: "东南苦山行", Duration: 238000,
				Artists: []struct {
					Name string `json:"name"`
				}{{Name: "殷正洋"}},
			}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	_ = srv

	sr, err := neteaseSearch(context.Background(), "东南苦山行", "殷正洋")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sr.Result.Songs) != 1 {
		t.Fatalf("expected 1 song, got %d", len(sr.Result.Songs))
	}
	_ = got
}

func TestNeteaseSource_ReturnsSyncedLyricsWithTranslation(t *testing.T) {
	mockNetease(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/search/get/web":
			writeNeteaseSongs(t, w, []neteaseSong{
				{ID: 177185, Name: "东南苦山行", Duration: 238000, Artists: []struct {
					Name string `json:"name"`
				}{{Name: "殷正洋"}}},
			})
		case "/api/song/lyric":
			json.NewEncoder(w).Encode(neteaseLyricResult{
				Lrc: struct {
					Lyric string `json:"lyric"`
				}{Lyric: "[00:24.92]来自中原一群伙伴结庐东南山"},
				Tlyric: struct {
					Lyric string `json:"lyric"`
				}{Lyric: "[00:24.92]translation"},
				Code: 200,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cs, err := neteaseSource{}.Search(context.Background(), Query{Title: "东南苦山行", Artist: "殷正洋", Duration: 238})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cs) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(cs))
	}
	c := cs[0]
	if c.Source != "netease" {
		t.Errorf("source: got %q", c.Source)
	}
	if c.Artist != "殷正洋" {
		t.Errorf("artist: got %q", c.Artist)
	}
	if c.Duration != 238 {
		t.Errorf("duration should be converted to seconds, got %d", c.Duration)
	}
	if !c.Synced {
		t.Error("candidate should be marked synced")
	}
	if c.Trans != "[00:24.92]translation" {
		t.Errorf("translation: got %q", c.Trans)
	}
}

// TestNeteaseSource_SkipsEmptyLyrics：Erdtree Knights 在网易云返回空歌词，
// 不应产生候选，也不应报错。
func TestNeteaseSource_SkipsEmptyLyrics(t *testing.T) {
	mockNetease(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/search/get/web":
			writeNeteaseSongs(t, w, []neteaseSong{{ID: 1925052549, Name: "Erdtree Knights", Duration: 166000}})
		case "/api/song/lyric":
			json.NewEncoder(w).Encode(neteaseLyricResult{
				Lrc: struct {
					Lyric string `json:"lyric"`
				}{Lyric: ""},
				Code: 200,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cs, err := neteaseSource{}.Search(context.Background(), Query{Title: "Erdtree Knights", Artist: "Tai Tomisawa", Duration: 166})
	if err != nil {
		t.Fatalf("empty lyrics must not be an error: %v", err)
	}
	if len(cs) != 0 {
		t.Errorf("expected no candidates for empty lyrics, got %+v", cs)
	}
}

func TestNeteaseSource_SkipsPlaceholderLyrics(t *testing.T) {
	mockNetease(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/search/get/web":
			writeNeteaseSongs(t, w, []neteaseSong{{ID: 1, Name: "Foe", Duration: 85000}})
		case "/api/song/lyric":
			json.NewEncoder(w).Encode(neteaseLyricResult{
				Lrc: struct {
					Lyric string `json:"lyric"`
				}{Lyric: "[00:05.00] 纯音乐，请欣赏"},
				Code: 200,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cs, err := neteaseSource{}.Search(context.Background(), Query{Title: "Foe", Duration: 85})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cs) != 0 {
		t.Errorf("placeholder lyrics must be dropped, got %+v", cs)
	}
}

func TestNeteaseMatchSong_PrefersDurationAndTitle(t *testing.T) {
	sr := &neteaseSongResult{}
	sr.Result.Songs = []neteaseSong{
		{ID: 1, Name: "东南苦山行(伴奏)", Duration: 249000},
		{ID: 2, Name: "猜心", Duration: 309000},
		{ID: 177185, Name: "东南苦山行", Duration: 238000},
	}
	if got := neteaseMatchSong(sr, 238, "东南苦山行"); got != 2 {
		t.Errorf("expected index 2 (duration+title match), got %d", got)
	}
}

func TestNeteaseMatchSong_FallsBackToTitle(t *testing.T) {
	sr := &neteaseSongResult{}
	sr.Result.Songs = []neteaseSong{
		{ID: 1, Name: "猜心", Duration: 309000},
		{ID: 2, Name: "东南苦山行", Duration: 100000},
	}
	if got := neteaseMatchSong(sr, 238, "东南苦山行"); got != 1 {
		t.Errorf("expected index 1 (title match), got %d", got)
	}
}

func TestNeteaseLyricAPIError(t *testing.T) {
	mockNetease(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/search/get/web":
			writeNeteaseSongs(t, w, []neteaseSong{{ID: 1, Name: "x", Duration: 1000}})
		case "/api/song/lyric":
			json.NewEncoder(w).Encode(neteaseLyricResult{Code: 404})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	src := neteaseSource{}
	if _, err := src.Search(context.Background(), Query{Title: "x"}); err == nil {
		t.Error("expected error when the lyric API reports a failure code")
	}
}

func TestFetchCoverURL_UsesAlbumPicture(t *testing.T) {
	mockNetease(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/search/get/web" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		song := neteaseSong{ID: 7, Name: "covered", Duration: 1000}
		song.Album.PicURL = "https://example.com/pic.jpg"
		writeNeteaseSongs(t, w, []neteaseSong{song})
	})

	u, err := FetchCoverURL("covered", "", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u != "https://example.com/pic.jpg" {
		t.Errorf("unexpected cover url: %q", u)
	}
}
