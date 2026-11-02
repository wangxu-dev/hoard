package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wangxu-dev/hoard/internal/douyin"
	"github.com/wangxu-dev/hoard/internal/store"
)

type actionFake struct {
	identity string
	liked    bool
	favorite bool
	writes   int
	timeout  bool
	unknown  bool
	mismatch bool
	session  string
}

func (f *actionFake) UseSession(name string, _ bool) error { f.session = name; return nil }
func (f *actionFake) Close()                               {}
func (f *actionFake) Status() douyin.Status                { return douyin.Status{Ready: true} }
func (f *actionFake) Identity(context.Context) (douyin.Identity, error) {
	return douyin.Identity{LoginState: douyin.LoginState{LoggedIn: true}, RemoteID: f.identity}, nil
}
func (f *actionFake) Request(_ context.Context, op douyin.Operation, p map[string]string) (douyin.PageReply, error) {
	if op == douyin.OpSetLike || op == douyin.OpSetFavorite {
		f.writes++
		if op == douyin.OpSetLike {
			f.liked = p["type"] == "1"
		} else {
			f.favorite = p["action"] == "1"
		}
		if f.timeout {
			return douyin.PageReply{}, errors.New("timeout")
		}
	}
	if f.unknown && f.writes > 0 {
		return douyin.PageReply{}, errors.New("timeout")
	}
	like, fav := 0, 0
	if f.liked {
		like = 1
	}
	if f.favorite {
		fav = 1
	}
	id := "123"
	if f.mismatch {
		id = "456"
	}
	return douyin.PageReply{Status: 200, Body: fmt.Sprintf(`{"status_code":0,"aweme_detail":{"aweme_id":"%s","aweme_type":0,"user_digged":%d,"collect_stat":%d}}`, id, like, fav)}, nil
}
func TestSyncRejectsPageBeforeAction(t *testing.T) {
	a := &App{}
	a.actionRevision.Add(1)
	s := &Sync{app: a}
	if err := s.store("a", "like", false, douyin.ListPage{}, 0); err == nil || err.Error() != "ACTIONS_CHANGED" {
		t.Fatalf("stale page: %v", err)
	}
}

func TestAccountActions(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		kind                           string
		on, timeout, unknown, mismatch bool
		identity, wantErr              string
		writes                         int
	}{
		{name: "like", kind: "like", on: true, identity: "owner-a", writes: 1},
		{name: "favorite", kind: "favorite", on: true, identity: "owner-a", writes: 1},
		{name: "already off", kind: "like", identity: "owner-a"},
		{name: "timeout confirmed", kind: "like", on: true, timeout: true, identity: "owner-a", writes: 1},
		{name: "unknown result", kind: "like", on: true, unknown: true, identity: "owner-a", wantErr: "ACTION_UNKNOWN", writes: 1},
		{name: "wrong identity", kind: "like", on: true, identity: "other", wantErr: "IDENTITY_MISMATCH"},
		{name: "wrong work", kind: "like", on: true, identity: "owner-a", mismatch: true, wantErr: "STATE_UNKNOWN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := store.Open(filepath.Join(t.TempDir(), "library.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			ctx := context.Background()
			for _, id := range []string{"a", "b"} {
				if _, err := db.UpsertAccount(ctx, store.Account{ID: id, Source: "douyin", RemoteID: "owner-" + id, SessionName: "session-" + id, Status: "active"}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.SaveWorks(ctx, "b", "like", false, []store.Work{{RemoteID: "123", Kind: "video", Title: "Example"}}); err != nil {
				t.Fatal(err)
			}
			f := &actionFake{identity: tc.identity, timeout: tc.timeout, unknown: tc.unknown, mismatch: tc.mismatch}
			a := &App{Store: db}
			actions := &Actions{app: a, engine: func() actionEngine { return f }}
			_, err = actions.Set("a", "123", tc.kind, tc.on)
			if (err == nil) != (tc.wantErr == "") || err != nil && err.Error() != tc.wantErr {
				t.Fatalf("error %v, want %s", err, tc.wantErr)
			}
			if f.writes != tc.writes {
				t.Fatalf("writes %d", f.writes)
			}
			if f.session != "session-a" {
				t.Fatal("wrong session")
			}
			count, _ := db.CountWorks(ctx, "b", "like")
			if count != 1 {
				t.Fatal("other account changed")
			}
			count, _ = db.CountWorks(ctx, "a", tc.kind)
			want := int64(0)
			if tc.on && tc.wantErr == "" {
				want = 1
			}
			if count != want {
				t.Fatalf("count %d want %d", count, want)
			}
		})
	}
}
