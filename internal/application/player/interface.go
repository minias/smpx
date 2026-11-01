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

	// SetHandlers 재생 종료 / 외부 입력(미디어 키) 콜백 등록
	//  - onEnded: 사용자가 Stop 하지 않았는데 재생이 끝났을 때 (err != nil 이면 비정상 종료)
	//  - onCommand: "playpause" | "next" | "prev" | "stop"
	SetHandlers(onEnded func(err error), onCommand func(command string))
}
