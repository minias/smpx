// internal/application/service/player_service.go
package service

import (
	"github.com/minias/smpx/internal/infrastructure/player"
	"github.com/minias/smpx/internal/infrastructure/ytdlp"
)

type PlayerService struct {
	youtubeClient *ytdlp.Client
	player        *player.Player
}

func NewPlayerService() *PlayerService {
	return &PlayerService{
		youtubeClient: ytdlp.NewClient(),
		player:        player.NewPlayer(),
	}
}

func (s *PlayerService) PlayYoutube(url string) error {
	streamURLs, err := s.youtubeClient.ExtractStreamURLs(url)
	if err != nil {
		return err
	}

	return s.player.Play(streamURLs)
}

func (s *PlayerService) Stop() error {
	return s.player.Stop()
}

func (s *PlayerService) Pause() error {
	return s.player.Pause()
}

func (s *PlayerService) Resume() error {
	return s.player.Resume()
}

func (s *PlayerService) SetVolume(volume int) error {
	return s.player.SetVolume(volume)
}
