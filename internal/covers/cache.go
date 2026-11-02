// Package covers keeps list covers on disk and serves them to the page, so
// a cover downloads from the CDN once and scrolling back never refetches it.
package covers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// Scheme is the URL scheme the page loads covers from.
	Scheme       = "hoard-cover"
	maxBytes     = 2 << 30
	trimBytes    = maxBytes * 9 / 10
	fetchSlots   = 6
	prefetchJobs = 2
	maxCover     = 8 << 20
)

// Lookup returns the candidate CDN URLs of one cover, best first.
type Lookup func(ctx context.Context, kind, id string) ([]string, error)

// Item names one cover and its candidate URLs.
type Item struct {
	Kind string
	ID   string
	URLs []string
}

// Cache stores covers under dir, keyed by kind, item id and URL.
type Cache struct {
	dir      string
	lookup   Lookup
	client   *http.Client
	slots    *gate
	queue    chan Item
	mu       sync.Mutex
	inflight map[string]*call
	size     atomic.Int64
	trimming atomic.Bool
	hits     atomic.Int64
	misses   atomic.Int64
	fetched  atomic.Int64
	failed   atomic.Int64
	prefetch atomic.Int64
}

type call struct {
	done chan struct{}
	path string
	err  error
}

// base is where the page loads covers from. WebView2 serves a custom scheme under
// http://<scheme>.localhost, so Windows pages cannot use the scheme URL itself.
var base = Scheme + "://localhost/"

func init() {
	if runtime.GOOS == "windows" {
		base = "http://" + Scheme + ".localhost/"
	}
}

// URL is the page URL of a cover.
func URL(kind, id string) string {
	return base + url.PathEscape(kind) + "/" + url.PathEscape(id)
}

// GalleryURL is the page URL of one gallery image.
func GalleryURL(id string, index int) string {
	return base + "g/" + url.PathEscape(id) + "/" + strconv.Itoa(index)
}

func New(dir string, lookup Lookup) (*Cache, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	c := &Cache{dir: dir, lookup: lookup, client: &http.Client{Timeout: 30 * time.Second},
		slots: &gate{free: fetchSlots}, queue: make(chan Item, 4096), inflight: map[string]*call{}}
	c.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !Allowed(req.URL.String()) {
			return errors.New("invalid cover redirect")
		}
		return nil
	}
	go c.measure()
	for i := 0; i < prefetchJobs; i++ {
		go c.work()
	}
	go c.report()
	return c, nil
}

// Allowed reports whether raw is an https Douyin CDN URL.
func Allowed(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	for _, host := range []string{"hdslb.com", "douyinpic.com", "douyinstatic.com", "douyinvod.com", "douyin.com", "iesdouyin.com", "bytecdn.cn", "bytedance.com", "douyincdn.com", "byteimg.com", "pstatp.com"} {
		if u.Hostname() == host || strings.HasSuffix(u.Hostname(), "."+host) {
			return true
		}
	}
	return false
}

func (c *Cache) path(kind, id, raw string) string {
	sum := sha256.Sum256([]byte(raw))
	key := sha256.Sum256([]byte(kind + "\x00" + id))
	name := hex.EncodeToString(key[:12]) + "-" + hex.EncodeToString(sum[:6])
	return filepath.Join(c.dir, name[:2], name)
}

// find returns a stored file for any of the URLs.
func (c *Cache) find(kind, id string, urls []string) string {
	for _, raw := range urls {
		p := c.path(kind, id, raw)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// Prefetch queues covers to download in the background; a full queue drops them.
func (c *Cache) Prefetch(items []Item) {
	for _, it := range items {
		if len(it.URLs) == 0 {
			continue
		}
		select {
		case c.queue <- it:
		default:
			return
		}
	}
}

func (c *Cache) work() {
	for it := range c.queue {
		if c.find(it.Kind, it.ID, it.URLs) != "" {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		if _, err := c.fetch(ctx, it.Kind, it.ID, it.URLs); err == nil {
			c.prefetch.Add(1)
		}
		cancel()
	}
}

// fetch downloads one cover once even when several callers ask for it.
func (c *Cache) fetch(ctx context.Context, kind, id string, urls []string) (string, error) {
	for {
		p, err := c.fetchOnce(ctx, kind, id, urls)
		if err == nil || ctx.Err() != nil || !errors.Is(err, context.Canceled) {
			return p, err
		}
	}
}

func (c *Cache) fetchOnce(ctx context.Context, kind, id string, urls []string) (string, error) {
	key := kind + "/" + id
	c.mu.Lock()
	if cl, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		select {
		case <-cl.done:
			return cl.path, cl.err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	cl := &call{done: make(chan struct{})}
	c.inflight[key] = cl
	c.mu.Unlock()
	go func() {
		// A request still waiting for a slot when its card scrolls away gives
		// up; one that started downloading finishes, so the cover lands on disk.
		if err := c.slots.acquire(ctx); err != nil {
			cl.err = err
		} else {
			dctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			cl.path, cl.err = c.download(dctx, kind, id, urls)
			cancel()
			c.slots.release()
		}
		c.mu.Lock()
		delete(c.inflight, key)
		c.mu.Unlock()
		close(cl.done)
	}()
	select {
	case <-cl.done:
		return cl.path, cl.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (c *Cache) download(ctx context.Context, kind, id string, urls []string) (string, error) {
	if p := c.find(kind, id, urls); p != "" {
		return p, nil
	}
	// Signed CDN URLs expire; the next entry of the URL list is tried in turn.
	for _, raw := range urls {
		if !Allowed(raw) {
			continue
		}
		p, err := c.get(ctx, kind, id, raw)
		if err == nil {
			c.fetched.Add(1)
			return p, nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
	}
	c.failed.Add(1)
	return "", errors.New("cover unavailable")
}

func (c *Cache) get(ctx context.Context, kind, id, raw string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Referer", "https://www.douyin.com/")
	if req.URL.Hostname() == "hdslb.com" || strings.HasSuffix(req.URL.Hostname(), ".hdslb.com") {
		req.Header.Set("Referer", "https://www.bilibili.com/")
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15")
	res, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cover status %d", res.StatusCode)
	}
	p := c.path(kind, id, raw)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".part-*")
	if err != nil {
		return "", err
	}
	n, err := io.Copy(tmp, io.LimitReader(res.Body, maxCover+1))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil && (n == 0 || n > maxCover) {
		err = errors.New("cover size out of range")
	}
	if err == nil {
		err = os.Rename(tmp.Name(), p)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}
	if c.size.Add(n) > maxBytes {
		go c.trim()
	}
	return p, nil
}

// ServeHTTP answers hoard-cover://localhost/<kind>/<id> from disk, fetching on a miss.
func (c *Cache) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if (len(parts) != 2 && len(parts) != 3) || (parts[0] != "w" && parts[0] != "m" && parts[0] != "g" && parts[0] != "a" && parts[0] != "account") || parts[1] == "" || (parts[0] == "g" && (len(parts) != 3 || parts[2] == "")) {
		http.NotFound(w, r)
		return
	}
	kind, id := parts[0], parts[1]
	if kind == "g" {
		id += ":" + parts[2]
	}
	urls, err := c.lookup(r.Context(), kind, id)
	if err != nil || len(urls) == 0 {
		http.NotFound(w, r)
		return
	}
	p := c.find(kind, id, urls)
	if p != "" {
		c.hits.Add(1)
		now := time.Now()
		_ = os.Chtimes(p, now, now)
	} else {
		c.misses.Add(1)
		if p, err = c.fetch(r.Context(), kind, id, urls); err != nil {
			http.Error(w, "cover unavailable", http.StatusBadGateway)
			return
		}
	}
	f, err := os.Open(p)
	if err != nil {
		http.Error(w, "cover unavailable", http.StatusBadGateway)
		return
	}
	defer f.Close()
	head := make([]byte, 512)
	n, _ := io.ReadFull(f, head)
	_, _ = f.Seek(0, io.SeekStart)
	w.Header().Set("Content-Type", http.DetectContentType(head[:n]))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = io.Copy(w, f)
}

func (c *Cache) measure() {
	var total int64
	_ = filepath.WalkDir(c.dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, e := d.Info(); e == nil {
				total += info.Size()
			}
		}
		return nil
	})
	if c.size.Add(total) > maxBytes {
		c.trim()
	}
}

// trim removes the least recently shown covers until the cache is under its cap.
func (c *Cache) trim() {
	if !c.trimming.CompareAndSwap(false, true) {
		return
	}
	defer c.trimming.Store(false)
	type file struct {
		path string
		size int64
		at   time.Time
	}
	var files []file
	_ = filepath.WalkDir(c.dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, e := d.Info(); e == nil {
				files = append(files, file{p, info.Size(), info.ModTime()})
			}
		}
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].at.Before(files[j].at) })
	removed := 0
	for _, f := range files {
		if c.size.Load() <= trimBytes {
			break
		}
		if os.Remove(f.path) == nil {
			c.size.Add(-f.size)
			removed++
		}
	}
	log.Printf("[hoard:cover] trimmed files=%d size_mb=%d", removed, c.size.Load()>>20)
}

func (c *Cache) report() {
	var last string
	for range time.Tick(15 * time.Second) {
		line := fmt.Sprintf("hits=%d misses=%d fetched=%d prefetched=%d failed=%d queued=%d size_mb=%d",
			c.hits.Load(), c.misses.Load(), c.fetched.Load(), c.prefetch.Load(), c.failed.Load(), len(c.queue), c.size.Load()>>20)
		if line != last {
			log.Printf("[hoard:cover] %s", line)
			last = line
		}
	}
}

// gate hands out a fixed number of download slots, newest waiter first, so
// the covers on screen now go ahead of ones asked for while scrolling past.
type gate struct {
	mu      sync.Mutex
	free    int
	waiters []chan struct{}
}

func (g *gate) acquire(ctx context.Context) error {
	g.mu.Lock()
	if g.free > 0 {
		g.free--
		g.mu.Unlock()
		return nil
	}
	ch := make(chan struct{})
	g.waiters = append(g.waiters, ch)
	g.mu.Unlock()
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		g.mu.Lock()
		for i, w := range g.waiters {
			if w == ch {
				g.waiters = append(g.waiters[:i], g.waiters[i+1:]...)
				g.mu.Unlock()
				return ctx.Err()
			}
		}
		g.mu.Unlock()
		g.release()
		return ctx.Err()
	}
}

func (g *gate) release() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if n := len(g.waiters); n > 0 {
		ch := g.waiters[n-1]
		g.waiters = g.waiters[:n-1]
		close(ch)
		return
	}
	g.free++
}
