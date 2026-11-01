// internal/infrastructure/ytdlp/client_test.go
package ytdlp

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock 테스트용 시계
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (f *fakeClock) now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

func (f *fakeClock) advance(d time.Duration) {
	f.mu.Lock()
	f.t = f.t.Add(d)
	f.mu.Unlock()
}

func okResolve(context.Context) (string, error) { return "/bin/yt-dlp", nil }

func newTestClient(run runFunc) (*Client, *fakeClock) {
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	return newClient(okResolve, run, clk.now), clk
}

func TestStreamURLCacheHitAndTTL(t *testing.T) {
	var calls int32
	var gotArgs []string
	c, clk := newTestClient(func(_ context.Context, bin string, args ...string) ([]byte, []byte, error) {
		atomic.AddInt32(&calls, 1)
		gotArgs = args
		return []byte("\nhttps://audio.example/1\nhttps://audio.example/2\n"), nil, nil
	})
	ctx := context.Background()
	url := "https://www.youtube.com/watch?v=x"

	for i := 0; i < 3; i++ {
		got, err := c.StreamURL(ctx, url)
		if err != nil || got != "https://audio.example/1" {
			t.Fatalf("got %q, %v", got, err)
		}
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	want := []string{"-f", audioFormat, "--no-playlist", "--no-warnings", "-g", "--", url}
	if strings.Join(gotArgs, " ") != strings.Join(want, " ") {
		t.Fatalf("args = %v", gotArgs)
	}

	clk.advance(29 * time.Minute)
	_, _ = c.StreamURL(ctx, url)
	if calls != 1 {
		t.Fatalf("before TTL calls = %d", calls)
	}
	clk.advance(2 * time.Minute)
	_, _ = c.StreamURL(ctx, url)
	if calls != 2 {
		t.Fatalf("after TTL calls = %d, want 2", calls)
	}
}

func TestStreamURLDedup(t *testing.T) {
	var calls int32
	release := make(chan struct{})
	c, _ := newTestClient(func(context.Context, string, ...string) ([]byte, []byte, error) {
		atomic.AddInt32(&calls, 1)
		<-release
		return []byte("https://audio.example/x\n"), nil, nil
	})

	const n = 20
	var wg sync.WaitGroup
	results := make([]string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], _ = c.StreamURL(context.Background(), "https://y/v")
		}(i)
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	for i, r := range results {
		if r != "https://audio.example/x" {
			t.Fatalf("result[%d] = %q", i, r)
		}
	}
}

func TestErrorIncludesStderrTail(t *testing.T) {
	c, _ := newTestClient(func(context.Context, string, ...string) ([]byte, []byte, error) {
		stderr := "line1\nline2\nline3\nline4\nline5\nERROR: [youtube] x: Video unavailable\r\n\n"
		return nil, []byte(stderr), errors.New("exit status 1")
	})
	_, err := c.StreamURL(context.Background(), "https://y/v")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "exit status 1") || !strings.Contains(msg, "Video unavailable") {
		t.Fatalf("msg = %q", msg)
	}
	if strings.Contains(msg, "line1") {
		t.Fatalf("tail should drop old lines: %q", msg)
	}
	// 실패는 캐시하지 않음
	if _, ok := c.streams.get("https://y/v"); ok {
		t.Fatal("error cached")
	}
}

func TestEmptyStdoutIsError(t *testing.T) {
	c, _ := newTestClient(func(context.Context, string, ...string) ([]byte, []byte, error) {
		return []byte("  \n"), nil, nil
	})
	if _, err := c.StreamURL(context.Background(), "https://y/v"); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidation(t *testing.T) {
	var calls int32
	c, _ := newTestClient(func(context.Context, string, ...string) ([]byte, []byte, error) {
		atomic.AddInt32(&calls, 1)
		return []byte("x"), nil, nil
	})
	for _, u := range []string{"", "   ", "--exec=rm", "-f"} {
		if _, err := c.StreamURL(context.Background(), u); err == nil {
			t.Fatalf("StreamURL(%q) expected error", u)
		}
		if _, err := c.Title(context.Background(), u); err == nil {
			t.Fatalf("Title(%q) expected error", u)
		}
	}
	if calls != 0 {
		t.Fatalf("run called %d times", calls)
	}
}

func TestTitleCacheAndFallback(t *testing.T) {
	var calls int32
	out := "  My Song  \n"
	c, clk := newTestClient(func(_ context.Context, _ string, args ...string) ([]byte, []byte, error) {
		atomic.AddInt32(&calls, 1)
		return []byte(out), nil, nil
	})
	ctx := context.Background()

	got, err := c.Title(ctx, "https://y/a")
	if err != nil || got != "My Song" {
		t.Fatalf("got %q, %v", got, err)
	}
	clk.advance(24 * time.Hour)
	_, _ = c.Title(ctx, "https://y/a")
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}

	out = ""
	got, err = c.Title(ctx, "https://y/b")
	if err != nil || got != "https://y/b" {
		t.Fatalf("fallback got %q, %v", got, err)
	}
}

func TestLazyBinaryResolve(t *testing.T) {
	var resolves int32
	fail := true
	c := newClient(func(context.Context) (string, error) {
		atomic.AddInt32(&resolves, 1)
		if fail {
			return "", errors.New("not found")
		}
		return "/x/yt-dlp", nil
	}, func(_ context.Context, bin string, _ ...string) ([]byte, []byte, error) {
		if bin != "/x/yt-dlp" {
			t.Errorf("bin = %q", bin)
		}
		return []byte("t"), nil, nil
	}, time.Now)

	if resolves != 0 {
		t.Fatal("resolved eagerly")
	}
	if _, err := c.Title(context.Background(), "https://y/a"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %v", err)
	}
	fail = false
	if _, err := c.Title(context.Background(), "https://y/a"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.StreamURL(context.Background(), "https://y/a"); err != nil {
		t.Fatal(err)
	}
	// 경로 캐시는 Manager 담당 — 호출마다 resolve
	if resolves != 3 {
		t.Fatalf("resolves = %d, want 3", resolves)
	}
}

func TestWaiterContextCancel(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	c, _ := newTestClient(func(context.Context, string, ...string) ([]byte, []byte, error) {
		close(started)
		<-release
		return []byte("u"), nil, nil
	})
	go func() { _, _ = c.StreamURL(context.Background(), "https://y/v") }()
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := c.StreamURL(ctx, "https://y/v"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
	close(release)
}
