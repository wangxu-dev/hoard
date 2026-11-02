package app

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/wangxu-dev/hoard/internal/covers"
	"github.com/wangxu-dev/hoard/internal/douyin"
	"github.com/wangxu-dev/hoard/internal/store"
)

// SyncEngine is the part of the account engine the list sync uses.
type SyncEngine interface {
	Request(context.Context, douyin.Operation, map[string]string) (douyin.PageReply, error)
	PageState(context.Context) douyin.PageState
	Recycle() error
	Visible() bool
	CookiePresent() bool
}

// SyncView is the progress of one list of one account.
type SyncView struct {
	AccountID   string `json:"accountId"`
	Kind        string `json:"kind"`
	Phase       string `json:"phase"`
	Status      string `json:"status"`
	Items       int    `json:"items"`
	RemoteTotal int    `json:"remoteTotal"`
	Pages       int    `json:"pages"`
	RetryAt     int64  `json:"retryAt"`
	ErrorCode   string `json:"errorCode"`
	LastSyncAt  int64  `json:"lastSyncAt"`
	// Fresh is how many new items the latest update check found; it is set
	// only on the progress event that ends the check.
	Fresh int `json:"fresh"`
}

// LibraryChange tells pages that a list gained items.
type LibraryChange struct {
	AccountID string `json:"accountId"`
	Kind      string `json:"kind"`
}

var (
	syncProgress   = mygo.NewEvent[SyncView]("sync:progress")
	libraryChanged = mygo.NewEvent[LibraryChange]("library:changed")
)

var syncKinds = []string{"like", "favorite", "music"}

var syncOps = map[string]douyin.Operation{"like": douyin.OpLikes, "favorite": douyin.OpFavorites, "music": douyin.OpMusic}

const (
	pageGap       = 1200 * time.Millisecond
	pageJitter    = 600 * time.Millisecond
	headPageLimit = 30
	segmentPages  = 24
	turnPages     = 10
	retryDelay    = 2 * time.Minute
)

// Sync pulls likes, favorites and music of the active account into the library.
type Sync struct {
	app         *App
	mu          sync.Mutex
	generation  int
	running     string
	runCtx      context.Context
	cancel      context.CancelFunc
	views       map[string]SyncView
	verifying   bool
	hideOnPage  bool
	lastRequest time.Time
	listPages   int
}

func NewSync(a *App) *Sync {
	s := &Sync{app: a, views: map[string]SyncView{}}
	a.loggedIn = func() {
		if err := s.Start(); err != nil {
			log.Printf("[hoard:sync] auto start skipped: %v", err)
		}
	}
	return s
}

func viewKey(accountID, kind string) string { return accountID + "|" + kind }

func toView(accountID string, r store.SyncRecord) SyncView {
	v := SyncView{AccountID: accountID, Kind: r.Kind, Phase: r.Phase, Status: r.Status, Items: r.Items, Pages: r.Pages, ErrorCode: r.ErrorCode, RemoteTotal: -1}
	if r.RemoteTotal != nil {
		v.RemoteTotal = *r.RemoteTotal
	}
	if r.RetryAt != nil {
		v.RetryAt = *r.RetryAt
	}
	if r.LastFullAt != nil {
		v.LastSyncAt = *r.LastFullAt
	}
	return v
}

// States returns the sync state of every list of every account.
func (s *Sync) States() ([]SyncView, error) {
	a := s.app
	if a.Store == nil {
		return nil, errors.New("STORE_FAILED")
	}
	ctx, cancel := accountContext()
	defer cancel()
	accounts, err := a.Store.Accounts(ctx)
	if err != nil {
		return nil, errors.New("STORE_FAILED")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []SyncView{}
	for _, account := range accounts {
		records, err := a.Store.SyncRecords(ctx, account.ID)
		if err != nil {
			return nil, errors.New("STORE_FAILED")
		}
		byKind := map[string]store.SyncRecord{}
		for _, r := range records {
			byKind[r.Kind] = r
		}
		for _, kind := range syncKinds {
			if v, ok := s.views[viewKey(account.ID, kind)]; ok && (s.running == account.ID || v.Status == "blocked" || v.Status == "verifying") {
				out = append(out, v)
				continue
			}
			r, ok := byKind[kind]
			if !ok {
				r = store.SyncRecord{Kind: kind, Phase: "idle", Status: "idle"}
			}
			v := toView(account.ID, r)
			if v.Status == "running" || v.Status == "queued" || v.Status == "cooling" || v.Status == "checking" {
				v.Status = "idle"
			}
			if account.Status == "signed_out" {
				v.Status = "signed_out"
			}
			out = append(out, v)
		}
	}
	return out, nil
}

// Start syncs the active account in the background. A running sync continues.
func (s *Sync) Start() error {
	log.Print("[hoard:sync] start requested")
	a := s.app
	a.mu.Lock()
	accountID, ctx := a.accountID, a.sessionCtx
	a.mu.Unlock()
	if a.Store == nil {
		log.Print("[hoard:sync] start rejected: store unavailable")
		return errors.New("STORE_FAILED")
	}
	if accountID == "" || ctx == nil || ctx.Err() != nil {
		log.Print("[hoard:sync] start rejected: no active account session")
		return errors.New("NO_ACCOUNT")
	}
	if _, ok := a.Engine.(SyncEngine); !ok {
		log.Print("[hoard:sync] start rejected: engine unavailable")
		return errors.New("SYNC_UNAVAILABLE")
	}
	s.mu.Lock()
	if s.running == accountID && s.runCtx != nil && s.runCtx.Err() == nil {
		s.mu.Unlock()
		log.Print("[hoard:sync] start ignored: task already running")
		return nil
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.generation++
	gen := s.generation
	runCtx, cancel := context.WithCancel(ctx)
	s.running, s.runCtx, s.cancel = accountID, runCtx, cancel
	s.mu.Unlock()
	log.Print("[hoard:sync] started")
	go func() {
		defer func() {
			cancel()
			s.mu.Lock()
			if s.generation == gen {
				s.running, s.runCtx, s.cancel = "", nil, nil
				s.hideOnPage = false
			}
			s.mu.Unlock()
			log.Print("[hoard:sync] stopped")
		}()
		s.run(runCtx, accountID)
		if runCtx.Err() == nil && s.unfinished(accountID) {
			time.AfterFunc(retryDelay, func() {
				if err := s.Start(); err != nil {
					log.Printf("[hoard:sync] retry skipped: %v", err)
				}
			})
		}
	}()
	return nil
}

// unfinished reports whether a list of the account still has backfill left
// after a run ended on its own without needing the user.
func (s *Sync) unfinished(accountID string) bool {
	ctx, cancel := accountContext()
	defer cancel()
	records, err := s.app.Store.SyncRecords(ctx, accountID)
	if err != nil {
		return false
	}
	left := false
	for _, r := range records {
		switch r.Status {
		case "blocked", "verifying", "signed_out":
			return false
		}
		if r.Phase != "done" && r.Status != "unsupported" {
			left = true
		}
	}
	return left
}

func (s *Sync) emit(v SyncView) {
	s.mu.Lock()
	s.views[viewKey(v.AccountID, v.Kind)] = v
	s.mu.Unlock()
	if err := syncProgress.Broadcast(v); err != nil {
		log.Print("[hoard:sync] progress delivery failed")
	}
}

func (s *Sync) save(accountID string, r *store.SyncRecord) {
	ctx, cancel := accountContext()
	defer cancel()
	if err := s.app.Store.PutSyncRecord(ctx, accountID, *r); err != nil {
		log.Printf("[hoard:sync] state write failed kind=%s", r.Kind)
	}
	if records, err := s.app.Store.SyncRecords(ctx, accountID); err == nil {
		for _, rec := range records {
			if rec.Kind == r.Kind {
				r.Items = rec.Items
			}
		}
	}
	s.emit(toView(accountID, *r))
}

func wait(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// request sends one request, one at a time, at least pageGap plus jitter apart.
func (s *Sync) request(ctx context.Context, op douyin.Operation, params map[string]string) (douyin.PageReply, error) {
	eng := s.app.Engine.(SyncEngine)
	for {
		s.app.requestMu.Lock()
		if s.app.frontRequests.Load() > 0 {
			s.app.requestMu.Unlock()
			if !wait(ctx, 20*time.Millisecond) {
				return douyin.PageReply{}, ctx.Err()
			}
			continue
		}
		break
	}
	defer s.app.requestMu.Unlock()
	if !s.app.lastRequest.IsZero() {
		gap := pageGap + time.Duration(rand.Int64N(int64(pageJitter)))
		if d := time.Until(s.app.lastRequest.Add(gap)); d > 0 && !wait(ctx, d) {
			return douyin.PageReply{}, ctx.Err()
		}
	}
	if ctx.Err() != nil {
		return douyin.PageReply{}, ctx.Err()
	}
	reply, err := eng.Request(ctx, op, params)
	s.app.lastRequest = time.Now()
	s.lastRequest = s.app.lastRequest
	return reply, err
}

func (s *Sync) waitReady(ctx context.Context) bool {
	if _, err := s.app.EnsureSession(); err != nil {
		return false
	}
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if s.app.Engine.Status().Ready {
			return true
		}
		if !wait(ctx, 300*time.Millisecond) {
			return false
		}
	}
	return false
}

func (s *Sync) run(ctx context.Context, accountID string) {
	a := s.app
	sctx, cancel := accountContext()
	records, err := a.Store.SyncRecords(sctx, accountID)
	cancel()
	if err != nil {
		log.Print("[hoard:sync] state read failed")
		return
	}
	byKind := map[string]*store.SyncRecord{}
	for i := range records {
		byKind[records[i].Kind] = &records[i]
	}
	for _, kind := range syncKinds {
		if byKind[kind] == nil {
			byKind[kind] = &store.SyncRecord{Kind: kind, Phase: "idle", Status: "idle"}
		}
		r := byKind[kind]
		if !(kind == "music" && r.Status == "unsupported") {
			r.Status = "queued"
		}
		v := toView(accountID, *r)
		s.emit(v)
	}
	if !s.waitReady(ctx) {
		log.Print("[hoard:sync] page not ready")
		for _, kind := range syncKinds {
			r := byKind[kind]
			if r.Status == "queued" {
				r.Status, r.ErrorCode = "paused", "PAGE_NOT_READY"
				s.save(accountID, r)
			}
		}
		return
	}
	var profile douyin.Profile
	for attempt := 0; attempt < 3; attempt++ {
		reply, err := s.request(ctx, douyin.OpSelf, nil)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			profile = douyin.Profile{Class: douyin.ResponseNotReady}
		} else {
			profile = douyin.ParseProfile(reply)
		}
		log.Printf("[hoard:sync] self class=%s http=%d", profile.Class, reply.Status)
		if profile.Class != douyin.ResponseNotReady {
			break
		}
		if !wait(ctx, 3*time.Second) {
			return
		}
	}
	switch profile.Class {
	case douyin.ResponseOK:
	case douyin.ResponseNotReady:
		if a.Engine.(SyncEngine).CookiePresent() {
			for _, kind := range syncKinds {
				r := byKind[kind]
				r.Status, r.ErrorCode = "paused", string(profile.Class)
				s.save(accountID, r)
			}
			return
		}
		fallthrough
	case douyin.ResponseAuthRequired:
		a.mu.Lock()
		if a.accountID == accountID {
			_ = a.signedOutLocked(accountID)
		}
		a.mu.Unlock()
		for _, kind := range syncKinds {
			r := byKind[kind]
			r.Status, r.ErrorCode = "signed_out", string(profile.Class)
			s.save(accountID, r)
		}
		return
	default:
		for _, kind := range syncKinds {
			r := byKind[kind]
			r.Status, r.ErrorCode = "blocked", string(profile.Class)
			s.save(accountID, r)
		}
		return
	}
	totals := map[string]int{"like": profile.Likes, "favorite": profile.Favorites}
	for kind, t := range totals {
		if t > 0 {
			byKind[kind].RemoteTotal = &t
		}
	}
	// Lists take turns a few pages at a time, so favorites and music do not
	// wait hours behind a long likes backfill.
	pending := append([]string(nil), syncKinds...)
	started := map[string]bool{}
	for len(pending) > 0 {
		next := []string{}
		for _, kind := range pending {
			stop, more := s.syncKind(ctx, accountID, profile.SecUID, byKind[kind], turnPages, !started[kind])
			started[kind] = true
			if stop {
				for _, other := range syncKinds {
					if o := byKind[other]; o.Status == "queued" {
						o.Status = "idle"
						s.save(accountID, o)
					}
				}
				return
			}
			if ctx.Err() != nil {
				return
			}
			if more {
				next = append(next, kind)
			}
		}
		pending = next
	}
}

type fetchOutcome string

const (
	fetchOK        fetchOutcome = ""
	fetchCancel    fetchOutcome = "cancel"
	fetchBlocked   fetchOutcome = "blocked"
	fetchSignedOut fetchOutcome = "signed_out"
	fetchRejected  fetchOutcome = "rejected"
	fetchPaused    fetchOutcome = "paused"
)

// Page sizes asked of the list endpoints. Likes accept 50; favorites and
// music reject anything above 20 with status code 5.
const (
	likePageSize = "50"
	listPageSize = "20"
)

func listParams(kind, secUID, cursor string) map[string]string {
	switch kind {
	case "like":
		return map[string]string{"sec_user_id": secUID, "max_cursor": cursor, "min_cursor": "0", "count": likePageSize, "publish_video_strategy_type": "2", "pc_client_type": "1"}
	default:
		return map[string]string{"cursor": cursor, "count": listPageSize}
	}
}

func (s *Sync) fetch(ctx context.Context, accountID string, r *store.SyncRecord, secUID, cursor string) (douyin.ListPage, fetchOutcome) {
	eng := s.app.Engine.(SyncEngine)
	op := syncOps[r.Kind]
	coolings := 0
	for attempt := 0; ; attempt++ {
		if eng.PageState(ctx).Captcha {
			r.ErrorCode = string(douyin.ResponseVerificationRequired)
			return douyin.ListPage{}, fetchBlocked
		}
		reply, err := s.request(ctx, op, listParams(r.Kind, secUID, cursor))
		if ctx.Err() != nil {
			return douyin.ListPage{}, fetchCancel
		}
		page := douyin.ListPage{Class: douyin.ResponseNotReady}
		if err == nil {
			page = douyin.ParseList(op, reply)
		}
		log.Printf("[hoard:sync] %s page=%d class=%s http=%d code=%d items=%d more=%v", r.Kind, r.Pages+1, page.Class, reply.Status, page.Code, page.Received, page.HasMore)
		if page.Class != douyin.ResponseVerificationRequired && eng.PageState(ctx).Captcha {
			page.Class = douyin.ResponseVerificationRequired
		}
		r.ErrorCode = string(page.Class)
		switch page.Class {
		case douyin.ResponseOK:
			r.ErrorCode, r.Failures = "", 0
			return page, fetchOK
		case douyin.ResponseRateLimited:
			coolings++
			r.Failures++
			if coolings > 4 {
				return page, fetchPaused
			}
			delay := time.Minute << min(r.Failures-1, 3)
			if delay > 10*time.Minute {
				delay = 10 * time.Minute
			}
			retry := time.Now().Add(delay).Unix()
			r.Status, r.RetryAt = "cooling", &retry
			s.save(accountID, r)
			log.Printf("[hoard:sync] %s cooling %s", r.Kind, delay)
			if !wait(ctx, delay) {
				return page, fetchCancel
			}
			r.Status, r.RetryAt = "running", nil
			s.save(accountID, r)
		case douyin.ResponseVerificationRequired, douyin.ResponseRiskControl:
			return page, fetchBlocked
		case douyin.ResponseAuthRequired:
			return page, fetchSignedOut
		case douyin.ResponseAPIRejected:
			return page, fetchRejected
		default:
			if attempt >= 2 {
				return page, fetchPaused
			}
			if attempt == 1 {
				_ = eng.Recycle()
				s.waitReady(ctx)
			}
			if !wait(ctx, 3*time.Second*time.Duration(1+attempt*2)) {
				return page, fetchCancel
			}
		}
	}
}

func worksOf(items []douyin.Work) []store.Work {
	out := make([]store.Work, 0, len(items))
	for _, it := range items {
		extra, _ := json.Marshal(cachedMetadata{Author: it.Author, AwemeType: it.AwemeType, SourceURL: it.PlayURL, Images: it.Images, Covers: it.Covers})
		w := store.Work{RemoteID: it.ID, Kind: it.Kind, Title: it.Title, PublishedAt: it.PublishedAt, DurationMS: it.DurationMS,
			Width: it.Width, Height: it.Height, ImageCount: len(it.Images), IsLive: it.IsLive, CoverURL: it.Cover, Extra: extra}
		if it.AuthorID != "" {
			w.Author = &store.Author{RemoteID: it.AuthorID, Nickname: it.Author, AvatarURL: it.AuthorIcon}
		}
		out = append(out, w)
	}
	return out
}

func musicsOf(items []douyin.Music) []store.Music {
	out := make([]store.Music, 0, len(items))
	for _, m := range items {
		extra, _ := json.Marshal(map[string]any{"play_url": m.PlayURL, "covers": m.Covers})
		out = append(out, store.Music{RemoteID: m.ID, Title: m.Title, Author: m.Author, DurationMS: m.DurationMS, CoverURL: m.Cover, Extra: extra})
	}
	return out
}

func pageIDs(p douyin.ListPage) []string {
	ids := []string{}
	for _, w := range p.Works {
		ids = append(ids, w.ID)
	}
	for _, m := range p.Musics {
		ids = append(ids, m.ID)
	}
	return ids
}

func (s *Sync) store(accountID, kind string, head bool, p douyin.ListPage, revision uint64) error {
	s.app.mu.Lock()
	defer s.app.mu.Unlock()
	if s.app.actionRevision.Load() != revision {
		return errors.New("ACTIONS_CHANGED")
	}
	ctx, cancel := accountContext()
	defer cancel()
	var err error
	if kind == "music" {
		_, err = s.app.Store.SaveMusics(ctx, accountID, head, musicsOf(p.Musics))
	} else {
		_, err = s.app.Store.SaveWorks(ctx, accountID, kind, head, worksOf(p.Works))
	}
	if err == nil && s.app.Covers != nil {
		s.app.Covers.Prefetch(coverItems(p))
	}
	if err == nil {
		if e := libraryChanged.Broadcast(LibraryChange{AccountID: accountID, Kind: kind}); e != nil {
			log.Print("[hoard:sync] library event delivery failed")
		}
	}
	return err
}

func (s *Sync) pageLanded() {
	s.mu.Lock()
	hide := s.hideOnPage
	s.hideOnPage = false
	s.mu.Unlock()
	if hide {
		s.app.HideDouyin()
	}
}

// syncKind syncs up to budget backfill pages of one list. It reports whether
// the whole run must stop and whether the list has pages left for a next turn.
func (s *Sync) syncKind(ctx context.Context, accountID, secUID string, r *store.SyncRecord, budget int, head bool) (bool, bool) {
	revision := s.app.actionRevision.Load()
	if r.Kind == "music" && r.Status == "unsupported" && time.Now().Unix()-r.UpdatedAt < 24*3600 {
		s.emit(toView(accountID, *r))
		return false, false
	}
	r.Status, r.ErrorCode, r.RetryAt = "running", "", nil
	s.save(accountID, r)
	failed := func(outcome fetchOutcome) bool {
		switch outcome {
		case fetchCancel:
			r.Status = "idle"
			s.save(accountID, r)
			return true
		case fetchBlocked:
			r.Status = "blocked"
			s.save(accountID, r)
			log.Printf("[hoard:sync] %s blocked %s", r.Kind, r.ErrorCode)
			return true
		case fetchSignedOut:
			r.Status = "signed_out"
			s.save(accountID, r)
			s.app.mu.Lock()
			if s.app.accountID == accountID {
				_ = s.app.signedOutLocked(accountID)
			}
			s.app.mu.Unlock()
			return true
		case fetchRejected:
			r.Status = "paused"
			if r.Kind == "music" && r.Items == 0 {
				r.Status = "unsupported"
			}
		default:
			r.Status = "paused"
		}
		s.save(accountID, r)
		return false
	}
	fail := func(outcome fetchOutcome) (bool, bool) { return failed(outcome), false }
	recycle := func() {
		s.listPages++
		if s.listPages%segmentPages == 0 {
			log.Printf("[hoard:sync] %s segment recycle", r.Kind)
			_ = s.app.Engine.(SyncEngine).Recycle()
			s.waitReady(ctx)
		}
	}
	found := 0
	if head && r.Items > 0 {
		if r.Phase == "done" {
			r.Status = "checking"
			s.save(accountID, r)
		}
		cursor := "0"
		var fresh douyin.ListPage
		for i := 0; i < headPageLimit; i++ {
			page, outcome := s.fetch(ctx, accountID, r, secUID, cursor)
			if outcome != fetchOK {
				return fail(outcome)
			}
			recycle()
			s.pageLanded()
			sctx, cancel := accountContext()
			known, err := s.app.Store.KnownWorks(sctx, accountID, r.Kind, pageIDs(page))
			cancel()
			if err != nil {
				return fail(fetchPaused)
			}
			stop := false
			for _, w := range page.Works {
				if known[w.ID] {
					stop = true
					break
				}
				fresh.Works = append(fresh.Works, w)
			}
			for _, m := range page.Musics {
				if known[m.ID] {
					stop = true
					break
				}
				fresh.Musics = append(fresh.Musics, m)
			}
			if stop || !page.HasMore || page.Cursor == "" || page.Cursor == cursor {
				break
			}
			cursor = page.Cursor
		}
		if len(fresh.Works)+len(fresh.Musics) > 0 {
			if err := s.store(accountID, r.Kind, true, fresh, revision); err != nil {
				log.Printf("[hoard:sync] %s head write failed", r.Kind)
				return fail(fetchPaused)
			}
		}
		found = len(fresh.Works) + len(fresh.Musics)
		log.Printf("[hoard:sync] %s head new=%d", r.Kind, found)
		r.Status = "running"
		s.save(accountID, r)
	}
	if r.Phase != "done" {
		cursor := r.BackfillCursor
		if cursor == "" {
			cursor = "0"
		}
		r.Phase = "backfill"
		empty := 0
		for turn := 0; ; turn++ {
			if turn >= budget {
				r.Status = "queued"
				s.save(accountID, r)
				return false, true
			}
			page, outcome := s.fetch(ctx, accountID, r, secUID, cursor)
			if outcome != fetchOK {
				return fail(outcome)
			}
			s.pageLanded()
			if err := s.store(accountID, r.Kind, false, page, revision); err != nil {
				log.Printf("[hoard:sync] %s page write failed", r.Kind)
				return fail(fetchPaused)
			}
			r.Pages++
			if !page.HasMore {
				break
			}
			if page.Cursor == "" || page.Cursor == cursor {
				r.ErrorCode = "CURSOR_STALLED"
				return fail(fetchPaused)
			}
			if page.Received == 0 {
				if empty++; empty > 6 {
					r.ErrorCode = "EMPTY_PAGES"
					return fail(fetchPaused)
				}
			} else {
				empty = 0
			}
			cursor = page.Cursor
			r.BackfillCursor = cursor
			s.save(accountID, r)
			recycle()
		}
		r.BackfillCursor = ""
	}
	now := time.Now().Unix()
	r.Phase, r.Status, r.ErrorCode, r.LastFullAt, r.Failures = "done", "idle", "", &now, 0
	s.save(accountID, r)
	if found > 0 {
		v := toView(accountID, *r)
		v.Fresh = found
		s.emit(v)
	}
	log.Printf("[hoard:sync] %s done items=%d", r.Kind, r.Items)
	return false, false
}

// Verify shows the account's Douyin window for a slider and resumes the
// sync on its own once the page no longer shows one.
func (s *Sync) Verify() error {
	a := s.app
	eng, ok := a.Engine.(SyncEngine)
	if !ok {
		return errors.New("SYNC_UNAVAILABLE")
	}
	a.mu.Lock()
	accountID, ctx := a.accountID, a.sessionCtx
	a.mu.Unlock()
	if accountID == "" || ctx == nil {
		return errors.New("NO_ACCOUNT")
	}
	if _, err := a.EnsureSession(); err != nil {
		return err
	}
	if err := a.ShowDouyin(); err != nil {
		return err
	}
	s.mu.Lock()
	if s.verifying {
		s.mu.Unlock()
		return nil
	}
	s.verifying = true
	blocked := []SyncView{}
	for _, v := range s.views {
		if v.AccountID == accountID && v.Status == "blocked" {
			blocked = append(blocked, v)
		}
	}
	s.mu.Unlock()
	for _, v := range blocked {
		v.Status = "verifying"
		s.emit(v)
	}
	log.Print("[hoard:sync] verification window shown")
	go func() {
		defer func() { s.mu.Lock(); s.verifying = false; s.mu.Unlock() }()
		started := time.Now()
		clear := 0
		for time.Since(started) < 10*time.Minute {
			if !wait(ctx, time.Second) {
				return
			}
			if time.Since(started) > 3*time.Second && !eng.Visible() {
				log.Print("[hoard:sync] verification window closed")
				for _, v := range blocked {
					v.Status = "blocked"
					s.emit(v)
				}
				return
			}
			st := eng.PageState(ctx)
			if st.Ready && !st.Captcha {
				clear++
			} else {
				clear = 0
			}
			if clear >= 3 {
				log.Print("[hoard:sync] page clear, resuming")
				s.mu.Lock()
				s.hideOnPage = true
				s.mu.Unlock()
				if err := s.Start(); err != nil {
					log.Printf("[hoard:sync] resume failed: %v", err)
				}
				return
			}
		}
		for _, v := range blocked {
			v.Status = "blocked"
			s.emit(v)
		}
	}()
	return nil
}

// coverItems lists the covers of a synced page for the background prefetch.
func coverItems(p douyin.ListPage) []covers.Item {
	out := make([]covers.Item, 0, len(p.Works)+len(p.Musics))
	for _, w := range p.Works {
		urls := append([]string{w.Cover}, w.Covers...)
		if len(w.Images) > 0 {
			urls = append(urls, w.Images[0])
		}
		out = append(out, covers.Item{Kind: "w", ID: w.ID, URLs: compact(urls)})
	}
	for _, m := range p.Musics {
		out = append(out, covers.Item{Kind: "m", ID: m.ID, URLs: compact(append([]string{m.Cover}, m.Covers...))})
	}
	return out
}

func compact(urls []string) []string {
	out := urls[:0]
	seen := map[string]bool{}
	for _, u := range urls {
		if u != "" && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}
