// internal/infrastructure/ytdlp/client.go
package ytdlp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/minias/smpx/internal/application/youtube"
	runtime2 "github.com/minias/smpx/internal/infrastructure/runtime"
)

// 스트림 URL 캐시 유효 시간
const streamTTL = 30 * time.Minute

// 에러 메시지에 포함할 stderr 최대 줄 수
const stderrTailLines = 5

var _ youtube.Resolver = (*Client)(nil)

// runFunc 외부 프로세스 실행 함수 (테스트 주입용)
type runFunc func(ctx context.Context, bin string, args ...string) (stdout, stderr []byte, err error)

// Client yt-dlp 기반 Resolver 구현
type Client struct {
	binMu   sync.Mutex
	bin     string
	resolve func(name string) (string, error)
	run     runFunc

	streams *memoCache
	titles  *memoCache
	flight  flightGroup
}

// NewClient 생성 (바이너리는 최초 사용 시 탐색)
func NewClient() *Client {
	return newClient(runtime2.ResolveBinary, execRun, time.Now)
}

func newClient(resolve func(string) (string, error), run runFunc, now func() time.Time) *Client {
	return &Client{
		resolve: resolve,
		run:     run,
		streams: newMemoCache(streamTTL, now),
		titles:  newMemoCache(0, now),
	}
}

// execRun 실제 프로세스 실행 (셸 미사용, 콘솔 창 숨김)
func execRun(ctx context.Context, bin string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	runtime2.HideConsole(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// binary yt-dlp 경로 지연 탐색 (성공 시에만 캐시)
func (c *Client) binary() (string, error) {
	c.binMu.Lock()
	defer c.binMu.Unlock()

	if c.bin != "" {
		return c.bin, nil
	}
	p, err := c.resolve("yt-dlp")
	if err != nil {
		return "", fmt.Errorf("yt-dlp not found: %w", err)
	}
	c.bin = p
	return p, nil
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
		out, err := c.exec(ctx, "-f", "bestaudio/best", "--no-playlist", "--no-warnings", "-g", "--", url)
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

// exec yt-dlp 실행 후 stdout 반환 (실패 시 stderr 꼬리 포함)
func (c *Client) exec(ctx context.Context, args ...string) ([]byte, error) {
	bin, err := c.binary()
	if err != nil {
		return nil, err
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
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}
