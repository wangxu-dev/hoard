package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/wangxu-dev/hoard/internal/douyin"
	"github.com/wangxu-dev/hoard/internal/relay"
	"github.com/wangxu-dev/hoard/internal/store"
)

func rememberForTest(a *App, state douyin.LoginState) error {
	if !state.LoggedIn {
		return nil
	}
	ctx := context.Background()
	pending, err := a.Store.CreatePendingAccount(ctx, uuid.NewString())
	if err != nil {
		return err
	}
	account, err := a.Store.ConfirmAccount(ctx, pending.ID, state.UID, "", state.Nickname, "")
	if err != nil {
		return err
	}
	if err := a.Store.ActivateAccount(ctx, account.ID); err != nil {
		return err
	}
	a.accountID, a.uid, a.nickname = account.ID, state.UID, state.Nickname
	return nil
}

type fakeSession struct {
	identity douyin.Identity
	data     string
	clears   int
}
type fakeAccountEngine struct {
	mu             sync.Mutex
	sessions       map[string]*fakeSession
	currentSource  string
	current        string
	open, shown    bool
	closes         int
	clearError     bool
	requestStarted chan struct{}
	releaseRequest chan struct{}
}

func (f *fakeAccountEngine) UseSourceSession(source, name string, show bool) error {
	f.mu.Lock()
	f.currentSource = source
	f.mu.Unlock()
	return f.UseSession(name, show)
}
func (f *fakeAccountEngine) UseSession(name string, show bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sessions[name] == nil {
		f.sessions[name] = &fakeSession{}
	}
	f.current, f.open, f.shown = name, true, show
	return nil
}
func (f *fakeAccountEngine) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.open {
		f.closes++
	}
	f.open = false
}
func (f *fakeAccountEngine) ClearSession(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.clearError {
		return errors.New("fake clear failure")
	}
	if f.sessions[name] == nil {
		f.sessions[name] = &fakeSession{}
	}
	f.sessions[name].data = ""
	f.sessions[name].identity = douyin.Identity{AuthRequired: true}
	f.sessions[name].clears++
	return nil
}
func (f *fakeAccountEngine) Identity(ctx context.Context) (douyin.Identity, error) {
	f.mu.Lock()
	state := f.sessions[f.current].identity
	started, release := f.requestStarted, f.releaseRequest
	f.requestStarted = nil
	f.mu.Unlock()
	if started != nil {
		close(started)
		<-release
	}
	return state, nil
}
func (f *fakeAccountEngine) FetchLikesContext(ctx context.Context, cursor string) (douyin.LikesResult, error) {
	f.mu.Lock()
	started, release := f.requestStarted, f.releaseRequest
	f.requestStarted = nil
	f.mu.Unlock()
	if started != nil {
		close(started)
		<-release
	}
	return douyin.LikesResult{OK: true, Items: []douyin.LikeItem{{ID: "late-work", Images: []string{}}}}, nil
}
func (f *fakeAccountEngine) OpenDouyin() (douyin.Status, error) {
	f.mu.Lock()
	f.shown = true
	f.mu.Unlock()
	return f.Status(), nil
}
func (f *fakeAccountEngine) EnsureSession() (douyin.Status, error) { return f.Status(), nil }
func (f *fakeAccountEngine) ShowDouyin() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shown = true
	return nil
}
func (f *fakeAccountEngine) HideDouyin() { f.mu.Lock(); defer f.mu.Unlock(); f.shown = false }
func (f *fakeAccountEngine) Status() douyin.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	return douyin.Status{Open: f.open, Ready: f.open}
}

func newAccountTestApp(t *testing.T) (*App, *Accounts, *fakeAccountEngine) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	media, err := relay.New()
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeAccountEngine{sessions: map[string]*fakeSession{}}
	a := &App{Store: db, Engine: f, Media: media}
	t.Cleanup(func() { CloseAccounts(a); _ = db.Close() })
	return a, &Accounts{app: a}, f
}
func addTestAccount(t *testing.T, a *App, s *Accounts, f *fakeAccountEngine, remote string) Account {
	t.Helper()
	pending, err := s.Add("douyin")
	if err != nil {
		t.Fatal(err)
	}
	if pending.Status != "pending" || !pending.Active {
		t.Fatal("add did not return an active pending account")
	}
	f.mu.Lock()
	f.sessions[f.current].identity = douyin.Identity{LoginState: douyin.LoginState{LoggedIn: true, UID: "uid-" + remote, Nickname: "Test account"}, RemoteID: remote, Avatar: "https://example.test/avatar"}
	f.sessions[f.current].data = "session-only-marker-" + remote
	f.mu.Unlock()
	if state, err := a.CheckLogin(); err != nil || !state.LoggedIn {
		t.Fatalf("identity confirmation: %v", err)
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range list {
		if account.Active {
			return account
		}
	}
	t.Fatal("no active account")
	return Account{}
}
func TestAccountLifecycle(t *testing.T) {
	for _, name := range []string{"reuse", "switch", "sign out", "remove active", "remove inactive", "restart", "identity expired", "transient failure", "identity mismatch", "cleanup retry"} {
		t.Run(name, func(t *testing.T) {
			a, s, f := newAccountTestApp(t)
			events := 0
			a.changed = func(accounts []Account) { events++ }
			first := addTestAccount(t, a, s, f, "test-sec-a")
			if err := a.persist(douyin.LikesResult{OK: true, Cursor: "1", Items: []douyin.LikeItem{{ID: "work-a", Images: []string{}}}}, true); err != nil {
				t.Fatal(err)
			}
			firstRow, err := a.Store.Account(context.Background(), first.ID)
			if err != nil {
				t.Fatal(err)
			}
			if name == "reuse" {
				reused := addTestAccount(t, a, s, f, "test-sec-a")
				list, _ := s.List()
				if reused.ID != first.ID || len(list) != 1 {
					t.Fatal("duplicate identity was not reused")
				}
				if len(f.sessions) != 2 {
					t.Fatal("duplicate did not create a temporary session")
				}
				currentRow, err := a.Store.Account(context.Background(), reused.ID)
				if err != nil {
					t.Fatal(err)
				}
				for sessionName, session := range f.sessions {
					if sessionName == firstRow.SessionName && session.clears != 1 {
						t.Fatal("old session was not cleared")
					}
					if sessionName == currentRow.SessionName && session.data == "" {
						t.Fatal("login session was not retained")
					}
				}
				result, err := a.CachedLikes()
				if err != nil || len(result.Items) != 1 {
					t.Fatal("reuse lost local data")
				}
				return
			}
			second := addTestAccount(t, a, s, f, "test-sec-b")
			if err := a.persist(douyin.LikesResult{OK: true, Cursor: "2", Items: []douyin.LikeItem{{ID: "work-b", Images: []string{}}}}, true); err != nil {
				t.Fatal(err)
			}
			secondRow, err := a.Store.Account(context.Background(), second.ID)
			if err != nil {
				t.Fatal(err)
			}
			if firstRow.SessionName == secondRow.SessionName || firstRow.SessionName != "hoard-account-"+first.ID {
				t.Fatal("session names are not isolated")
			}
			firstData := f.sessions[firstRow.SessionName].data
			switch name {
			case "switch":
				before := f.closes
				if _, err := s.Switch(first.ID); err != nil {
					t.Fatal(err)
				}
				result, err := a.CachedLikes()
				if err != nil || len(result.Items) != 1 || result.Items[0].ID != "work-a" {
					t.Fatal("switch read the wrong library")
				}
				if f.current != firstRow.SessionName || f.closes != before+1 || f.shown {
					t.Fatal("switch did not replace the engine window silently")
				}
			case "sign out":
				if err := s.SignOut(second.ID); err != nil {
					t.Fatal(err)
				}
				row, _ := a.Store.Account(context.Background(), second.ID)
				if row.Status != "signed_out" || f.sessions[secondRow.SessionName].data != "" || f.open {
					t.Fatal("sign out did not clear the selected session")
				}
				if n, err := a.Store.CountWorks(context.Background(), second.ID, "like"); err != nil || n != 1 {
					t.Fatal("sign out removed local relations")
				}
			case "remove active", "remove inactive", "cleanup retry":
				target := second
				if name == "remove inactive" {
					target = first
				}
				if name == "cleanup retry" {
					f.clearError = true
				}
				err := s.Remove(target.ID)
				if name == "cleanup retry" {
					if err == nil {
						t.Fatal("cleanup failure was hidden")
					}
					f.clearError = false
					if err := s.Remove(target.ID); err != nil {
						t.Fatal(err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
				list, _ := s.List()
				if len(list) != 1 || list[0].ID == target.ID {
					t.Fatal("account deletion failed")
				}
				if name == "remove inactive" && (!f.open || f.current != secondRow.SessionName || f.sessions[secondRow.SessionName].data == "") {
					t.Fatal("removal affected the other engine session")
				}
				if _, err := a.Store.SyncState(context.Background(), target.ID, "like"); !errors.Is(err, sql.ErrNoRows) {
					t.Fatal("deleted account still has collection state")
				}
				if n, err := a.Store.CountWorks(context.Background(), target.ID, "like"); err != nil || n != 0 {
					t.Fatal("deleted account still has relations")
				}
				remaining := first
				if target.ID == first.ID {
					remaining = second
				}
				if n, err := a.Store.CountWorks(context.Background(), remaining.ID, "like"); err != nil || n != 1 {
					t.Fatal("removal affected the other account's data")
				}
			case "restart":
				CloseAccounts(a)
				if err := RestoreAccounts(a); err != nil {
					t.Fatal(err)
				}
				if f.current != secondRow.SessionName || f.shown || a.accountID != second.ID {
					t.Fatal("restart did not restore the last session silently")
				}
				if state, err := a.CheckLogin(); err != nil || !state.LoggedIn {
					t.Fatal("persistent session was lost")
				}
			case "identity expired", "transient failure", "identity mismatch":
				f.mu.Lock()
				identity := douyin.Identity{AuthRequired: name == "identity expired"}
				if name == "identity mismatch" {
					identity = douyin.Identity{LoginState: douyin.LoginState{LoggedIn: true, UID: "unrelated-uid"}, RemoteID: "unrelated-sec"}
				}
				f.sessions[f.current].identity = identity
				f.mu.Unlock()
				_, err := a.CheckLogin()
				if name != "identity mismatch" && err != nil {
					t.Fatal(err)
				}
				row, _ := a.Store.Account(context.Background(), second.ID)
				want := "signed_out"
				if name == "transient failure" {
					want = "active"
				}
				if row.Status != want || row.RemoteID != "test-sec-b" {
					t.Fatal("identity handling changed the wrong status or identity")
				}
			}
			if name != "remove inactive" && f.sessions[firstRow.SessionName].data != firstData {
				t.Fatal("another session was changed")
			}
			if events < 4 {
				t.Fatal("account changes were not emitted")
			}
		})
	}
}

func TestLateAccountRequests(t *testing.T) {
	for _, method := range []string{"identity", "likes"} {
		t.Run(method, func(t *testing.T) {
			a, s, f := newAccountTestApp(t)
			first := addTestAccount(t, a, s, f, "test-sec-a")
			second := addTestAccount(t, a, s, f, "test-sec-b")
			started, release := make(chan struct{}), make(chan struct{})
			f.mu.Lock()
			f.requestStarted, f.releaseRequest = started, release
			f.mu.Unlock()
			done := make(chan error, 1)
			go func() {
				if method == "identity" {
					_, err := a.CheckLogin()
					done <- err
				} else {
					_, err := a.FetchLikes("0")
					done <- err
				}
			}()
			<-started
			if _, err := s.Switch(first.ID); err != nil {
				t.Fatal(err)
			}
			close(release)
			if err := <-done; err == nil || err.Error() != "ACCOUNT_CHANGED" {
				t.Fatal("stale request was accepted")
			}
			for _, account := range []Account{first, second} {
				if n, err := a.Store.CountWorks(context.Background(), account.ID, "like"); err != nil || n != 0 {
					t.Fatal("late result crossed the account boundary")
				}
			}
		})
	}
}

func TestLegacyAppCanReopenSignedOutAccount(t *testing.T) {
	for _, show := range []bool{false, true} {
		t.Run(fmt.Sprintf("show %t", show), func(t *testing.T) {
			a, s, f := newAccountTestApp(t)
			account := addTestAccount(t, a, s, f, "test-sec-a")
			if err := s.SignOut(account.ID); err != nil {
				t.Fatal(err)
			}
			var err error
			if show {
				_, err = a.OpenDouyin()
			} else {
				_, err = a.EnsureSession()
			}
			if err != nil || a.sessionCtx == nil || a.sessionCtx.Err() != nil || f.shown != show {
				t.Fatal("legacy entry did not restore the selected account session")
			}
			f.mu.Lock()
			f.sessions[f.current].identity = douyin.Identity{LoginState: douyin.LoginState{LoggedIn: true, UID: "uid-test-sec-a"}, RemoteID: "test-sec-a"}
			f.mu.Unlock()
			if state, err := a.CheckLogin(); err != nil || !state.LoggedIn {
				t.Fatal("legacy entry cannot complete identity confirmation")
			}
			list, err := s.List()
			if err != nil || len(list) != 1 || list[0].ID != account.ID {
				t.Fatal("legacy entry duplicated the account")
			}
		})
	}
}

func TestCleanupDoesNotBlockOtherAccount(t *testing.T) {
	for _, retryFails := range []bool{false, true} {
		t.Run(fmt.Sprintf("retry fails %t", retryFails), func(t *testing.T) {
			a, s, f := newAccountTestApp(t)
			first := addTestAccount(t, a, s, f, "test-sec-a")
			second := addTestAccount(t, a, s, f, "test-sec-b")
			f.clearError = true
			if err := s.Remove(first.ID); err == nil {
				t.Fatal("cleanup error was hidden")
			}
			CloseAccounts(a)
			f.clearError = retryFails
			if err := RestoreAccounts(a); err != nil {
				t.Fatal("another account's pending cleanup blocked restoration")
			}
			if a.accountID != second.ID || !f.open {
				t.Fatal("other account was not restored")
			}
			pending, err := a.Store.SessionCleanups(context.Background())
			if err != nil || (len(pending) == 1) != retryFails {
				t.Fatal("cleanup retry state is incorrect")
			}
		})
	}
}

func TestAccountIPCShape(t *testing.T) {
	typ := reflect.TypeOf(Account{})
	want := []string{"id", "source", "nickname", "avatar", "status", "active"}
	if typ.NumField() != len(want) {
		t.Fatal("account field count changed")
	}
	for i, tag := range want {
		if typ.Field(i).Tag.Get("json") != tag {
			t.Fatal("account JSON shape changed")
		}
	}
}
