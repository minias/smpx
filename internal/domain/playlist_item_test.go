// internal/domain/playlist_item_test.go
package domain

import "testing"

func TestVideoID(t *testing.T) {
	cases := map[string]string{
		"https://www.youtube.com/watch?v=cFw7034Jvn0&list=PLx&index=34": "cFw7034Jvn0",
		"https://youtu.be/jNQXAC9IVRw?si=abc":                           "jNQXAC9IVRw",
		"https://m.youtube.com/watch?v=abc123":                          "abc123",
		"https://music.youtube.com/watch?v=abc123":                      "abc123",
		"https://www.youtube.com/shorts/xyz789":                         "xyz789",
		"https://www.youtube.com/playlist?list=PLx":                     "https://www.youtube.com/playlist?list=PLx",
		"https://example.com/video":                                     "https://example.com/video",
		"not a url":                                                     "not a url",
	}
	for in, want := range cases {
		if got := VideoID(in); got != want {
			t.Errorf("VideoID(%q) = %q, want %q", in, got, want)
		}
	}
}
