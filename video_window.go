// video_window.go
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

// videoWindowEnv 이 환경변수가 있으면 영상 창 모드로 실행
//   - Wails v2 는 창을 하나만 지원하므로 같은 실행파일을 자식 프로세스로 띄움
//   - 개발 모드 플래그(wails dev 가 넘기는 인자)를 그대로 전달하기 위해 인자 대신 환경변수 사용
const videoWindowEnv = "SMPX_VIDEO_WINDOW"

// videoEvent 영상 창 → 메인 (fd 3 으로 JSON 한 줄씩)
type videoEvent struct {
	Event   string  `json:"event"` // ended | failed | paused | progress | command | closed
	Seq     uint64  `json:"seq"`
	Message string  `json:"message,omitempty"`
	Paused  bool    `json:"paused,omitempty"`
	Time    float64 `json:"time,omitempty"`
	Action  string  `json:"action,omitempty"` // command: playpause | next | prev | stop
}

// videoWindow 메인 프로세스에서 영상 창 자식 프로세스 관리
//   - 메인 → 자식: stdin (audioCommand JSON 한 줄씩)
//   - 자식 → 메인: fd 3 (videoEvent JSON 한 줄씩) — Wails 로그가 stdout 을 쓰므로 분리
type videoWindow struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	onEvent func(videoEvent)
}

func newVideoWindow(onEvent func(videoEvent)) *videoWindow {
	return &videoWindow{onEvent: onEvent}
}

// send 영상 창이 없으면 띄운 뒤 명령 전달
func (v *videoWindow) send(cmd audioCommand) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.cmd == nil {
		if err := v.startLocked(); err != nil {
			return err
		}
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	_, err = v.stdin.Write(append(data, '\n'))

	return err
}

// running 영상 창이 떠 있는지
func (v *videoWindow) running() bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	return v.cmd != nil
}

// close 영상 창 종료 (stdin 을 닫으면 자식이 스스로 종료)
func (v *videoWindow) close() {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.closeLocked()
}

func (v *videoWindow) closeLocked() {
	if v.cmd == nil {
		return
	}

	_ = v.stdin.Close()
	v.cmd = nil
	v.stdin = nil
}

func (v *videoWindow) startLocked() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	eventsR, eventsW, err := os.Pipe()
	if err != nil {
		return err
	}

	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Env = append(os.Environ(), videoWindowEnv+"=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.ExtraFiles = []*os.File{eventsW} // 자식의 fd 3

	stdin, err := cmd.StdinPipe()
	if err != nil {
		eventsR.Close()
		eventsW.Close()
		return err
	}

	if err := cmd.Start(); err != nil {
		eventsR.Close()
		eventsW.Close()
		return fmt.Errorf("영상 창 실행 실패: %w", err)
	}

	eventsW.Close()

	v.cmd = cmd
	v.stdin = stdin

	go v.readEvents(cmd, eventsR)

	return nil
}

func (v *videoWindow) readEvents(cmd *exec.Cmd, r *os.File) {
	defer r.Close()

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		var ev videoEvent
		if json.Unmarshal(sc.Bytes(), &ev) == nil {
			v.onEvent(ev)
		}
	}

	_ = cmd.Wait()

	// 사용자가 창을 닫은 경우 (메인이 close() 한 경우는 이미 cmd 가 비어 있음)
	v.mu.Lock()
	closedByUser := v.cmd == cmd
	if closedByUser {
		v.closeLocked()
	}
	v.mu.Unlock()

	if closedByUser {
		v.onEvent(videoEvent{Event: "closed"})
	}
}

// ---- 자식(영상 창) 프로세스 쪽 ----

// videoChannel 자식 프로세스에서 메인과 통신
type videoChannel struct {
	mu  sync.Mutex
	out *os.File
}

func openVideoChannel() (*videoChannel, error) {
	out := os.NewFile(3, "smpx-events")
	if out == nil {
		return nil, errors.New("영상 창 이벤트 채널이 없습니다")
	}

	return &videoChannel{out: out}, nil
}

func (c *videoChannel) emit(ev videoEvent) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	_, err = c.out.Write(append(data, '\n'))

	return err
}

// commands 메인에서 오는 명령 (stdin 이 닫히면 종료)
func (c *videoChannel) commands(handle func(audioCommand)) {
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 64*1024), 1<<20)

	for sc.Scan() {
		var cmd audioCommand
		if json.Unmarshal(sc.Bytes(), &cmd) == nil {
			handle(cmd)
		}
	}
}
