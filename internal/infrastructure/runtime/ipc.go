// internal/infrastructure/runtime/ipc.go
package runtime

import (
	"os"
	"path/filepath"
	runtime2 "runtime"

	"github.com/google/uuid"
)

func CreateIPCPath(name string) string {
	id := uuid.NewString()

	switch runtime2.GOOS {

	case "windows":
		// Windows Named Pipe
		return `\\.\pipe\` + name + "-" + id

	default:
		// macOS / Linux Unix Socket
		return filepath.Join(
			os.TempDir(),
			name+"-"+id+".sock",
		)
	}
}
