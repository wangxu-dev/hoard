package app

import (
	"context"
	"errors"
	"log"
	"slices"
	"time"

	"github.com/wangxu-dev/hoard/internal/bilibili"
	"github.com/wangxu-dev/hoard/internal/douyin"
	"github.com/wangxu-dev/hoard/internal/source"
	"github.com/wangxu-dev/hoard/internal/store"
)

type actionEngine interface {
	UseSourceSession(string, string, bool) error
	Close()
	Status() douyin.Status
	Identity(context.Context) (douyin.Identity, error)
}

type ActionState struct {
	Liked      bool                    `json:"liked"`
	Favorited  bool                    `json:"favorited"`
	WatchLater bool                    `json:"watchLater"`
	Folders    []bilibili.ActionFolder `json:"folders"`
	awemeType  string
}

func (s ActionState) value(kind string) bool {
	switch kind {
	case "like":
		return s.Liked
	case "favorite":
		return s.Favorited
	case "watch_later":
		return s.WatchLater
	}
	return false
}

type Actions struct {
	app    *App
	engine func() actionEngine
}

func NewActions(a *App) *Actions {
	return &Actions{app: a, engine: func() actionEngine { return source.NewEngine() }}
}

func (s *Actions) State(accountID, workID string) (ActionState, error) {
	out := ActionState{}
	if s.app.Store == nil {
		return out, errors.New("STORE_FAILED")
	}
	ctx, cancel := accountContext()
	defer cancel()
	for _, kind := range []string{"like", "favorite", "watch_later"} {
		owners, err := s.app.Store.ActionOwners(ctx, store.LibraryQuery{AccountID: accountID, Kind: kind}, []string{workID})
		if err != nil {
			return out, errors.New("STORE_FAILED")
		}
		on := len(owners[workID]) > 0
		switch kind {
		case "like":
			out.Liked = on
		case "favorite":
			out.Favorited = on
		case "watch_later":
			out.WatchLater = on
		}
	}
	return out, nil
}

func (s *Actions) Set(accountID, workID, kind string, on bool) (ActionState, error) {
	if kind != "like" && kind != "favorite" && kind != "watch_later" {
		return ActionState{}, errors.New("INVALID_ARGUMENT")
	}
	return s.run(accountID, workID, kind, on, nil, true)
}

func (s *Actions) Folders(accountID, workID string) ([]bilibili.ActionFolder, error) {
	state, err := s.run(accountID, workID, "favorite", false, nil, false)
	return state.Folders, err
}

func (s *Actions) SetFolders(accountID, workID string, folderIDs []string) (ActionState, error) {
	if len(folderIDs) > 1000 {
		return ActionState{}, errors.New("INVALID_ARGUMENT")
	}
	selected := slices.Clone(folderIDs)
	slices.Sort(selected)
	if len(slices.Compact(selected)) != len(folderIDs) {
		return ActionState{}, errors.New("INVALID_ARGUMENT")
	}
	// An empty selection explicitly removes the video from all folders.
	if folderIDs == nil {
		folderIDs = []string{}
	}
	return s.run(accountID, workID, "favorite", len(folderIDs) > 0, folderIDs, true)
}

type actionSession struct {
	engine      actionEngine
	account     store.Account
	work        store.MediaRecord
	lastRequest time.Time
	aidValue    string
}

func (s *actionSession) request(ctx context.Context, call func() error) error {
	if delay := pageGap - time.Since(s.lastRequest); delay > 0 && !wait(ctx, delay) {
		return ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	err := call()
	s.lastRequest = time.Now()
	return err
}

func (s *Actions) run(accountID, workID, kind string, on bool, folderIDs []string, change bool) (result ActionState, resultErr error) {
	started := time.Now()
	stage := "account"
	defer func() {
		code := "OK"
		if resultErr != nil {
			code = resultErr.Error()
		}
		log.Printf("[hoard:actions] request finished kind=%s stage=%s code=%s elapsed=%s success=%t", kind, stage, code, time.Since(started).Round(time.Millisecond), resultErr == nil)
	}()
	empty := ActionState{}
	if accountID == "" {
		return empty, errors.New("INVALID_ARGUMENT")
	}
	a := s.app
	a.frontRequests.Add(1)
	defer a.frontRequests.Add(-1)
	a.actionMu.Lock()
	defer a.actionMu.Unlock()
	if a.Store == nil {
		return empty, errors.New("STORE_FAILED")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	account, err := a.Store.Account(ctx, accountID)
	if err != nil {
		return empty, errors.New("ACCOUNT_NOT_FOUND")
	}
	if account.Status != "active" || account.RemoteID == "" {
		return empty, errors.New("AUTH_REQUIRED")
	}
	if account.Source == "douyin" {
		if !douyin.ValidWorkID(workID) || kind == "watch_later" || folderIDs != nil || !change {
			return empty, errors.New("INVALID_ARGUMENT")
		}
	} else if account.Source == "bilibili" {
		if !bilibili.ValidWorkID(workID) {
			return empty, errors.New("INVALID_ARGUMENT")
		}
	} else {
		return empty, errors.New("UNSUPPORTED_SOURCE")
	}
	stage = "work"
	rec, err := a.Store.MediaRecord(ctx, "video", workID)
	if err != nil || rec.Source != account.Source {
		return empty, errors.New("UNAVAILABLE")
	}
	stage = "session"
	engine := s.engine()
	defer engine.Close()
	if err := engine.UseSourceSession(account.Source, account.SessionName, false); err != nil {
		return empty, errors.New("SESSION_OPEN_FAILED")
	}
	for !engine.Status().Ready {
		if !wait(ctx, 100*time.Millisecond) {
			return empty, errors.New("REQUEST_FAILED")
		}
	}
	stage = "identity"
	identity, err := engine.Identity(ctx)
	if err != nil || !identity.LoggedIn {
		return empty, errors.New("AUTH_REQUIRED")
	}
	if identity.RemoteID != account.RemoteID && identity.UID != account.RemoteID {
		return empty, errors.New("IDENTITY_MISMATCH")
	}
	session := actionSession{engine: engine, account: account, work: rec, lastRequest: time.Now()}
	stage = "read_before"
	before, err := session.read(ctx, kind)
	if err != nil {
		return empty, err
	}
	if !change {
		return before, nil
	}
	changed := before.value(kind) != on
	if folderIDs != nil {
		selected := []string{}
		for _, f := range before.Folders {
			if f.Selected {
				selected = append(selected, f.ID)
			}
		}
		selected = slices.Sorted(slices.Values(selected))
		folderIDs = slices.Sorted(slices.Values(folderIDs))
		changed = !slices.Equal(selected, folderIDs)
	}
	accountValid := func() bool {
		now, err := a.Store.Account(ctx, accountID)
		return err == nil && now.Source == account.Source && now.Status == "active" && now.SessionName == account.SessionName && now.RemoteID == account.RemoteID
	}
	if !accountValid() {
		return empty, errors.New("ACCOUNT_CHANGED")
	}
	after := before
	if changed {
		a.actionRevision.Add(1)
		// A timed-out write may have succeeded; read back without repeating it.
		stage = "write"
		writeErr := session.write(ctx, kind, on, folderIDs, before)
		if writeErr != nil {
			log.Print("[hoard:actions] write failed; checking remote state")
		}
		if errors.Is(writeErr, bilibili.ErrInvalid) {
			return empty, errors.New("INVALID_ARGUMENT")
		}
		stage = "read_after"
		after, err = session.read(ctx, kind)
		if err != nil {
			return empty, errors.New("ACTION_UNKNOWN")
		}
	}
	stage = "confirm"
	a.mu.Lock()
	defer a.mu.Unlock()
	if !accountValid() {
		return empty, errors.New("ACCOUNT_CHANGED")
	}
	confirmed := after.value(kind)
	if account.Source == "bilibili" {
		folders := []store.FolderSelection(nil)
		if kind == "favorite" && after.Folders != nil {
			folders = []store.FolderSelection{}
			for _, f := range after.Folders {
				folders = append(folders, store.FolderSelection{Folder: store.Folder{ID: f.ID, Title: f.Title, Count: f.Count}, Selected: f.Selected})
			}
		}
		err = a.Store.ConfirmBilibiliAction(ctx, accountID, workID, kind, confirmed, folders)
	} else {
		err = a.Store.ConfirmWorkAction(ctx, accountID, workID, kind, confirmed)
	}
	if err != nil {
		return empty, errors.New("STORE_FAILED")
	}
	a.actionRevision.Add(1)
	if err := libraryChanged.Broadcast(LibraryChange{AccountID: accountID, Kind: kind}); err != nil {
		log.Print("[hoard:actions] event delivery failed")
	}
	if confirmed != on {
		return after, errors.New("ACTION_NOT_APPLIED")
	}
	if folderIDs != nil {
		selected := []string{}
		for _, f := range after.Folders {
			if f.Selected {
				selected = append(selected, f.ID)
			}
		}
		slices.Sort(selected)
		if !slices.Equal(selected, folderIDs) {
			return after, errors.New("ACTION_NOT_APPLIED")
		}
	}
	return after, nil
}
