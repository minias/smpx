// internal/domain/stream.go
package domain

// Stream 재생용 스트림 주소
//   - 오디오 전용: AudioURL 만 사용
//   - 화면 보기: VideoURL(영상) + AudioURL(오디오), 영상에 소리가 포함된 단일 스트림이면 AudioURL 은 비어 있음
type Stream struct {
	AudioURL string `json:"audioUrl,omitempty"`
	VideoURL string `json:"videoUrl,omitempty"`

	// 표시용 제목 (영상 창 제목, 미디어 정보)
	Title string `json:"title,omitempty"`
}
