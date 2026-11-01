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

// 재생목록 조회는 곡 수에 비례해 오래 걸림
const expandTimeout = 3 * time.Minute

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

	audio.SetEndedHandler(s.handleEnded)

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

// AddResult 추가 결과
type AddResult struct {
	Added   int `json:"added"`
	Skipped int `json:"skipped"` // 이미 있는 곡
}

// AddTrack YouTube URL 추가 (재생목록 URL 이면 목록 전체, 영상 ID 기준 중복 제외)
func (s *PlayerService) AddTrack(url string) (AddResult, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return AddResult{}, fmt.Errorf("URL을 입력하세요")
	}

	// 단일 영상이 이미 있으면 yt-dlp 호출 없이 종료
	if !strings.Contains(url, "list=") && s.contains(url) {
		return AddResult{Skipped: 1}, ErrDuplicateURL
	}

	ctx, cancel := context.WithTimeout(context.Background(), expandTimeout)
	defer cancel()

	found, err := s.youtube.Expand(ctx, url)
	if err != nil {
		return AddResult{}, err
	}

	s.mu.Lock()

	// 조회 중 추가된 곡이 있을 수 있으므로 lock 안에서 중복 판별
	seen := make(map[string]bool, len(s.state.Playlist)+len(found))
	for _, item := range s.state.Playlist {
		seen[domain.VideoID(item.URL)] = true
	}

	var result AddResult
	for _, item := range found {
		id := domain.VideoID(item.URL)
		if seen[id] {
			result.Skipped++
			continue
		}
		seen[id] = true
		s.state.Playlist = append(s.state.Playlist, item)
		result.Added++
	}

	items := s.state.clone().Playlist
	s.mu.Unlock()

	if result.Added == 0 {
		return result, ErrDuplicateURL
	}

	s.emitState()

	return result, s.repo.Save(items)
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
	return s.playAt(index, 0)
}

// SetShowVideo 화면 보기 전환 (재생 중이면 현재 위치부터 이어서 재생)
func (s *PlayerService) SetShowVideo(on bool) error {
	s.mu.Lock()

	if s.state.ShowVideo == on {
		s.mu.Unlock()
		return nil
	}

	s.state.ShowVideo = on
	resume := s.state.State == domain.Playing || s.state.State == domain.Paused
	index := s.state.CurrentIndex
	s.mu.Unlock()

	s.emitState()

	if !resume || index < 0 {
		return nil
	}

	return s.playAt(index, s.player.Position())
}

func (s *PlayerService) playAt(index int, startAt float64) error {
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
	video := s.state.ShowVideo

	s.state.CurrentIndex = index
	s.state.State = domain.Loading
	s.mu.Unlock()

	s.emitState()

	// yt-dlp 는 수 초 걸리므로 lock 밖에서 실행
	ctx, cancel := context.WithTimeout(context.Background(), ytdlpTimeout)
	defer cancel()

	stream, err := s.youtube.Streams(ctx, item.URL, video)

	s.mu.Lock()

	if seq != s.playSeq {
		// 대기 중 Stop / 다른 곡 재생 요청이 들어옴
		s.mu.Unlock()
		return nil
	}

	if err == nil {
		stream.Title = item.Title
		err = s.player.Play(stream, s.state.Volume, startAt)
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

// SyncPaused 플레이어가 스스로 일시정지/재개한 경우 상태 동기화 (예: 이어폰 분리)
func (s *PlayerService) SyncPaused(paused bool) {
	s.mu.Lock()

	switch {
	case paused && s.state.State == domain.Playing:
		s.state.State = domain.Paused
	case !paused && s.state.State == domain.Paused:
		s.state.State = domain.Playing
	default:
		s.mu.Unlock()
		return
	}

	s.mu.Unlock()
	s.emitState()
}

// Shutdown 앱 종료 시 재생 정리
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

	id := domain.VideoID(url)
	for _, item := range s.state.Playlist {
		if domain.VideoID(item.URL) == id {
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
