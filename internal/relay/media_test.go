package relay

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMediaRelayClear(t *testing.T) {
	for _, count := range []int{0, 1, 2} {
		t.Run(fmt.Sprintf("registrations %d", count), func(t *testing.T) {
			r := &Relay{base: "http://127.0.0.1:1234", urls: map[string]string{}}
			urls := []string{}
			for i := 0; i < count; i++ {
				urls = append(urls, r.Register("https://v.douyinvod.com/test-video"))
			}
			r.Clear()
			for _, url := range urls {
				response := httptest.NewRecorder()
				r.ServeHTTP(response, httptest.NewRequest("GET", url, nil))
				if response.Code != 404 {
					t.Fatal("previous account registration remains accessible")
				}
			}
			if len(r.urls) != 0 || r.Register("https://v.douyinvod.com/new-video") == "" {
				t.Fatal("relay cannot register after clearing")
			}
		})
	}
}

func TestMediaRelayRange(t *testing.T) {
	r := &Relay{base: "http://127.0.0.1:1234", urls: map[string]string{"1": "https://v.douyinvod.com/video"}}
	r.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Range") != "bytes=0-3" || req.Header.Get("Referer") != "https://www.douyin.com/" {
			t.Fatal("missing playback headers")
		}
		return &http.Response{StatusCode: 206, Header: http.Header{"Content-Type": {"video/mp4"}, "Content-Range": {"bytes 0-3/8"}}, Body: io.NopCloser(strings.NewReader("data"))}, nil
	})}
	req := httptest.NewRequest("GET", r.base+"/video/1", nil)
	req.Header.Set("Range", "bytes=0-3")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 206 || w.Body.String() != "data" || w.Header().Get("Content-Range") != "bytes 0-3/8" {
		t.Fatalf("invalid range response: %v", w)
	}
}

func TestMediaRelayRejectsTargets(t *testing.T) {
	for _, raw := range []string{"http://127.0.0.1/", "https://douyinvod.com.evil.test/", "https://user@v.douyinvod.com/", "https://v.douyinvod.com:444/"} {
		if allowedMedia(raw) {
			t.Fatalf("accepted %s", raw)
		}
	}
	r := &Relay{base: "http://127.0.0.1:1234", urls: map[string]string{}}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", r.base+"/video/unknown", nil))
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
}

func TestMediaRelayWarmHead(t *testing.T) {
	const body = "0123456789abcdefghij"
	calls := []string{}
	r := &Relay{base: "http://127.0.0.1:1234", urls: map[string]string{"1": "https://v.douyinvod.com/video"}}
	r.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.Header.Get("Range"))
		var from, to int
		fmt.Sscanf(req.Header.Get("Range"), "bytes=%d-%d", &from, &to)
		if from == 0 {
			// The head request: the upstream returns only the first ten bytes.
			return &http.Response{StatusCode: 206, Header: http.Header{"Content-Type": {"video/mp4"}, "Content-Range": {"bytes 0-9/20"}}, Body: io.NopCloser(strings.NewReader(body[:10]))}, nil
		}
		return &http.Response{StatusCode: 206, Header: http.Header{"Content-Range": {fmt.Sprintf("bytes %d-%d/20", from, to)}}, Body: io.NopCloser(strings.NewReader(body[from : to+1]))}, nil
	})}
	r.Warm(r.base + "/video/1")
	waitDone(r.heads["https://v.douyinvod.com/video"])
	for _, c := range []struct {
		rng, want, contentRange string
		upstream                int
	}{
		{"bytes=2-5", "2345", "bytes 2-5/20", 0},
		{"bytes=0-", body, "bytes 0-19/20", 1},
		{"bytes=8-12", "89abc", "bytes 8-12/20", 1},
	} {
		calls = calls[:0]
		req := httptest.NewRequest("GET", r.base+"/video/1", nil)
		req.Header.Set("Range", c.rng)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 206 || w.Body.String() != c.want || w.Header().Get("Content-Range") != c.contentRange || len(calls) != c.upstream {
			t.Fatalf("%s: code=%d body=%q range=%q upstream=%v", c.rng, w.Code, w.Body.String(), w.Header().Get("Content-Range"), calls)
		}
	}
}

func waitDone(h *head) {
	h.mu.Lock()
	for !h.done {
		h.grown.Wait()
	}
	h.mu.Unlock()
}

// A player that opens a video while its head still downloads gets bytes as they arrive.
func TestMediaRelayStreamsFillingHead(t *testing.T) {
	pr, pw := io.Pipe()
	r := &Relay{base: "http://127.0.0.1:1234", urls: map[string]string{"1": "https://v.douyinvod.com/video"}}
	r.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 206, Header: http.Header{"Content-Range": {"bytes 0-7/8"}}, Body: pr}, nil
	})}
	r.Warm(r.base + "/video/1")
	pw.Write([]byte("0123"))
	served := make(chan string)
	go func() {
		req := httptest.NewRequest("GET", r.base+"/video/1", nil)
		req.Header.Set("Range", "bytes=0-7")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		served <- w.Body.String()
	}()
	pw.Write([]byte("4567"))
	pw.Close()
	if got := <-served; got != "01234567" {
		t.Fatalf("body %q", got)
	}
}

// A head whose fetch fails is dropped, so a later open fetches it again.
func TestMediaRelayForgetsFailedHead(t *testing.T) {
	r := &Relay{base: "http://127.0.0.1:1234", urls: map[string]string{"1": "https://v.douyinvod.com/video"}}
	r.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	r.Warm(r.base + "/video/1")
	r.mu.RLock()
	h := r.heads["https://v.douyinvod.com/video"]
	r.mu.RUnlock()
	if h != nil {
		waitDone(h)
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.heads) != 0 {
		t.Fatal("failed head kept")
	}
}
