// video_app.go
package main

import (
	"context"

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
}

func (v *VideoApp) startup(ctx context.Context) {
	v.ctx = ctx

	go func() {
		v.ch.commands(func(cmd audioCommand) {
			if cmd.Title != "" {
				runtime.WindowSetTitle(ctx, cmd.Title)
			}
			runtime.EventsEmit(ctx, "video:command", cmd)
		})

		// 메인 프로세스가 종료되었거나 화면 보기를 끔
		runtime.Quit(ctx)
	}()
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
