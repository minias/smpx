// internal/infrastructure/ytdlp/manager_test.go
package ytdlp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func releaseServer(t *testing.T, body []byte, sum string) *httptest.Server {
	t.Helper()

	asset, err := assetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skip(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/SHA2-256SUMS":
			w.Write([]byte("deadbeef  yt-dlp.exe\n" + sum + "  " + asset + "\n"))
		case "/" + asset:
			w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

func TestInstallVerifiesChecksum(t *testing.T) {
	body := []byte("#!/bin/sh\necho ok\n")
	h := sha256.Sum256(body)
	srv := releaseServer(t, body, hex.EncodeToString(h[:]))

	m := newManager(filepath.Join(t.TempDir(), "bin"), srv.URL)

	p, err := m.Resolve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if p != m.Path() || !isExecutable(p) {
		t.Fatalf("path = %q, executable = %v", p, isExecutable(p))
	}
	if got, _ := os.ReadFile(p); string(got) != string(body) {
		t.Fatalf("content = %q", got)
	}
	if m.lastUpdate().IsZero() {
		t.Error("update stamp not written")
	}
}

func TestInstallRejectsChecksumMismatch(t *testing.T) {
	t.Setenv("PATH", "") // 시스템 yt-dlp 대체 경로 차단

	srv := releaseServer(t, []byte("tampered"), strings.Repeat("0", 64))
	m := newManager(filepath.Join(t.TempDir(), "bin"), srv.URL)

	err := m.installLocked(context.Background())
	if err == nil || !strings.Contains(err.Error(), "체크섬") {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(m.Path()); !os.IsNotExist(statErr) {
		t.Error("binary written despite mismatch")
	}
}

func TestChecksumFor(t *testing.T) {
	sums := []byte("AAA  yt-dlp\nBBB *yt-dlp_macos\n")

	if got, err := checksumFor(sums, "yt-dlp_macos"); err != nil || got != "bbb" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := checksumFor(sums, "yt-dlp_linux"); err == nil {
		t.Fatal("missing entry should fail")
	}
}

func TestAssetName(t *testing.T) {
	cases := map[string]string{
		"darwin/arm64": "yt-dlp_macos",
		"darwin/amd64": "yt-dlp_macos",
		"linux/amd64":  "yt-dlp_linux",
		"linux/arm64":  "yt-dlp_linux_aarch64",
	}
	for platform, want := range cases {
		goos, goarch, _ := strings.Cut(platform, "/")
		if got, err := assetName(goos, goarch); err != nil || got != want {
			t.Errorf("%s: got %q, %v", platform, got, err)
		}
	}
	if _, err := assetName("windows", "amd64"); err == nil {
		t.Error("windows should be unsupported")
	}
}
