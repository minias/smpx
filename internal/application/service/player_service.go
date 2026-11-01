// internal/application/service/player_service.go
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/minias/smpx/internal/application/player"
	"github.com/minias/smpx/internal/application/playlist"
	"github.com/minias/smpx/internal/application/youtube"
	"github.com/minias/smpx/internal/domain"
)

var (
	ErrEmptyPlaylist = errors.New("플레이리스트가 비어 있습니다")
	ErrDuplicateURL  = errors.New("이미 추가된 URL입니다")
	ErrInvalidIndex  = errors.New("잘못된 플레이리스트 위치입니다")
)

const ytdlpTimeout = 60 * time.Second

// Listener 상태 변경 알림 (UI 바인딩용)
type Listener struct {
	OnStateChanged func(state PlayerState)
	OnTrackChanged func(item domain.PlaylistItem)
	OnError        func(err error)
}

// PlayerService 플레이어 오케스트레이션 (smp 의 PlayerFacade + UseCases)
type PlayerService struct {
	youtube youtube.Resolver
	player  player.AudioPlayer
	repo    playlist.Repository

	mu       sync.Mutex
	state    PlayerState
	listener Listener

	// 재생 요청 순번: yt-dlp 대기 중 다른 요청이 들어오면 이전 요청은 폐기
	playSeq uint64
}

func NewPlayerService(
	yt youtube.Resolver,
	audio player.AudioPlayer,
	repo playlist.Repository,
) *PlayerService {
	s := &PlayerService{
		youtube: yt,
		player:  audio,
		repo:    repo,
		state:   newPlayerState(),
	}

	audio.SetHandlers(s.handleEnded, s.handleCommand)

	return s
}

func (s *PlayerService) SetListener(l Listener) {
	s.mu.Lock()
	s.listener = l
	s.mu.Unlock()
}

// LoadPlaylist 저장된 플레이리스트 로드
func (s *PlayerService) LoadPlaylist() error {
	items, err := s.repo.Load()
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.state.Playlist = items
	s.state.CurrentIndex = -1
	s.mu.Unlock()

	s.emitState()

	return nil
}

func (s *PlayerService) State() PlayerState {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.state.clone()
}

// AddTrack YouTube URL 추가 (제목 조회 후 저장)
func (s *PlayerService) AddTrack(url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return fmt.Errorf("URL을 입력하세요")
	}

	if s.contains(url) {
		return ErrDuplicateURL
	}

	ctx, cancel := context.WithTimeout(context.Background(), ytdlpTimeout)
	defer cancel()

	title, err := s.youtube.Title(ctx, url)
	if err != nil {
		return err
	}

	s.mu.Lock()
	// 제목 조회 중 같은 URL 이 추가됐을 수 있음
	for _, item := range s.state.Playlist {
		if item.URL == url {
			s.mu.Unlock()
			return ErrDuplicateURL
		}
	}

	s.state.Playlist = append(s.state.Playlist, domain.PlaylistItem{
		Title: title,
		URL:   url,
	})
	items := s.state.clone().Playlist
	s.mu.Unlock()

	s.emitState()

	return s.repo.Save(items)
}

// RemoveTrack 항목 삭제 (재생 중인 곡이면 정지)
func (s *PlayerService) RemoveTrack(index int) error {
	s.mu.Lock()

	if index < 0 || index >= len(s.state.Playlist) {
		s.mu.Unlock()
		return ErrInvalidIndex
	}

	switch {
	case index == s.state.CurrentIndex:
		s.stopLocked()
		s.state.CurrentIndex = -1

	case index < s.state.CurrentIndex:
		s.state.CurrentIndex--
	}

	s.state.Playlist = append(s.state.Playlist[:index], s.state.Playlist[index+1:]...)
	items := s.state.clone().Playlist
	s.mu.Unlock()

	s.emitState()

	return s.repo.Save(items)
}

// Play 특정 트랙 재생 (범위 밖이면 0번)
func (s *PlayerService) Play(index int) error {
	s.mu.Lock()

	if len(s.state.Playlist) == 0 {
		s.mu.Unlock()
		return ErrEmptyPlaylist
	}

	if index < 0 || index >= len(s.state.Playlist) {
		index = 0
	}

	s.playSeq++
	seq := s.playSeq
	item := s.state.Playlist[index]

	s.state.CurrentIndex = index
	s.state.State = domain.Loading
	s.mu.Unlock()

	s.emitState()

	// yt-dlp 는 수 초 걸리므로 lock 밖에서 실행
	ctx, cancel := context.WithTimeout(context.Background(), ytdlpTimeout)
	defer cancel()

	streamURL, err := s.youtube.StreamURL(ctx, item.URL)

	s.mu.Lock()

	if seq != s.playSeq {
		// 대기 중 Stop / 다른 곡 재생 요청이 들어옴
		s.mu.Unlock()
		return nil
	}

	if err == nil {
		err = s.player.Play(streamURL, s.state.Volume)
	}

	if err != nil {
		s.state.State = domain.Stopped
		s.mu.Unlock()
		s.emitState()
		return err
	}

	s.state.State = domain.Playing
	onTrack := s.listener.OnTrackChanged
	s.mu.Unlock()

	s.emitState()

	if onTrack != nil {
		onTrack(item)
	}

	return nil
}

// PlayPause 재생/일시정지 토글
func (s *PlayerService) PlayPause() error {
	s.mu.Lock()

	switch s.state.State {
	case domain.Playing:
		if err := s.player.Pause(); err != nil {
			s.mu.Unlock()
			return err
		}
		s.state.State = domain.Paused
		s.mu.Unlock()
		s.emitState()
		return nil

	case domain.Paused:
		if err := s.player.Resume(); err != nil {
			s.mu.Unlock()
			return err
		}
		s.state.State = domain.Playing
		s.mu.Unlock()
		s.emitState()
		return nil

	case domain.Loading:
		s.mu.Unlock()
		return nil
	}

	index := s.state.CurrentIndex
	s.mu.Unlock()

	return s.Play(index)
}

// Next 다음 곡 (수동: 마지막 곡이면 처음으로)
func (s *PlayerService) Next() error {
	s.mu.Lock()
	n := len(s.state.Playlist)
	index := s.state.CurrentIndex
	s.mu.Unlock()

	if n == 0 {
		return ErrEmptyPlaylist
	}

	return s.Play((index + 1) % n)
}

// Previous 이전 곡 (첫 곡이면 마지막으로)
func (s *PlayerService) Previous() error {
	s.mu.Lock()
	n := len(s.state.Playlist)
	index := s.state.CurrentIndex
	s.mu.Unlock()

	if n == 0 {
		return ErrEmptyPlaylist
	}

	if index < 0 {
		index = 0
	}

	return s.Play((index - 1 + n) % n)
}

func (s *PlayerService) Stop() error {
	s.mu.Lock()
	err := s.stopLocked()
	s.mu.Unlock()

	s.emitState()

	return err
}

func (s *PlayerService) SetVolume(volume int) error {
	volume = max(0, min(100, volume))

	s.mu.Lock()
	s.state.Volume = volume
	err := s.player.SetVolume(volume)
	s.mu.Unlock()

	s.emitState()

	return err
}

func (s *PlayerService) SetLoopMode(mode domain.LoopMode) error {
	if !mode.Valid() {
		return fmt.Errorf("알 수 없는 루프 모드: %s", mode)
	}

	s.mu.Lock()
	s.state.LoopMode = mode
	s.mu.Unlock()

	s.emitState()

	return nil
}

// CycleLoopMode none → all → single → none
func (s *PlayerService) CycleLoopMode() domain.LoopMode {
	s.mu.Lock()
	s.state.LoopMode = s.state.LoopMode.Next()
	mode := s.state.LoopMode
	s.mu.Unlock()

	s.emitState()

	return mode
}

// Shutdown 앱 종료 시 mpv 정리
func (s *PlayerService) Shutdown() {
	s.mu.Lock()
	_ = s.stopLocked()
	s.mu.Unlock()
}

func (s *PlayerService) stopLocked() error {
	// 진행 중인 yt-dlp 요청 무효화
	s.playSeq++
	s.state.State = domain.Stopped

	return s.player.Stop()
}

func (s *PlayerService) contains(url string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, item := range s.state.Playlist {
		if item.URL == url {
			return true
		}
	}

	return false
}

// handleEnded 곡이 끝나면 루프 모드에 따라 다음 곡 결정 (smp NextTrackUseCase)
func (s *PlayerService) handleEnded(playErr error) {
	s.mu.Lock()

	if s.state.State != domain.Playing && s.state.State != domain.Paused {
		s.mu.Unlock()
		return
	}

	if playErr != nil {
		s.state.State = domain.Stopped
		s.mu.Unlock()
		s.emitState()
		s.emitError(fmt.Errorf("재생 실패: %w", playErr))
		return
	}

	n := len(s.state.Playlist)
	index := s.state.CurrentIndex
	next := -1

	switch s.state.LoopMode {
	case domain.LoopSingle:
		next = index

	case domain.LoopAll:
		if n > 0 {
			next = (index + 1) % n
		}

	default:
		if index+1 < n {
			next = index + 1
		}
	}

	if next < 0 {
		s.state.State = domain.Stopped
		s.mu.Unlock()
		s.emitState()
		return
	}

	s.mu.Unlock()

	if err := s.Play(next); err != nil {
		s.emitError(err)
	}
}

// handleCommand mpv 로 들어온 미디어 키 처리
func (s *PlayerService) handleCommand(command string) {
	var err error

	switch command {
	case "playpause":
		err = s.PlayPause()
	case "next":
		err = s.Next()
	case "prev":
		err = s.Previous()
	case "stop":
		err = s.Stop()
	}

	if err != nil {
		s.emitError(err)
	}
}

func (s *PlayerService) emitState() {
	s.mu.Lock()
	cb := s.listener.OnStateChanged
	state := s.state.clone()
	s.mu.Unlock()

	if cb != nil {
		cb(state)
	}
}

func (s *PlayerService) emitError(err error) {
	s.mu.Lock()
	cb := s.listener.OnError
	s.mu.Unlock()

	if cb != nil {
		cb(err)
	}
}
