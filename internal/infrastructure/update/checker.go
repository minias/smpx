// internal/infrastructure/update/checker.go
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// 기본 GitHub API 주소
const defaultBaseURL = "https://api.github.com"

// Info 새 버전 정보
type Info struct {
	Version string `json:"version"`
	URL     string `json:"url"`
}

// Checker GitHub 최신 릴리스 확인기
type Checker struct {
	repo           string
	currentVersion string
	baseURL        string
	client         *http.Client
}

// NewChecker 확인기 생성 (repo 예: "minias/smpx")
func NewChecker(repo, currentVersion string) *Checker {
	return &Checker{
		repo:           repo,
		currentVersion: currentVersion,
		baseURL:        defaultBaseURL,
		client:         &http.Client{Timeout: 10 * time.Second},
	}
}

// 릴리스 응답 중 필요한 필드
type release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// Check 최신 릴리스가 현재 버전보다 새로우면 Info 반환, 아니면 nil
func (c *Checker) Check(ctx context.Context) (*Info, error) {
	url := strings.TrimRight(c.baseURL, "/") + "/repos/" + c.repo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("요청 생성 실패: %w", err)
	}
	req.Header.Set("User-Agent", "smpx")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("릴리스 조회 실패: %w", err)
	}
	defer resp.Body.Close()

	// 릴리스 없음
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("릴리스 조회 실패: HTTP %d", resp.StatusCode)
	}

	var rel release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return nil, fmt.Errorf("릴리스 응답 파싱 실패: %w", err)
	}

	latest := strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	if !isNewer(latest, c.currentVersion) {
		return nil, nil
	}
	return &Info{Version: latest, URL: rel.HTMLURL}, nil
}

// isNewer latest 가 current 보다 새 버전인지 (형식 오류는 false)
func isNewer(latest, current string) bool {
	l, ok := parseVersion(latest)
	if !ok {
		return false
	}
	c, ok := parseVersion(current)
	if !ok {
		return false
	}
	for i := 0; i < 3; i++ {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// parseVersion major.minor.patch 파싱 (pre-release/build 접미사 무시)
func parseVersion(s string) ([3]int, bool) {
	var v [3]int
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		v[i] = n
	}
	return v, true
}
