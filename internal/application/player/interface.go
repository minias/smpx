// internal/application/player/interface.go
package player

import "github.com/minias/smpx/internal/domain"

// AudioPlayer 오디오 재생 포트 (Application → Infrastructure)
type AudioPlayer interface {
	// Play 스트림 재생 (기존 재생은 정지), startAt 초 위치부터
	//  - stream.VideoURL 이 있으면 별도 영상 창에서 재생
	Play(stream domain.Stream, volume int, startAt float64) error
	Stop() error
	Pause() error
	Resume() error

	// SetVolume 0 ~ 100
	SetVolume(volume int) error

	// Position 현재 곡 재생 위치(초) — 화면 보기 전환 시 이어서 재생
	Position() float64

	// SetEndedHandler 사용자가 Stop 하지 않았는데 재생이 끝났을 때 (err != nil 이면 재생 실패)
	SetEndedHandler(onEnded func(err error))
}
