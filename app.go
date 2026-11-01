// app.go
package main

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/minias/smpx/internal/application/service"
	"github.com/minias/smpx/internal/domain"
	"github.com/minias/smpx/internal/infrastructure/update"
)

// AppVersion 현재 앱 버전 (업데이트 확인에 사용)
const AppVersion = "0.1.0"

// UpdateRepo GitHub 릴리스 저장소 (owner/name)
const UpdateRepo = "minias/smpx"

const defaultTitle = "SMPX"

// App Wails 바인딩 (UI ↔ PlayerService)
type App struct {
	ctx           context.Context
	playerService *service.PlayerService
	updater       *update.Checker

	notifyMu      sync.Mutex
	notifyEnabled bool
}

func NewApp(playerService *service.PlayerService, updater *update.Checker) *App {
	return &App{
		playerService: playerService,
		updater:       updater,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	a.initNotifications()

	a.playerService.SetListener(service.Listener{
		OnStateChanged: func(state service.PlayerState) {
			runtime.EventsEmit(ctx, "player:state", state)
		},
		OnTrackChanged: a.onTrackChanged,
		OnError: func(err error) {
			runtime.EventsEmit(ctx, "player:error", err.Error())
		},
	})

	if err := a.playerService.LoadPlaylist(); err != nil {
		runtime.EventsEmit(ctx, "player:error", "플레이리스트 로드 실패: "+err.Error())
	}
}

func (a *App) shutdown(ctx context.Context) {
	a.playerService.Shutdown()

	if a.notificationsEnabled() {
		runtime.CleanupNotifications(ctx)
	}
}

// onSecondInstanceLaunch 중복 실행 시 기존 창을 앞으로
func (a *App) onSecondInstanceLaunch() {
	if a.ctx == nil {
		return
	}

	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
}

// initNotifications 데스크톱 알림 초기화 (실패 시 알림 비활성화)
func (a *App) initNotifications() {
	if err := runtime.InitializeNotifications(a.ctx); err != nil {
		// 예: macOS 개발 모드(번들 ID 없음) — 알림 없이 동작
		return
	}

	a.notifyMu.Lock()
	a.notifyEnabled = true
	a.notifyMu.Unlock()

	go func() {
		ok, err := runtime.RequestNotificationAuthorization(a.ctx)
		if err != nil || !ok {
			a.notifyMu.Lock()
			a.notifyEnabled = false
			a.notifyMu.Unlock()
		}
	}()
}

func (a *App) notificationsEnabled() bool {
	a.notifyMu.Lock()
	defer a.notifyMu.Unlock()

	return a.notifyEnabled
}

func (a *App) onTrackChanged(item domain.PlaylistItem) {
	runtime.WindowSetTitle(a.ctx, "재생중: "+item.Title)

	if !a.notificationsEnabled() {
		return
	}

	_ = runtime.SendNotification(a.ctx, runtime.NotificationOptions{
		ID:    uuid.NewString(),
		Title: "재생중",
		Body:  item.Title,
	})
}

// ---- 바인딩 메서드 ----

func (a *App) GetState() service.PlayerState {
	return a.playerService.State()
}

func (a *App) AddTrack(url string) error {
	return a.playerService.AddTrack(url)
}

func (a *App) RemoveTrack(index int) error {
	return a.playerService.RemoveTrack(index)
}

func (a *App) Play(index int) error {
	return a.playerService.Play(index)
}

func (a *App) PlayPause() error {
	return a.playerService.PlayPause()
}

func (a *App) Next() error {
	return a.playerService.Next()
}

func (a *App) Previous() error {
	return a.playerService.Previous()
}

func (a *App) Stop() error {
	err := a.playerService.Stop()
	runtime.WindowSetTitle(a.ctx, defaultTitle)

	return err
}

func (a *App) SetVolume(volume int) error {
	return a.playerService.SetVolume(volume)
}

// CycleLoopMode none → all → single 순환, 새 모드 반환
func (a *App) CycleLoopMode() string {
	return string(a.playerService.CycleLoopMode())
}

// CheckForUpdate 새 버전이 있으면 정보 반환, 없으면 nil
func (a *App) CheckForUpdate() (*update.Info, error) {
	return a.updater.Check(a.ctx)
}

func (a *App) OpenURL(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}

func (a *App) Version() string {
	return AppVersion
}
