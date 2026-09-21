package lyric

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	userAgent   = "cmus-lyric v2.1.0 (https://github.com/index-null/cmus-lyric)"
	httpTimeout = 10 * time.Second
)

// withTimeout 返回一个带默认超时的上下文，供没有上下文的入口使用。
func withTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), httpTimeout)
}

// httpGet 发起一次带默认超时的 GET 请求。
func httpGet(reqURL, ua, referer string) ([]byte, error) {
	ctx, cancel := withTimeout()
	defer cancel()
	return httpGetCtx(ctx, reqURL, ua, referer)
}

// httpGetCtx 在给定上下文下发起 GET 请求；上下文没有截止时间时补一个默认超时。
func httpGetCtx(ctx context.Context, reqURL, ua, referer string) ([]byte, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, httpTimeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ua)
	if referer != "" {
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

// Save 把文本写入文件。
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
