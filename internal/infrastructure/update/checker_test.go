// internal/infrastructure/update/checker_test.go
package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 테스트용 서버와 확인기 생성
func newTestChecker(t *testing.T, current string, status int, body string) *Checker {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/minias/smpx/releases/latest" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("User-Agent") != "smpx" {
			t.Errorf("unexpected user-agent %q", r.Header.Get("User-Agent"))
		}
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("unexpected accept %q", r.Header.Get("Accept"))
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	c := NewChecker("minias/smpx", current)
	c.baseURL = srv.URL
	return c
}

const relURL = "https://github.com/minias/smpx/releases/tag/v1.2.0"

func TestCheckNewer(t *testing.T) {
	c := newTestChecker(t, "1.1.9", 200, `{"tag_name":"v1.2.0","html_url":"`+relURL+`"}`)
	info, err := c.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info == nil || info.Version != "1.2.0" || info.URL != relURL {
		t.Fatalf("unexpected info %#v", info)
	}
}

func TestCheckSame(t *testing.T) {
	c := newTestChecker(t, "v1.2.0", 200, `{"tag_name":"v1.2.0","html_url":"`+relURL+`"}`)
	info, err := c.Check(context.Background())
	if err != nil || info != nil {
		t.Fatalf("want nil,nil got %#v,%v", info, err)
	}
}

func TestCheckOlder(t *testing.T) {
	c := newTestChecker(t, "1.10.0", 200, `{"tag_name":"v1.9.9","html_url":"`+relURL+`"}`)
	info, err := c.Check(context.Background())
	if err != nil || info != nil {
		t.Fatalf("want nil,nil got %#v,%v", info, err)
	}
}

func TestCheckNotFound(t *testing.T) {
	c := newTestChecker(t, "1.0.0", 404, `{"message":"Not Found"}`)
	info, err := c.Check(context.Background())
	if err != nil || info != nil {
		t.Fatalf("want nil,nil got %#v,%v", info, err)
	}
}

func TestCheckMalformedTag(t *testing.T) {
	c := newTestChecker(t, "1.0.0", 200, `{"tag_name":"latest","html_url":"`+relURL+`"}`)
	info, err := c.Check(context.Background())
	if err != nil || info != nil {
		t.Fatalf("want nil,nil got %#v,%v", info, err)
	}
}

func TestCheckMalformedJSON(t *testing.T) {
	c := newTestChecker(t, "1.0.0", 200, `{not json`)
	if _, err := c.Check(context.Background()); err == nil {
		t.Fatal("want error for malformed json")
	}
}

func TestCheckServerError(t *testing.T) {
	c := newTestChecker(t, "1.0.0", 500, ``)
	if _, err := c.Check(context.Background()); err == nil {
		t.Fatal("want error for 500")
	}
}

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"1.0.1", "1.0.0", true},
		{"2.0.0", "1.99.99", true},
		{"1.0.0-beta", "0.9.0", true},
		{"1.0.0", "1.0.0-rc1", false},
		{"1.0", "0.9.0", false},
		{"1.0.0", "dev", false},
		{"x.y.z", "1.0.0", false},
	}
	for _, tc := range cases {
		if got := isNewer(tc.latest, tc.current); got != tc.want {
			t.Errorf("isNewer(%q,%q)=%v want %v", tc.latest, tc.current, got, tc.want)
		}
	}
}
