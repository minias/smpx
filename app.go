// app.go
package main

import (
	"context"

	"github.com/minias/smpx/internal/application/service"
)

type App struct {
	ctx           context.Context
	playerService *service.PlayerService
}

func NewApp() *App {
	return &App{
		playerService: service.NewPlayerService(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) PlayYoutube(url string) error {
	return a.playerService.PlayYoutube(url)
}

func (a *App) Stop() error {
	return a.playerService.Stop()
}

func (a *App) Pause() error {
	return a.playerService.Pause()
}

func (a *App) Resume() error {
	return a.playerService.Resume()
}

func (a *App) SetVolume(volume int) error {
	return a.playerService.SetVolume(volume)
}
