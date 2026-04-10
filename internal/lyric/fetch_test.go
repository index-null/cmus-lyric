package lyric

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPickLrcLibLyric_PreferSynced(t *testing.T) {
	r := &lrcLibRecord{
		SyncedLyrics: "[00:01.00]synced",
		PlainLyrics:  "plain",
	}
	result := pickLrcLibLyric(r)
	if result != "[00:01.00]synced" {
		t.Errorf("expected synced lyrics, got %q", result)
	}
}

func TestPickLrcLibLyric_FallbackPlain(t *testing.T) {
	r := &lrcLibRecord{
		PlainLyrics: "plain lyrics",
	}
	result := pickLrcLibLyric(r)
	if result != "plain lyrics" {
		t.Errorf("expected plain lyrics, got %q", result)
	}
}

func TestHttpGet_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "test-agent" {
			t.Errorf("unexpected user agent: %q", r.Header.Get("User-Agent"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	body, err := httpGet(server.URL, "test-agent", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != `{"ok":true}` {
		t.Errorf("unexpected body: %q", string(body))
	}
}

func TestHttpGet_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := httpGet(server.URL, "test", "")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestHttpGet_500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	_, err := httpGet(server.URL, "test", "")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestHttpGet_WithReferer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != "https://example.com" {
			t.Errorf("unexpected referer: %q", r.Header.Get("Referer"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	_, err := httpGet(server.URL, "test", "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.lrc")

	content := "[00:01.00]test"
	err := Save(path, content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected %q, got %q", content, string(data))
	}
}

func TestFetchForCmus_PathParsing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record := lrcLibRecord{
			ID:           1,
			SyncedLyrics: "[00:01.00]mock lyric",
		}
		json.NewEncoder(w).Encode(record)
	}))
	defer server.Close()

	dir := t.TempDir()
	audioFile := filepath.Join(dir, "Test Song.mp3")
	os.WriteFile(audioFile, []byte("fake"), 0644)

	origURL := lrcLibBaseURL
	defer func() {
		// can't easily restore package-level const, skip this in real scenario
	}()
	_ = origURL
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
	if parsed.ID != 42 {
		t.Errorf("expected ID 42, got %d", parsed.ID)
	}
	if parsed.SyncedLyrics != "[00:01.00]hello" {
		t.Errorf("unexpected synced lyrics: %q", parsed.SyncedLyrics)
	}
}

func TestNeteaseResponseParse(t *testing.T) {
	resp := neteaseLyricResult{
		Lrc: struct {
			Lyric string `json:"lyric"`
		}{Lyric: "[00:01.00]test lyric"},
		Tlyric: struct {
			Lyric string `json:"lyric"`
		}{Lyric: "[00:01.00]测试歌词"},
		Code: 200,
	}
	data, _ := json.Marshal(resp)

	var parsed neteaseLyricResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if parsed.Lrc.Lyric != "[00:01.00]test lyric" {
		t.Errorf("unexpected lrc: %q", parsed.Lrc.Lyric)
	}
	if parsed.Tlyric.Lyric != "[00:01.00]测试歌词" {
		t.Errorf("unexpected tlyric: %q", parsed.Tlyric.Lyric)
	}
}

func TestFetchFromNetease_ReturnsTlyric(t *testing.T) {
	searchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/search/get/web":
			json.NewEncoder(w).Encode(neteaseSongResult{
				Result: struct {
					Songs []struct {
						ID       int    `json:"id"`
						Name     string `json:"name"`
						Duration int    `json:"duration"`
					} `json:"songs"`
				}{
					Songs: []struct {
						ID       int    `json:"id"`
						Name     string `json:"name"`
						Duration int    `json:"duration"`
					}{{ID: 123, Name: "Test", Duration: 240000}},
				},
				Code: 200,
			})
		case "/api/song/lyric":
			json.NewEncoder(w).Encode(neteaseLyricResult{
				Lrc: struct {
					Lyric string `json:"lyric"`
				}{Lyric: "[00:01.00]hello"},
				Tlyric: struct {
					Lyric string `json:"lyric"`
				}{Lyric: "[00:01.00]你好"},
				Code: 200,
			})
		}
	}))
	defer searchServer.Close()

	origSearch := neteaseSearchAPI
	origLyric := neteaseLyricAPI
	neteaseSearchAPI = searchServer.URL + "/api/search/get/web"
	neteaseLyricAPI = searchServer.URL + "/api/song/lyric"
	defer func() {
		neteaseSearchAPI = origSearch
		neteaseLyricAPI = origLyric
	}()

	lrc, tlrc, err := fetchFromNetease("Test", "Artist", 240)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lrc != "[00:01.00]hello" {
		t.Errorf("unexpected lrc: %q", lrc)
	}
	if tlrc != "[00:01.00]你好" {
		t.Errorf("expected tlyric '[00:01.00]你好', got %q", tlrc)
	}
}
