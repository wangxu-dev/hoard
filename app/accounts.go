package app

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/google/uuid"
	"github.com/wangxu-dev/hoard/internal/douyin"
	"github.com/wangxu-dev/hoard/internal/store"
)

type Account struct {
	ID       string `json:"id"`
	Source   string `json:"source"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Status   string `json:"status"`
	Active   bool   `json:"active"`
}

type Accounts struct{ app *App }

var accountsChanged = mygo.NewEvent[[]Account]("accounts:changed")

func NewAccounts(a *App) *Accounts {
	a.changed = func(accounts []Account) {
		if err := accountsChanged.Broadcast(accounts); err != nil {
			log.Print("[hoard:accounts] event delivery failed")
		}
	}
	return &Accounts{app: a}
}

func accountView(account store.Account, active string) Account {
	return Account{ID: account.ID, Source: account.Source, Nickname: account.Nickname, Avatar: account.AvatarURL, Status: account.Status, Active: account.ID == active}
}

func accountContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

func (a *App) listLocked() ([]Account, error) {
	if a.Store == nil {
		return nil, errors.New("STORE_FAILED")
	}
	ctx, cancel := accountContext()
	defer cancel()
	accounts, err := a.Store.Accounts(ctx)
	if err != nil {
		return nil, errors.New("STORE_FAILED")
	}
	result := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, accountView(account, a.accountID))
	}
	return result, nil
}

func (a *App) emitLocked() {
	if a.changed == nil {
		return
	}
	accounts, err := a.listLocked()
	if err != nil {
		log.Print("[hoard:accounts] account refresh failed")
		return
	}
	a.changed(accounts)
}

func (s *Accounts) List() ([]Account, error) {
	s.app.mu.Lock()
	defer s.app.mu.Unlock()
	return s.app.listLocked()
}

func (s *Accounts) Add(source string) (Account, error) {
	s.app.mu.Lock()
	defer s.app.mu.Unlock()
	return s.app.addLocked(source)
}

func (a *App) addLocked(source string) (Account, error) {
	if source != "douyin" && source != "bilibili" {
		return Account{}, errors.New("UNSUPPORTED_SOURCE")
	}
	if a.Store == nil {
		return Account{}, errors.New("STORE_FAILED")
	}
	ctx, cancel := accountContext()
	defer cancel()
	account, err := a.Store.CreatePendingSourceAccount(ctx, uuid.NewString(), source)
	if err != nil {
		return Account{}, errors.New("STORE_FAILED")
	}
	if err := a.activateLocked(account, true); err != nil {
		return Account{}, err
	}
	a.emitLocked()
	return accountView(account, a.accountID), nil
}

func (s *Accounts) Switch(id string) (Account, error) {
	a := s.app
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.Store == nil {
		return Account{}, errors.New("STORE_FAILED")
	}
	ctx, cancel := accountContext()
	defer cancel()
	account, err := a.Store.Account(ctx, id)
	if err != nil {
		return Account{}, errors.New("ACCOUNT_NOT_FOUND")
	}
	if err := a.activateLocked(account, account.Status != "active"); err != nil {
		return Account{}, err
	}
	a.emitLocked()
	return accountView(account, a.accountID), nil
}

func (a *App) stopLocked() {
	if a.cancelSession != nil {
		a.cancelSession()
	}
	a.sessionCtx = nil
	a.Engine.Close()
	if a.Media != nil {
		a.Media.Clear()
	}
}

func (a *App) activateLocked(account store.Account, show bool) error {
	ctx, cancel := accountContext()
	defer cancel()
	if err := a.Store.ActivateAccount(ctx, account.ID); err != nil {
		return errors.New("STORE_FAILED")
	}
	a.stopLocked()
	a.accountID, a.uid, a.nickname = account.ID, account.RemoteID, account.Nickname
	a.sessionCtx, a.cancelSession = context.WithCancel(context.Background())
	var openErr error
	if engine, ok := a.Engine.(interface {
		UseSourceSession(string, string, bool) error
	}); ok {
		openErr = engine.UseSourceSession(account.Source, account.SessionName, show)
	} else if account.Source == "douyin" {
		openErr = a.Engine.UseSession(account.SessionName, show)
	} else {
		openErr = errors.New("UNSUPPORTED_SOURCE")
	}
	if openErr != nil {
		a.cancelSession()
		return errors.New("SESSION_OPEN_FAILED")
	}
	go a.watchIdentity(a.sessionCtx, show || account.Status == "pending")
	return nil
}

func (a *App) watchIdentity(ctx context.Context, waitForLogin bool) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if ctx.Err() != nil {
				return
			}
			state, err := a.CheckLogin()
			if ctx.Err() != nil {
				return
			}
			if err == nil && state.LoggedIn {
				if waitForLogin {
					go a.saveLogin(ctx)
				}
				if a.loggedIn != nil && ctx.Err() == nil {
					go a.loggedIn()
				}
				return
			}
			if err == nil && !waitForLogin && state.Detail == "AUTH_REQUIRED" {
				return
			}
		}
	}
}

func (a *App) saveLogin(ctx context.Context) {
	engine, ok := a.Engine.(interface{ SaveLogin(context.Context) bool })
	if !ok || !engine.SaveLogin(ctx) {
		if ctx.Err() == nil {
			log.Print("[hoard:accounts] save login not confirmed; window kept open")
		}
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if ctx.Err() == nil && a.sessionCtx == ctx {
		a.Engine.HideDouyin()
		log.Print("[hoard:accounts] save login prompt dismissed; window hidden")
	}
}

func (a *App) confirmLocked(identity douyin.Identity) error {
	ctx, cancel := accountContext()
	defer cancel()
	account, err := a.Store.Account(ctx, a.accountID)
	if err != nil {
		return errors.New("STORE_FAILED")
	}
	if !identity.LoggedIn {
		if identity.AuthRequired && account.Status != "pending" {
			return a.signedOutLocked(account.ID)
		}
		return nil
	}
	if identity.RemoteID == "" {
		return errors.New("INVALID_IDENTITY")
	}
	if account.Status != "pending" && account.RemoteID != identity.RemoteID && account.RemoteID != identity.UID {
		if err := a.signedOutLocked(account.ID); err != nil {
			return err
		}
		return errors.New("AUTH_REQUIRED")
	}
	if account.Status == "active" && account.RemoteID == identity.RemoteID && account.Nickname == identity.Nickname && account.AvatarURL == identity.Avatar {
		return nil
	}
	confirmed, err := a.Store.ConfirmAccount(ctx, account.ID, identity.RemoteID, identity.UID, identity.Nickname, identity.Avatar)
	if err != nil {
		return errors.New("STORE_FAILED")
	}
	if confirmed.ID != account.ID {
		a.stopLocked()
		a.accountID, a.uid, a.nickname = confirmed.ID, confirmed.RemoteID, confirmed.Nickname
		a.emitLocked()
		cleanupName, err := a.Store.Setting(ctx, "session_cleanup:"+confirmed.ID)
		if err != nil {
			return errors.New("STORE_FAILED")
		}
		if err := a.clearDeletedSessionLocked(confirmed.ID, cleanupName); err != nil {
			return err
		}
		if err := a.activateLocked(confirmed, false); err != nil {
			return err
		}
	} else {
		a.uid, a.nickname = identity.UID, identity.Nickname
	}
	a.emitLocked()
	return nil
}

func (a *App) signedOutLocked(id string) error {
	ctx, cancel := accountContext()
	defer cancel()
	if err := a.Store.SetAccountStatus(ctx, id, "signed_out"); err != nil {
		return errors.New("STORE_FAILED")
	}
	a.emitLocked()
	return nil
}

func (s *Accounts) SignOut(id string) error {
	a := s.app
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.Store == nil {
		return errors.New("STORE_FAILED")
	}
	ctx, cancel := accountContext()
	defer cancel()
	account, err := a.Store.Account(ctx, id)
	if err != nil {
		return errors.New("ACCOUNT_NOT_FOUND")
	}
	if id == a.accountID {
		a.stopLocked()
	}
	clearErr := a.Engine.ClearSession(account.SessionName)
	if err := a.signedOutLocked(id); err != nil {
		return err
	}
	if clearErr != nil {
		return errors.New("SESSION_CLEAR_FAILED")
	}
	return nil
}

func (s *Accounts) Remove(id string) error {
	a := s.app
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.Store == nil {
		return errors.New("STORE_FAILED")
	}
	ctx, cancel := accountContext()
	defer cancel()
	account, err := a.Store.Account(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		name, err := a.Store.Setting(ctx, "session_cleanup:"+id)
		if err != nil {
			return errors.New("ACCOUNT_NOT_FOUND")
		}
		if err := a.clearDeletedSessionLocked(id, name); err != nil {
			return err
		}
		if _, err := a.Store.CleanupOrphanWorks(ctx); err != nil {
			return errors.New("STORE_FAILED")
		}
		return nil
	}
	if err != nil {
		return errors.New("STORE_FAILED")
	}
	if id == a.accountID {
		a.stopLocked()
	}
	if err := a.Store.DeleteAccount(ctx, id); err != nil {
		return errors.New("STORE_FAILED")
	}
	if id == a.accountID {
		a.accountID, a.uid, a.nickname = "", "", ""
	}
	a.emitLocked()
	if err := a.clearDeletedSessionLocked(id, account.SessionName); err != nil {
		return err
	}
	if _, err := a.Store.CleanupOrphanWorks(ctx); err != nil {
		return errors.New("STORE_FAILED")
	}
	return nil
}

func (a *App) clearDeletedSessionLocked(id, name string) error {
	if err := a.Engine.ClearSession(name); err != nil {
		return errors.New("SESSION_CLEAR_FAILED")
	}
	ctx, cancel := accountContext()
	defer cancel()
	if err := a.Store.FinishSessionCleanup(ctx, id); err != nil {
		return errors.New("STORE_FAILED")
	}
	return nil
}

func (a *App) restoreLocked() error {
	if a.Store == nil {
		return errors.New("STORE_FAILED")
	}
	ctx, cancel := accountContext()
	defer cancel()
	cleanup, err := a.Store.SessionCleanups(ctx)
	if err != nil {
		return errors.New("STORE_FAILED")
	}
	cleanupPending := false
	for id, name := range cleanup {
		if err := a.clearDeletedSessionLocked(id, name); err != nil {
			cleanupPending = true
			log.Print("[hoard:accounts] session cleanup pending")
		}
	}
	if !cleanupPending {
		if _, err := a.Store.CleanupOrphanWorks(ctx); err != nil {
			return errors.New("STORE_FAILED")
		}
	}
	id, err := a.Store.Setting(ctx, "active_account")
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return errors.New("STORE_FAILED")
	}
	account, err := a.Store.Account(ctx, id)
	if err != nil {
		return errors.New("STORE_FAILED")
	}
	return a.activateLocked(account, false)
}

func RestoreAccounts(a *App) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.restoreLocked()
}

func CloseAccounts(a *App) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopLocked()
}
