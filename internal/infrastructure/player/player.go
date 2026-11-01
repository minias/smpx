// internal/infrastructure/player/player.go
package player

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	appplayer "github.com/minias/smpx/internal/application/player"
	runtime2 "github.com/minias/smpx/internal/infrastructure/runtime"
)

var _ appplayer.AudioPlayer = (*Player)(nil)

const (
	// IPC 연결 대기 시간
	ipcConnectTimeout = 3 * time.Second

	// stderr 보관 최대 크기
	stderrTailSize = 4 * 1024
)

// 미디어 키 → client-message 매핑
const inputConf = `PLAYPAUSE script-message smpx playpause
PLAY script-message smpx playpause
PAUSE script-message smpx playpause
PLAYONLY script-message smpx playpause
PAUSEONLY script-message smpx playpause
NEXT script-message smpx next
PREV script-message smpx prev
STOP script-message smpx stop
`

var errNotPlaying = errors.New("재생 중이 아닙니다")

// Player mpv 프로세스 기반 오디오 플레이어 (thread-safe)
type Player struct {
	mu sync.Mutex

	cmd        *exec.Cmd
	ipc        *ipcClient
	socketPath string

	onEnded   func(err error)
	onCommand func(command string)
}

func NewPlayer() *Player {
	return &Player{}
}

// SetHandlers 재생 종료 / 미디어 키 콜백 등록
func (p *Player) SetHandlers(onEnded func(err error), onCommand func(command string)) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.onEnded = onEnded
	p.onCommand = onCommand
}

// Play 기존 재생 정지 후 새 스트림 재생
func (p *Player) Play(streamURL string, volume int) error {
	if streamURL == "" {
		return errors.New("스트림 URL 이 비어 있습니다")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.stopLocked()

	binaryPath, err := runtime2.ResolveBinary("mpv")
	if err != nil {
		return fmt.Errorf("mpv 를 찾을 수 없습니다: %w", err)
	}

	socketPath := runtime2.CreateIPCPath("smpx-mpv")

	args := []string{
		"--no-video",
		"--force-window=no",
		"--idle=no",
		"--no-terminal",
		"--no-input-default-bindings",
		"--input-media-keys=yes",
	}

	if confPath, err := ensureInputConf(); err == nil {
		args = append(args, "--input-conf="+confPath)
	}

	args = append(args,
		fmt.Sprintf("--volume=%d", clampVolume(volume)),
		"--input-ipc-server="+socketPath,
		"--",
		streamURL,
	)

	cmd := exec.Command(binaryPath, args...)
	runtime2.HideConsole(cmd)

	stderr := &tailBuffer{max: stderrTailSize}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("mpv 실행 실패: %w", err)
	}

	// 프로세스 종료 감시
	done := make(chan struct{})
	waitErr := make(chan error, 1)

	go func() {
		err := cmd.Wait()
		waitErr <- err
		close(done)
		p.handleExit(cmd, socketPath, err, stderr)
	}()

	ipc, err := connectIPC(socketPath, ipcConnectTimeout, done)
	if err != nil {
		_ = cmd.Process.Kill()

		select {
		case werr := <-waitErr:
			return exitError(werr, stderr)
		default:
		}

		return err
	}

	p.cmd = cmd
	p.ipc = ipc
	p.socketPath = socketPath

	go ipc.readLoop(p.dispatchCommand)

	return nil
}

// Stop 재생 정지 (재생 중이 아니면 nil)
func (p *Player) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stopLocked()

	return nil
}

// stopLocked p.mu 보유 상태에서 호출
// Wait 은 감시 goroutine 이 담당 (중복 Wait 방지)
func (p *Player) stopLocked() {
	cmd := p.cmd
	ipc := p.ipc

	p.cmd = nil
	p.ipc = nil
	p.socketPath = ""

	if ipc != nil {
		_ = ipc.Close()
	}

	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func (p *Player) Pause() error {
	return p.setProperty("pause", true)
}

func (p *Player) Resume() error {
	return p.setProperty("pause", false)
}

// SetVolume 0 ~ 100 (재생 중이 아니면 nil)
func (p *Player) SetVolume(volume int) error {
	p.mu.Lock()
	running := p.ipc != nil
	p.mu.Unlock()

	if !running {
		return nil
	}

	return p.setProperty("volume", clampVolume(volume))
}

func (p *Player) setProperty(name string, value any) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ipc == nil {
		return errNotPlaying
	}

	return p.ipc.command("set_property", name, value)
}

// handleExit 프로세스 종료 처리
func (p *Player) handleExit(cmd *exec.Cmd, socketPath string, waitErr error, stderr *tailBuffer) {
	// 강제 종료 시 남은 unix socket 정리
	if goruntime.GOOS != "windows" {
		_ = os.Remove(socketPath)
	}

	p.mu.Lock()

	// Stop / 새 Play 로 교체된 경우 무시
	if p.cmd != cmd {
		p.mu.Unlock()
		return
	}

	ipc := p.ipc

	p.cmd = nil
	p.ipc = nil
	p.socketPath = ""

	onEnded := p.onEnded

	p.mu.Unlock()

	if ipc != nil {
		_ = ipc.Close()
	}

	if onEnded != nil {
		onEnded(exitError(waitErr, stderr))
	}
}

// dispatchCommand 미디어 키 명령 전달 (콜백이 Play/Stop 재진입 가능하도록 별도 goroutine)
func (p *Player) dispatchCommand(command string) {
	p.mu.Lock()
	onCommand := p.onCommand
	p.mu.Unlock()

	if onCommand == nil {
		return
	}

	go onCommand(command)
}

// exitError 종료 코드 → error (정상 종료 시 nil)
func exitError(waitErr error, stderr *tailBuffer) error {
	if waitErr == nil {
		return nil
	}

	msg := strings.TrimSpace(stderr.String())
	if msg != "" {
		return fmt.Errorf("mpv 비정상 종료: %w: %s", waitErr, msg)
	}

	return fmt.Errorf("mpv 비정상 종료: %w", waitErr)
}

func clampVolume(volume int) int {
	return min(max(volume, 0), 100)
}

var (
	inputConfOnce sync.Once
	inputConfPath string
	inputConfErr  error
)

// ensureInputConf 미디어 키 input.conf 1회 생성
func ensureInputConf() (string, error) {
	inputConfOnce.Do(func() {
		path := filepath.Join(os.TempDir(), "smpx-input.conf")

		if err := os.WriteFile(path, []byte(inputConf), 0o644); err != nil {
			inputConfErr = err
			return
		}

		inputConfPath = path
	})

	return inputConfPath, inputConfErr
}

// tailBuffer 마지막 max 바이트만 보관하는 writer
type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func (t *tailBuffer) Write(b []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.buf = append(t.buf, b...)

	if over := len(t.buf) - t.max; over > 0 {
		t.buf = append(t.buf[:0], t.buf[over:]...)
	}

	return len(b), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return string(t.buf)
}
