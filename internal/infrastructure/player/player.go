// internal/infrastructure/player/player.go
package player

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	runtime2 "github.com/minias/smpx/internal/infrastructure/runtime"
)

type Player struct {
	cmd        *exec.Cmd
	ipc        *IPCClient
	socketPath string
}

func NewPlayer() *Player {
	return &Player{}
}

func (p *Player) Play(streamURLs []string) error {
	if err := p.Stop(); err != nil {
		return err
	}

	binaryPath := runtime2.ResolveBinary("mpv")

	if err := runtime2.EnsureExecutable(binaryPath); err != nil {
		return err
	}

	// OS별 안전한 IPC 경로 생성
	socketPath := runtime2.CreateIPCPath("smpx-mpv")

	args := []string{
		"--idle=no",
		"--title=SMPX",
		"--input-ipc-server=" + socketPath,
	}

	switch len(streamURLs) {

	case 0:
		return fmt.Errorf("empty stream urls")

	case 1:
		// audio only or progressive stream
		args = append(
			args,
			"--force-window=no",
			streamURLs[0],
		)

	default:
		// adaptive streaming
		videoURL := streamURLs[0]
		audioURL := streamURLs[1]

		args = append(
			args,
			"--force-window=yes",
			"--autofit=1280x720",
			videoURL,
			"--audio-file="+audioURL,
		)
	}

	p.cmd = exec.Command(binaryPath, args...)

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf(
			"failed to start mpv: %w",
			err,
		)
	}

	p.socketPath = socketPath

	// mpv IPC socket 생성 대기
	for i := 0; i < 20; i++ {
		ipc, err := NewIPCClient(socketPath)
		if err == nil {
			p.ipc = ipc
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

func (p *Player) Stop() error {
	if p.ipc != nil {
		_ = p.ipc.Close()
		p.ipc = nil
	}

	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	if err := p.cmd.Process.Kill(); err != nil {
		return err
	}

	_, _ = p.cmd.Process.Wait()

	p.cmd = nil

	// unix socket cleanup
	if p.socketPath != "" {
		_ = os.Remove(p.socketPath)
		p.socketPath = ""
	}

	return nil
}

func (p *Player) Pause() error {
	if p.ipc == nil {
		return fmt.Errorf("ipc not connected")
	}

	return p.ipc.Command([]interface{}{
		"set_property",
		"pause",
		true,
	})
}

func (p *Player) Resume() error {
	if p.ipc == nil {
		return fmt.Errorf("ipc not connected")
	}

	return p.ipc.Command([]interface{}{
		"set_property",
		"pause",
		false,
	})
}

func (p *Player) SetVolume(volume int) error {
	if p.ipc == nil {
		return fmt.Errorf("ipc not connected")
	}

	if volume < 0 {
		volume = 0
	}

	if volume > 100 {
		volume = 100
	}

	return p.ipc.Command([]interface{}{
		"set_property",
		"volume",
		volume,
	})
}
