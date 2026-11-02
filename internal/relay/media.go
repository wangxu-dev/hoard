package relay

import (
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Only media returned by Douyin is registered; the endpoint accepts no target URL.
type Relay struct {
	mu     sync.RWMutex
	urls   map[string]string
	heads  map[string]*head
	base   string
	client *http.Client
}

func allowedMedia(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	for _, host := range []string{"bilivideo.com", "bilivideo.cn", "upos-hz-mirrorakam.akamaized.net", "douyinvod.com", "douyinpic.com", "douyinstatic.com", "byteimg.com", "pstatp.com", "douyin.com", "iesdouyin.com", "bytecdn.cn", "bytedance.com", "douyincdn.com", "snssdk.com", "amemv.com"} {
		if u.Hostname() == host || strings.HasSuffix(u.Hostname(), "."+host) {
			return true
		}
	}
	return false
}

func New() (*Relay, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	r := &Relay{urls: make(map[string]string), base: "http://" + listener.Addr().String(), client: &http.Client{Timeout: 2 * time.Minute}}
	r.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !allowedMedia(req.URL.String()) {
			return fmt.Errorf("invalid media redirect")
		}
		return nil
	}
	server := &http.Server{Handler: r, ReadHeaderTimeout: 5 * time.Second}
	go server.Serve(listener)
	return r, nil
}

func (r *Relay) Clear() {
	r.mu.Lock()
	clear(r.urls)
	clear(r.heads)
	r.mu.Unlock()
}

func (r *Relay) Register(raw string) string {
	if !allowedMedia(raw) {
		return ""
	}
	key := rand.Text()
	r.mu.Lock()
	r.urls[key] = raw
	r.mu.Unlock()
	return r.base + "/video/" + url.PathEscape(key)
}

func (r *Relay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Host != strings.TrimPrefix(r.base, "http://") {
		http.Error(w, "invalid host", 403)
		return
	}
	if req.Method != "GET" && req.Method != "HEAD" {
		w.WriteHeader(405)
		return
	}
	r.mu.RLock()
	raw := r.urls[strings.TrimPrefix(req.URL.Path, "/video/")]
	r.mu.RUnlock()
	if raw == "" {
		http.NotFound(w, req)
		return
	}
	if r.serveHead(w, req, raw) {
		return
	}
	upstream, err := http.NewRequestWithContext(req.Context(), req.Method, raw, nil)
	if err != nil {
		http.Error(w, "invalid media", 502)
		return
	}
	upstream.Header.Set("Referer", mediaReferer(raw))
	upstream.Header.Set("User-Agent", "Mozilla/5.0")
	for _, name := range []string{"Range", "If-Range"} {
		if v := req.Header.Get(name); v != "" {
			upstream.Header.Set(name, v)
		}
	}
	res, err := r.client.Do(upstream)
	if err != nil {
		http.Error(w, "media unavailable", 502)
		return
	}
	defer res.Body.Close()
	for _, name := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
		if value := res.Header.Get(name); value != "" {
			w.Header().Set(name, value)
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(res.StatusCode)
	if req.Method != "HEAD" {
		io.Copy(w, res.Body)
	}
}
