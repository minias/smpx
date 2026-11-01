// internal/application/player/interface.go
package player

// AudioPlayer 오디오 재생 포트 (Application → Infrastructure)
type AudioPlayer interface {
	// Play 스트림 URL 재생 (기존 재생은 정지)
	Play(streamURL string, volume int) error
	Stop() error
	Pause() error
	Resume() error

	// SetVolume 0 ~ 100
	SetVolume(volume int) error

	// SetEndedHandler 사용자가 Stop 하지 않았는데 재생이 끝났을 때 (err != nil 이면 재생 실패)
	SetEndedHandler(onEnded func(err error))
}
