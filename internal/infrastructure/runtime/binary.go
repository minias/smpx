// internal/infrastructure/runtime/binary.go
package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	runtime2 "runtime"
	"strings"
)

// ResolveBinary 실행 파일(mpv, yt-dlp) 경로 탐색
func ResolveBinary(name string) (string, error) {
	goos := runtime2.GOOS
	if goos == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		name += ".exe"
	}

	var candidates []string

	// 실행 파일 기준 경로
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		exeDir := filepath.Dir(exe)

		candidates = append(candidates, filepath.Join(exeDir, name))

		// macOS .app 번들 내부
		if goos == "darwin" {
			candidates = append(candidates,
				filepath.Join(exeDir, "..", "Resources", "bin", name))
		}

		candidates = append(candidates,
			filepath.Join(exeDir, "resources", "bin", goos, name))
	}

	// wails dev 는 프로젝트 루트에서 실행
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, "resources", "bin", goos, name))
	}

	for _, p := range candidates {
		if isRegularFile(p) {
			return finalize(p), nil
		}
	}

	// PATH 탐색
	if p, err := exec.LookPath(name); err == nil {
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		return finalize(p), nil
	}
	candidates = append(candidates, "$PATH")

	// Finder 실행 시 PATH 가 최소화되므로 대표 경로 직접 확인
	var extra []string
	switch goos {
	case "darwin":
		extra = []string{"/opt/homebrew/bin", "/usr/local/bin"}
	case "linux":
		extra = []string{"/usr/bin", "/usr/local/bin"}
	}
	for _, dir := range extra {
		p := filepath.Join(dir, name)
		candidates = append(candidates, p)
		if isRegularFile(p) {
			return finalize(p), nil
		}
	}

	return "", fmt.Errorf("binary %q not found; searched: %s",
		name, strings.Join(candidates, ", "))
}

// isRegularFile 일반 파일 여부
func isRegularFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular()
}

// finalize 실행 권한 보정 (windows 제외)
func finalize(p string) string {
	p = filepath.Clean(p)
	if runtime2.GOOS == "windows" {
		return p
	}
	if info, err := os.Stat(p); err == nil && info.Mode().Perm()&0o111 == 0 {
		_ = os.Chmod(p, 0o755)
	}
	return p
}
