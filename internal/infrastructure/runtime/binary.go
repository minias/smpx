// internal/infrastructure/runtime/binary.go
package runtime

import (
	"os"
	"path/filepath"
	runtime2 "runtime"
)

func ResolveBinary(name string) string {
	basePath := "resources/bin"

	switch runtime2.GOOS {
	case "darwin":
		return filepath.Join(basePath, "darwin", name)

	case "windows":
		return filepath.Join(basePath, "windows", name+".exe")

	default:
		return filepath.Join(basePath, name)
	}
}

func EnsureExecutable(path string) error {
	return os.Chmod(path, 0755)
}
