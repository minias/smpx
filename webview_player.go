// webview_player.go
package main

import (
	"context"
	"errors"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	appplayer "github.com/minias/smpx/internal/application/player"
	"github.com/minias/smpx/internal/domain"
	"github.com/minias/smpx/internal/infrastructure/streamproxy"
)

var _ appplayer.AudioPlayer = (*webviewPlayer)(nil)

var errNotPlaying = errors.New("재생 중이 아닙니다")

// audioCommand 재생 제어 명령
//   - 메인 창 <audio>: "audio:command" 이벤트
//   - 영상 창: stdin 으로 전달 → "video:command" 이벤트
type audioCommand struct {
	Action   string  `json:"action"` // play | pause | resume | stop | volume
	Seq      uint64  `json:"seq"`
	URL      string  `json:"url,omitempty"`      // 오디오
	VideoURL string  `json:"videoUrl,omitempty"` // 영상 (영상 창 전용)
	Volume   int     `json:"volume"`
	StartAt  float64 `json:"startAt,omitempty"`
	Title    string  `json:"title,omitempty"`
}

// webviewPlayer WebView 로 재생하는 AudioPlayer
//   - 오디오 전용: 메인 창의 HTML5 <audio>
//   - 화면 보기: 별도 영상 창(자식 프로세스)의 <video> + <audio>
//   - 재생 종료 / 실패 / 일시정지 / 위치 통보는 seq 로 이전 곡 이벤트를 무시
type webviewPlayer struct {
	mu      sync.Mutex
	ctx     context.Context
	seq     uint64
	active  bool
	video   bool // 현재 곡을 영상 창에서 재생 중
	pos     float64
	onEnded func(error)

	// 영상 창에서 온 사용자 조작 (미디어 키 / 창 닫기)
	onCommand     func(action string)
	onVideoClosed func()

	window *videoWindow

	// WebView 의 큰 범위 요청을 나눠 받는 로컬 프록시 (nil 이면 원본 주소 사용)
	proxy *streamproxy.Proxy
}

func newWebviewPlayer(proxy *streamproxy.Proxy) *webviewPlayer {
	p := &webviewPlayer{proxy: proxy}
	p.window = newVideoWindow(p.handleVideoEvent)
	return p
}

func (p *webviewPlayer) setContext(ctx context.Context) {
	p.mu.Lock()
	p.ctx = ctx
	p.mu.Unlock()
}

func (p *webviewPlayer) setVideoHandlers(onCommand func(string), onClosed func()) {
	p.mu.Lock()
	p.onCommand = onCommand
	p.onVideoClosed = onClosed
	p.mu.Unlock()
}

func (p *webviewPlayer) SetEndedHandler(onEnded func(error)) {
	p.mu.Lock()
	p.onEnded = onEnded
	p.mu.Unlock()
}

func (p *webviewPlayer) Play(stream domain.Stream, volume int, startAt float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 이전 출력 정지
	p.stopOutputLocked()

	p.seq++
	p.active = true
	p.video = stream.VideoURL != ""
	p.pos = startAt

	cmd := audioCommand{
		Action:   "play",
		Seq:      p.seq,
		URL:      p.wrap(stream.AudioURL),
		VideoURL: p.wrap(stream.VideoURL),
		Volume:   volume,
		StartAt:  startAt,
		Title:    stream.Title,
	}

	if p.video {
		return p.window.send(cmd)
	}

	// 화면 보기를 끄면 영상 창도 닫음
	p.window.close()

	return p.emitLocked(cmd)
}

func (p *webviewPlayer) wrap(rawURL string) string {
	if p.proxy == nil {
		return rawURL
	}
	return p.proxy.Wrap(rawURL)
}

func (p *webviewPlayer) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.active {
		return nil
	}

	p.stopOutputLocked()
	p.active = false

	return nil
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

	return p.sendLocked(audioCommand{Action: "volume", Seq: p.seq, Volume: max(0, min(100, volume))})
}

func (p *webviewPlayer) Position() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.pos
}

func (p *webviewPlayer) control(action string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.active {
		return errNotPlaying
	}

	return p.sendLocked(audioCommand{Action: action, Seq: p.seq})
}

// stopOutputLocked 현재 출력(메인 <audio> 또는 영상 창) 정지
func (p *webviewPlayer) stopOutputLocked() {
	if !p.active {
		return
	}

	cmd := audioCommand{Action: "stop", Seq: p.seq}
	if p.video {
		if p.window.running() {
			_ = p.window.send(cmd)
		}
		return
	}

	_ = p.emitLocked(cmd)
}

// sendLocked 현재 출력 대상으로 명령 전달
func (p *webviewPlayer) sendLocked(cmd audioCommand) error {
	if p.video && p.active {
		return p.window.send(cmd)
	}

	return p.emitLocked(cmd)
}

// ended 곡 종료 / 재생 실패 통보
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

// progress 재생 위치 통보
func (p *webviewPlayer) progress(seq uint64, t float64) {
	p.mu.Lock()
	if p.active && seq == p.seq {
		p.pos = t
	}
	p.mu.Unlock()
}

// current 현재 재생 중인 곡의 seq 인지
func (p *webviewPlayer) current(seq uint64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.active && seq == p.seq
}

// closeVideo 앱 종료 시 영상 창 정리
func (p *webviewPlayer) closeVideo() {
	p.window.close()
}

// handleVideoEvent 영상 창 → 메인
func (p *webviewPlayer) handleVideoEvent(ev videoEvent) {
	switch ev.Event {
	case "ended":
		p.ended(ev.Seq, nil)

	case "failed":
		p.ended(ev.Seq, errors.New(ev.Message))

	case "progress":
		p.progress(ev.Seq, ev.Time)

	case "paused":
		p.mu.Lock()
		cb := p.onCommand
		ok := p.active && ev.Seq == p.seq
		p.mu.Unlock()

		if ok && cb != nil {
			if ev.Paused {
				cb("sync-paused")
			} else {
				cb("sync-playing")
			}
		}

	case "command":
		p.mu.Lock()
		cb := p.onCommand
		p.mu.Unlock()

		if cb != nil {
			cb(ev.Action)
		}

	case "closed":
		p.mu.Lock()
		wasVideo := p.video && p.active
		cb := p.onVideoClosed
		p.mu.Unlock()

		if wasVideo && cb != nil {
			cb()
		}
	}
}

func (p *webviewPlayer) emitLocked(cmd audioCommand) error {
	if p.ctx == nil {
		return errors.New("UI 가 아직 준비되지 않았습니다")
	}

	runtime.EventsEmit(p.ctx, "audio:command", cmd)

	return nil
}
