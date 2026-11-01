// internal/infrastructure/ytdlp/client.go
package ytdlp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/minias/smpx/internal/application/youtube"
	"github.com/minias/smpx/internal/domain"
)

// 스트림 URL 캐시 유효 시간
const streamTTL = 30 * time.Minute

// 에러 메시지에 포함할 stderr 최대 줄 수
const stderrTailLines = 5

// WebView(WKWebView / WebKitGTK) 재생용 오디오 형식
// opus(webm) 우선: YouTube DASH m4a 는 WKWebView 에서 길이가 2배로 잡혀 곡 종료가 늦어짐
const audioFormat = "bestaudio[acodec=opus]/bestaudio[ext=m4a]/bestaudio"

// 화면 보기용: H.264 영상(최대 1080p) + opus 오디오, 없으면 소리 포함 단일 스트림
const videoFormat = "bv*[vcodec^=avc1][height<=1080]+ba[acodec=opus]/bv*[vcodec^=avc1][height<=1080]+ba/b[vcodec^=avc1]/b"

var _ youtube.Resolver = (*Client)(nil)

// runFunc 외부 프로세스 실행 함수 (테스트 주입용)
type runFunc func(ctx context.Context, bin string, args ...string) (stdout, stderr []byte, err error)

// Client yt-dlp 기반 Resolver 구현
type Client struct {
	resolve func(ctx context.Context) (string, error)
	run     runFunc

	// 실패 시 yt-dlp 업데이트 후 1회 재시도 (YouTube 변경 대응)
	update func(ctx context.Context) bool

	streams *memoCache
	titles  *memoCache
	flight  flightGroup
}

// NewClient 생성 (yt-dlp 경로는 Manager 가 관리)
func NewClient(m *Manager) *Client {
	c := newClient(m.Resolve, execRun, time.Now)
	c.update = m.ForceUpdate
	return c
}

func newClient(resolve func(context.Context) (string, error), run runFunc, now func() time.Time) *Client {
	return &Client{
		resolve: resolve,
		run:     run,
		update:  func(context.Context) bool { return false },
		streams: newMemoCache(streamTTL, now),
		titles:  newMemoCache(0, now),
	}
}

// execRun 실제 프로세스 실행 (셸 미사용)
func execRun(ctx context.Context, bin string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// validateURL 빈 값 / 옵션 주입 방지
func validateURL(url string) (string, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return "", errors.New("yt-dlp: empty url")
	}
	if strings.HasPrefix(url, "-") {
		return "", fmt.Errorf("yt-dlp: invalid url %q", url)
	}
	return url, nil
}

// StreamURL 오디오 스트림 URL 추출 (30분 캐시, 동일 URL 중복 실행 방지)
func (c *Client) StreamURL(ctx context.Context, url string) (string, error) {
	url, err := validateURL(url)
	if err != nil {
		return "", err
	}
	if v, ok := c.streams.get(url); ok {
		return v, nil
	}

	return c.flight.do(ctx, "stream\x00"+url, func() (string, error) {
		// 선행 호출이 채웠을 수 있으므로 재확인
		if v, ok := c.streams.get(url); ok {
			return v, nil
		}
		out, err := c.exec(ctx, "-f", audioFormat, "--no-playlist", "--no-warnings", "-g", "--", url)
		if err != nil {
			return "", err
		}
		line := firstLine(out)
		if line == "" {
			return "", errors.New("yt-dlp: empty stream url")
		}
		c.streams.set(url, line)
		return line, nil
	})
}

// Streams 재생 스트림 (video=false 면 오디오만)
func (c *Client) Streams(ctx context.Context, url string, video bool) (domain.Stream, error) {
	if !video {
		audio, err := c.StreamURL(ctx, url)
		return domain.Stream{AudioURL: audio}, err
	}

	url, err := validateURL(url)
	if err != nil {
		return domain.Stream{}, err
	}

	key := "video\x00" + url
	joined, ok := c.streams.get(key)
	if !ok {
		joined, err = c.flight.do(ctx, key, func() (string, error) {
			if v, ok := c.streams.get(key); ok {
				return v, nil
			}
			out, err := c.exec(ctx, "-f", videoFormat, "--no-playlist", "--no-warnings", "-g", "--", url)
			if err != nil {
				return "", err
			}
			lines := nonEmptyLines(out)
			if len(lines) == 0 {
				return "", errors.New("yt-dlp: empty stream url")
			}
			v := strings.Join(lines, "\n")
			c.streams.set(key, v)
			return v, nil
		})
		if err != nil {
			return domain.Stream{}, err
		}
	}

	// -g 출력 순서: 영상, 오디오 (단일 스트림이면 한 줄)
	lines := strings.Split(joined, "\n")
	st := domain.Stream{VideoURL: lines[0]}
	if len(lines) > 1 {
		st.AudioURL = lines[1]
	}
	return st, nil
}

// Title 영상 제목 조회 (비어 있으면 url 반환, 캐시 유지)
func (c *Client) Title(ctx context.Context, url string) (string, error) {
	url, err := validateURL(url)
	if err != nil {
		return "", err
	}
	if v, ok := c.titles.get(url); ok {
		return v, nil
	}

	return c.flight.do(ctx, "title\x00"+url, func() (string, error) {
		if v, ok := c.titles.get(url); ok {
			return v, nil
		}
		out, err := c.exec(ctx, "--no-playlist", "--no-warnings", "--print", "title", "--", url)
		if err != nil {
			return "", err
		}
		title := firstLine(out)
		if title == "" {
			title = url
		}
		c.titles.set(url, title)
		return title, nil
	})
}

// exec yt-dlp 실행 후 stdout 반환 (실패 시 업데이트 후 1회 재시도)
func (c *Client) exec(ctx context.Context, args ...string) ([]byte, error) {
	out, err := c.execOnce(ctx, args...)
	if err == nil || ctx.Err() != nil || !c.update(ctx) {
		return out, err
	}
	return c.execOnce(ctx, args...)
}

func (c *Client) execOnce(ctx context.Context, args ...string) ([]byte, error) {
	bin, err := c.resolve(ctx)
	if err != nil {
		return nil, fmt.Errorf("yt-dlp not found: %w", err)
	}
	stdout, stderr, err := c.run(ctx, bin, args...)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("yt-dlp: %w", ctxErr)
		}
		return nil, formatError(err, stderr)
	}
	return stdout, nil
}

// formatError 에러 + stderr 마지막 몇 줄
func formatError(err error, stderr []byte) error {
	tail := tailLines(string(stderr), stderrTailLines)
	if tail == "" {
		return fmt.Errorf("yt-dlp failed: %w", err)
	}
	return fmt.Errorf("yt-dlp failed: %w: %s", err, tail)
}

// tailLines 비어 있지 않은 마지막 n 줄
func tailLines(s string, n int) string {
	var lines []string
	for _, l := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " | ")
}

// firstLine 비어 있지 않은 첫 줄
func firstLine(b []byte) string {
	if lines := nonEmptyLines(b); len(lines) > 0 {
		return lines[0]
	}
	return ""
}

func nonEmptyLines(b []byte) []string {
	var lines []string
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}
