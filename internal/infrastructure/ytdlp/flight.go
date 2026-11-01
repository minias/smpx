// internal/infrastructure/ytdlp/flight.go
package ytdlp

import (
	"context"
	"sync"
)

// flightCall 진행 중인 단일 호출
type flightCall struct {
	done  chan struct{}
	value string
	err   error
}

// flightGroup 키별 중복 호출 억제 (singleflight 축소판)
type flightGroup struct {
	mu    sync.Mutex
	calls map[string]*flightCall
}

// do 같은 키의 동시 호출은 fn 을 한 번만 실행하고 결과 공유
// 대기자는 자신의 ctx 취소 시 먼저 반환
func (g *flightGroup) do(ctx context.Context, key string, fn func() (string, error)) (string, error) {
	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[string]*flightCall)
	}
	if c, ok := g.calls[key]; ok {
		g.mu.Unlock()
		select {
		case <-c.done:
			return c.value, c.err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	c := &flightCall{done: make(chan struct{})}
	g.calls[key] = c
	g.mu.Unlock()

	// panic 시에도 대기자 해제
	defer func() {
		g.mu.Lock()
		delete(g.calls, key)
		g.mu.Unlock()
		close(c.done)
	}()

	c.value, c.err = fn()
	return c.value, c.err
}
