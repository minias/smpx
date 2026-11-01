// internal/infrastructure/player/dial_unix.go
//go:build !windows

package player

import (
	"io"
	"net"
)

// dialIPC unix domain socket 연결
func dialIPC(path string) (io.ReadWriteCloser, error) {
	return net.Dial("unix", path)
}
