// internal/application/player/interface.go
package player

type AudioPlayer interface {
	Play(urls []string) error
	Stop() error
	Pause() error
	Resume() error
}
