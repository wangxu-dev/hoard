package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestBilibiliActionMembership(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, account := range []string{"a", "b"} {
		if _, err := db.UpsertAccount(ctx, Account{ID: account, Source: "bilibili", RemoteID: account, SessionName: account, Status: "active"}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.SaveWorks(ctx, account, "watch_later", true, []Work{{RemoteID: "BV0000000001", Kind: "video", Title: "Example"}}); err != nil {
			t.Fatal(err)
		}
	}
	folders := []FolderSelection{{Folder: Folder{ID: "one", Title: "One", Count: 1}, Selected: true}, {Folder: Folder{ID: "two", Title: "Two", Count: 1}, Selected: true}}
	for _, tc := range []struct {
		name              string
		first, second, on bool
	}{{"both", true, true, true}, {"one remains", false, true, true}, {"none", false, false, false}} {
		t.Run(tc.name, func(t *testing.T) {
			folders[0].Selected, folders[1].Selected = tc.first, tc.second
			if err := db.ConfirmBilibiliAction(ctx, "a", "BV0000000001", "favorite", tc.on, folders); err != nil {
				t.Fatal(err)
			}
			for i, want := range []bool{tc.first, tc.second} {
				members, err := db.FolderMembers(ctx, "a", folders[i].ID)
				if err != nil {
					t.Fatal(err)
				}
				if (len(members) == 1) != want {
					t.Fatal("wrong folder membership")
				}
			}
			state, err := db.PersonalState(ctx, "a", "BV0000000001")
			if err != nil || state.Favorited == nil || *state.Favorited != tc.on {
				t.Fatalf("state=%+v err=%v", state, err)
			}
			count, err := db.CountWorks(ctx, "a", "favorite")
			if err != nil || (count == 1) != tc.on {
				t.Fatalf("count=%d err=%v", count, err)
			}
		})
	}
	if err := db.ConfirmBilibiliAction(ctx, "a", "BV0000000001", "watch_later", false, nil); err != nil {
		t.Fatal(err)
	}
	count, _ := db.CountWorks(ctx, "b", "watch_later")
	if count != 1 {
		t.Fatal("other account changed")
	}
	if err := db.ConfirmBilibiliAction(ctx, "a", "BV0000000001", "favorite", false, []FolderSelection{{Folder: Folder{ID: "bad", Count: 1}, Selected: true}}); err == nil {
		t.Fatal("inconsistent state accepted")
	}
	members, _ := db.FolderMembers(ctx, "a", "bad")
	if len(members) != 0 {
		t.Fatal("failed transaction changed members")
	}
	yes := true
	if err := db.SavePersonalState(ctx, "a", "BV0000000001", PersonalState{Liked: &yes}); err != nil {
		t.Fatal(err)
	}
	owners, err := db.ActionOwners(ctx, LibraryQuery{Source: "bilibili", Kind: "like"}, []string{"BV0000000001"})
	if err != nil || len(owners["BV0000000001"]) != 1 || owners["BV0000000001"][0] != "a" {
		t.Fatalf("owners=%v err=%v", owners, err)
	}
}
