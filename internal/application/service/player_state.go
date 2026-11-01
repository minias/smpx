// internal/application/service/player_state.go
package service

import "github.com/minias/smpx/internal/domain"

// PlayerState 플레이어 상태 (순수 상태, PlayerService.mu 로 보호)
type PlayerState struct {
	Playlist     []domain.PlaylistItem `json:"playlist"`
	CurrentIndex int                   `json:"currentIndex"`
	State        domain.PlaybackState  `json:"state"`
	LoopMode     domain.LoopMode       `json:"loopMode"`
	Volume       int                   `json:"volume"`
	ShowVideo    bool                  `json:"showVideo"`
}

func newPlayerState() PlayerState {
	return PlayerState{
		Playlist:     []domain.PlaylistItem{},
		CurrentIndex: -1,
		State:        domain.Stopped,
		LoopMode:     domain.LoopNone,
		Volume:       50,
	}
}

// clone UI 전달용 복사본
func (s PlayerState) clone() PlayerState {
	s.Playlist = append([]domain.PlaylistItem{}, s.Playlist...)
	return s
}
