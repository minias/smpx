// app.go
package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/minias/smpx/internal/application/service"
	"github.com/minias/smpx/internal/domain"
	"github.com/minias/smpx/internal/infrastructure/update"
	"github.com/minias/smpx/internal/infrastructure/ytdlp"
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
	audio         *webviewPlayer
	ytdlp         *ytdlp.Manager
	updater       *update.Checker

	notifyMu      sync.Mutex
	notifyEnabled bool
}

func NewApp(
	playerService *service.PlayerService,
	audio *webviewPlayer,
	ytdlpManager *ytdlp.Manager,
	updater *update.Checker,
) *App {
	return &App{
		playerService: playerService,
		audio:         audio,
		ytdlp:         ytdlpManager,
		updater:       updater,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.audio.setContext(ctx)

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

	go a.prepareYtdlp(ctx)
}

// prepareYtdlp yt-dlp 설치(최초 실행) / 하루 한 번 업데이트
func (a *App) prepareYtdlp(ctx context.Context) {
	runtime.EventsEmit(ctx, "ytdlp:status", "preparing")

	prepCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := a.ytdlp.Prepare(prepCtx); err != nil {
		runtime.EventsEmit(ctx, "ytdlp:status", "error")
		runtime.EventsEmit(ctx, "player:error", err.Error())
		return
	}

	runtime.EventsEmit(ctx, "ytdlp:status", "ready")
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

// AddTrack URL 추가 (재생목록이면 전체), 추가 / 중복 곡 수 반환
func (a *App) AddTrack(url string) (service.AddResult, error) {
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

// ---- <audio> → Go 통보 ----

// AudioEnded 곡이 끝까지 재생됨
func (a *App) AudioEnded(seq uint64) {
	a.audio.ended(seq, nil)
}

// AudioFailed 스트림 재생 실패 (네트워크 / 형식 / 만료 등)
func (a *App) AudioFailed(seq uint64, message string) {
	a.audio.ended(seq, errors.New(message))
}

// AudioPaused OS / 브라우저가 일시정지·재개한 경우 (미디어 키는 MediaSession 으로 직접 호출)
func (a *App) AudioPaused(seq uint64, paused bool) {
	if a.audio.current(seq) {
		a.playerService.SyncPaused(paused)
	}
}
