package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestUpdateFallbackAndRetry(t *testing.T) {
	primaryCalls, backupCalls := 0, 0
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { primaryCalls++; http.Error(w, "offline", 502) }))
	defer primary.Close()
	healthy := false
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backupCalls++
		if !healthy {
			http.Error(w, "offline", 503)
			return
		}
		fmt.Fprint(w, `{"tag_name":"v0.2.0","body":"notes"}`)
	}))
	defer backup.Close()
	u := testUpdates(t, primary.URL)
	u.fallback = backup.URL
	now := time.Unix(1800000000, 0)
	u.now = func() time.Time { return now }
	first, _ := u.Check("0.1.0", false)
	if first.Error != "BAD_RESPONSE" || primaryCalls != 1 || backupCalls != 1 {
		t.Fatalf("fallback %+v %d %d", first, primaryCalls, backupCalls)
	}
	cached, _ := u.Check("0.1.0", false)
	if cached.Error != first.Error || primaryCalls != 1 {
		t.Fatal("cached failure lost")
	}
	healthy = true
	now = now.Add(6 * time.Minute)
	next, _ := u.Check("0.1.0", false)
	if next.Status != "available" || next.Error != "" || backupCalls != 2 {
		t.Fatalf("retry %+v calls=%d", next, backupCalls)
	}
}

func TestUpdateAssetsAndLinks(t *testing.T) {
	for _, platform := range []string{"darwin-arm64", "darwin-amd64", "windows-amd64", "windows-arm64", "linux-amd64", "linux-arm64"} {
		t.Run(platform, func(t *testing.T) {
			name := releaseAssetName("0.2.0", platform)
			link := RepoURL + "/releases/download/v0.2.0/" + name
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"tag_name":"v0.2.0","assets":[{"name":%q,"browser_download_url":%q}]}`, name, link)
			}))
			defer server.Close()
			u := testUpdates(t, server.URL)
			u.platform = platform
			info, _ := u.Check("0.1.0", true)
			if info.URL != link || !allowedUpdateLink(updateProxy+link) {
				t.Fatalf("asset %+v", info)
			}
		})
	}
	for _, raw := range []string{updateProxy + "https://evil.example/file", updateProxy + RepoURL + "/releases", updateProxy + RepoURL + "/releases/download/v0.2.0/evil.exe", RepoURL + "/../other/repo", RepoURL + ".evil.example", RepoURL + "/%2e%2e/evil", RepoURL + "?redirect=evil"} {
		if allowedUpdateLink(raw) {
			t.Errorf("unsafe link %s", raw)
		}
	}
}

func TestUpdatesRejectNonStableAndForeignAsset(t *testing.T) {
	for _, tc := range []struct {
		body      string
		wantError string
	}{{`{"tag_name":"v0.2.0","draft":true}`, "BAD_RESPONSE"}, {`{"tag_name":"v0.2.0-beta.1","prerelease":true}`, "BAD_RESPONSE"}, {`{"tag_name":"v0.2.0","assets":[{"name":"hoard-0.2.0-arm64.dmg","browser_download_url":"https://evil.example/installer"}]}`, ""}} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tc.body) }))
		u := testUpdates(t, server.URL)
		u.platform = "darwin-arm64"
		info, _ := u.Check("0.1.0", true)
		server.Close()
		if info.Error != tc.wantError {
			t.Fatalf("response %+v", info)
		}
		if info.URL != "" && info.URL != RepoURL+"/releases" {
			t.Fatalf("foreign asset selected %+v", info)
		}
	}
}
