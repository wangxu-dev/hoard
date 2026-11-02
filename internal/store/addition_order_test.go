package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSourceAdditionOrder(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "order.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.UpsertAccount(ctx, Account{ID: "test", Source: "bilibili", RemoteID: "42", SessionName: "test", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	for _, work := range []Work{{RemoteID: "first", Kind: "video", AddedAt: 300, PublishedAt: 100}, {RemoteID: "second", Kind: "video", AddedAt: 200, PublishedAt: 400}} {
		if _, err = db.SaveWorks(ctx, "test", "favorite", true, []Work{work}); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ sort, want string }{{"recent", "first"}, {"published", "second"}} {
		items, _, err := db.Library(ctx, LibraryQuery{AccountID: "test", Kind: "favorite", Sort: tc.sort, Limit: 10})
		if err != nil || len(items) != 2 || items[0].RemoteID != tc.want {
			t.Fatalf("%s: %+v %v", tc.sort, items, err)
		}
	}
	added, err := db.SaveWorks(ctx, "test", "favorite", true, []Work{{RemoteID: "second", Kind: "video", AddedAt: 500, PublishedAt: 600}})
	if err != nil || added != 0 {
		t.Fatalf("resync: %d %v", added, err)
	}
	items, _, err := db.Library(ctx, LibraryQuery{AccountID: "test", Kind: "favorite", Sort: "recent", Limit: 10})
	if err != nil || items[0].RemoteID != "second" || items[0].PublishedAt != 600 {
		t.Fatalf("updated order: %+v %v", items, err)
	}
}
