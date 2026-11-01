// internal/domain/playback_state.go
package domain

// PlaybackState 플레이어 재생 상태
type PlaybackState string

const (
	Stopped PlaybackState = "stopped"
	Loading PlaybackState = "loading"
	Playing PlaybackState = "playing"
	Paused  PlaybackState = "paused"
)
