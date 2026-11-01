package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/minias/smpx/internal/application/service"
	"github.com/minias/smpx/internal/infrastructure/storage"
	"github.com/minias/smpx/internal/infrastructure/streamproxy"
	"github.com/minias/smpx/internal/infrastructure/update"
	"github.com/minias/smpx/internal/infrastructure/ytdlp"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if os.Getenv(videoWindowEnv) != "" {
		if err := runVideoWindow(); err != nil {
			fmt.Fprintln(os.Stderr, "영상 창 오류:", err)
			os.Exit(1)
		}
		return
	}

	repo, err := storage.NewPlaylistRepository()
	if err != nil {
		fmt.Fprintln(os.Stderr, "플레이리스트 저장소 초기화 실패:", err)
		os.Exit(1)
	}

	ytdlpManager, err := ytdlp.NewManager()
	if err != nil {
		fmt.Fprintln(os.Stderr, "yt-dlp 관리자 초기화 실패:", err)
		os.Exit(1)
	}

	// 프록시를 못 띄우면 원본 주소로 재생 (큰 스트림은 403 가능)
	proxy, err := streamproxy.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, "스트림 프록시 시작 실패:", err)
		proxy = nil
	}

	audio := newWebviewPlayer(proxy)

	playerService := service.NewPlayerService(
		ytdlp.NewClient(ytdlpManager),
		audio,
		repo,
	)

	app := NewApp(playerService, audio, ytdlpManager, update.NewChecker(UpdateRepo, AppVersion))

	err = wails.Run(&options.App{
		Title:     defaultTitle,
		Width:     520,
		Height:    640,
		MinWidth:  420,
		MinHeight: 480,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.minias.smpx",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				app.onSecondInstanceLaunch()
			},
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
