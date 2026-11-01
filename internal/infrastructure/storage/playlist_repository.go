// internal/infrastructure/storage/playlist_repository.go
package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/minias/smpx/internal/application/playlist"
	"github.com/minias/smpx/internal/domain"
)

// 포트 구현 확인
var _ playlist.Repository = (*PlaylistRepository)(nil)

// PlaylistRepository JSON 파일 기반 플레이리스트 저장소
type PlaylistRepository struct {
	mu   sync.Mutex
	path string
}

// NewPlaylistRepository 사용자 설정 디렉터리(smpx/playlist.json) 기반 저장소 생성
func NewPlaylistRepository() (*PlaylistRepository, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("설정 디렉터리 조회 실패: %w", err)
	}
	dir = filepath.Join(dir, "smpx")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("설정 디렉터리 생성 실패: %w", err)
	}
	return NewPlaylistRepositoryAt(filepath.Join(dir, "playlist.json")), nil
}

// NewPlaylistRepositoryAt 지정 경로 기반 저장소 생성 (테스트용)
func NewPlaylistRepositoryAt(path string) *PlaylistRepository {
	return &PlaylistRepository{path: path}
}

// Path 저장 파일 경로
func (r *PlaylistRepository) Path() string {
	return r.path
}

// Load 플레이리스트 읽기 (파일 없으면 빈 목록)
func (r *PlaylistRepository) Load() ([]domain.PlaylistItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := os.ReadFile(r.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []domain.PlaylistItem{}, nil
		}
		return nil, fmt.Errorf("플레이리스트 읽기 실패: %w", err)
	}

	var raw []domain.PlaylistItem
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("플레이리스트 파싱 실패: %w", err)
	}

	// url 비어있는 항목 제외
	items := make([]domain.PlaylistItem, 0, len(raw))
	for _, it := range raw {
		if it.URL == "" {
			continue
		}
		items = append(items, it)
	}
	return items, nil
}

// Save 플레이리스트 저장 (임시 파일 + rename 원자적 쓰기)
func (r *PlaylistRepository) Save(items []domain.PlaylistItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if items == nil {
		items = []domain.PlaylistItem{}
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("플레이리스트 직렬화 실패: %w", err)
	}

	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("디렉터리 생성 실패: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".playlist-*.json.tmp")
	if err != nil {
		return fmt.Errorf("임시 파일 생성 실패: %w", err)
	}
	tmpName := tmp.Name()
	// 실패 시 임시 파일 정리
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("임시 파일 쓰기 실패: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("임시 파일 동기화 실패: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("임시 파일 닫기 실패: %w", err)
	}
	if err := os.Rename(tmpName, r.path); err != nil {
		return fmt.Errorf("플레이리스트 교체 실패: %w", err)
	}
	ok = true
	return nil
}
