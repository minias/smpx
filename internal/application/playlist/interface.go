// internal/application/playlist/interface.go
package playlist

import "github.com/minias/smpx/internal/domain"

// Repository 플레이리스트 저장소 포트
type Repository interface {
	Load() ([]domain.PlaylistItem, error)
	Save(items []domain.PlaylistItem) error
}
