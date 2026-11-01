// internal/infrastructure/player/ipc.go
package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

// ipcClient mpv JSON IPC 클라이언트 (unix socket / named pipe)
type ipcClient struct {
	conn io.ReadWriteCloser

	// 쓰기 직렬화
	wmu sync.Mutex

	closeOnce sync.Once
}

// ipcEvent mpv 에서 수신한 이벤트 라인
type ipcEvent struct {
	Event string            `json:"event"`
	Args  []json.RawMessage `json:"args"`
}

// connectIPC IPC 서버가 열릴 때까지 재시도 (timeout 또는 abort 시 실패)
func connectIPC(path string, timeout time.Duration, abort <-chan struct{}) (*ipcClient, error) {
	deadline := time.Now().Add(timeout)

	var lastErr error

	for {
		conn, err := dialIPC(path)
		if err == nil {
			return &ipcClient{conn: conn}, nil
		}

		lastErr = err

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("mpv IPC 연결 실패: %w", lastErr)
		}

		select {
		case <-abort:
			return nil, fmt.Errorf("mpv 가 IPC 연결 전에 종료되었습니다")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// command {"command":[...]}\n 전송
func (c *ipcClient) command(args ...any) error {
	data, err := json.Marshal(map[string]any{
		"command": args,
	})
	if err != nil {
		return err
	}

	data = append(data, '\n')

	c.wmu.Lock()
	defer c.wmu.Unlock()

	_, err = c.conn.Write(data)

	return err
}

// readLoop 연결이 닫힐 때까지 이벤트 수신
// client-message (args[0] == "smpx") 수신 시 onMessage(args[1]) 호출
func (c *ipcClient) readLoop(onMessage func(command string)) {
	scanner := bufio.NewScanner(c.conn)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()

		var ev ipcEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}

		if ev.Event != "client-message" || len(ev.Args) < 2 {
			continue
		}

		var target, cmd string

		if json.Unmarshal(ev.Args[0], &target) != nil || target != "smpx" {
			continue
		}

		if json.Unmarshal(ev.Args[1], &cmd) != nil || cmd == "" {
			continue
		}

		if onMessage != nil {
			onMessage(cmd)
		}
	}
}

// Close 연결 종료 (중복 호출 안전)
func (c *ipcClient) Close() error {
	var err error

	c.closeOnce.Do(func() {
		err = c.conn.Close()
	})

	return err
}
