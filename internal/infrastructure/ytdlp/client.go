// internal/infrastructure/ytdlp/client.go
package ytdlp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/minias/smpx/internal/application/youtube"
	"github.com/minias/smpx/internal/domain"
)

// 스트림 URL 캐시 유효 시간
const streamTTL = 30 * time.Minute

// 에러 메시지에 포함할 stderr 최대 줄 수
const stderrTailLines = 5

// 재생목록에서 가져올 최대 곡 수
const maxPlaylistItems = 500

// 재생목록의 재생 불가 항목
var unavailableTitles = map[string]bool{
	"[Private video]": true,
	"[Deleted video]": true,
}

// WebView(WKWebView / WebKitGTK) 재생용 오디오 형식
// opus(webm) 우선: YouTube DASH m4a 는 WKWebView 에서 길이가 2배로 잡혀 곡 종료가 늦어짐
const audioFormat = "bestaudio[acodec=opus]/bestaudio[ext=m4a]/bestaudio"

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

// Expand URL 을 플레이리스트 항목으로 변환
//   - 재생목록 URL(list=...): 목록 전체 (최대 maxPlaylistItems, 비공개/삭제 영상 제외)
//   - 단일 영상 URL: 한 곡 (제목이 없으면 URL)
func (c *Client) Expand(ctx context.Context, url string) ([]domain.PlaylistItem, error) {
	url, err := validateURL(url)
	if err != nil {
		return nil, err
	}

	out, err := c.exec(ctx,
		"--flat-playlist", "--yes-playlist", "--no-warnings",
		"--playlist-end", strconv.Itoa(maxPlaylistItems),
		"--print", "%(webpage_url,url)s\t%(title)s",
		"--", url)
	if err != nil {
		return nil, err
	}

	var items []domain.PlaylistItem
	for _, line := range nonEmptyLines(out) {
		link, title, _ := strings.Cut(line, "\t")
		title = strings.TrimSpace(title)

		if link == "" || link == "NA" || unavailableTitles[title] {
			continue
		}
		if title == "" || title == "NA" {
			title = link
		}
		items = append(items, domain.PlaylistItem{Title: title, URL: link})
	}

	// 단일 영상은 사용자가 입력한 URL 유지
	if len(items) == 1 && !strings.Contains(url, "list=") {
		items[0].URL = url
	}

	if len(items) == 0 {
		return nil, errors.New("추가할 수 있는 영상이 없습니다")
	}

	return items, nil
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
