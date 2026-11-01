// internal/infrastructure/streamproxy/proxy_test.go
package streamproxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// youtubeLike 10MB 넘는 범위 요청은 403 (YouTube 동작 흉내)
func youtubeLike(t *testing.T, data []byte, maxRange int64) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		start, end, _, err := parseRange(r.Header.Get("Range"))
		if err != nil || end < 0 || end-start+1 > maxRange {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if end >= int64(len(data)) {
			end = int64(len(data)) - 1
		}
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(data[start : end+1])
	}))
	t.Cleanup(srv.Close)

	return srv, &calls
}

func newTestProxy(t *testing.T) (*Proxy, *httptest.Server) {
	t.Helper()
	p := newProxy("")
	front := httptest.NewServer(p)
	t.Cleanup(front.Close)
	p.base = front.URL
	return p, front
}

func get(t *testing.T, url, rangeHeader string) (*http.Response, []byte) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, body
}

func TestFullRangeSplitIntoChunks(t *testing.T) {
	data := bytes.Repeat([]byte("0123456789"), (chunkSize*2+500)/10)
	upstream, calls := youtubeLike(t, data, chunkSize)
	p, _ := newTestProxy(t)

	url := p.Wrap(upstream.URL + "/videoplayback")
	resp, body := get(t, url, fmt.Sprintf("bytes=0-%d", len(data)-1))

	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !bytes.Equal(body, data) {
		t.Fatalf("body mismatch: got %d bytes, want %d", len(body), len(data))
	}
	if want := fmt.Sprintf("bytes 0-%d/%d", len(data)-1, len(data)); resp.Header.Get("Content-Range") != want {
		t.Fatalf("Content-Range = %q", resp.Header.Get("Content-Range"))
	}
	if *calls != 3 {
		t.Fatalf("upstream calls = %d, want 3", *calls)
	}
}

func TestSmallRangeAndOpenEnded(t *testing.T) {
	data := []byte(strings.Repeat("x", 1000))
	upstream, _ := youtubeLike(t, data, chunkSize)
	p, _ := newTestProxy(t)
	url := p.Wrap(upstream.URL)

	resp, body := get(t, url, "bytes=0-1")
	if resp.StatusCode != http.StatusPartialContent || len(body) != 2 || resp.Header.Get("Content-Range") != "bytes 0-1/1000" {
		t.Fatalf("0-1: %d %d %q", resp.StatusCode, len(body), resp.Header.Get("Content-Range"))
	}

	resp, body = get(t, url, "bytes=900-")
	if len(body) != 100 || resp.Header.Get("Content-Range") != "bytes 900-999/1000" {
		t.Fatalf("900-: %d %q", len(body), resp.Header.Get("Content-Range"))
	}

	resp, body = get(t, url, "")
	if resp.StatusCode != http.StatusOK || len(body) != 1000 {
		t.Fatalf("no range: %d %d", resp.StatusCode, len(body))
	}
}

func TestUnknownTokenAndUpstreamError(t *testing.T) {
	p, front := newTestProxy(t)

	if resp, _ := get(t, front.URL+"/s/deadbeef", ""); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown token status = %d", resp.StatusCode)
	}

	forbidden := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer forbidden.Close()

	if resp, _ := get(t, p.Wrap(forbidden.URL), "bytes=0-1"); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("upstream 403 status = %d", resp.StatusCode)
	}
}

func TestWrapEvictsOldEntries(t *testing.T) {
	p := newProxy("http://x")
	first := p.Wrap("https://a")
	for i := 0; i < maxEntries; i++ {
		p.Wrap(fmt.Sprintf("https://b/%d", i))
	}
	token := strings.TrimPrefix(first, "http://x/s/")
	if _, ok := p.lookup(token); ok {
		t.Fatal("oldest entry not evicted")
	}
	if p.Wrap("") != "" {
		t.Fatal("empty url should stay empty")
	}
}
