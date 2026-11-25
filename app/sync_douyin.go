package app

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"slices"
	"time"

	"github.com/wangxu-dev/hoard/internal/covers"
	"github.com/wangxu-dev/hoard/internal/douyin"
	"github.com/wangxu-dev/hoard/internal/store"
	"github.com/wangxu-dev/hoard/internal/syncpolicy"
)

// request sends one background request through the shared interactive-priority lane.
func (s *Sync) request(ctx context.Context, op douyin.Operation, params map[string]string, gap time.Duration) (douyin.PageReply, error) {
	eng := s.app.Engine.(SyncEngine)
	if err := s.lockBackground(ctx, gap); err != nil {
		return douyin.PageReply{}, err
	}
	defer s.app.requestMu.Unlock()
	if ctx.Err() != nil {
		return douyin.PageReply{}, ctx.Err()
	}
	reply, err := eng.Request(ctx, op, params)
	s.app.lastRequest = time.Now()
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
		reply, err := s.request(ctx, douyin.OpSelf, nil, douyinSyncGap)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			profile = douyin.Profile{Class: douyin.ResponseNotReady}
		} else {
			profile = douyin.ParseProfile(reply)
		}
		log.Printf("[hoard:sync] self class=%s %s", profile.Class, douyin.ProfileDiagnostics(reply))
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
	if ctx.Err() == nil {
		s.syncFolders(ctx, accountID)
	}
}

func (s *Sync) syncFolders(ctx context.Context, accountID string) {
	cursor := "0"
	var folders []douyin.Folder
	seenFolders := map[string]bool{}
	enumerated := false
	for pageNum := 0; pageNum < 20; pageNum++ {
		reply, err := s.request(ctx, douyin.OpFolders, map[string]string{"cursor": cursor, "count": listPageSize}, douyinSyncGap)
		if ctx.Err() != nil {
			log.Printf("[hoard:sync] folders paused class=%s", douyin.ResponseNotReady)
			return
		}
		var page douyin.FolderPage
		if err != nil {
			page.Class = douyin.ResponseNotReady
		} else {
			page = douyin.ParseFolders(reply)
		}
		if page.Class != douyin.ResponseOK {
			log.Printf("[hoard:sync] folders paused class=%s", page.Class)
			return
		}
		for _, f := range page.Folders {
			if !seenFolders[f.ID] {
				seenFolders[f.ID] = true
				folders = append(folders, f)
			}
		}
		if !page.HasMore || page.Cursor == "" || page.Cursor == cursor {
			enumerated = true
			break
		}
		cursor = page.Cursor
	}
	keepIDs := make([]string, 0, len(folders))
	position := 0
	for _, f := range folders {
		if ctx.Err() != nil {
			return
		}
		itemCursor := "0"
		var members []store.FolderWork
		seenWorks := map[string]bool{}
		folderComplete := false
		for pageNum := 0; pageNum < 50; pageNum++ {
			reply, err := s.request(ctx, douyin.OpFolderItems, map[string]string{
				"collects_id": f.ID,
				"cursor":      itemCursor,
				"count":       listPageSize,
			}, douyinSyncGap)
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				break
			}
			page := douyin.ParseList(douyin.OpFolderItems, reply)
			if page.Class != douyin.ResponseOK {
				break
			}
			if len(page.Works) > 0 {
				sctx, cancel := accountContext()
				_, saveErr := s.app.Store.SaveWorks(sctx, accountID, "favorite", false, worksOf(page.Works))
				cancel()
				if saveErr != nil {
					log.Printf("[hoard:sync] folder works write failed")
					break
				}
				if s.app.Covers != nil {
					s.app.Covers.Prefetch(coverItems(page))
				}
				for _, w := range page.Works {
					if !seenWorks[w.ID] {
						seenWorks[w.ID] = true
						members = append(members, store.FolderWork{RemoteID: w.ID, FavoritedAt: 0})
					}
				}
			}
			if !page.HasMore {
				folderComplete = true
				break
			}
			if page.Cursor == "" || page.Cursor == itemCursor {
				break
			}
			itemCursor = page.Cursor
		}
		if folderComplete {
			sctx, cancel := accountContext()
			err := s.app.Store.ReplaceFolderMembership(sctx, accountID, store.Folder{
				ID:       f.ID,
				Title:    f.Title,
				Count:    f.Count,
				Kind:     "favorite",
				Position: position,
			}, members)
			cancel()
			if err != nil {
				log.Printf("[hoard:sync] folder membership write failed")
			}
		}
		position++
		keepIDs = append(keepIDs, f.ID)
	}
	if ctx.Err() != nil {
		return
	}
	if enumerated {
		sctx, cancel := accountContext()
		if err := s.app.Store.PruneFolders(sctx, accountID, keepIDs); err != nil {
			log.Printf("[hoard:sync] folders prune failed")
		}
		cancel()
	}
	if err := libraryChanged.Broadcast(LibraryChange{AccountID: accountID, Kind: "favorite"}); err != nil {
		log.Print("[hoard:sync] library event delivery failed")
	}
	log.Printf("[hoard:sync] folders done count=%d", len(keepIDs))
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
	likePageSize      = "50"
	listPageSize      = "20"
	recommendPageSize = "18"
)

func listParams(kind, secUID, cursor string) map[string]string {
	switch kind {
	case "like":
		return map[string]string{"sec_user_id": secUID, "max_cursor": cursor, "min_cursor": "0", "count": likePageSize, "publish_video_strategy_type": "2", "pc_client_type": "1"}
	case "recommend":
		return map[string]string{
			"sec_user_id":     secUID,
			"max_cursor":      cursor,
			"min_cursor":      "0",
			"whale_cut_token": "",
			"count":           recommendPageSize,
			"from":            "1",
			"pc_client_type":  "1",
		}
	default:
		return map[string]string{"cursor": cursor, "count": listPageSize}
	}
}

func (s *Sync) fetch(ctx context.Context, accountID string, r *store.SyncRecord, secUID, cursor string) (douyin.ListPage, fetchOutcome) {
	eng := s.app.Engine.(SyncEngine)
	op := syncOps[r.Kind]
	coolings := 0
	for attempt := 0; ; attempt++ {
		if !s.resumeCooling(ctx, accountID, r) {
			return douyin.ListPage{}, fetchCancel
		}
		if eng.PageState(ctx).Captcha {
			r.ErrorCode = string(douyin.ResponseVerificationRequired)
			return douyin.ListPage{}, fetchBlocked
		}
		started := time.Now()
		reply, err := s.request(ctx, op, listParams(r.Kind, secUID, cursor), syncpolicy.Gap(r.DelayMS, douyinSyncGap))
		if ctx.Err() != nil {
			return douyin.ListPage{}, fetchCancel
		}
		page := douyin.ListPage{Class: douyin.ResponseNotReady}
		if err == nil {
			page = douyin.ParseList(op, reply)
		}
		log.Printf("[hoard:sync] %s page=%d class=%s http=%d code=%d items=%d more=%v elapsed_ms=%d delay_ms=%d", r.Kind, r.Pages+1, page.Class, reply.Status, page.Code, page.Received, page.HasMore, time.Since(started).Milliseconds(), r.DelayMS)
		if r.Kind == "recommend" && page.Class != douyin.ResponseOK {
			log.Printf("[hoard:sync] recommend shape %s", douyin.ListShapeDiagnostics(reply))
		}
		if page.Class != douyin.ResponseVerificationRequired && eng.PageState(ctx).Captcha {
			page.Class = douyin.ResponseVerificationRequired
		}
		r.ErrorCode = string(page.Class)
		switch page.Class {
		case douyin.ResponseOK:
			r.ErrorCode = ""
			recoverPace(r, douyinSyncGap)
			return page, fetchOK
		case douyin.ResponseRateLimited:
			coolings++
			s.slow(accountID, r, douyinSyncGap)
			if coolings >= 4 {
				return page, fetchPaused
			}
		case douyin.ResponseVerificationRequired:
			return page, fetchBlocked
		case douyin.ResponseRiskControl:
			s.slow(accountID, r, douyinSyncGap)
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

func samePage(a, b douyin.ListPage) bool {
	x, y := pageIDs(a), pageIDs(b)
	slices.Sort(x)
	slices.Sort(y)
	return slices.Equal(x, y)
}

func (s *Sync) store(accountID, kind string, head bool, p douyin.ListPage, revision uint64, record *store.SyncRecord) error {
	s.app.mu.Lock()
	defer s.app.mu.Unlock()
	if s.app.actionRevision.Load() != revision {
		return errors.New("ACTIONS_CHANGED")
	}
	ctx, cancel := accountContext()
	defer cancel()
	var err error
	if record != nil {
		err = s.app.Store.SaveSyncPage(ctx, accountID, head, worksOf(p.Works), musicsOf(p.Musics), *record)
	} else if kind == "music" {
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

func (s *Sync) reconcileHead(accountID, kind string, seenIDs []string, complete bool, revision uint64) (int, error) {
	s.app.mu.Lock()
	defer s.app.mu.Unlock()
	if s.app.actionRevision.Load() != revision {
		return 0, errors.New("ACTIONS_CHANGED")
	}
	ctx, cancel := accountContext()
	defer cancel()
	removed, err := s.app.Store.ReconcileHeadRelations(ctx, accountID, kind, seenIDs, complete)
	if err == nil && removed > 0 {
		if e := libraryChanged.Broadcast(LibraryChange{AccountID: accountID, Kind: kind}); e != nil {
			log.Print("[hoard:sync] library event delivery failed")
		}
	}
	return removed, err
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
	if !s.resumeCooling(ctx, accountID, r) {
		return true, false
	}
	r.Status, r.ErrorCode = "running", ""
	s.save(accountID, r)
	failed := func(outcome fetchOutcome) bool {
		switch outcome {
		case fetchCancel:
			r.Status = "idle"
			if r.RetryAt != nil && *r.RetryAt > time.Now().Unix() {
				r.Status = "cooling"
			}
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
		var seenIDs []string
		hitKnown := false
		headComplete := false
		for i := 0; i < headPageLimit; i++ {
			page, outcome := s.fetch(ctx, accountID, r, secUID, cursor)
			if outcome != fetchOK {
				return fail(outcome)
			}
			recycle()
			s.pageLanded()
			ids := pageIDs(page)
			seenIDs = append(seenIDs, ids...)
			sctx, cancel := accountContext()
			known, err := s.app.Store.KnownWorks(sctx, accountID, r.Kind, ids)
			cancel()
			if err != nil {
				return fail(fetchPaused)
			}
			stop := len(page.Works)+len(page.Musics) > 0
			for _, w := range page.Works {
				if known[w.ID] {
					hitKnown = true
				} else {
					stop = false
					fresh.Works = append(fresh.Works, w)
				}
			}
			for _, m := range page.Musics {
				if known[m.ID] {
					hitKnown = true
				} else {
					stop = false
					fresh.Musics = append(fresh.Musics, m)
				}
			}
			if stop || !page.HasMore || page.Cursor == "" || page.Cursor == cursor {
				headComplete = !page.HasMore
				break
			}
			cursor = page.Cursor
		}
		if len(fresh.Works)+len(fresh.Musics) > 0 {
			if err := s.store(accountID, r.Kind, true, fresh, revision, nil); err != nil {
				log.Printf("[hoard:sync] %s head write failed", r.Kind)
				return fail(fetchPaused)
			}
		}
		if hitKnown || headComplete {
			if removed, err := s.reconcileHead(accountID, r.Kind, seenIDs, headComplete, revision); err != nil {
				log.Printf("[hoard:sync] %s head reconcile failed", r.Kind)
			} else if removed > 0 {
				log.Printf("[hoard:sync] %s head removed=%d", r.Kind, removed)
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
		seenPages := map[string]bool{}
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
			if !page.HasMore {
				confirmed, outcome := s.fetch(ctx, accountID, r, secUID, cursor)
				if outcome != fetchOK {
					return fail(outcome)
				}
				if confirmed.HasMore || !samePage(page, confirmed) {
					r.ErrorCode = "END_UNCONFIRMED"
					return fail(fetchPaused)
				}
			}
			fingerprint, _ := json.Marshal(pageIDs(page))
			if page.Received > 0 && seenPages[string(fingerprint)] {
				r.ErrorCode = "CURSOR_STALLED"
				return fail(fetchPaused)
			}
			seenPages[string(fingerprint)] = true
			if page.HasMore && (page.Cursor == "" || page.Cursor == cursor) {
				r.ErrorCode = "CURSOR_STALLED"
				return fail(fetchPaused)
			}
			next := *r
			next.Pages++
			if page.Received > 0 || !page.HasMore {
				next.BackfillCursor = page.Cursor
			}
			if !page.HasMore {
				now := time.Now().Unix()
				next.Phase, next.Status, next.BackfillCursor, next.LastFullAt = "done", "idle", "", &now
			}
			if err := s.store(accountID, r.Kind, false, page, revision, &next); err != nil {
				log.Printf("[hoard:sync] %s page write failed", r.Kind)
				return fail(fetchPaused)
			}
			*r = next
			if !page.HasMore {
				break
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
