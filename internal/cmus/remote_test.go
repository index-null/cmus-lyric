package cmus

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestParseStatus_Playing(t *testing.T) {
	output := `status playing
file /home/user/music/Artist - Song.flac
duration 240
position 120
tag artist Pink Floyd
tag title Comfortably Numb
tag album The Wall
`
	track := ParseStatus(output)

	if track.Status != "playing" {
		t.Errorf("expected status 'playing', got %q", track.Status)
	}
	if track.File != "/home/user/music/Artist - Song.flac" {
		t.Errorf("unexpected file: %q", track.File)
	}
	if track.Duration != 240 {
		t.Errorf("expected duration 240, got %d", track.Duration)
	}
	if track.Position != 120 {
		t.Errorf("expected position 120, got %d", track.Position)
	}
	if track.Artist != "Pink Floyd" {
		t.Errorf("expected artist 'Pink Floyd', got %q", track.Artist)
	}
	if track.Title != "Comfortably Numb" {
		t.Errorf("expected title 'Comfortably Numb', got %q", track.Title)
	}
	if track.Album != "The Wall" {
		t.Errorf("expected album 'The Wall', got %q", track.Album)
	}
}

func TestParseStatus_Stopped(t *testing.T) {
	track := ParseStatus("status stopped\n")
	if track.Status != "stopped" {
		t.Errorf("expected status 'stopped', got %q", track.Status)
	}
}

func TestParseStatus_Empty(t *testing.T) {
	track := ParseStatus("")
	if track.Status != "stopped" {
		t.Errorf("expected status 'stopped', got %q", track.Status)
	}
}

func TestParseStatus_Paused(t *testing.T) {
	output := `status paused
file /music/test.mp3
duration 180
position 60
tag artist TestArtist
tag title TestTitle
tag album TestAlbum
`
	track := ParseStatus(output)

	if track.Status != "paused" {
		t.Errorf("expected status 'paused', got %q", track.Status)
	}
	if track.Position != 60 {
		t.Errorf("expected position 60, got %d", track.Position)
	}
}

func TestParseStatus_NoTags(t *testing.T) {
	output := `status playing
file /music/unknown.mp3
duration 300
position 0
`
	track := ParseStatus(output)

	if track.Status != "playing" {
		t.Errorf("expected status 'playing', got %q", track.Status)
	}
	if track.Artist != "" {
		t.Errorf("expected empty artist, got %q", track.Artist)
	}
	if track.Title != "" {
		t.Errorf("expected empty title, got %q", track.Title)
	}
}

func TestParseStatus_MalformedFirstLine(t *testing.T) {
	track := ParseStatus("garbage")
	if track.Status != "stopped" {
		t.Errorf("expected status 'stopped' for malformed input, got %q", track.Status)
	}
}

func TestParseStatus_UnicodeArtist(t *testing.T) {
	output := `status playing
file /music/test.flac
duration 200
position 50
tag artist 周杰伦
tag title 晴天
tag album 叶惠美
`
	track := ParseStatus(output)

	if track.Artist != "周杰伦" {
		t.Errorf("expected artist '周杰伦', got %q", track.Artist)
	}
	if track.Title != "晴天" {
		t.Errorf("expected title '晴天', got %q", track.Title)
	}
}

func TestSocketPath_EnvOverride(t *testing.T) {
	t.Setenv("CMUS_SOCKET", "/tmp/test-cmus-socket")
	p := socketPath()
	if p != "/tmp/test-cmus-socket" {
		t.Errorf("expected '/tmp/test-cmus-socket', got %q", p)
	}
}

func TestSocketPath_XDGRuntime(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "cmus-socket")
	os.WriteFile(sock, []byte{}, 0644)

	t.Setenv("CMUS_SOCKET", "")
	t.Setenv("XDG_RUNTIME_DIR", dir)
	p := socketPath()
	if p != sock {
		t.Errorf("expected %q, got %q", sock, p)
	}
}

func TestSocketPath_DefaultFallback(t *testing.T) {
	t.Setenv("CMUS_SOCKET", "")
	t.Setenv("XDG_RUNTIME_DIR", "")
	p := socketPath()
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "cmus", "socket")
	if p != expected {
		t.Errorf("expected %q, got %q", expected, p)
	}
}

func TestQuerySocket_MockServer(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "cmus-lyric-test.sock")

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("failed to create unix listener: %v", err)
	}
	defer ln.Close()

	mockOutput := "status playing\nfile /music/test.mp3\nduration 240\nposition 120\ntag artist TestArtist\ntag title TestTitle\n"
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 64)
		conn.Read(buf)
		conn.Write([]byte(mockOutput))
		conn.Close()
	}()

	t.Setenv("CMUS_SOCKET", sockPath)
	track := Remote()
	if track.Status != "playing" {
		t.Errorf("expected status 'playing', got %q", track.Status)
	}
	if track.Title != "TestTitle" {
		t.Errorf("expected title 'TestTitle', got %q", track.Title)
	}
	if track.Duration != 240 {
		t.Errorf("expected duration 240, got %d", track.Duration)
	}
}
