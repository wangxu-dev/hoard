package app

import (
	"context"
	"github.com/wangxu-dev/hoard/internal/store"
	"path/filepath"
	"testing"
)

// Uses a synthetic test store only, never the user's application data.
func TestInterestLibraryPreservesNativeAuthorIdentity(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "fixture.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.UpsertAccount(ctx, store.Account{ID: "fixture-account", Source: "douyin", RemoteID: "fixture-owner", SessionName: "fixture", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	works := []store.Work{{RemoteID: "fixture-a", Kind: "video", Title: "摄影", Author: &store.Author{RemoteID: "fixture-author-a", Nickname: "同一个昵称"}}, {RemoteID: "fixture-b", Kind: "gallery", Title: "旅行", Author: &store.Author{RemoteID: "fixture-author-b", Nickname: "同一个昵称"}}, {RemoteID: "fixture-c", Kind: "video", Title: "无作者"}}
	if _, err = db.SaveWorks(ctx, "fixture-account", "like", false, works); err != nil {
		t.Fatal(err)
	}
	library := NewLibrary(&App{Store: db})
	for _, account := range []string{"", "fixture-account"} {
		page, err := library.List(LibraryQuery{AccountID: account, Source: "douyin", Kind: "like", Type: "all", Sort: "published", Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 3 {
			t.Fatalf("unexpected total %d", page.Total)
		}
		ids := map[string]string{}
		for _, work := range page.Items {
			ids[work.ID] = work.AuthorID
		}
		if ids["fixture-a"] != "fixture-author-a" || ids["fixture-b"] != "fixture-author-b" || ids["fixture-c"] != "" {
			t.Fatal("native identities missing, merged by nickname, or fabricated")
		}
	}
}
