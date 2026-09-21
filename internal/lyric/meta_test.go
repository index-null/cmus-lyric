package lyric

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProbe_MissingFile(t *testing.T) {
	if _, err := Probe(filepath.Join(t.TempDir(), "nope.mp3"), 100); err == nil {
		t.Error("expected an error for a missing file")
	}
}

func TestProbe_SizeAndBitrate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "song.mp3")
	// 400000 bytes over 10s == 320 kbps
	if err := os.WriteFile(path, make([]byte, 400000), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := Probe(path, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Size != 400000 {
		t.Errorf("size: got %d", info.Size)
	}
	if info.Bitrate != 320 {
		t.Errorf("bitrate: got %d, want 320", info.Bitrate)
	}
	if info.Duration != 10 {
		t.Errorf("duration: got %d", info.Duration)
	}
	if info.Path != path {
		t.Errorf("path: got %q", info.Path)
	}
}

func TestProbe_UnknownDurationMeansUnknownBitrate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "song.mp3")
	if err := os.WriteFile(path, make([]byte, 1234), 0644); err != nil {
		t.Fatal(err)
	}
	info, err := Probe(path, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Bitrate != 0 {
		t.Errorf("bitrate should be 0 without a duration, got %d", info.Bitrate)
	}
}

// TestProbe_ToleratesUnreadableTags：非音频文件（或没有标签的音频）不应让探测失败，
// 仍然要给出文件大小 / 码率这类可用信息。
func TestProbe_ToleratesUnreadableTags(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "weird.mp3")
	if err := os.WriteFile(path, []byte("not really audio"), 0644); err != nil {
		t.Fatal(err)
	}
	info, err := Probe(path, 5)
	if err != nil {
		t.Fatalf("missing tags must not fail probing: %v", err)
	}
	if info.Size != int64(len("not really audio")) {
		t.Errorf("size: got %d", info.Size)
	}
	if info.FileType != "" {
		t.Errorf("file type should be empty for an unknown file, got %q", info.FileType)
	}
}

func TestBitrateOf(t *testing.T) {
	tests := []struct {
		size     int64
		duration int
		want     int
	}{
		{0, 100, 0},
		{400000, 10, 320},
		{1000, 1, 8},
		{1000000, 0, 0},
	}
	for _, tt := range tests {
		if got := BitrateOf(tt.size, tt.duration); got != tt.want {
			t.Errorf("BitrateOf(%d,%d) = %d, want %d", tt.size, tt.duration, got, tt.want)
		}
	}
}

func TestHumanSize(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{5 * 1024 * 1024, "5.0 MB"},
		{2 * 1024 * 1024 * 1024, "2.0 GB"},
	}
	for _, tt := range tests {
		if got := HumanSize(tt.in); got != tt.want {
			t.Errorf("HumanSize(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAudioInfo_SummaryIncludesBitrate(t *testing.T) {
	info := AudioInfo{FileType: "MP3", Bitrate: 320}
	if got := info.BitrateLabel(); got != "320 kbps" {
		t.Errorf("BitrateLabel = %q", got)
	}
	empty := AudioInfo{}
	if got := empty.BitrateLabel(); got != "-" {
		t.Errorf("BitrateLabel for unknown bitrate = %q, want '-'", got)
	}
}
