// internal/infrastructure/runtime/binary_test.go
package runtime

import (
	"os"
	"path/filepath"
	runtime2 "runtime"
	"testing"
)

// cwd 기준 resources/bin/<GOOS> 탐색 확인
func TestResolveBinaryCwdFallback(t *testing.T) {
	dir := t.TempDir()
	name := "smpx-test-bin"
	file := name
	if runtime2.GOOS == "windows" {
		file += ".exe"
	}

	binDir := filepath.Join(dir, "resources", "bin", runtime2.GOOS)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(binDir, file)
	if err := os.WriteFile(want, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(dir)

	got, err := ResolveBinary(name)
	if err != nil {
		t.Fatal(err)
	}
	gotEval, _ := filepath.EvalSymlinks(got)
	wantEval, _ := filepath.EvalSymlinks(want)
	if gotEval != wantEval {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// 존재하지 않는 바이너리는 에러
func TestResolveBinaryNotFound(t *testing.T) {
	if _, err := ResolveBinary("smpx-definitely-missing-bin"); err == nil {
		t.Fatal("expected error")
	}
}
