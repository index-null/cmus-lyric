package lyric

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

var lrcLibBaseURL = "https://lrclib.net/api"

type lrcLibRecord struct {
	ID           int     `json:"id"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

// lrclibSource 使用 LRCLIB（lrcget 使用的开放歌词库）。
type lrclibSource struct{}

func (lrclibSource) Name() string { return "lrclib" }

func (s lrclibSource) Search(ctx context.Context, q Query) ([]Candidate, error) {
	var out []Candidate
	// lrclib 的 /get 与 /search 经常返回同一首歌的多条记录，按「曲名+艺人+时长」去重。
	seen := make(map[string]bool)
	// 纯音乐条目只保留一条，否则选择列表会被「无歌词」刷屏。
	instrumentalSeen := false

	add := func(r *lrcLibRecord) {
		if r == nil {
			return
		}
		duration := int(r.Duration)
		key := strings.ToLower(r.TrackName) + "\x00" + strings.ToLower(r.ArtistName) + "\x00" + strconv.Itoa(duration)
		if seen[key] {
			return
		}
		seen[key] = true

		c := Candidate{
			Source:   s.Name(),
			SourceID: strconv.Itoa(r.ID),
			Title:    r.TrackName,
			Artist:   r.ArtistName,
			Album:    r.AlbumName,
			Duration: duration,
			Score:    Score(q, r.TrackName, r.ArtistName, r.AlbumName, duration),
		}

		content := NormalizeLRC(pickLrcLibLyric(r))
		// instrumental=true 或「纯音乐，请欣赏」这类占位内容都不能当成歌词。
		if r.Instrumental || content == "" || IsPlaceholder(content) {
			if instrumentalSeen {
				return
			}
			instrumentalSeen = true
			c.Instrumental = true
		} else {
			c.LRC = content
			c.Synced = HasTimestamps(content)
		}
		out = append(out, c)
	}

	if q.Artist != "" && q.Duration > 0 {
		if r, err := lrclibGet(ctx, q.Title, q.Artist, q.Duration); err == nil {
			add(r)
		}
	}

	if records, err := lrclibSearch(ctx, q.Title, q.Artist); err == nil {
		for i := range records {
			add(&records[i])
		}
	}

	if len(out) == 0 {
		keyword := q.Title
		if q.Artist != "" {
			keyword = q.Title + " " + q.Artist
		}
		if records, err := lrclibSearchQ(ctx, keyword); err == nil {
			for i := range records {
				add(&records[i])
			}
		}
	}

	if len(out) == 0 {
		return nil, errors.New("not found on LRCLIB")
	}

	Rank(out)
	if len(out) > maxCandidates {
		out = out[:maxCandidates]
	}
	return out, nil
}

func pickLrcLibLyric(r *lrcLibRecord) string {
	if len(r.SyncedLyrics) > 0 {
		return r.SyncedLyrics
	}
	return r.PlainLyrics
}

func lrclibGet(ctx context.Context, name, artist string, duration int) (*lrcLibRecord, error) {
	params := url.Values{}
	params.Set("track_name", name)
	params.Set("artist_name", artist)
	params.Set("duration", strconv.Itoa(duration))

	body, err := httpGetCtx(ctx, lrcLibBaseURL+"/get?"+params.Encode(), userAgent, "")
	if err != nil {
		return nil, err
	}

	record := &lrcLibRecord{}
	if err := json.Unmarshal(body, record); err != nil {
		return nil, errors.New("lrclib parse error")
	}
	if record.ID == 0 {
		return nil, errors.New("lrclib: track not found")
	}
	return record, nil
}

func lrclibSearch(ctx context.Context, name, artist string) ([]lrcLibRecord, error) {
	params := url.Values{}
	if artist != "" {
		params.Set("track_name", name)
		params.Set("artist_name", artist)
	} else {
		params.Set("q", name)
	}

	body, err := httpGetCtx(ctx, lrcLibBaseURL+"/search?"+params.Encode(), userAgent, "")
	if err != nil {
		return nil, err
	}

	var records []lrcLibRecord
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, errors.New("lrclib parse error")
	}
	return records, nil
}

func lrclibSearchQ(ctx context.Context, keyword string) ([]lrcLibRecord, error) {
	params := url.Values{}
	params.Set("q", keyword)

	body, err := httpGetCtx(ctx, lrcLibBaseURL+"/search?"+params.Encode(), userAgent, "")
	if err != nil {
		return nil, err
	}

	var records []lrcLibRecord
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, errors.New("lrclib parse error")
	}
	return records, nil
}
