package app

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/egoist/mygo"
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

var syncKinds = []string{"like", "favorite", "recommend", "music"}

var syncOps = map[string]douyin.Operation{"like": douyin.OpLikes, "favorite": douyin.OpFavorites, "recommend": douyin.OpRecommend, "music": douyin.OpMusic}

const (
	pageGap       = 1200 * time.Millisecond
	douyinSyncGap = 300 * time.Millisecond
	headPageLimit = 30
	segmentPages  = 24
	turnPages     = 10
	retryDelay    = 2 * time.Minute
)

// Sync pulls likes, favorites and music of the active account into the library.
type Sync struct {
	app        *App
	mu         sync.Mutex
	generation int
	running    string
	runCtx     context.Context
	cancel     context.CancelFunc
	views      map[string]SyncView
	verifying  bool
	hideOnPage bool
	listPages  int
}

func NewSync(a *App) *Sync {
	s := &Sync{app: a, views: map[string]SyncView{}}
	a.loggedIn = func() {
		if err := s.AutoStart(); err != nil {
			log.Printf("[hoard:sync] auto start skipped: %v", err)
		}
	}
	a.loginBlocked = func(ctx context.Context) {
		a.mu.Lock()
		id, current := a.accountID, a.sessionCtx
		a.mu.Unlock()
		if current != ctx || ctx.Err() != nil {
			return
		}
		s.mu.Lock()
		status := s.views[viewKey(id, "like")].Status
		s.mu.Unlock()
		if status == "blocked" || status == "verifying" {
			return
		}
		records, err := a.Store.SyncRecords(ctx, id)
		if err != nil {
			return
		}
		byKind := map[string]store.SyncRecord{}
		for _, record := range records {
			byKind[record.Kind] = record
		}
		for _, kind := range syncKinds {
			record := byKind[kind]
			record.Kind = kind
			view := toView(id, record)
			view.Status, view.ErrorCode = "blocked", "NEED_VERIFY"
			s.emit(view)
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
		kinds := syncKinds
		if account.Source == "bilibili" {
			kinds = []string{"favorite", "watch_later"}
		}
		records, err := a.Store.SyncRecords(ctx, account.ID)
		if err != nil {
			return nil, errors.New("STORE_FAILED")
		}
		byKind := map[string]store.SyncRecord{}
		for _, r := range records {
			byKind[r.Kind] = r
		}
		for _, kind := range kinds {
			if v, ok := s.views[viewKey(account.ID, kind)]; ok && (s.running == account.ID || v.Status == "blocked" || v.Status == "verifying") {
				out = append(out, v)
				continue
			}
			r, ok := byKind[kind]
			if !ok {
				r = store.SyncRecord{Kind: kind, Phase: "idle", Status: "idle"}
			}
			v := toView(account.ID, r)
			if v.Status == "running" || v.Status == "queued" || v.Status == "checking" || (v.Status == "cooling" && v.RetryAt <= time.Now().Unix()) {
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

// AutoStart applies the per-account freshness rule after session restoration.
func (s *Sync) AutoStart() error { return s.start(true) }

// Start explicitly syncs the active account, regardless of freshness.
func (s *Sync) Start() error { return s.start(false) }

func (s *Sync) start(automatic bool) error {
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
	if automatic {
		account, err := a.Store.Account(ctx, accountID)
		if err != nil {
			return errors.New("STORE_FAILED")
		}
		records, err := a.Store.SyncRecords(ctx, accountID)
		if err != nil {
			return errors.New("STORE_FAILED")
		}
		if !automaticSyncDue(account.Source, records, time.Now().Unix()) {
			log.Print("[hoard:sync] auto start skipped: account synced within 180 seconds")
			return nil
		}
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
		account, err := a.Store.Account(runCtx, accountID)
		if err != nil {
			return
		}
		for runCtx.Err() == nil {
			if account.Source == "bilibili" {
				s.runBilibili(runCtx, accountID)
			} else {
				s.run(runCtx, accountID)
			}
			if runCtx.Err() != nil || !s.unfinished(accountID) {
				return
			}
			if !wait(runCtx, retryDelay) {
				return
			}
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
		if (r.Phase != "done" || r.Status == "paused" || r.Status == "cooling") && r.Status != "unsupported" {
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
