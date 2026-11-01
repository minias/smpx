// internal/application/service/player_service_test.go
package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/minias/smpx/internal/domain"
)

type fakeResolver struct {
	block chan struct{} // nil 이 아니면 StreamURL 이 대기
	err   error
}

func (f *fakeResolver) Title(_ context.Context, url string) (string, error) {
	return "title:" + url, nil
}

func (f *fakeResolver) StreamURL(_ context.Context, url string) (string, error) {
	if f.block != nil {
		<-f.block
	}
	if f.err != nil {
		return "", f.err
	}
	return "stream:" + url, nil
}

type fakePlayer struct {
	mu      sync.Mutex
	played  []string
	stops   int
	paused  bool
	volume  int
	onEnded func(error)
}

func (f *fakePlayer) Play(streamURL string, volume int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.played = append(f.played, streamURL)
	f.volume = volume
	f.paused = false
	return nil
}

func (f *fakePlayer) Stop() error {
	f.mu.Lock()
	f.stops++
	f.mu.Unlock()
	return nil
}

func (f *fakePlayer) Pause() error  { f.paused = true; return nil }
func (f *fakePlayer) Resume() error { f.paused = false; return nil }

func (f *fakePlayer) SetVolume(volume int) error {
	f.volume = volume
	return nil
}

func (f *fakePlayer) SetHandlers(onEnded func(error), _ func(string)) {
	f.onEnded = onEnded
}

func (f *fakePlayer) last() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.played) == 0 {
		return ""
	}
	return f.played[len(f.played)-1]
}

type memRepo struct{ items []domain.PlaylistItem }

func (r *memRepo) Load() ([]domain.PlaylistItem, error) { return r.items, nil }

func (r *memRepo) Save(items []domain.PlaylistItem) error {
	r.items = items
	return nil
}

func newTestService(t *testing.T, urls ...string) (*PlayerService, *fakePlayer, *fakeResolver, *memRepo) {
	t.Helper()

	repo := &memRepo{}
	for _, u := range urls {
		repo.items = append(repo.items, domain.PlaylistItem{Title: u, URL: u})
	}

	yt := &fakeResolver{}
	audio := &fakePlayer{}
	s := NewPlayerService(yt, audio, repo)

	if err := s.LoadPlaylist(); err != nil {
		t.Fatal(err)
	}

	return s, audio, yt, repo
}

func TestAutoAdvanceByLoopMode(t *testing.T) {
	cases := []struct {
		mode      domain.LoopMode
		start     int
		wantURL   string
		wantState domain.PlaybackState
	}{
		{domain.LoopNone, 0, "stream:b", domain.Playing},
		{domain.LoopNone, 2, "stream:c", domain.Stopped}, // 마지막 곡 후 정지
		{domain.LoopAll, 2, "stream:a", domain.Playing},  // 처음으로
		{domain.LoopSingle, 1, "stream:b", domain.Playing},
	}

	for _, tc := range cases {
		t.Run(string(tc.mode), func(t *testing.T) {
			s, audio, _, _ := newTestService(t, "a", "b", "c")
			_ = s.SetLoopMode(tc.mode)

			if err := s.Play(tc.start); err != nil {
				t.Fatal(err)
			}

			audio.onEnded(nil)

			if got := audio.last(); got != tc.wantURL {
				t.Errorf("last played = %q, want %q", got, tc.wantURL)
			}
			if got := s.State().State; got != tc.wantState {
				t.Errorf("state = %q, want %q", got, tc.wantState)
			}
		})
	}
}

func TestEndedWithErrorStopsAndReports(t *testing.T) {
	s, audio, _, _ := newTestService(t, "a", "b")

	var reported error
	s.SetListener(Listener{OnError: func(err error) { reported = err }})

	_ = s.Play(0)
	audio.onEnded(errors.New("exit status 2"))

	if s.State().State != domain.Stopped {
		t.Errorf("state = %q, want stopped", s.State().State)
	}
	if reported == nil {
		t.Error("error not reported")
	}
	if len(audio.played) != 1 {
		t.Errorf("played %d times, want 1 (no auto-advance on error)", len(audio.played))
	}
}

func TestEndedAfterStopIsIgnored(t *testing.T) {
	s, audio, _, _ := newTestService(t, "a", "b")

	_ = s.Play(0)
	_ = s.Stop()
	audio.onEnded(nil)

	if len(audio.played) != 1 {
		t.Errorf("played %d times, want 1", len(audio.played))
	}
}

func TestStopDuringLoadingCancelsPlay(t *testing.T) {
	s, audio, yt, _ := newTestService(t, "a")
	yt.block = make(chan struct{})

	done := make(chan error)
	go func() { done <- s.Play(0) }()

	// Loading 상태가 될 때까지 대기
	for s.State().State != domain.Loading {
	}

	_ = s.Stop()
	close(yt.block)

	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if len(audio.played) != 0 {
		t.Errorf("player started after Stop: %v", audio.played)
	}
	if s.State().State != domain.Stopped {
		t.Errorf("state = %q, want stopped", s.State().State)
	}
}

func TestPlayPauseToggle(t *testing.T) {
	s, audio, _, _ := newTestService(t, "a")

	_ = s.PlayPause() // stopped → play
	if s.State().State != domain.Playing || len(audio.played) != 1 {
		t.Fatalf("want playing, got %q", s.State().State)
	}

	_ = s.PlayPause()
	if s.State().State != domain.Paused || !audio.paused {
		t.Fatalf("want paused, got %q", s.State().State)
	}

	_ = s.PlayPause()
	if s.State().State != domain.Playing || audio.paused || len(audio.played) != 1 {
		t.Fatalf("want resumed without replay, got %q", s.State().State)
	}
}

func TestNextPreviousWrap(t *testing.T) {
	s, audio, _, _ := newTestService(t, "a", "b", "c")

	_ = s.Play(2)
	_ = s.Next()
	if audio.last() != "stream:a" {
		t.Errorf("next from last = %q", audio.last())
	}

	_ = s.Previous()
	if audio.last() != "stream:c" {
		t.Errorf("previous from first = %q", audio.last())
	}
}

func TestAddTrackDedupAndSave(t *testing.T) {
	s, _, _, repo := newTestService(t)

	if err := s.AddTrack("  u1 "); err != nil {
		t.Fatal(err)
	}
	if err := s.AddTrack("u1"); !errors.Is(err, ErrDuplicateURL) {
		t.Errorf("duplicate err = %v", err)
	}
	if len(repo.items) != 1 || repo.items[0].Title != "title:u1" {
		t.Errorf("saved = %+v", repo.items)
	}
}

func TestRemoveTrackAdjustsIndex(t *testing.T) {
	s, audio, _, repo := newTestService(t, "a", "b", "c")

	_ = s.Play(2)
	_ = s.RemoveTrack(0)

	st := s.State()
	if st.CurrentIndex != 1 || st.State != domain.Playing {
		t.Errorf("after removing earlier track: index=%d state=%q", st.CurrentIndex, st.State)
	}

	stops := audio.stops
	_ = s.RemoveTrack(1) // 재생 중인 곡 삭제

	st = s.State()
	if st.CurrentIndex != -1 || st.State != domain.Stopped || audio.stops != stops+1 {
		t.Errorf("after removing current: index=%d state=%q", st.CurrentIndex, st.State)
	}
	if len(repo.items) != 1 || repo.items[0].URL != "b" {
		t.Errorf("saved = %+v", repo.items)
	}
}

func TestVolumeClampedAndKeptForNextPlay(t *testing.T) {
	s, audio, _, _ := newTestService(t, "a")

	_ = s.SetVolume(150)
	_ = s.Play(0)

	if audio.volume != 100 || s.State().Volume != 100 {
		t.Errorf("volume = %d", audio.volume)
	}
}
