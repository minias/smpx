// internal/infrastructure/ytdlp/client.go
package ytdlp

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	runtime2 "github.com/minias/smpx/internal/infrastructure/runtime"
)

type Client struct {
	binaryPath string
}

func NewClient() *Client {
	binaryPath := runtime2.ResolveBinary("yt-dlp")

	_ = runtime2.EnsureExecutable(binaryPath)

	return &Client{
		binaryPath: binaryPath,
	}
}

func (c *Client) ExtractStreamURLs(videoURL string) ([]string, error) {
	cmd := exec.Command(
		c.binaryPath,
		"-f",
		"bv*+ba/b",
		"-g",
		videoURL,
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf(
			"yt-dlp failed: %v: %s",
			err,
			stderr.String(),
		)
	}

	output := strings.TrimSpace(stdout.String())

	if output == "" {
		return nil, fmt.Errorf("empty stream urls")
	}

	streamURLs := strings.Split(output, "\n")

	return streamURLs, nil
}
