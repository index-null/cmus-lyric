package lyric

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

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

	if _, err := httpGet(server.URL, "test", ""); err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestHttpGet_500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	if _, err := httpGet(server.URL, "test", ""); err == nil {
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

	if _, err := httpGet(server.URL, "test", "https://example.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.lrc")

	content := "[00:01.00]test"
	if err := Save(path, content); err != nil {
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
