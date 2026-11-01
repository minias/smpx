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

	// StreamURL 오디오 스트림 URL 추출
	StreamURL(ctx context.Context, url string) (string, error)
}
