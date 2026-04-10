package lyric

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildLyricMap_Standard(t *testing.T) {
	lines := []string{
		"[00:12.34]Hello World",
		"[01:05.67]Second line",
		"[03:00.00]Third line",
	}
	m := BuildLyricMap(lines)

	if len(m) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(m))
	}
	if m[1234] != "Hello World" {
		t.Errorf("expected 'Hello World' at 1234, got %q", m[1234])
	}
	if m[6567] != "Second line" {
		t.Errorf("expected 'Second line' at 6567, got %q", m[6567])
	}
	if m[18000] != "Third line" {
		t.Errorf("expected 'Third line' at 18000, got %q", m[18000])
	}
}

func TestBuildLyricMap_SingleDigitCS(t *testing.T) {
	lines := []string{"[00:05.3]Test"}
	m := BuildLyricMap(lines)

	if m[530] != "Test" {
		t.Errorf("expected 'Test' at 530 (single digit cs*10), got keys: %v", m)
	}
}

func TestBuildLyricMap_ThreeDigitMS(t *testing.T) {
	lines := []string{"[00:10.456]Test"}
	m := BuildLyricMap(lines)

	if m[1045] != "Test" {
		t.Errorf("expected 'Test' at 1045 (3-digit ms/10), got keys: %v", m)
	}
}

func TestBuildLyricMap_NoDecimal(t *testing.T) {
	lines := []string{"[02:30]No decimal"}
	m := BuildLyricMap(lines)

	if m[15000] != "No decimal" {
		t.Errorf("expected 'No decimal' at 15000, got keys: %v", m)
	}
}

func TestBuildLyricMap_EmptyText(t *testing.T) {
	lines := []string{"[00:01.00]", "[00:02.00]  "}
	m := BuildLyricMap(lines)

	if len(m) != 0 {
		t.Errorf("expected 0 entries for empty text lines, got %d", len(m))
	}
}

func TestBuildLyricMap_MetadataLines(t *testing.T) {
	lines := []string{
		"[ti:Song Title]",
		"[ar:Artist Name]",
		"[00:05.00]Actual lyric",
	}
	m := BuildLyricMap(lines)

	if len(m) != 1 {
		t.Errorf("expected 1 entry (metadata should be ignored), got %d", len(m))
	}
}

func TestBuildLyricMap_Nil(t *testing.T) {
	m := BuildLyricMap(nil)
	if len(m) != 0 {
		t.Errorf("expected 0 entries for nil input, got %d", len(m))
	}
}

func TestBuildLines_WithTranslation(t *testing.T) {
	lines := []string{
		"[00:05.00]Hello",
		"[00:10.00]World",
	}
	tlines := []string{
		"[00:05.00]你好",
		"[00:10.00]世界",
	}
	result := BuildLines(lines, tlines)

	if len(result) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(result))
	}
	if result[0].Text != "Hello" || result[0].Trans != "你好" {
		t.Errorf("line 0: text=%q trans=%q", result[0].Text, result[0].Trans)
	}
	if result[1].Text != "World" || result[1].Trans != "世界" {
		t.Errorf("line 1: text=%q trans=%q", result[1].Text, result[1].Trans)
	}
}

func TestBuildLines_SortedByTime(t *testing.T) {
	lines := []string{
		"[00:20.00]Third",
		"[00:05.00]First",
		"[00:10.00]Second",
	}
	result := BuildLines(lines, nil)

	if len(result) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(result))
	}
	if result[0].Text != "First" {
		t.Errorf("expected first line 'First', got %q", result[0].Text)
	}
	if result[1].Text != "Second" {
		t.Errorf("expected second line 'Second', got %q", result[1].Text)
	}
	if result[2].Text != "Third" {
		t.Errorf("expected third line 'Third', got %q", result[2].Text)
	}
}

func TestFindCurrentLine_Basic(t *testing.T) {
	lyrics := []Line{
		{TimeCS: 0, Text: "Intro"},
		{TimeCS: 500, Text: "Line 1"},
		{TimeCS: 1000, Text: "Line 2"},
		{TimeCS: 1500, Text: "Line 3"},
	}

	tests := []struct {
		posCS    int
		expected int
	}{
		{-1, -1},
		{0, 0},
		{250, 0},
		{500, 1},
		{999, 1},
		{1000, 2},
		{1499, 2},
		{1500, 3},
		{9999, 3},
	}

	for _, tt := range tests {
		got := FindCurrentLine(lyrics, tt.posCS)
		if got != tt.expected {
			t.Errorf("FindCurrentLine(posCS=%d): expected %d, got %d", tt.posCS, tt.expected, got)
		}
	}
}

func TestFindCurrentLine_Empty(t *testing.T) {
	got := FindCurrentLine(nil, 100)
	if got != -1 {
		t.Errorf("expected -1 for nil lyrics, got %d", got)
	}
}

func TestToUTF8_ValidUTF8(t *testing.T) {
	input := []byte("Hello 世界")
	result := ToUTF8(input)
	if string(result) != "Hello 世界" {
		t.Errorf("unexpected result: %q", string(result))
	}
}

func TestToUTF8_GBK(t *testing.T) {
	// "你好" in GBK: 0xC4E3 0xBAC3
	gbk := []byte{0xC4, 0xE3, 0xBA, 0xC3}
	result := ToUTF8(gbk)
	if string(result) != "你好" {
		t.Errorf("expected '你好', got %q", string(result))
	}
}

func TestLoad_FromFile(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "song.mp3")
	lrcPath := filepath.Join(dir, "song.lrc")

	if err := os.WriteFile(audioPath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}
	lrcContent := "[00:05.00]Hello\n[00:10.00]World\n"
	if err := os.WriteFile(lrcPath, []byte(lrcContent), 0644); err != nil {
		t.Fatal(err)
	}

	result := Load(audioPath, "")
	if len(result) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(result))
	}
	if result[0].Text != "Hello" {
		t.Errorf("expected 'Hello', got %q", result[0].Text)
	}
}

func TestLoad_WithTranslation(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "song.mp3")
	lrcPath := filepath.Join(dir, "song.lrc")
	tlrcPath := filepath.Join(dir, "song.t.lrc")

	os.WriteFile(audioPath, []byte("fake"), 0644)
	os.WriteFile(lrcPath, []byte("[00:05.00]Hello\n"), 0644)
	os.WriteFile(tlrcPath, []byte("[00:05.00]你好\n"), 0644)

	result := Load(audioPath, "")
	if len(result) != 1 {
		t.Fatalf("expected 1 line, got %d", len(result))
	}
	if result[0].Trans != "你好" {
		t.Errorf("expected translation '你好', got %q", result[0].Trans)
	}
}

func TestLoad_NotFound(t *testing.T) {
	result := Load("/nonexistent/path/song.mp3", "")
	if result != nil {
		t.Errorf("expected nil for missing lyric file, got %v", result)
	}
}

func TestLoad_ByTitle(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "01-track.mp3")
	lrcPath := filepath.Join(dir, "Real Title.lrc")

	os.WriteFile(audioPath, []byte("fake"), 0644)
	os.WriteFile(lrcPath, []byte("[00:01.00]Found by title\n"), 0644)

	result := Load(audioPath, "Real Title")
	if len(result) != 1 {
		t.Fatalf("expected 1 line, got %d", len(result))
	}
	if result[0].Text != "Found by title" {
		t.Errorf("expected 'Found by title', got %q", result[0].Text)
	}
}

func TestLoad_LyricExtension(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "song.flac")
	lyricPath := filepath.Join(dir, "song.lyric")

	os.WriteFile(audioPath, []byte("fake"), 0644)
	os.WriteFile(lyricPath, []byte("[00:01.00]Lyric extension\n"), 0644)

	result := Load(audioPath, "")
	if len(result) != 1 {
		t.Fatalf("expected 1 line, got %d", len(result))
	}
	if result[0].Text != "Lyric extension" {
		t.Errorf("expected 'Lyric extension', got %q", result[0].Text)
	}
}

func TestLoad_InvalidPath(t *testing.T) {
	result := Load("noext", "")
	if result != nil {
		t.Errorf("expected nil for path without dot/slash, got %v", result)
	}
}

func TestLoadEmbedded_NonAudioFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fake.mp3")
	os.WriteFile(path, []byte("not a real mp3"), 0644)

	result := loadEmbedded(path)
	if result != nil {
		t.Errorf("expected nil for non-audio file, got %v", result)
	}
}

func TestLoadEmbedded_MissingFile(t *testing.T) {
	result := loadEmbedded("/nonexistent/file.mp3")
	if result != nil {
		t.Errorf("expected nil for missing file, got %v", result)
	}
}
