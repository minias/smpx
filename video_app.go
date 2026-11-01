// video_app.go
package main

import (
	"context"
	"sync"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// VideoApp 영상 창(자식 프로세스) 바인딩
//   - 프론트엔드는 window.go.main.VideoApp 존재 여부로 영상 화면을 렌더링
type VideoApp struct {
	ctx context.Context
	ch  *videoChannel

	// 프론트엔드가 이벤트를 구독하기 전에 온 명령은 보관 후 Ready 에서 전달
	mu      sync.Mutex
	ready   bool
	pending []audioCommand
	fitted  bool // 최초 맞춤 시에만 화면 가운데로 이동
}

const (
	// 영상 창이 차지할 수 있는 최대 화면 비율
	maxScreenRatio = 0.8

	// 작은 영상도 이 너비까지는 키워서 표시
	minVideoWidth = 640
)

func (v *VideoApp) startup(ctx context.Context) {
	v.ctx = ctx

	// macOS: 영상 창은 Dock 아이콘 없이 (메인 앱 아이콘 하나만 표시)
	setVideoWindowAccessory()

	go func() {
		v.ch.commands(v.forward)

		// 메인 프로세스가 종료되었거나 화면 보기를 끔
		runtime.Quit(ctx)
	}()
}

func (v *VideoApp) forward(cmd audioCommand) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.ready {
		v.pending = append(v.pending, cmd)
		return
	}

	v.emitLocked(cmd)
}

func (v *VideoApp) emitLocked(cmd audioCommand) {
	if cmd.Title != "" {
		runtime.WindowSetTitle(v.ctx, cmd.Title)
	}
	runtime.EventsEmit(v.ctx, "video:command", cmd)
}

// Ready 프론트엔드가 "video:command" 를 구독한 뒤 호출
func (v *VideoApp) Ready() {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.ready = true
	for _, cmd := range v.pending {
		v.emitLocked(cmd)
	}
	v.pending = nil
}

// Fit 영상 원본 크기(비율)에 맞춰 창 크기 조정 (화면의 80% 이내)
//   - innerW/innerH: 현재 웹뷰(내용 영역) 크기, 제목 표시줄 등 창 테두리 보정용
func (v *VideoApp) Fit(width, height, innerW, innerH int) {
	if width <= 0 || height <= 0 {
		return
	}

	maxW, maxH := 1280.0, 720.0
	if screens, err := runtime.ScreenGetAll(v.ctx); err == nil {
		for _, sc := range screens {
			if sc.IsCurrent || sc.IsPrimary {
				maxW = float64(sc.Size.Width) * maxScreenRatio
				maxH = float64(sc.Size.Height) * maxScreenRatio
				if sc.IsCurrent {
					break
				}
			}
		}
	}

	// WindowSetSize 는 테두리 포함 크기 → 내용 영역이 영상 비율이 되도록 보정
	frameW, frameH := runtime.WindowGetSize(v.ctx)
	chromeW, chromeH := max(0, frameW-innerW), max(0, frameH-innerH)

	w, h := float64(width), float64(height)
	scale := min((maxW-float64(chromeW))/w, (maxH-float64(chromeH))/h, max(1, minVideoWidth/w))

	runtime.WindowSetSize(v.ctx, int(w*scale)+chromeW, int(h*scale)+chromeH)

	v.mu.Lock()
	first := !v.fitted
	v.fitted = true
	v.mu.Unlock()

	if first {
		runtime.WindowCenter(v.ctx)
	}
}

// ToggleFullscreen 전체 화면 전환 (Enter)
func (v *VideoApp) ToggleFullscreen() {
	if runtime.WindowIsFullscreen(v.ctx) {
		runtime.WindowUnfullscreen(v.ctx)
		return
	}
	runtime.WindowFullscreen(v.ctx)
}

// ExitFullscreen 전체 화면 해제 (Esc)
func (v *VideoApp) ExitFullscreen() {
	if runtime.WindowIsFullscreen(v.ctx) {
		runtime.WindowUnfullscreen(v.ctx)
	}
}

// Report 영상 창 → 메인 이벤트 전달
func (v *VideoApp) Report(ev videoEvent) {
	_ = v.ch.emit(ev)
}

func runVideoWindow() error {
	ch, err := openVideoChannel()
	if err != nil {
		return err
	}

	app := &VideoApp{ch: ch}

	return wails.Run(&options.App{
		Title:     "SMPX 화면",
		Width:     960,
		Height:    540,
		MinWidth:  320,
		MinHeight: 180,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})
}
