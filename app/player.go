package app

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wangxu-dev/hoard/internal/covers"
	"github.com/wangxu-dev/hoard/internal/douyin"
	"github.com/wangxu-dev/hoard/internal/store"
)

type playerEngine interface {
	Request(context.Context, douyin.Operation, map[string]string) (douyin.PageReply, error)
}

// MediaImage is one gallery image and its live photo clip, if any.
type MediaImage struct {
	URL     string `json:"url"`
	LiveURL string `json:"liveUrl,omitempty"`
}

// MediaMusic is the sound of a gallery or a saved sound.
type MediaMusic struct {
	Title  string `json:"title"`
	Author string `json:"author"`
	URL    string `json:"url"`
}

// MediaResult is what the viewer needs to show and play one item. Media URLs
// point at the local relay or cover cache, never at the CDN.
type MediaResult struct {
	Kind         string       `json:"kind"`
	Videos       []string     `json:"videos"`
	Images       []MediaImage `json:"images"`
	Music        MediaMusic   `json:"music"`
	Author       string       `json:"author"`
	AuthorAvatar string       `json:"authorAvatar"`
	Desc         string       `json:"desc"`
	PublishedAt  int64        `json:"publishedAt"`
	// Complete is false when stored data stood in for the detail, so live photos
	// and the sound title may be missing until a refresh.
	Complete bool `json:"complete"`
	// ExpiresAt is when the first play URL stops working, in unix milliseconds; 0 when unknown.
	ExpiresAt int64 `json:"expiresAt"`
}

// Player resolves playable media for the viewer. It keeps no URLs: the
// frontend caches results while they are fresh and asks again when one fails.
type Player struct {
	app      *App
	mu       sync.Mutex
	inflight map[string]*playerCall
}

type playerCall struct {
	done   chan struct{}
	detail douyin.Detail
	err    error
}

func NewPlayer(a *App) *Player {
	return &Player{app: a, inflight: map[string]*playerCall{}}
}

func mediaExpiry(raw string) time.Time {
	u, err := url.Parse(raw)
	if err != nil {
		return time.Time{}
	}
	// Signed play URLs carry their issue time in dy_q and stop working about three
	// hours later; two hours keeps a margin for a long viewing session.
	if issued, err := strconv.ParseInt(u.Query().Get("dy_q"), 10, 64); err == nil && issued > 0 {
		return time.Unix(issued, 0).Add(2 * time.Hour)
	}
	for _, key := range []string{"x-expires", "expires", "deadline", "x-oss-expires"} {
		if value := u.Query().Get(key); value != "" {
			if n, err := strconv.ParseInt(value, 10, 64); err == nil {
				if n > 1e12 {
					n /= 1000
				}
				return time.Unix(n, 0)
			}
		}
	}
	return time.Time{}
}

func mediaValid(raw string) bool {
	exp := mediaExpiry(raw)
	return raw != "" && (exp.IsZero() || time.Now().Add(time.Minute).Before(exp))
}

func mediaTTL(detail douyin.Detail) time.Time {
	var earliest time.Time
	all := append([]string{}, detail.Videos...)
	for _, im := range detail.Images {
		all = append(all, im.URL, im.LiveURL)
	}
	all = append(all, detail.Music.URL)
	for _, raw := range all {
		exp := mediaExpiry(raw)
		if !exp.IsZero() && (earliest.IsZero() || exp.Before(earliest)) {
			earliest = exp
		}
	}
	if earliest.IsZero() {
		return earliest
	}
	return earliest.Add(-time.Minute)
}

// Media returns playable media for one item from a fresh item detail request.
// A gallery answers at once from its cached images; Refresh then adds its
// sound and live photos.
func (p *Player) Media(kind, id string) (MediaResult, error) {
	return p.media(kind, id, false)
}

// Refresh forces one fresh detail request after the current media candidates fail.
func (p *Player) Refresh(kind, id string) (MediaResult, error) {
	return p.media(kind, id, true)
}

func (p *Player) media(kind, id string, force bool) (out MediaResult, resultErr error) {
	start, source, class := time.Now(), "detail", "OK"
	defer func() {
		log.Printf("[hoard:player] media kind=%s class=%s ms=%d source=%s", kind, class, time.Since(start).Milliseconds(), source)
	}()
	if id == "" || (kind != "video" && kind != "gallery" && kind != "music") {
		class = "INVALID_REQUEST"
		return out, errors.New("INVALID_ID")
	}
	if p.app.Store == nil || p.app.Media == nil {
		class = "UNAVAILABLE"
		return out, errors.New("UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if kind == "music" {
		source = "store"
		out, resultErr = p.music(ctx, id)
		if resultErr != nil {
			class = "UNAVAILABLE"
		}
		return
	}
	rec, err := p.app.Store.MediaRecord(ctx, kind, id)
	if err != nil {
		class = "UNAVAILABLE"
		return out, errors.New("UNAVAILABLE")
	}
	if !force {
		// A gallery shows its cached images at once; the viewer then asks again
		// with force for the sound and live photos.
		if rec.Kind == "gallery" {
			if d, ok := storedImages(rec); ok {
				source = "cover-cache"
				return p.result(rec, d), nil
			}
		}
	}
	detail, err := p.resolve(ctx, id)
	if err != nil {
		class = err.Error()
		if d, ok := storedImages(rec); ok && rec.Kind == "gallery" {
			source = "cover-cache"
			return p.result(rec, d), nil
		}
		return out, err
	}
	if detail.Author.Avatar != "" && !rec.HasAvatar {
		if p.app.Store.FillAuthorAvatar(ctx, id, detail.Author.Avatar) == nil {
			rec.HasAvatar = true
		}
	}
	out = p.result(rec, detail)
	out.Complete = true
	if exp := mediaTTL(detail); !exp.IsZero() {
		out.ExpiresAt = exp.UnixMilli()
	}
	return out, nil
}

// Warm loads the opening bytes of videos the viewer will likely show next.
func (p *Player) Warm(urls []string) {
	if p.app.Media == nil {
		return
	}
	for _, u := range urls {
		p.app.Media.Warm(u)
	}
}

// resolve runs one detail request per item at a time; callers arriving while
// it runs share its result.
func (p *Player) resolve(ctx context.Context, id string) (douyin.Detail, error) {
	p.mu.Lock()
	if call, ok := p.inflight[id]; ok {
		p.mu.Unlock()
		select {
		case <-call.done:
			return call.detail, call.err
		case <-ctx.Done():
			return douyin.Detail{}, errors.New("CANCELED")
		}
	}
	call := &playerCall{done: make(chan struct{})}
	p.inflight[id] = call
	p.mu.Unlock()
	call.detail, call.err = p.detail(ctx, id)
	p.mu.Lock()
	delete(p.inflight, id)
	p.mu.Unlock()
	close(call.done)
	return call.detail, call.err
}

// storedImages lists a gallery's images through the cover cache, which keeps
// them on disk like covers. Sound and live photos always come from a detail.
func storedImages(rec store.MediaRecord) (douyin.Detail, bool) {
	var stored struct {
		Images []string `json:"images"`
	}
	_ = json.Unmarshal(rec.Extra, &stored)
	if len(stored.Images) == 0 {
		return douyin.Detail{}, false
	}
	d := douyin.Detail{Class: douyin.ResponseOK}
	for _, raw := range stored.Images {
		d.Images = append(d.Images, douyin.DetailImage{URL: raw})
	}
	return d, true
}

// detail sends one item detail request through the shared request gate,
// ahead of the next sync page and at least 1.2 s after any other request.
func (p *Player) detail(ctx context.Context, id string) (douyin.Detail, error) {
	eng, ok := p.app.Engine.(playerEngine)
	if !ok {
		return douyin.Detail{}, errors.New("UNAVAILABLE")
	}
	p.app.mu.Lock()
	session := p.app.sessionCtx
	p.app.mu.Unlock()
	if session == nil {
		return douyin.Detail{}, errors.New("AUTH_REQUIRED")
	}
	// Playback waits on this request, so the next sync page goes after it.
	p.app.frontRequests.Add(1)
	defer p.app.frontRequests.Add(-1)
	p.app.requestMu.Lock()
	if !p.app.lastRequest.IsZero() {
		if d := 1200*time.Millisecond - time.Since(p.app.lastRequest); d > 0 && !wait(ctx, d) {
			p.app.requestMu.Unlock()
			return douyin.Detail{}, errors.New("CANCELED")
		}
	}
	reply, err := eng.Request(session, douyin.OpDetail, map[string]string{"aweme_id": id})
	p.app.lastRequest = time.Now()
	p.app.requestMu.Unlock()
	if err != nil {
		return douyin.Detail{}, errors.New("REQUEST_FAILED")
	}
	detail := douyin.ParseDetail(reply)
	if detail.Class != douyin.ResponseOK {
		log.Printf("[hoard:player] detail rejected class=%v status=%d code=%d bytes=%d keys=%s problem=%q", detail.Class, reply.Status, statusCode(reply.Body), len(reply.Body), topKeys(reply.Body), detail.Problem)
	}
	if detail.Class == douyin.ResponseOK {
		log.Printf("[hoard:player] detail sources=%s videos=%d images=%d music=%t", detail.Sources, len(detail.Videos), len(detail.Images), detail.Music.URL != "")
	}
	switch detail.Problem {
	case "DELETED", "PRIVATE", "REVIEWING", "WITHHELD":
		return detail, errors.New(detail.Problem)
	}
	switch detail.Class {
	case douyin.ResponseOK:
		return detail, nil
	case douyin.ResponseVerificationRequired, douyin.ResponseRiskControl:
		return detail, errors.New("NEED_VERIFY")
	case douyin.ResponseRateLimited:
		return detail, errors.New("COOLING")
	case douyin.ResponseAuthRequired:
		return detail, errors.New("AUTH_REQUIRED")
	}
	return detail, errors.New("UNAVAILABLE")
}

func (p *Player) music(ctx context.Context, id string) (MediaResult, error) {
	r, err := p.app.Store.MediaRecord(ctx, "music", id)
	if err != nil {
		return MediaResult{}, errors.New("UNAVAILABLE")
	}
	var meta struct {
		PlayURL string `json:"play_url"`
	}
	_ = json.Unmarshal(r.Extra, &meta)
	relay := ""
	if mediaValid(meta.PlayURL) {
		relay = p.app.Media.Register(meta.PlayURL)
	}
	if relay == "" {
		return MediaResult{}, errors.New("UNAVAILABLE")
	}
	return MediaResult{Kind: "music", Music: MediaMusic{Title: r.Title, Author: r.Author, URL: relay}, Author: r.Author, Desc: r.Title}, nil
}

// result combines stored metadata with resolved media. The stored author,
// avatar, caption and date always show, also when no detail was requested.
func (p *Player) result(rec store.MediaRecord, d douyin.Detail) MediaResult {
	out := MediaResult{Kind: rec.Kind, Author: rec.Author, Desc: rec.Title, PublishedAt: rec.PublishedAt,
		Music: MediaMusic{Title: d.Music.Title, Author: d.Music.Author}}
	if out.Author == "" {
		out.Author = d.Author.Nickname
	}
	if out.Desc == "" {
		out.Desc = d.Desc
	}
	if out.PublishedAt == 0 {
		out.PublishedAt = d.PublishedAt
	}
	if rec.HasAvatar && p.app.Covers != nil {
		out.AuthorAvatar = covers.URL("a", rec.RemoteID)
	}
	out.Music.URL = p.app.Media.Register(d.Music.URL)
	for _, v := range d.Videos {
		if u := p.app.Media.Register(v); u != "" {
			out.Videos = append(out.Videos, u)
		}
	}
	for n, i := range d.Images {
		imageURL := i.URL
		if p.app.Covers != nil {
			imageURL = covers.GalleryURL(rec.RemoteID, n)
		}
		out.Images = append(out.Images, MediaImage{URL: imageURL, LiveURL: p.app.Media.Register(i.LiveURL)})
	}
	return out
}

// topKeys lists the top-level field names of a JSON reply, never its values.
func topKeys(body string) string {
	var obj map[string]json.RawMessage
	if json.Unmarshal([]byte(body), &obj) != nil {
		return "-"
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func statusCode(body string) int64 {
	var env struct {
		Code int64 `json:"status_code"`
	}
	_ = json.Unmarshal([]byte(body), &env)
	return env.Code
}
