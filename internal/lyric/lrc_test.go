package lyric

import "testing"

func TestNormalizeLRC_FullWidthBrackets(t *testing.T) {
	// 某些歌词源（尤其是国内站点）会输出全角方括号，导致时间戳无法被解析。
	got := NormalizeLRC("［00:05.00］纯音乐，请欣赏\r\n［00:10.00］第二行")
	want := "[00:05.00]纯音乐，请欣赏\n[00:10.00]第二行"
	if got != want {
		t.Errorf("NormalizeLRC:\n got=%q\nwant=%q", got, want)
	}
}

func TestNormalizeLRC_StripsBOM(t *testing.T) {
	got := NormalizeLRC("\ufeff[00:01.00]hello")
	if got != "[00:01.00]hello" {
		t.Errorf("expected BOM removed, got %q", got)
	}
}

func TestNormalizeLRC_Empty(t *testing.T) {
	if got := NormalizeLRC(""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestIsPlaceholder(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"empty", "", true},
		{"whitespace", "   \n\t\n", true},
		{"纯音乐经典占位", "[00:05.00]纯音乐，请欣赏", true},
		{"纯音乐全角", "［00:05.00］纯音乐，请欣赏", true},
		{"没有填词的纯音乐", "[00:00.00]此歌曲为没有填词的纯音乐，请您欣赏", true},
		{"英文 instrumental", "[00:00.00]instrumental", true},
		{"英文 no lyrics", "[00:00.00]No lyrics", true},
		{"真实歌词", "[00:24.92]来自中原一群伙伴结庐东南山", false},
		{"带元信息的真实歌词", "[00:00.00] 作词 : 陈云山\n[00:08.30]词：陈云山\n[00:24.92]来自中原", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPlaceholder(tt.content); got != tt.want {
				t.Errorf("IsPlaceholder(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

func TestStripTimestamps(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"[00:12.34]Hello World", "Hello World"},
		{"[01:05.67]Hello [02:00.00]World", "Hello World"},
		{"no timestamp", "no timestamp"},
		{"[ti:Title]", "[ti:Title]"},
	}
	for _, tt := range tests {
		if got := StripTimestamps(tt.in); got != tt.want {
			t.Errorf("StripTimestamps(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTimestampLines(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"plain text\nmore text", 0},
		{"[00:01.00]a\n[00:02.00]b", 2},
		{"［00:01.00］a\n[00:02.00]b", 2},
		{"[ti:x]\n[00:01.00]a", 1},
	}
	for _, tt := range tests {
		if got := TimestampLines(tt.in); got != tt.want {
			t.Errorf("TimestampLines(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestHasTimestamps(t *testing.T) {
	if HasTimestamps("plain") {
		t.Error("plain text should not have timestamps")
	}
	if !HasTimestamps("[00:01.00]x") {
		t.Error("lrc should have timestamps")
	}
}

func TestParseOffset(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"[offset:0]\n[00:01.00]a", 0},
		{"[offset:500]\n[00:01.00]a", 50},
		{"[offset:-500]\n[00:01.00]a", -50},
		{"[ti:x]\n[00:01.00]a", 0},
	}
	for _, tt := range tests {
		if got := ParseOffset(tt.in); got != tt.want {
			t.Errorf("ParseOffset(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
