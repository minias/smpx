// internal/application/youtube/interface.go
package youtube

import (
	"context"

	"github.com/minias/smpx/internal/domain"
)

// Resolver YouTube 메타 정보 / 스트림 URL 조회 포트
type Resolver interface {
	// Title 영상 제목 조회
	Title(ctx context.Context, url string) (string, error)

	// Streams 재생 스트림 추출 (video=true 면 영상 포함)
	Streams(ctx context.Context, url string, video bool) (domain.Stream, error)
}
