// internal/infrastructure/player/ipc.go
package player

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	runtime2 "runtime"

	"github.com/google/uuid"
)

type IPCClient struct {
	conn net.Conn
}

func NewIPCClient(socketPath string) (*IPCClient, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, err
	}

	return &IPCClient{
		conn: conn,
	}, nil
}

func (c *IPCClient) Command(command []interface{}) error {
	payload := map[string]interface{}{
		"command": command,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	data = append(data, '\n')

	_, err = c.conn.Write(data)

	return err
}

func (c *IPCClient) Read() (map[string]interface{}, error) {
	reader := bufio.NewReader(c.conn)

	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	var response map[string]interface{}

	if err := json.Unmarshal(line, &response); err != nil {
		return nil, err
	}

	return response, nil
}

func (c *IPCClient) Close() error {
	if c.conn == nil {
		return nil
	}

	return c.conn.Close()
}

func CreateIPCPath(name string) string {
	id := uuid.NewString()

	switch runtime2.GOOS {

	case "windows":
		return `\\\\.\\pipe\\` + name + "-" + id

	default:
		return filepath.Join(
			os.TempDir(),
			name+"-"+id+".sock",
		)
	}
}
