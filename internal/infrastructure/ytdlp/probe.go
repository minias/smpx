// internal/infrastructure/ytdlp/probe.go
package ytdlp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// 403 우회용 YouTube 클라이언트 순서 (빈 값 = yt-dlp 기본)
//   - 일부 영상은 기본 클라이언트 스트림이 추가 인증(PO 토큰)을 요구해 앞부분 이후 403
//   - android_vr 은 인증 없이 받을 수 있는 경우가 많음 (주로 360p 단일 스트림)
var playerClients = []string{"", "android_vr"}

var errStreamBlocked = errors.New("YouTube가 이 영상의 스트림을 차단했습니다 (403). 잠시 후 다시 시도하거나 다른 영상을 재생하세요")

// probeOffset 스트림 앞부분만 허용되는 경우를 걸러내기 위해 확인할 위치
const probeOffset = 1 << 20

var probeClient = &http.Client{Timeout: 10 * time.Second}

// extract 스트림 주소 추출 + 실제 접근 가능 여부 확인, 막혀 있으면 다른 클라이언트로 재시도
func (c *Client) extract(ctx context.Context, url, format string) ([]string, error) {
	var lastErr error
	blocked := false

	for _, client := range playerClients {
		args := []string{"-f", format, "--no-playlist", "--no-warnings", "-g"}
		if client != "" {
			args = append(args, "--extractor-args", "youtube:player_client="+client)
		}
		args = append(args, "--", url)

		out, err := c.exec(ctx, args...)
		if err != nil {
			if ctx.Err() != nil {
				return nil, err
			}
			lastErr = err
			continue
		}

		lines := nonEmptyLines(out)
		if len(lines) == 0 {
			lastErr = errors.New("yt-dlp: empty stream url")
			continue
		}

		if err := c.probeAll(ctx, lines); err != nil {
			blocked = blocked || errors.Is(err, errStreamBlocked)
			lastErr = err
			continue
		}

		return lines, nil
	}

	if blocked {
		return nil, errStreamBlocked
	}

	return nil, lastErr
}

func (c *Client) probeAll(ctx context.Context, urls []string) error {
	for _, u := range urls {
		if err := c.probe(ctx, u); err != nil {
			return err
		}
	}
	return nil
}

// probeStream 스트림 중간 구간을 요청해 403 여부 확인
//   - 네트워크 오류 등 판단할 수 없는 경우는 통과 (재생 단계에서 처리)
func probeStream(ctx context.Context, rawURL string) error {
	start := int64(probeOffset)
	if u, err := url.Parse(rawURL); err == nil {
		if clen, err := strconv.ParseInt(u.Query().Get("clen"), 10, 64); err == nil && clen > 0 && clen-1 < start {
			start = clen - 1
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, start+1023))

	resp, err := probeClient.Do(req)
	if err != nil {
		return nil
	}
	resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusForbidden, http.StatusNotFound, http.StatusGone:
		return errStreamBlocked
	default:
		return nil
	}
}
