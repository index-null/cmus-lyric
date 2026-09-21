package lyric

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
)

var ovhBaseURL = "https://api.lyrics.ovh/v1"

// ovhSource 使用 lyrics.ovh，作为没有时间轴时的兜底纯文本来源。
type ovhSource struct{}

func (ovhSource) Name() string { return "lyrics.ovh" }

func (s ovhSource) Search(ctx context.Context, q Query) ([]Candidate, error) {
	if q.Title == "" || q.Artist == "" {
		return nil, errors.New("lyrics.ovh needs both artist and title")
	}

	reqURL := ovhBaseURL + "/" + url.PathEscape(q.Artist) + "/" + url.PathEscape(q.Title)
	body, err := httpGetCtx(ctx, reqURL, userAgent, "")
	if err != nil {
		return nil, err
	}

	var resp struct {
		Lyrics string `json:"lyrics"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, errors.New("lyrics.ovh parse error")
	}

	content := NormalizeLRC(resp.Lyrics)
	if content == "" || IsPlaceholder(content) {
		return nil, nil
	}

	return []Candidate{{
		Source:   s.Name(),
		Title:    q.Title,
		Artist:   q.Artist,
		Duration: q.Duration,
		LRC:      content,
		Synced:   HasTimestamps(content),
		Score:    Score(q, q.Title, q.Artist, "", 0),
	}}, nil
}
