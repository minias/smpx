// internal/domain/playlist_item.go
package domain

// PlaylistItem 플레이리스트 항목 (smp playlist.json 과 동일한 포맷)
type PlaylistItem struct {
	// 사용자에게 보여줄 제목
	Title string `json:"title"`

	// 실제 재생용 YouTube URL
	URL string `json:"url"`
}
