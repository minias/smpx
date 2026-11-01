// internal/infrastructure/runtime/ipc.go
package runtime

import (
	"os"
	"path/filepath"
	runtime2 "runtime"

	"github.com/google/uuid"
)

// macOS unix socket 경로 길이 제한(104 bytes) 여유값
const maxSocketPathLen = 100

// CreateIPCPath mpv IPC 경로 생성
func CreateIPCPath(name string) string {
	id := uuid.NewString()

	switch runtime2.GOOS {

	case "windows":
		// Windows Named Pipe
		return `\\.\pipe\` + name + "-" + id

	default:
		// macOS / Linux Unix Socket
		filename := name + "-" + id + ".sock"
		p := filepath.Join(os.TempDir(), filename)
		if len(p) > maxSocketPathLen {
			// 경로가 길면 /tmp 사용
			p = filepath.Join("/tmp", filename)
		}
		return p
	}
}
