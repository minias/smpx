// internal/infrastructure/runtime/hide_windows.go
//go:build windows

package runtime

import (
	"os/exec"
	"syscall"
)

// HideConsole 자식 프로세스 콘솔 창 숨김
func HideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
