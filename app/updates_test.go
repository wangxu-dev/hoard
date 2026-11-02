package app

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"v0.2.0", "0.1.0", true},
		{"v0.1.0", "0.1.0", false},
		{"v0.1.0", "0.2.0", false},
		{"v1.0.0", "0.9.9", true},
		{"v0.10.0", "0.9.0", true},
		{"v0.1.0", "0.1.0-rc.1", true},
		{"v0.1.0-rc.2", "0.1.0-rc.1", true},
		{"v0.1.0-rc.10", "0.1.0-rc.9", true},
		{"v0.1.0-rc.1", "0.1.0", false},
		{"v0.1.0-beta", "0.1.0-alpha", true},
		{"v0.1.0-rc.1.1", "0.1.0-rc.1", true},
		{"v0.2.0", "dev", false},
		{"garbage", "0.1.0", false},
	}
	for _, c := range cases {
		if got := newer(c.a, c.b); got != c.want {
			t.Errorf("newer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestUpdatesCheck(t *testing.T) {
	type reply struct {
		status int
		body   string
	}
	release := `{"tag_name":"v0.2.0","name":"Hoard 0.2.0","body":"notes","html_url":"https://github.com/wangxu-dev/hoard/releases/tag/v0.2.0"}`
	cases := map[string]struct {
		reply      reply
		current    string
		wantStatus string
		wantError  string
		wantNotes  string
	}{
		"available":    {reply{200, release}, "0.1.0", "available", "", "notes"},
		"current":      {reply{200, release}, "0.2.0", "current", "", ""},
		"dev build":    {reply{200, release}, "dev", "current", "", ""},
		"no release":   {reply{404, `{}`}, "0.1.0", "unknown", "NOT_FOUND", ""},
		"rate limited": {reply{403, `{}`}, "0.1.0", "unknown", "RATE_LIMITED", ""},
		"bad body":     {reply{200, `{"tag_name":"latest"}`}, "0.1.0", "unknown", "BAD_RESPONSE", ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(c.reply.status)
				_, _ = w.Write([]byte(c.reply.body))
			}))
			defer server.Close()
			u := testUpdates(t, server.URL)
			info, err := u.Check(c.current, true)
			if err != nil || info.Status != c.wantStatus || info.Error != c.wantError || info.Notes != c.wantNotes {
				t.Fatalf("got %+v, %v", info, err)
			}
		})
	}
}

func TestUpdatesCheckOncePerHour(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.0","html_url":"https://example.com/elsewhere"}`))
	}))
	defer server.Close()
	u := testUpdates(t, server.URL)
	now := time.Unix(1_800_000_000, 0)
	u.now = func() time.Time { return now }
	first, _ := u.Check("0.1.0", false)
	if first.NextCheckAt != now.Add(time.Hour).UnixMilli() {
		t.Fatalf("hourly deadline: %+v", first)
	}
	if first.Status != "available" || first.URL != RepoURL+"/releases" {
		t.Fatalf("first check %+v", first)
	}
	now = now.Add(59 * time.Minute)
	cached, _ := u.Check("0.1.0", false)
	if requests.Load() != 1 || cached.Status != "available" {
		t.Fatalf("a check within an hour asked again: %d requests, %+v", requests.Load(), cached)
	}
	now = now.Add(time.Minute)
	if _, _ = u.Check("0.1.0", false); requests.Load() != 2 {
		t.Fatalf("check did not run at the hourly boundary: %d", requests.Load())
	}
	if _, _ = u.Check("0.1.0", true); requests.Load() != 3 {
		t.Fatalf("a manual check did not ask: %d requests", requests.Load())
	}
	now = now.Add(time.Hour)
	if _, _ = u.Check("0.1.0", false); requests.Load() != 4 {
		t.Fatalf("a check an hour later did not ask: %d requests", requests.Load())
	}
}

func TestUpdatesOfflineKeepsLastRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.0","body":"notes","html_url":"https://github.com/wangxu-dev/hoard/releases/tag/v0.2.0"}`))
	}))
	u := testUpdates(t, server.URL)
	if info, _ := u.Check("0.1.0", true); info.Status != "available" {
		t.Fatalf("online check %+v", info)
	}
	server.Close()
	info, _ := u.Check("0.1.0", true)
	if info.Error != "OFFLINE" || info.Status != "available" || info.Latest != "v0.2.0" {
		t.Fatalf("offline check %+v", info)
	}
}

func TestUpdatesOpenRefusesOtherSites(t *testing.T) {
	if err := NewUpdates().Open("https://example.com/"); err == nil {
		t.Fatal("opened a foreign URL")
	}
	if err := NewUpdates().Open(RepoURL + ".evil.example/"); err == nil {
		t.Fatal("opened a look-alike URL")
	}
}

func testUpdates(t *testing.T, url string) *Updates {
	dir := t.TempDir()
	u := NewUpdates()
	u.url = url
	u.fallback = ""
	u.client = &http.Client{Timeout: time.Second}
	u.dir = func() (string, error) { return dir, nil }
	return u
}
