package lyric

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	lrcLibBaseURL = "https://lrclib.net/api"
	userAgent     = "cmus-lyric v2.0.0 (https://github.com/index-null/cmus-lyric)"
	httpTimeout   = 10 * time.Second
)

var (
	neteaseSearchAPI = "https://music.163.com/api/search/get/web"
	neteaseLyricAPI  = "https://music.163.com/api/song/lyric"
	neteaseDetailAPI = "https://music.163.com/api/song/detail"
)

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

type neteaseSongResult struct {
	Result struct {
		Songs []struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			Duration int    `json:"duration"`
			Album    struct {
				PicURL string `json:"picUrl"`
			} `json:"album"`
		} `json:"songs"`
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

func Fetch(dir, name, artist string, duration int) error {
	content, tlyric, err := fetchFromLrcLib(name, artist, duration)
	if err != nil {
		content, tlyric, err = fetchFromNetease(name, artist, duration)
		if err != nil {
			return err
		}
	}

	if len(content) == 0 {
		return fmt.Errorf("lyric content is empty")
	}

	path := dir + "/" + name + ".lrc"
	if err := save(path, strings.NewReader(content)); err != nil {
		return err
	}

	if len(tlyric) > 0 {
		tpath := dir + "/" + name + ".t.lrc"
		_ = save(tpath, strings.NewReader(tlyric))
	}

	return nil
}

func FetchForCmus(file string, dt int, artist, title string) error {
	_, _, err := FetchContent(file, dt, artist, title)
	return err
}

func FetchContent(file string, dt int, artist, title string) (string, string, error) {
	pathIdx := strings.LastIndexAny(file, ".")
	titleIdx := strings.LastIndexAny(file, "/")
	dir := file[:titleIdx]

	name := title
	if len(name) == 0 {
		name = file[titleIdx+1 : pathIdx]
	}

	content, tlyric, err := fetchFromLrcLib(name, artist, dt)
	if err != nil {
		content, tlyric, err = fetchFromNetease(name, artist, dt)
		if err != nil {
			return "", "", err
		}
	}

	if len(content) == 0 {
		return "", "", fmt.Errorf("lyric content is empty")
	}

	path := dir + "/" + name + ".lrc"
	if saveErr := save(path, strings.NewReader(content)); saveErr != nil {
		_ = SaveToCache(artist, name, content, tlyric)
	} else if len(tlyric) > 0 {
		tpath := dir + "/" + name + ".t.lrc"
		_ = save(tpath, strings.NewReader(tlyric))
	}

	return content, tlyric, nil
}

func fetchFromLrcLib(name, artist string, duration int) (string, string, error) {
	if len(artist) > 0 && duration > 0 {
		record, err := lrclibGet(name, artist, duration)
		if err == nil {
			return pickLrcLibLyric(record), "", nil
		}
	}

	record, err := lrclibSearch(name, artist)
	if err == nil {
		return pickLrcLibLyric(record), "", nil
	}

	q := name
	if len(artist) > 0 {
		q = name + " " + artist
	}
	record, err = lrclibSearchQ(q)
	if err != nil {
		return "", "", err
	}
	return pickLrcLibLyric(record), "", nil
}

func pickLrcLibLyric(r *lrcLibRecord) string {
	if len(r.SyncedLyrics) > 0 {
		return r.SyncedLyrics
	}
	return r.PlainLyrics
}

func lrclibGet(name, artist string, duration int) (*lrcLibRecord, error) {
	params := url.Values{}
	params.Set("track_name", name)
	params.Set("artist_name", artist)
	params.Set("duration", strconv.Itoa(duration))

	body, err := httpGet(lrcLibBaseURL+"/get?"+params.Encode(), userAgent, "")
	if err != nil {
		return nil, err
	}

	record := &lrcLibRecord{}
	if err := json.Unmarshal(body, record); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	if record.ID == 0 {
		return nil, fmt.Errorf("track not found")
	}
	return record, nil
}

func lrclibSearch(name, artist string) (*lrcLibRecord, error) {
	params := url.Values{}
	if len(artist) > 0 {
		params.Set("track_name", name)
		params.Set("artist_name", artist)
	} else {
		params.Set("q", name)
	}

	body, err := httpGet(lrcLibBaseURL+"/search?"+params.Encode(), userAgent, "")
	if err != nil {
		return nil, err
	}

	var results []lrcLibRecord
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("not found on LRCLIB")
	}

	for i := range results {
		if len(results[i].SyncedLyrics) > 0 {
			return &results[i], nil
		}
	}
	return &results[0], nil
}

func lrclibSearchQ(q string) (*lrcLibRecord, error) {
	params := url.Values{}
	params.Set("q", q)

	body, err := httpGet(lrcLibBaseURL+"/search?"+params.Encode(), userAgent, "")
	if err != nil {
		return nil, err
	}

	var results []lrcLibRecord
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("not found on LRCLIB")
	}

	for i := range results {
		if len(results[i].SyncedLyrics) > 0 {
			return &results[i], nil
		}
	}
	return &results[0], nil
}

func fetchFromNetease(name, artist string, duration int) (string, string, error) {
	sr, err := neteaseSearch(name, artist)
	if err != nil {
		return "", "", err
	}
	return neteaseGetLyric(sr.Result.Songs[neteaseMatchSong(sr, duration)].ID)
}

func neteaseSearch(name, artist string) (*neteaseSongResult, error) {
	query := name
	if len(artist) > 0 {
		query = name + " " + artist
	}

	params := url.Values{}
	params.Set("s", query)
	params.Set("type", "1")
	params.Set("offset", "0")
	params.Set("total", "true")
	params.Set("limit", "10")

	body, err := httpGet(neteaseSearchAPI+"?"+params.Encode(), "Mozilla/5.0", "https://music.163.com")
	if err != nil {
		return nil, fmt.Errorf("netease search: %w", err)
	}

	var sr neteaseSongResult
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, fmt.Errorf("netease parse: %w", err)
	}
	if sr.Code != 200 || len(sr.Result.Songs) == 0 {
		return nil, fmt.Errorf("not found on Netease")
	}
	return &sr, nil
}

func neteaseMatchSong(sr *neteaseSongResult, duration int) int {
	for i, s := range sr.Result.Songs {
		if duration > 0 && s.Duration/1000 == duration {
			return i
		}
	}
	return 0
}

func neteaseGetLyric(id int) (string, string, error) {
	params := url.Values{}
	params.Set("id", strconv.Itoa(id))
	params.Set("lv", "-1")
	params.Set("tv", "-1")

	body, err := httpGet(neteaseLyricAPI+"?"+params.Encode(), "Mozilla/5.0", "https://music.163.com")
	if err != nil {
		return "", "", fmt.Errorf("netease lyric: %w", err)
	}

	var lr neteaseLyricResult
	if err := json.Unmarshal(body, &lr); err != nil {
		return "", "", fmt.Errorf("netease lyric parse: %w", err)
	}
	if lr.Code != 200 {
		return "", "", fmt.Errorf("netease lyric API error: code=%d", lr.Code)
	}

	return lr.Lrc.Lyric, lr.Tlyric.Lyric, nil
}

func httpGet(reqURL, ua, referer string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ua)
	if len(referer) > 0 {
		req.Header.Set("Referer", referer)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("not found (404)")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

type neteaseDetailResult struct {
	Songs []struct {
		Album struct {
			PicURL string `json:"picUrl"`
		} `json:"album"`
	} `json:"songs"`
	Code int `json:"code"`
}

func FetchCoverURL(name, artist string, duration int) (string, error) {
	sr, err := neteaseSearch(name, artist)
	if err != nil {
		return "", err
	}

	idx := neteaseMatchSong(sr, duration)
	if picURL := sr.Result.Songs[idx].Album.PicURL; picURL != "" {
		return picURL, nil
	}
	return neteaseGetCoverByDetail(sr.Result.Songs[idx].ID)
}

func neteaseGetCoverByDetail(id int) (string, error) {
	params := url.Values{}
	params.Set("id", strconv.Itoa(id))
	params.Set("ids", fmt.Sprintf("[%d]", id))

	body, err := httpGet(neteaseDetailAPI+"?"+params.Encode(), "Mozilla/5.0", "https://music.163.com")
	if err != nil {
		return "", fmt.Errorf("netease detail: %w", err)
	}

	var dr neteaseDetailResult
	if err := json.Unmarshal(body, &dr); err != nil {
		return "", fmt.Errorf("netease detail parse: %w", err)
	}
	if dr.Code != 200 || len(dr.Songs) == 0 || dr.Songs[0].Album.PicURL == "" {
		return "", fmt.Errorf("netease detail: no cover")
	}
	return dr.Songs[0].Album.PicURL, nil
}

func FetchCoverData(coverURL string) ([]byte, error) {
	return httpGet(coverURL, "Mozilla/5.0", "https://music.163.com")
}

func Save(path, content string) error {
	return save(path, strings.NewReader(content))
}

func save(path string, src io.Reader) error {
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("write error: %w", err)
	}
	defer out.Close()

	if _, err = io.Copy(out, src); err != nil {
		return fmt.Errorf("write error: %w", err)
	}
	return nil
}
