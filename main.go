package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/minias/smpx/internal/application/service"
	"github.com/minias/smpx/internal/infrastructure/player"
	"github.com/minias/smpx/internal/infrastructure/storage"
	"github.com/minias/smpx/internal/infrastructure/update"
	"github.com/minias/smpx/internal/infrastructure/ytdlp"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	repo, err := storage.NewPlaylistRepository()
	if err != nil {
		fmt.Fprintln(os.Stderr, "플레이리스트 저장소 초기화 실패:", err)
		os.Exit(1)
	}

	playerService := service.NewPlayerService(
		ytdlp.NewClient(),
		player.NewPlayer(),
		repo,
	)

	app := NewApp(playerService, update.NewChecker(UpdateRepo, AppVersion))

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
