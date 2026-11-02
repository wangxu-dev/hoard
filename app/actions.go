package app

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/wangxu-dev/hoard/internal/douyin"
	"github.com/wangxu-dev/hoard/internal/store"
)

type actionEngine interface {
	UseSession(string, bool) error
	Close()
	Status() douyin.Status
	Identity(context.Context) (douyin.Identity, error)
	Request(context.Context, douyin.Operation, map[string]string) (douyin.PageReply, error)
}

type Actions struct {
	app    *App
	engine func() actionEngine
}

func NewActions(a *App) *Actions {
	return &Actions{app: a, engine: func() actionEngine { return &douyin.Engine{} }}
}

func (s *Actions) State(accountID, workID string) (douyin.ActionState, error) {
	returnState := douyin.ActionState{}
	if s.app.Store == nil {
		return returnState, errors.New("STORE_FAILED")
	}
	ctx, cancel := accountContext()
	defer cancel()
	for _, kind := range []string{"like", "favorite"} {
		owners, err := s.app.Store.LibraryOwners(ctx, store.LibraryQuery{AccountID: accountID, Kind: kind}, []string{workID})
		if err != nil {
			return returnState, errors.New("STORE_FAILED")
		}
		if kind == "like" {
			returnState.Liked = len(owners[workID]) > 0
		} else {
			returnState.Favorited = len(owners[workID]) > 0
		}
	}
	return returnState, nil
}
func (s *Actions) Set(accountID, workID, kind string, on bool) (douyin.ActionState, error) {
	if kind != "like" && kind != "favorite" {
		return douyin.ActionState{}, errors.New("INVALID_ARGUMENT")
	}
	return s.run(accountID, workID, kind, on)
}

func (s *Actions) run(accountID, workID, kind string, on bool) (result douyin.ActionState, resultErr error) {
	started := time.Now()
	log.Print("[hoard:actions] request queued")
	defer func() {
		log.Printf("[hoard:actions] request finished elapsed=%s success=%t", time.Since(started).Round(time.Millisecond), resultErr == nil)
	}()
	empty := douyin.ActionState{}
	if accountID == "" || !douyin.ValidWorkID(workID) {
		return empty, errors.New("INVALID_ARGUMENT")
	}
	a := s.app
	// Serialize platform operations without holding the application mutex during network waits.
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
	if account.Source != "douyin" || account.Status != "active" || account.RemoteID == "" {
		return empty, errors.New("AUTH_REQUIRED")
	}
	if _, err := a.Store.MediaRecord(ctx, "video", workID); err != nil {
		return empty, errors.New("UNAVAILABLE")
	}
	engine := s.engine()
	defer engine.Close()
	if err := engine.UseSession(account.SessionName, false); err != nil {
		return empty, errors.New("SESSION_OPEN_FAILED")
	}
	for !engine.Status().Ready {
		if !wait(ctx, 100*time.Millisecond) {
			return empty, errors.New("REQUEST_FAILED")
		}
	}
	identity, err := engine.Identity(ctx)
	lastRequest := time.Now()
	if err != nil || !identity.LoggedIn {
		return empty, errors.New("AUTH_REQUIRED")
	}
	if identity.RemoteID != account.RemoteID && identity.UID != account.RemoteID {
		return empty, errors.New("IDENTITY_MISMATCH")
	}
	request := func(op douyin.Operation, params map[string]string) (douyin.PageReply, error) {
		if delay := 1200*time.Millisecond - time.Since(lastRequest); delay > 0 && !wait(ctx, delay) {
			return douyin.PageReply{}, ctx.Err()
		}
		reply, err := engine.Request(ctx, op, params)
		lastRequest = time.Now()
		return reply, err
	}
	read := func() (douyin.ActionState, error) {
		reply, err := request(douyin.OpDetail, map[string]string{"aweme_id": workID})
		if err != nil {
			return empty, errors.New("REQUEST_FAILED")
		}
		return douyin.ParseActionState(reply, workID)
	}
	before, err := read()
	if err != nil {
		return empty, err
	}
	if kind == "" {
		return before, nil
	}
	current := before.Liked
	if kind == "favorite" {
		current = before.Favorited
	}
	after := before
	if current != on {
		op := douyin.OpSetLike
		value := "0"
		if on {
			value = "1"
		}
		params := map[string]string{"aweme_id": workID, "item_type": "0", "type": value}
		if kind == "favorite" {
			op = douyin.OpSetFavorite
			params = map[string]string{"aweme_id": workID, "action": value, "aweme_type": before.AwemeType}
		}
		accountNow, accountErr := a.Store.Account(ctx, accountID)
		if accountErr != nil || accountNow.Status != "active" || accountNow.SessionName != account.SessionName || accountNow.RemoteID != account.RemoteID {
			return empty, errors.New("ACCOUNT_CHANGED")
		}
		// Even a timed-out write may have succeeded. Never repeat it; verify by reading.
		a.actionRevision.Add(1)
		_, _ = request(op, params)
		after, err = read()
		if err != nil {
			return empty, errors.New("ACTION_UNKNOWN")
		}
	}
	confirmed := after.Liked
	if kind == "favorite" {
		confirmed = after.Favorited
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	accountNow, accountErr := a.Store.Account(ctx, accountID)
	if accountErr != nil || accountNow.SessionName != account.SessionName || accountNow.Status != "active" {
		return empty, errors.New("ACCOUNT_CHANGED")
	}
	if err := a.Store.ConfirmWorkAction(ctx, accountID, workID, kind, confirmed); err != nil {
		return empty, errors.New("STORE_FAILED")
	}
	a.actionRevision.Add(1)
	if err := libraryChanged.Broadcast(LibraryChange{AccountID: accountID, Kind: kind}); err != nil {
		log.Print("[hoard:actions] event delivery failed")
	}
	if confirmed != on {
		return after, errors.New("ACTION_NOT_APPLIED")
	}
	return after, nil
}
