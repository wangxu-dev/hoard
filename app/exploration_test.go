package app

import (
	"context"
	"github.com/wangxu-dev/hoard/internal/explore"
	"github.com/wangxu-dev/hoard/internal/store"
	"path/filepath"
	"testing"
	"time"
)

func TestExplorationNativeServiceRoundTrip(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "fixture.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	_, err = db.UpsertAccount(ctx, store.Account{ID: "a", Source: "douyin", RemoteID: "owner", SessionName: "a", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.SaveWorks(ctx, "a", "like", false, []store.Work{{RemoteID: "real-fixture-id", Kind: "video", Title: "摄影", Author: &store.Author{RemoteID: "author-fixture", Nickname: "Name", AvatarURL: "https://fixture.invalid/a"}, CoverURL: "https://fixture.invalid/c"}})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	svc := NewExploration(func() (*store.Store, string) { return db, dir })
	defer CloseExploration(svc)
	task, err := svc.Start(explore.Scope{AccountID: "a", Source: "douyin"}, "view")
	if err != nil {
		t.Fatal(err)
	}
	m, _ := svc.get()
	wait, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err = m.Wait(wait, task.ID); err != nil {
		t.Fatal(err)
	}
	s, err := svc.Result(task.ID)
	if err != nil || s == nil {
		t.Fatal(err)
	}
	if s.SchemaVersion != explore.SchemaVersion || len(s.Scopes) != 1 || s.Scopes[0].Works[0].Author.ID != "douyin:author-fixture" || s.Meta.Consistency != "single read-only WAL transaction" {
		t.Fatal("native DTO contract failed")
	}
	second, err := svc.Start(s.Scope, "view")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Wait(wait, second.ID); err != nil {
		t.Fatal(err)
	}
	v, err := svc.Status(second.ID)
	if err != nil || !v.CacheHit {
		t.Fatal("native cache miss on identical request")
	}
	if _, err = svc.Result(task.ID); err == nil {
		t.Fatal("old request still readable")
	}
	if err = svc.Cancel(second.ID); err != nil {
		t.Fatal(err)
	}
}
