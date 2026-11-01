// internal/infrastructure/streamproxy/proxy.go
package streamproxy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// chunkSize YouTube 는 큰 범위 요청을 403 으로 거부 (클라이언트에 따라 4~10MB 이상)
//   - WebView 는 파일 전체 범위(bytes=0-끝)를 요청하므로 나눠서 받아 이어 붙임
const chunkSize = 2 << 20

// maxEntries 등록 주소 보관 개수 (오래된 것부터 삭제)
const maxEntries = 32

// Proxy WebView → 127.0.0.1 → YouTube 스트림 중계
//   - 등록한 주소만 무작위 토큰으로 중계 (임의 주소 프록시 아님)
type Proxy struct {
	base     string
	upstream *http.Client

	mu     sync.Mutex
	urls   map[string]string
	tokens []string
}

// Start 127.0.0.1 임의 포트에서 시작
func Start() (*Proxy, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	p := newProxy("http://" + ln.Addr().String())

	go func() {
		_ = http.Serve(ln, p)
	}()

	return p, nil
}

func newProxy(base string) *Proxy {
	return &Proxy{
		base:     base,
		upstream: &http.Client{},
		urls:     make(map[string]string),
	}
}

// Wrap 원본 주소를 프록시 주소로 변환 (빈 값은 그대로)
func (p *Proxy) Wrap(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return rawURL
	}
	token := hex.EncodeToString(b)

	p.mu.Lock()
	defer p.mu.Unlock()

	p.urls[token] = rawURL
	p.tokens = append(p.tokens, token)

	for len(p.tokens) > maxEntries {
		delete(p.urls, p.tokens[0])
		p.tokens = p.tokens[1:]
	}

	return p.base + "/s/" + token
}

func (p *Proxy) lookup(token string) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	u, ok := p.urls[token]
	return u, ok
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	target, ok := p.lookup(strings.TrimPrefix(r.URL.Path, "/s/"))
	if !ok {
		http.NotFound(w, r)
		return
	}

	start, end, hasRange, err := parseRange(r.Header.Get("Range"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusRequestedRangeNotSatisfiable)
		return
	}

	// 첫 조각으로 전체 크기 / 형식 확인
	first, total, err := p.fetch(r.Context(), target, start, chunkEnd(start, end))
	if err != nil {
		var se statusError
		if errors.As(err, &se) {
			http.Error(w, se.Error(), int(se))
		} else {
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
		return
	}
	defer first.Body.Close()

	if end < 0 || end >= total {
		end = total - 1
	}

	h := w.Header()
	h.Set("Content-Type", first.Header.Get("Content-Type"))
	h.Set("Accept-Ranges", "bytes")
	h.Set("Content-Length", strconv.FormatInt(end-start+1, 10))

	if hasRange {
		h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	if r.Method == http.MethodHead {
		return
	}

	// 첫 조각 전달 후 나머지 조각을 이어서 전달
	pos := start
	n, err := io.Copy(w, first.Body)
	pos += n
	if err != nil {
		return
	}

	for pos <= end {
		resp, _, err := p.fetch(r.Context(), target, pos, chunkEnd(pos, end))
		if err != nil {
			return // 응답 도중이라 상태 코드를 바꿀 수 없음 → 연결 종료
		}
		n, err := io.Copy(w, resp.Body)
		resp.Body.Close()
		pos += n
		if err != nil || n == 0 {
			return
		}
	}
}

type statusError int

func (e statusError) Error() string {
	return "upstream " + strconv.Itoa(int(e))
}

// fetch 원본에서 [start, end] 범위 요청, 전체 크기 반환
func (p *Proxy) fetch(ctx context.Context, target string, start, end int64) (*http.Response, int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	resp, err := p.upstream.Do(req)
	if err != nil {
		return nil, 0, err
	}

	switch resp.StatusCode {
	case http.StatusPartialContent:
		total, err := totalFromContentRange(resp.Header.Get("Content-Range"))
		if err != nil {
			resp.Body.Close()
			return nil, 0, err
		}
		return resp, total, nil

	case http.StatusOK:
		// 범위 미지원 서버: 전체가 옴
		if start != 0 || resp.ContentLength < 0 {
			resp.Body.Close()
			return nil, 0, errors.New("upstream does not support range requests")
		}
		return resp, resp.ContentLength, nil

	default:
		resp.Body.Close()
		return nil, 0, statusError(resp.StatusCode)
	}
}

// chunkEnd 조각 끝 위치 (end < 0 이면 끝 미지정)
func chunkEnd(start, end int64) int64 {
	last := start + chunkSize - 1
	if end >= 0 && end < last {
		return end
	}
	return last
}

// parseRange "bytes=a-b" / "bytes=a-" (없으면 처음부터 끝까지)
func parseRange(h string) (start, end int64, ok bool, err error) {
	if h == "" {
		return 0, -1, false, nil
	}

	spec, found := strings.CutPrefix(h, "bytes=")
	if !found || strings.Contains(spec, ",") {
		return 0, 0, false, errors.New("unsupported range")
	}

	a, b, _ := strings.Cut(spec, "-")
	if a == "" {
		return 0, 0, false, errors.New("suffix range not supported")
	}

	start, err = strconv.ParseInt(a, 10, 64)
	if err != nil || start < 0 {
		return 0, 0, false, errors.New("invalid range")
	}

	end = -1
	if b != "" {
		end, err = strconv.ParseInt(b, 10, 64)
		if err != nil || end < start {
			return 0, 0, false, errors.New("invalid range")
		}
	}

	return start, end, true, nil
}

// totalFromContentRange "bytes 0-1/12345" → 12345
func totalFromContentRange(h string) (int64, error) {
	_, total, found := strings.Cut(h, "/")
	if !found || total == "*" {
		return 0, fmt.Errorf("unknown content length: %q", h)
	}
	return strconv.ParseInt(total, 10, 64)
}
