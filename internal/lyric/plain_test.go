package lyric

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func mockOvh(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	old := ovhBaseURL
	ovhBaseURL = srv.URL
	t.Cleanup(func() {
		srv.Close()
		ovhBaseURL = old
	})
}

func TestOvhSource_PlainLyrics(t *testing.T) {
	mockOvh(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Coldplay/Yellow" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(`{"lyrics":"Look at the stars\nlook how they shine for you"}`))
	})

	cs, err := ovhSource{}.Search(context.Background(), Query{Title: "Yellow", Artist: "Coldplay"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cs) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(cs))
	}
	if cs[0].Synced {
		t.Error("lyrics.ovh only provides plain lyrics")
	}
	if cs[0].LRC != "Look at the stars\nlook how they shine for you" {
		t.Errorf("unexpected content: %q", cs[0].LRC)
	}
}

func TestOvhSource_NotFound(t *testing.T) {
	mockOvh(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	cs, err := ovhSource{}.Search(context.Background(), Query{Title: "nope", Artist: "nope"})
	if err == nil {
		t.Error("expected an error for a missing track")
	}
	if len(cs) != 0 {
		t.Errorf("expected no candidates, got %d", len(cs))
	}
}

func TestOvhSource_EmptyLyrics(t *testing.T) {
	mockOvh(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"lyrics":"   "}`))
	})

	src := ovhSource{}
	if cs, _ := src.Search(context.Background(), Query{Title: "a", Artist: "b"}); len(cs) != 0 {
		t.Errorf("expected empty lyrics to be dropped, got %+v", cs)
	}
}
