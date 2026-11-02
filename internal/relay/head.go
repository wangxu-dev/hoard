package relay

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The opening bytes of videos the viewer will likely show next stay in memory,
// so they start without waiting on the network. Memory stays bounded: at most
// headEntries heads of headBytes each.
const (
	headBytes   = 2 << 20
	headEntries = 12
)

// head holds the opening bytes of one video. Bytes become readable as they arrive,
// so a player can start on a head that is still filling.
type head struct {
	meta  chan struct{} // closed once the upstream headers are known, or the fetch failed
	mu    sync.Mutex
	grown *sync.Cond
	data  []byte
	total int64
	ctype string
	ok    bool // headers arrived and describe a usable file
	done  bool // no more bytes will arrive
	used  time.Time
}

func newHead() *head {
	h := &head{meta: make(chan struct{}), used: time.Now()}
	h.grown = sync.NewCond(&h.mu)
	return h
}

// Warm fetches the opening bytes of a registered relay URL in the background.
func (r *Relay) Warm(relayURL string) {
	raw := r.lookup(relayURL)
	if raw == "" {
		return
	}
	r.mu.Lock()
	if h, ok := r.heads[raw]; ok {
		h.used = time.Now()
		r.mu.Unlock()
		return
	}
	if r.heads == nil {
		r.heads = map[string]*head{}
	}
	h := newHead()
	r.heads[raw] = h
	r.evictLocked()
	r.mu.Unlock()
	go r.fill(raw, h)
}

func (r *Relay) lookup(relayURL string) string {
	u, err := url.Parse(relayURL)
	if err != nil || "http://"+u.Host != r.base || !strings.HasPrefix(u.Path, "/video/") {
		return ""
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.urls[strings.TrimPrefix(u.Path, "/video/")]
}

func (r *Relay) evictLocked() {
	for len(r.heads) > headEntries {
		oldest := ""
		for k, h := range r.heads {
			if oldest == "" || h.used.Before(r.heads[oldest].used) {
				oldest = k
			}
		}
		delete(r.heads, oldest)
	}
}

func (r *Relay) fill(raw string, h *head) {
	failed := true
	defer func() {
		h.mu.Lock()
		h.done = true
		h.grown.Broadcast()
		h.mu.Unlock()
		if failed {
			// A head that never got going is forgotten, so the next open tries again.
			r.mu.Lock()
			if r.heads[raw] == h {
				delete(r.heads, raw)
			}
			r.mu.Unlock()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := r.upstream(ctx, "GET", raw, fmt.Sprintf("bytes=0-%d", headBytes-1))
	if err != nil {
		close(h.meta)
		return
	}
	defer res.Body.Close()
	var total int64
	switch res.StatusCode {
	case 206:
		total = rangeTotal(res.Header.Get("Content-Range"))
	case 200:
		total = res.ContentLength
	}
	if total <= 0 {
		close(h.meta)
		return
	}
	h.mu.Lock()
	h.total, h.ctype, h.ok = total, res.Header.Get("Content-Type"), true
	h.data = make([]byte, 0, min(total, headBytes))
	h.mu.Unlock()
	close(h.meta)
	failed = false
	body := io.LimitReader(res.Body, headBytes)
	buf := make([]byte, 32<<10)
	for {
		n, err := body.Read(buf)
		if n > 0 {
			h.mu.Lock()
			h.data = append(h.data, buf[:n]...)
			h.grown.Broadcast()
			h.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

func (r *Relay) upstream(ctx context.Context, method, raw, byteRange string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "https://www.douyin.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	if byteRange != "" {
		req.Header.Set("Range", byteRange)
	}
	return r.client.Do(req)
}

// rangeTotal reads the full size from a Content-Range such as "bytes 0-9/100".
func rangeTotal(value string) int64 {
	i := strings.LastIndexByte(value, '/')
	if i < 0 {
		return 0
	}
	n, err := strconv.ParseInt(value[i+1:], 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// parseRange reads a single "bytes=a-b" or "bytes=a-" range; no header means the whole file.
func parseRange(value string, total int64) (start, end int64, ok bool) {
	if value == "" {
		return 0, total - 1, true
	}
	spec, found := strings.CutPrefix(value, "bytes=")
	if !found || strings.Contains(spec, ",") {
		return 0, 0, false
	}
	a, b, found := strings.Cut(spec, "-")
	if !found || a == "" {
		return 0, 0, false
	}
	start, err := strconv.ParseInt(a, 10, 64)
	if err != nil || start < 0 || start >= total {
		return 0, 0, false
	}
	end = total - 1
	if b != "" {
		if end, err = strconv.ParseInt(b, 10, 64); err != nil || end < start {
			return 0, 0, false
		}
		end = min(end, total-1)
	}
	return start, end, true
}

// serveHead answers a request that starts inside a head: the cached bytes first,
// waiting for them while the head still fills, then the rest of the range from upstream.
func (r *Relay) serveHead(w http.ResponseWriter, req *http.Request, raw string) bool {
	r.mu.Lock()
	h := r.heads[raw]
	if h != nil {
		h.used = time.Now()
	}
	r.mu.Unlock()
	if h == nil {
		return false
	}
	select {
	case <-h.meta:
	case <-req.Context().Done():
		return true
	}
	if !h.ok {
		return false
	}
	start, end, ok := parseRange(req.Header.Get("Range"), h.total)
	if !ok || start >= min(h.total, headBytes) {
		return false
	}
	if h.ctype != "" {
		w.Header().Set("Content-Type", h.ctype)
	}
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
	w.Header().Set("Cache-Control", "no-store")
	if req.Header.Get("Range") != "" {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, h.total))
		w.WriteHeader(206)
	} else {
		w.WriteHeader(200)
	}
	if req.Method == "HEAD" {
		return true
	}
	// A player that goes away must not stay parked on a head that stalls.
	stop := context.AfterFunc(req.Context(), func() {
		h.mu.Lock()
		h.grown.Broadcast()
		h.mu.Unlock()
	})
	defer stop()
	flusher, _ := w.(http.Flusher)
	pos := start
	for pos <= end {
		h.mu.Lock()
		for int64(len(h.data)) <= pos && !h.done && req.Context().Err() == nil {
			h.grown.Wait()
		}
		chunk := h.data[min(pos, int64(len(h.data))):min(end+1, int64(len(h.data)))]
		h.mu.Unlock()
		if req.Context().Err() != nil {
			return true
		}
		if len(chunk) == 0 {
			break // the head is complete or stopped short; upstream has the rest
		}
		if _, err := w.Write(chunk); err != nil {
			return true
		}
		if flusher != nil {
			flusher.Flush()
		}
		pos += int64(len(chunk))
	}
	if pos > end {
		return true
	}
	res, err := r.upstream(req.Context(), "GET", raw, fmt.Sprintf("bytes=%d-%d", pos, end))
	if err != nil {
		return true
	}
	defer res.Body.Close()
	if res.StatusCode == 206 {
		io.Copy(w, res.Body)
	}
	return true
}
