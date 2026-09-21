package lyric

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

var (
	neteaseSearchAPI = "https://music.163.com/api/search/get/web"
	neteaseLyricAPI  = "https://music.163.com/api/song/lyric"
	neteaseDetailAPI = "https://music.163.com/api/song/detail"
)

// neteaseSong 是搜索结果里的一首歌。
type neteaseSong struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Duration int    `json:"duration"`
	Artists  []struct {
		Name string `json:"name"`
	} `json:"artists"`
	Album struct {
		Name   string `json:"name"`
		PicURL string `json:"picUrl"`
	} `json:"album"`
}

type neteaseSongResult struct {
	Result struct {
		Songs []neteaseSong `json:"songs"`
	} `json:"result"`
	Code int `json:"code"`
}

type neteaseLyricResult struct {
	Lrc struct {
		Lyric string `json:"lyric"`
	} `json:"lrc"`
	Tlyric struct {
		Lyric string `json:"lyric"`
	} `json:"tlyric"`
	Code int `json:"code"`
}

// neteaseSource 使用网易云音乐的公开搜索/歌词接口。
type neteaseSource struct{}

func (neteaseSource) Name() string { return "netease" }

func (s neteaseSource) Search(ctx context.Context, q Query) ([]Candidate, error) {
	sr, err := neteaseSearch(ctx, q.Title, q.Artist)
	if err != nil {
		return nil, fmt.Errorf("netease search: %w", err)
	}
	if len(sr.Result.Songs) == 0 {
		return nil, errors.New("not found on Netease")
	}

	indexes := neteaseRankedIndexes(sr, q)
	limit := min(len(indexes), 3)

	var (
		out      []Candidate
		failures int
	)
	for _, idx := range indexes[:limit] {
		song := sr.Result.Songs[idx]
		lrc, trans, err := neteaseGetLyric(ctx, song.ID)
		if err != nil {
			failures++
			continue
		}
		lrc = NormalizeLRC(lrc)
		if lrc == "" || IsPlaceholder(lrc) {
			continue
		}

		var artistNames []string
		for _, a := range song.Artists {
			artistNames = append(artistNames, a.Name)
		}
		artist := strings.Join(artistNames, "/")
		duration := song.Duration / 1000

		out = append(out, Candidate{
			Source:   s.Name(),
			SourceID: strconv.Itoa(song.ID),
			Title:    song.Name,
			Artist:   artist,
			Album:    song.Album.Name,
			Duration: duration,
			LRC:      lrc,
			Trans:    NormalizeLRC(trans),
			Synced:   HasTimestamps(lrc),
			Score:    Score(q, song.Name, artist, song.Album.Name, duration),
		})
	}

	if len(out) == 0 {
		if failures > 0 {
			return nil, errors.New("netease lyric unavailable")
		}
		return nil, nil
	}

	Rank(out)
	return out, nil
}

// neteaseRankedIndexes 按与查询的匹配度给搜索结果排序。
func neteaseRankedIndexes(sr *neteaseSongResult, q Query) []int {
	indexes := make([]int, 0, len(sr.Result.Songs))
	for i := range sr.Result.Songs {
		indexes = append(indexes, i)
	}

	scoreOf := func(i int) float64 {
		song := sr.Result.Songs[i]
		artist := ""
		if len(song.Artists) > 0 {
			artist = song.Artists[0].Name
		}
		return Score(q, song.Name, artist, song.Album.Name, song.Duration/1000)
	}

	sort.SliceStable(indexes, func(a, b int) bool {
		return scoreOf(indexes[a]) > scoreOf(indexes[b])
	})
	return indexes
}

func neteaseSearch(ctx context.Context, name, artist string) (*neteaseSongResult, error) {
	query := name
	if artist != "" {
		query = name + " " + artist
	}

	params := url.Values{}
	params.Set("s", query)
	params.Set("type", "1")
	params.Set("offset", "0")
	params.Set("total", "true")
	params.Set("limit", "10")

	body, err := httpGetCtx(ctx, neteaseSearchAPI+"?"+params.Encode(), "Mozilla/5.0", "https://music.163.com")
	if err != nil {
		return nil, err
	}

	var sr neteaseSongResult
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, errors.New("netease parse error")
	}
	if sr.Code != 200 || len(sr.Result.Songs) == 0 {
		return nil, errors.New("not found on Netease")
	}
	return &sr, nil
}

// neteaseMatchSong 保留给封面查询：优先时长+标题匹配，其次标题匹配。
func neteaseMatchSong(sr *neteaseSongResult, duration int, title string) int {
	if duration > 0 {
		for i, s := range sr.Result.Songs {
			songDuration := s.Duration / 1000
			if songDuration >= duration-2 && songDuration <= duration+2 && titleSimilar(s.Name, title) {
				return i
			}
		}
	}
	for i, s := range sr.Result.Songs {
		if titleSimilar(s.Name, title) {
			return i
		}
	}
	return 0
}

func titleSimilar(a, b string) bool {
	return strings.Contains(a, b) || strings.Contains(b, a)
}

func neteaseGetLyric(ctx context.Context, id int) (string, string, error) {
	params := url.Values{}
	params.Set("id", strconv.Itoa(id))
	params.Set("lv", "-1")
	params.Set("tv", "-1")

	body, err := httpGetCtx(ctx, neteaseLyricAPI+"?"+params.Encode(), "Mozilla/5.0", "https://music.163.com")
	if err != nil {
		return "", "", err
	}

	var lr neteaseLyricResult
	if err := json.Unmarshal(body, &lr); err != nil {
		return "", "", errors.New("netease lyric parse error")
	}
	if lr.Code != 200 {
		return "", "", fmt.Errorf("netease lyric API error: code=%d", lr.Code)
	}

	return lr.Lrc.Lyric, lr.Tlyric.Lyric, nil
}

type neteaseDetailResult struct {
	Songs []struct {
		Album struct {
			PicURL string `json:"picUrl"`
		} `json:"album"`
	} `json:"songs"`
	Code int `json:"code"`
}

// FetchCoverURL 返回最匹配歌曲的封面地址。
func FetchCoverURL(name, artist string, duration int) (string, error) {
	ctx, cancel := withTimeout()
	defer cancel()

	sr, err := neteaseSearch(ctx, name, artist)
	if err != nil {
		return "", err
	}

	idx := neteaseMatchSong(sr, duration, name)
	if picURL := sr.Result.Songs[idx].Album.PicURL; picURL != "" {
		return picURL, nil
	}
	return neteaseGetCoverByDetail(ctx, sr.Result.Songs[idx].ID)
}

func neteaseGetCoverByDetail(ctx context.Context, id int) (string, error) {
	params := url.Values{}
	params.Set("id", strconv.Itoa(id))
	params.Set("ids", fmt.Sprintf("[%d]", id))

	body, err := httpGetCtx(ctx, neteaseDetailAPI+"?"+params.Encode(), "Mozilla/5.0", "https://music.163.com")
	if err != nil {
		return "", fmt.Errorf("netease detail: %w", err)
	}

	var dr neteaseDetailResult
	if err := json.Unmarshal(body, &dr); err != nil {
		return "", errors.New("netease detail parse error")
	}
	if dr.Code != 200 || len(dr.Songs) == 0 || dr.Songs[0].Album.PicURL == "" {
		return "", errors.New("netease detail: no cover")
	}
	return dr.Songs[0].Album.PicURL, nil
}

// FetchCoverData 下载封面图片数据。
func FetchCoverData(coverURL string) ([]byte, error) {
	ctx, cancel := withTimeout()
	defer cancel()
	return httpGetCtx(ctx, coverURL, "Mozilla/5.0", "https://music.163.com")
}
