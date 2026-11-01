// webview_player.go
package main

import (
	"context"
	"errors"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	appplayer "github.com/minias/smpx/internal/application/player"
)

var _ appplayer.AudioPlayer = (*webviewPlayer)(nil)

var errNotPlaying = errors.New("재생 중이 아닙니다")

// audioCommand 프론트엔드 <audio> 제어 이벤트 ("audio:command")
type audioCommand struct {
	Action string `json:"action"` // play | pause | resume | stop | volume
	Seq    uint64 `json:"seq"`
	URL    string `json:"url,omitempty"`
	Volume int    `json:"volume"`
}

// webviewPlayer WebView 의 HTML5 <audio> 로 재생하는 AudioPlayer
//   - Go → JS: "audio:command" 이벤트
//   - JS → Go: App.AudioEnded / App.AudioFailed / App.AudioPaused (seq 로 이전 곡 이벤트 무시)
type webviewPlayer struct {
	mu      sync.Mutex
	ctx     context.Context
	seq     uint64
	active  bool
	onEnded func(error)
}

func newWebviewPlayer() *webviewPlayer {
	return &webviewPlayer{}
}

func (p *webviewPlayer) setContext(ctx context.Context) {
	p.mu.Lock()
	p.ctx = ctx
	p.mu.Unlock()
}

func (p *webviewPlayer) SetEndedHandler(onEnded func(error)) {
	p.mu.Lock()
	p.onEnded = onEnded
	p.mu.Unlock()
}

func (p *webviewPlayer) Play(streamURL string, volume int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.seq++
	p.active = true

	return p.emitLocked(audioCommand{Action: "play", Seq: p.seq, URL: streamURL, Volume: volume})
}

func (p *webviewPlayer) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.active {
		return nil
	}

	p.active = false

	return p.emitLocked(audioCommand{Action: "stop", Seq: p.seq})
}

func (p *webviewPlayer) Pause() error {
	return p.control("pause")
}

func (p *webviewPlayer) Resume() error {
	return p.control("resume")
}

func (p *webviewPlayer) SetVolume(volume int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.emitLocked(audioCommand{Action: "volume", Seq: p.seq, Volume: max(0, min(100, volume))})
}

func (p *webviewPlayer) control(action string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.active {
		return errNotPlaying
	}

	return p.emitLocked(audioCommand{Action: action, Seq: p.seq})
}

// ended JS 에서 곡 종료 / 재생 실패 통보
func (p *webviewPlayer) ended(seq uint64, err error) {
	p.mu.Lock()

	if !p.active || seq != p.seq {
		p.mu.Unlock()
		return
	}

	p.active = false
	cb := p.onEnded
	p.mu.Unlock()

	if cb != nil {
		cb(err)
	}
}

// current 현재 재생 중인 곡의 seq 인지
func (p *webviewPlayer) current(seq uint64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.active && seq == p.seq
}

func (p *webviewPlayer) emitLocked(cmd audioCommand) error {
	if p.ctx == nil {
		return errors.New("UI 가 아직 준비되지 않았습니다")
	}

	runtime.EventsEmit(p.ctx, "audio:command", cmd)

	return nil
}
