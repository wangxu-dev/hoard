package covers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAllowed(t *testing.T) {
	for raw, want := range map[string]bool{
		"https://p3-pc-sign.douyinpic.com/a.jpeg": true,
		"http://p3-pc-sign.douyinpic.com/a.jpeg":  false,
		"https://example.com/a.jpeg":              false,
		"https://douyinpic.com.evil.com/a.jpeg":   false,
	} {
		if got := Allowed(raw); got != want {
			t.Errorf("Allowed(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestServeFromDisk(t *testing.T) {
	const raw = "https://p3-pc-sign.douyinpic.com/a.jpeg"
	c, err := New(t.TempDir(), func(context.Context, string, string) ([]string, error) { return []string{raw}, nil })
	if err != nil {
		t.Fatal(err)
	}
	p := c.path("w", "42", raw)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	png := []byte("\x89PNG\r\n\x1a\n0000")
	if err := os.WriteFile(p, png, 0o600); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	c.ServeHTTP(rec, httptest.NewRequest("GET", URL("w", "42"), nil))
	body, _ := io.ReadAll(rec.Body)
	if rec.Code != 200 || string(body) != string(png) || rec.Header().Get("Content-Type") != "image/png" || c.hits.Load() != 1 {
		t.Fatalf("code=%d type=%q hits=%d", rec.Code, rec.Header().Get("Content-Type"), c.hits.Load())
	}
	rec = httptest.NewRecorder()
	c.ServeHTTP(rec, httptest.NewRequest("GET", Scheme+"://localhost/x/42", nil))
	if rec.Code != 404 {
		t.Fatalf("bad kind code=%d", rec.Code)
	}
}

// A native read-only preview must never turn an image miss into a network fetch.
func TestCacheOnlyPreview(t *testing.T) {
	const raw = "https://p3-pc-sign.douyinpic.com/preview.jpeg"
	c, err := New(t.TempDir(), func(context.Context, string, string) ([]string, error) { return []string{raw}, nil })
	if err != nil {
		t.Fatal(err)
	}
	called := 0
	c.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		called++
		return nil, fmt.Errorf("network forbidden in test")
	})
	request := func() *httptest.ResponseRecorder {
		r := httptest.NewRecorder()
		c.ServeHTTP(r, httptest.NewRequest("GET", URL("w", "preview")+"?cacheOnly=1", nil))
		return r
	}
	if r := request(); r.Code != 404 || called != 0 {
		t.Fatalf("cache miss code=%d network=%d", r.Code, called)
	}
	p := c.path("w", "preview", raw)
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("cached-image"), 0600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(p)
	r := request()
	after, _ := os.Stat(p)
	if r.Code != 200 || r.Body.String() != "cached-image" || called != 0 {
		t.Fatalf("cache hit code=%d network=%d", r.Code, called)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("cache-only preview touched cache timestamps")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
