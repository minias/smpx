// internal/infrastructure/ytdlp/manager.go
package ytdlp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	// GitHub 최신 릴리스 다운로드 경로
	releaseBaseURL = "https://github.com/yt-dlp/yt-dlp/releases/latest/download"

	// 자동 업데이트 주기
	updateInterval = 24 * time.Hour

	// 재생 실패 시 강제 업데이트 최소 간격 (연속 실패 시 매번 업데이트 방지)
	forceUpdateInterval = time.Hour

	stampFile = ".last-update"
)

// Manager 사용자 폴더에 yt-dlp 를 설치하고 최신으로 유지
//   - 최초 실행: GitHub 릴리스에서 OS 용 단일 실행파일 다운로드 + SHA-256 검증
//   - 이후: 하루 한 번 `yt-dlp -U` (yt-dlp 자체 서명 검증 포함)
//   - 다운로드 불가 시 PATH / Homebrew 의 yt-dlp 사용
type Manager struct {
	dir     string
	baseURL string
	http    *http.Client
	now     func() time.Time

	mu         sync.Mutex
	lastForced time.Time
}

// NewManager UserConfigDir/smpx/bin 에 yt-dlp 관리
func NewManager() (*Manager, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}

	return newManager(filepath.Join(base, "smpx", "bin"), releaseBaseURL), nil
}

func newManager(dir, baseURL string) *Manager {
	return &Manager{
		dir:     dir,
		baseURL: baseURL,
		http:    &http.Client{Timeout: 5 * time.Minute},
		now:     time.Now,
	}
}

// Path 관리 중인 yt-dlp 실행파일 경로
func (m *Manager) Path() string {
	return filepath.Join(m.dir, "yt-dlp")
}

// Resolve 사용할 yt-dlp 경로 (없으면 설치 시도, 실패 시 시스템 yt-dlp)
func (m *Manager) Resolve(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if isExecutable(m.Path()) {
		return m.Path(), nil
	}

	installErr := m.installLocked(ctx)
	if installErr == nil {
		return m.Path(), nil
	}

	if p := systemBinary(); p != "" {
		return p, nil
	}

	return "", fmt.Errorf("yt-dlp 설치 실패: %w", installErr)
}

// Prepare 앱 시작 시 호출: 설치 확인 + 주기적 업데이트
func (m *Manager) Prepare(ctx context.Context) error {
	if _, err := m.Resolve(ctx); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if !isExecutable(m.Path()) || m.now().Sub(m.lastUpdate()) < updateInterval {
		return nil
	}

	return m.updateLocked(ctx)
}

// ForceUpdate 재생 실패 시 호출, 실제로 업데이트를 시도했으면 true
func (m *Manager) ForceUpdate(ctx context.Context) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !isExecutable(m.Path()) || m.now().Sub(m.lastForced) < forceUpdateInterval {
		return false
	}

	m.lastForced = m.now()

	return m.updateLocked(ctx) == nil
}

// installLocked 릴리스 바이너리 다운로드 → 체크섬 검증 → 원자적 교체
func (m *Manager) installLocked(ctx context.Context) error {
	asset, err := assetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	sums, err := m.fetch(ctx, m.baseURL+"/SHA2-256SUMS", 1<<20)
	if err != nil {
		return err
	}

	want, err := checksumFor(sums, asset)
	if err != nil {
		return err
	}

	data, err := m.fetch(ctx, m.baseURL+"/"+asset, 200<<20)
	if err != nil {
		return err
	}

	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("yt-dlp 체크섬 불일치: %s", asset)
	}

	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(m.dir, ".yt-dlp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), m.Path()); err != nil {
		return err
	}

	m.touchLocked()

	return nil
}

// updateLocked yt-dlp 자체 업데이트
func (m *Manager) updateLocked(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, m.Path(), "-U")
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return formatError(err, stderr.Bytes())
	}

	m.touchLocked()

	return nil
}

func (m *Manager) fetch(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "smpx")

	resp, err := m.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("다운로드 실패 %s: %s", url, resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("다운로드 크기 초과: %s", url)
	}

	return data, nil
}

func (m *Manager) lastUpdate() time.Time {
	info, err := os.Stat(filepath.Join(m.dir, stampFile))
	if err != nil {
		return time.Time{}
	}

	return info.ModTime()
}

func (m *Manager) touchLocked() {
	p := filepath.Join(m.dir, stampFile)
	now := m.now()

	if err := os.WriteFile(p, nil, 0o644); err == nil {
		_ = os.Chtimes(p, now, now)
	}
}

// assetName OS / 아키텍처별 yt-dlp 단일 실행파일 이름
func assetName(goos, goarch string) (string, error) {
	switch {
	case goos == "darwin":
		return "yt-dlp_macos", nil // universal
	case goos == "linux" && goarch == "amd64":
		return "yt-dlp_linux", nil
	case goos == "linux" && goarch == "arm64":
		return "yt-dlp_linux_aarch64", nil
	default:
		return "", fmt.Errorf("지원하지 않는 플랫폼: %s/%s", goos, goarch)
	}
}

// checksumFor SHA2-256SUMS 에서 파일 해시 찾기 ("<hash>  <name>")
func checksumFor(sums []byte, name string) (string, error) {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			return strings.ToLower(fields[0]), nil
		}
	}

	return "", errors.New("체크섬 목록에 없음: " + name)
}

// systemBinary PATH 또는 Homebrew 의 yt-dlp (Finder 실행 앱은 PATH 가 짧음)
func systemBinary() string {
	if p, err := exec.LookPath("yt-dlp"); err == nil {
		return p
	}

	for _, dir := range []string{"/opt/homebrew/bin", "/usr/local/bin", "/usr/bin"} {
		if p := filepath.Join(dir, "yt-dlp"); isExecutable(p) {
			return p
		}
	}

	return ""
}

func isExecutable(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0
}
