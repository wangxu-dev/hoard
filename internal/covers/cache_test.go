package covers

import (
	"context"
	"io"
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
