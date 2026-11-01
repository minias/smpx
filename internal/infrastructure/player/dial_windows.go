// internal/infrastructure/player/dial_windows.go
//go:build windows

package player

import (
	"io"
	"os"
	"syscall"
)

// dialIPC named pipe 연결
// FILE_FLAG_OVERLAPPED 필수: 동기 핸들은 대기 중인 read 가 write 를 막아 교착됨
// (Go 1.25+ 에서 overlapped 핸들의 os.File 은 poller 에 등록됨)
func dialIPC(path string) (io.ReadWriteCloser, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	h, err := syscall.CreateFile(
		p,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_FLAG_OVERLAPPED,
		0,
	)
	if err != nil {
		return nil, err
	}

	return os.NewFile(uintptr(h), path), nil
}
