// internal/infrastructure/runtime/temp.go
package runtime

import (
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

func CreateTempSocketPath(name string) string {
	filename := name + "-" + uuid.NewString()

	return filepath.Join(
		os.TempDir(),
		filename+".sock",
	)
}
