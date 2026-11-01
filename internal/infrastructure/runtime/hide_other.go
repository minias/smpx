// internal/infrastructure/runtime/hide_other.go
//go:build !windows

package runtime

import "os/exec"

// HideConsole windows 외 OS 에서는 no-op
func HideConsole(cmd *exec.Cmd) {}
