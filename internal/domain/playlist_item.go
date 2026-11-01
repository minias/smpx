// internal/domain/playlist_item.go
package domain

import (
	"net/url"
	"strings"
)

// PlaylistItem 플레이리스트 항목 (smp playlist.json 과 동일한 포맷)
type PlaylistItem struct {
	// 사용자에게 보여줄 제목
	Title string `json:"title"`

	// 실제 재생용 YouTube URL
	URL string `json:"url"`
}

// VideoID YouTube 영상 ID (중복 판별용), 알 수 없으면 URL 그대로
//   - watch?v=ID, youtu.be/ID, shorts/ID, embed/ID, live/ID
func VideoID(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		return rawURL
	}

	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	host = strings.TrimPrefix(host, "m.")
	host = strings.TrimPrefix(host, "music.")

	switch host {
	case "youtu.be":
		if id := strings.Trim(u.Path, "/"); id != "" {
			return id
		}

	case "youtube.com", "youtube-nocookie.com":
		if id := u.Query().Get("v"); id != "" {
			return id
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) == 2 {
			switch parts[0] {
			case "shorts", "embed", "live", "v":
				return parts[1]
			}
		}
	}

	return rawURL
}
