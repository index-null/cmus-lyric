package lyric

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	lrcLibBaseURL    = "https://lrclib.net/api"
	neteaseSearchAPI = "https://music.163.com/api/search/get/web"
	neteaseLyricAPI  = "https://music.163.com/api/song/lyric"
	userAgent        = "cmus-lyric v2.0.0 (https://github.com/index-null/cmus-lyric)"
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
	content, err := fetchFromLrcLib(name, artist, duration)
	if err != nil {
		content, err = fetchFromNetease(name, artist, duration)
		if err != nil {
			return err
		}
	}

	if len(content) == 0 {
		return fmt.Errorf("lyric content is empty")
	}

	path := dir + "/" + name + ".lrc"
	return save(path, strings.NewReader(content))
}

func FetchForCmus(file string, dt int, artist, title string) error {
	pathIdx := strings.LastIndexAny(file, ".")
	titleIdx := strings.LastIndexAny(file, "/")
	dir := file[:titleIdx]

	if len(title) == 0 {
		title = file[titleIdx+1 : pathIdx]
	}

	return Fetch(dir, title, artist, dt)
}

// --- LRCLIB ---

func fetchFromLrcLib(name, artist string, duration int) (string, error) {
	if len(artist) > 0 && duration > 0 {
		record, err := lrclibGet(name, artist, duration)
		if err == nil {
			return pickLrcLibLyric(record), nil
		}
	}

	record, err := lrclibSearch(name, artist)
	if err != nil {
		return "", err
	}
	return pickLrcLibLyric(record), nil
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
		return nil, fmt.Errorf("parse error: %v", err)
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
		return nil, fmt.Errorf("parse error: %v", err)
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

// --- Netease ---

func fetchFromNetease(name, artist string, duration int) (string, error) {
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
		return "", fmt.Errorf("netease search error: %v", err)
	}

	var sr neteaseSongResult
	if err := json.Unmarshal(body, &sr); err != nil {
		return "", fmt.Errorf("netease parse error: %v", err)
	}
	if sr.Code != 200 || len(sr.Result.Songs) == 0 {
		return "", fmt.Errorf("not found on Netease")
	}

	songID := 0
	for _, s := range sr.Result.Songs {
		if duration > 0 && s.Duration/1000 == duration {
			songID = s.ID
			break
		}
	}
	if songID == 0 {
		songID = sr.Result.Songs[0].ID
	}

	return neteaseGetLyric(songID)
}

func neteaseGetLyric(id int) (string, error) {
	params := url.Values{}
	params.Set("id", strconv.Itoa(id))
	params.Set("lv", "-1")
	params.Set("tv", "-1")

	body, err := httpGet(neteaseLyricAPI+"?"+params.Encode(), "Mozilla/5.0", "https://music.163.com")
	if err != nil {
		return "", fmt.Errorf("netease lyric error: %v", err)
	}

	var lr neteaseLyricResult
	if err := json.Unmarshal(body, &lr); err != nil {
		return "", fmt.Errorf("netease lyric parse error: %v", err)
	}
	if lr.Code != 200 {
		return "", fmt.Errorf("netease lyric API error: code=%d", lr.Code)
	}

	return lr.Lrc.Lyric, nil
}

// --- HTTP / IO ---

func httpGet(reqURL, ua, referer string) ([]byte, error) {
	req, err := http.NewRequest("GET", reqURL, nil)
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

func save(path string, src io.Reader) error {
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("write error: %v", err)
	}
	defer out.Close()

	if _, err = io.Copy(out, src); err != nil {
		return fmt.Errorf("write error: %v", err)
	}
	return nil
}
