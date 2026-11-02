package app

import (
	"context"
	"testing"

	"github.com/wangxu-dev/hoard/internal/douyin"
)

func TestAddSourceAccount(t *testing.T) {
	for _, source := range []string{"douyin", "bilibili", "unsupported"} {
		t.Run(source, func(t *testing.T) {
			a, s, f := newAccountTestApp(t)
			pending, err := s.Add(source)
			if source == "unsupported" {
				if err == nil {
					t.Fatal("unsupported source accepted")
				}
				accounts, _ := s.List()
				if len(accounts) != 0 {
					t.Fatal("unsupported account persisted")
				}
				return
			}
			if err != nil || pending.Source != source || f.currentSource != source || !f.shown {
				t.Fatalf("source login route: %v", err)
			}
			f.mu.Lock()
			f.sessions[f.current].identity = douyin.Identity{LoginState: douyin.LoginState{LoggedIn: true, UID: "42", Nickname: "Example"}, RemoteID: "42"}
			f.mu.Unlock()
			if state, err := a.CheckLogin(); err != nil || !state.LoggedIn {
				t.Fatalf("confirm: %v", err)
			}
			account, err := a.Store.Account(context.Background(), pending.ID)
			if err != nil || account.Source != source || account.Status != "active" {
				t.Fatal("source lost after confirmation")
			}
			if err := s.SignOut(account.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Switch(account.ID); err != nil || f.currentSource != source || !f.shown {
				t.Fatal("relogin used wrong source")
			}
		})
	}
}
