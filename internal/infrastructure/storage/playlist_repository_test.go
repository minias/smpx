// internal/infrastructure/storage/playlist_repository_test.go
package storage

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/minias/smpx/internal/domain"
)

func TestLoadMissingFile(t *testing.T) {
	r := NewPlaylistRepositoryAt(filepath.Join(t.TempDir(), "playlist.json"))
	items, err := r.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items == nil || len(items) != 0 {
		t.Fatalf("want empty non-nil slice, got %#v", items)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	p := filepath.Join(t.TempDir(), "playlist.json")
	if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPlaylistRepositoryAt(p).Load(); err == nil {
		t.Fatal("want error for invalid json")
	}
}

func TestLoadSkipsEmptyURL(t *testing.T) {
	p := filepath.Join(t.TempDir(), "playlist.json")
	src := `[{"title":"a","url":"https://youtu.be/a"},{"title":"b","url":""},{"title":"c"}]`
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	items, err := NewPlaylistRepositoryAt(p).Load()
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.PlaylistItem{{Title: "a", URL: "https://youtu.be/a"}}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("got %#v want %#v", items, want)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "playlist.json")
	r := NewPlaylistRepositoryAt(p)
	want := []domain.PlaylistItem{
		{Title: "첫번째", URL: "https://youtu.be/1"},
		{Title: "second", URL: "https://youtu.be/2"},
	}
	if err := r.Save(want); err != nil {
		t.Fatal(err)
	}
	// 덮어쓰기 확인
	if err := r.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := r.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	data, _ := os.ReadFile(p)
	if !strings.Contains(string(data), "\n  {") || !strings.Contains(string(data), `"title"`) {
		t.Fatalf("expected indented json with title/url keys, got %s", data)
	}
	// 임시 파일 잔존 없음
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected only playlist.json, got %d entries", len(entries))
	}
}

func TestSaveNil(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "playlist.json")
	r := NewPlaylistRepositoryAt(p)
	if err := r.Save(nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "[]" {
		t.Fatalf("want [], got %q", data)
	}
}

func TestSaveConcurrent(t *testing.T) {
	r := NewPlaylistRepositoryAt(filepath.Join(t.TempDir(), "playlist.json"))
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := r.Save([]domain.PlaylistItem{{Title: "x", URL: "u"}}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	items, err := r.Load()
	if err != nil || len(items) != 1 {
		t.Fatalf("got %v %v", items, err)
	}
}
