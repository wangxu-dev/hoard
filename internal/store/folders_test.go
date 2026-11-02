package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestFolderMembershipIsolation(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, account := range []string{"a", "b"} {
		if _, err := s.UpsertAccount(ctx, Account{ID: account, Source: "bilibili", RemoteID: account, SessionName: account, Status: "active"}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.SaveWorks(ctx, account, "favorite", false, []Work{{RemoteID: "BV1abcdefghj", Kind: "video", Title: "Example"}}); err != nil {
			t.Fatal(err)
		}
		for _, folder := range []string{"one", "two"} {
			if err := s.SaveFolderMembership(ctx, account, Folder{folder, "Example", 1}, []FolderWork{{"BV1abcdefghj", 100}}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := s.SaveFolderMembership(ctx, "a", Folder{"one", "Example", 1}, []FolderWork{{"BV1abcdefghj", 200}}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM folder_works`).Scan(&count); err != nil || count != 4 {
		t.Fatalf("members %d %v", count, err)
	}
	if err := s.SaveFolderMembership(ctx, "a", Folder{"bad", "Rollback", 1}, []FolderWork{{"missing", 0}}); err == nil {
		t.Fatal("missing work accepted")
	}
	folders, err := s.Folders(ctx, "a")
	if err != nil || len(folders) != 2 {
		t.Fatalf("folders %+v %v", folders, err)
	}
	if err := s.DeleteAccount(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	folders, err = s.Folders(ctx, "b")
	if err != nil || len(folders) != 2 {
		t.Fatal("other account folders changed")
	}
}
