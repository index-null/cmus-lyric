package lyric

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
)

var (
	kugouSearchURL   = "http://mobilecdn.kugou.com/api/v3/search/song"
	kugouKrcURL      = "https://krcs.kugou.com/search"
	kugouDownloadURL = "https://lyrics.kugou.com/download"
)

type kugouSong struct {
	Hash       string `json:"hash"`
	SongName   string `json:"songname"`
	SingerName string `json:"singername"`
	AlbumName  string `json:"album_name"`
	Duration   int    `json:"duration"`
	FileName   string `json:"filename"`
}

type kugouSearchResult struct {
	Status  int `json:"status"`
	ErrCode int `json:"errcode"`
	Data    struct {
		Info []kugouSong `json:"info"`
	} `json:"data"`
}

type kugouKrcCandidate struct {
	ID        string `json:"id"`
	AccessKey string `json:"accesskey"`
	Song      string `json:"song"`
	Singer    string `json:"singer"`
	Duration  int    `json:"duration"`
}

type kugouKrcResult struct {
	Status     int                 `json:"status"`
	Candidates []kugouKrcCandidate `json:"candidates"`
}

type kugouDownloadResult struct {
	Status    int    `json:"status"`
	Charset   string `json:"charset"`
	Content   string `json:"content"`
	Info      string `json:"info"`
	ErrorCode int    `json:"error_code"`
}

// kugouSource 使用酷狗的歌词接口（中文曲目覆盖很好）。
type kugouSource struct{}

func (kugouSource) Name() string { return "kugou" }

func (s kugouSource) Search(ctx context.Context, q Query) ([]Candidate, error) {
	keyword := q.Title
	if q.Artist != "" {
		keyword = q.Title + " " + q.Artist
	}

	params := url.Values{}
	params.Set("format", "json")
	params.Set("keyword", keyword)
	params.Set("page", "1")
	params.Set("pagesize", "8")
	params.Set("showtype", "1")

	body, err := httpGetCtx(ctx, kugouSearchURL+"?"+params.Encode(), userAgent, "")
	if err != nil {
		return nil, err
	}

	var sr kugouSearchResult
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, errors.New("kugou parse error")
	}
	if len(sr.Data.Info) == 0 {
		return nil, errors.New("not found on Kugou")
	}

	songs := sr.Data.Info
	sort.SliceStable(songs, func(i, j int) bool {
		return Score(q, songs[i].SongName, songs[i].SingerName, songs[i].AlbumName, songs[i].Duration) >
			Score(q, songs[j].SongName, songs[j].SingerName, songs[j].AlbumName, songs[j].Duration)
	})

	limit := min(len(songs), 2)

	var (
		out     []Candidate
		lastErr error
	)
	for _, song := range songs[:limit] {
		candidate, err := s.fetchLyric(ctx, q, song)
		if err != nil {
			lastErr = err
			continue
		}
		if candidate == nil {
			continue
		}
		out = append(out, *candidate)
	}

	if len(out) == 0 {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, nil
	}

	Rank(out)
	return out, nil
}

func (s kugouSource) fetchLyric(ctx context.Context, q Query, song kugouSong) (*Candidate, error) {
	params := url.Values{}
	params.Set("ver", "1")
	params.Set("man", "yes")
	params.Set("client", "pc")
	params.Set("keyword", song.SongName)
	params.Set("duration", strconv.Itoa(song.Duration*1000))
	params.Set("hash", song.Hash)

	body, err := httpGetCtx(ctx, kugouKrcURL+"?"+params.Encode(), userAgent, "")
	if err != nil {
		return nil, err
	}

	var kr kugouKrcResult
	if uerr := json.Unmarshal(body, &kr); uerr != nil {
		return nil, errors.New("kugou krc parse error")
	}
	if len(kr.Candidates) == 0 {
		return nil, nil
	}
	best := kr.Candidates[0]

	lrc, err := kugouDownload(ctx, best.ID, best.AccessKey)
	if err != nil {
		return nil, err
	}

	lrc = NormalizeLRC(lrc)
	if lrc == "" || IsPlaceholder(lrc) {
		return nil, nil
	}

	artist := song.SingerName
	if artist == "" {
		artist = best.Singer
	}

	return &Candidate{
		Source:   s.Name(),
		SourceID: best.ID,
		Title:    song.SongName,
		Artist:   artist,
		Album:    song.AlbumName,
		Duration: song.Duration,
		LRC:      lrc,
		Synced:   HasTimestamps(lrc),
		Score:    Score(q, song.SongName, artist, song.AlbumName, song.Duration),
	}, nil
}

func kugouDownload(ctx context.Context, id, accessKey string) (string, error) {
	params := url.Values{}
	params.Set("ver", "1")
	params.Set("client", "pc")
	params.Set("fmt", "lrc")
	params.Set("charset", "utf8")
	params.Set("id", id)
	params.Set("accesskey", accessKey)

	body, err := httpGetCtx(ctx, kugouDownloadURL+"?"+params.Encode(), userAgent, "")
	if err != nil {
		return "", err
	}

	var dr kugouDownloadResult
	if uerr := json.Unmarshal(body, &dr); uerr != nil {
		return "", errors.New("kugou download parse error")
	}
	if dr.Status != 200 || dr.Content == "" {
		return "", fmt.Errorf("kugou download failed: %s", dr.Info)
	}

	raw, err := base64.StdEncoding.DecodeString(dr.Content)
	if err != nil {
		return "", errors.New("kugou download: bad base64")
	}
	return string(ToUTF8(raw)), nil
}
