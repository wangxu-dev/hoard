package app

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/wangxu-dev/hoard/internal/store"
)

type playbackSession struct {
	account store.Account
	player  *Player
	cancel  context.CancelFunc
}

// Resolve uses a work owner's session without switching the account being synced.
func (p *Player) Resolve(kind, id, accountID string, refresh bool, partID string) (MediaResult, error) {
	if p.app.Store == nil || p.app.Media == nil {
		return MediaResult{}, errors.New("UNAVAILABLE")
	}
	if kind != "video" && kind != "gallery" && kind != "music" || id == "" {
		return MediaResult{}, errors.New("INVALID_ID")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rec, err := p.app.Store.MediaRecord(ctx, kind, id)
	if err != nil {
		return MediaResult{}, errors.New("UNAVAILABLE")
	}
	// Saved music only needs the local relay; it has no account-specific detail call.
	if kind == "music" {
		return p.media(kind, id, refresh)
	}
	accounts, err := p.app.Store.PlaybackAccounts(ctx, rec.Source, id)
	if err != nil {
		return MediaResult{}, errors.New("STORE_FAILED")
	}
	p.app.mu.Lock()
	active := p.app.accountID
	p.app.mu.Unlock()
	account, err := playbackAccount(accounts, accountID, active)
	if err != nil {
		return MediaResult{}, err
	}
	log.Printf("[hoard:player] session source=%s scoped=%t active_match=%t", account.Source, accountID != "", account.ID == active)
	if kind == "gallery" && !refresh {
		if detail, ok := storedImages(rec); ok {
			return p.result(rec, detail), nil
		}
	}
	session, err := p.playbackSession(account)
	if err != nil {
		return MediaResult{}, err
	}
	current, err := p.app.Store.Account(ctx, account.ID)
	if err != nil || current.Status != "active" || current.SessionName != account.SessionName || current.RemoteID != account.RemoteID {
		return MediaResult{}, errors.New("ACCOUNT_CHANGED")
	}
	for !session.player.app.Engine.Status().Ready {
		if session.player.app.sessionCtx.Err() != nil {
			return MediaResult{}, errors.New("ACCOUNT_CHANGED")
		}
		if !wait(ctx, 100*time.Millisecond) {
			return MediaResult{}, errors.New("SESSION_NOT_READY")
		}
	}
	return session.player.media(kind, id, refresh, partID)
}

func playbackAccount(accounts []store.Account, requested, active string) (store.Account, error) {
	var first store.Account
	for _, account := range accounts {
		if requested != "" {
			if account.ID != requested {
				continue
			}
			if account.Status != "active" {
				return store.Account{}, errors.New("AUTH_REQUIRED")
			}
			return account, nil
		}
		if account.Status != "active" {
			continue
		}
		if account.ID == active {
			return account, nil
		}
		if first.ID == "" {
			first = account
		}
	}
	if requested != "" {
		return store.Account{}, errors.New("UNAVAILABLE")
	}
	if first.ID != "" {
		return first, nil
	}
	return store.Account{}, errors.New("AUTH_REQUIRED")
}

func (p *Player) playbackSession(account store.Account) (*playbackSession, error) {
	p.sessionsMu.Lock()
	defer p.sessionsMu.Unlock()
	if old := p.sessions[account.ID]; old != nil {
		if old.account.SessionName == account.SessionName && old.account.RemoteID == account.RemoteID {
			return old, nil
		}
		old.cancel()
		old.player.app.Engine.Close()
		delete(p.sessions, account.ID)
	}
	engine := p.engine()
	opener, ok := engine.(interface {
		UseSourceSession(string, string, bool) error
	})
	var err error
	if ok {
		err = opener.UseSourceSession(account.Source, account.SessionName, false)
	} else if account.Source == "douyin" {
		err = engine.UseSession(account.SessionName, false)
	} else {
		err = errors.New("UNSUPPORTED_SOURCE")
	}
	if err != nil {
		engine.Close()
		return nil, errors.New("SESSION_OPEN_FAILED")
	}
	ctx, cancel := context.WithCancel(context.Background())
	app := &App{Store: p.app.Store, Media: p.app.Media, Covers: p.app.Covers, Engine: engine, accountID: account.ID, sessionCtx: ctx, cancelSession: cancel}
	worker := &Player{app: app, gate: p.app, inflight: map[string]*playerCall{}}
	session := &playbackSession{account: account, player: worker, cancel: cancel}
	p.sessions[account.ID] = session
	return session, nil
}

func (p *Player) closeAccount(id string) {
	p.sessionsMu.Lock()
	defer p.sessionsMu.Unlock()
	for key, session := range p.sessions {
		if id != "" && key != id {
			continue
		}
		session.cancel()
		session.player.app.Engine.Close()
		delete(p.sessions, key)
	}
}
