// internal/application/youtube/interface.go
package youtube

import (
	"context"

	"github.com/minias/smpx/internal/domain"
)

// Resolver YouTube 메타 정보 / 스트림 URL 조회 포트
type Resolver interface {
	// Expand URL → 플레이리스트 항목 (재생목록이면 전체, 단일 영상이면 1개)
	Expand(ctx context.Context, url string) ([]domain.PlaylistItem, error)

	// Streams 재생 스트림 추출 (video=true 면 영상 포함)
	Streams(ctx context.Context, url string, video bool) (domain.Stream, error)
}
