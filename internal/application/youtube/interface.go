// internal/application/youtube/interface.go
package youtube

import "context"

// Resolver YouTube 메타 정보 / 스트림 URL 조회 포트
type Resolver interface {
	// Title 영상 제목 조회
	Title(ctx context.Context, url string) (string, error)

	// StreamURL 오디오 스트림 URL 추출
	StreamURL(ctx context.Context, url string) (string, error)
}
