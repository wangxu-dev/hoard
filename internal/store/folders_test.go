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
			if err := s.SaveFolderMembership(ctx, account, Folder{ID: folder, Title: "Example", Count: 1}, []FolderWork{{"BV1abcdefghj", 100}}); err != nil {
				t.Fatal(err)
			}
		}
	}
	known, err := s.KnownWorks(ctx, "b", "favorite", []string{"BV1abcdefghj", "missing"})
	if err != nil || !known["BV1abcdefghj"] || known["missing"] {
		t.Fatalf("source known: %v %v", known, err)
	}
	if err := s.SaveFolderMembership(ctx, "a", Folder{ID: "one", Title: "Example", Count: 1}, []FolderWork{{"BV1abcdefghj", 200}}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM folder_works`).Scan(&count); err != nil || count != 4 {
		t.Fatalf("members %d %v", count, err)
	}
	if err := s.SaveFolderMembership(ctx, "a", Folder{ID: "bad", Title: "Rollback", Count: 1}, []FolderWork{{"missing", 0}}); err == nil {
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

func TestReplaceFolderMembershipAndPruneFolders(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, account := range []string{"a", "b"} {
		if _, err := s.UpsertAccount(ctx, Account{ID: account, Source: "douyin", RemoteID: account, SessionName: account, Status: "active"}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.SaveWorks(ctx, account, "favorite", false, []Work{
			{RemoteID: "101", Kind: "video", Title: "One"},
			{RemoteID: "102", Kind: "video", Title: "Two"},
			{RemoteID: "103", Kind: "video", Title: "Three"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ReplaceFolderMembership(ctx, "a", Folder{ID: "f1", Title: "First", Count: 2, Position: 0}, []FolderWork{{RemoteID: "101"}, {RemoteID: "102"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceFolderMembership(ctx, "a", Folder{ID: "f2", Title: "Second", Count: 1, Position: 1}, []FolderWork{{RemoteID: "103"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceFolderMembership(ctx, "b", Folder{ID: "f1", Title: "Other", Count: 1, Position: 0}, []FolderWork{{RemoteID: "101"}}); err != nil {
		t.Fatal(err)
	}

	// Replace f1 for account a: removes 101, adds 103, leaves f2 and account b untouched.
	if err := s.ReplaceFolderMembership(ctx, "a", Folder{ID: "f1", Title: "First Renamed", Count: 2, Position: 0}, []FolderWork{{RemoteID: "102"}, {RemoteID: "103"}}); err != nil {
		t.Fatal(err)
	}
	f1Members, err := s.FolderMembers(ctx, "a", "f1")
	if err != nil || len(f1Members) != 2 {
		t.Fatalf("f1 members = %v, err=%v", f1Members, err)
	}
	if _, ok := f1Members["101"]; ok {
		t.Fatalf("stale member 101 still present: %v", f1Members)
	}
	f2Members, err := s.FolderMembers(ctx, "a", "f2")
	if err != nil || len(f2Members) != 1 {
		t.Fatalf("f2 members changed: %v, err=%v", f2Members, err)
	}
	bMembers, err := s.FolderMembers(ctx, "b", "f1")
	if err != nil || len(bMembers) != 1 {
		t.Fatalf("other account members changed: %v, err=%v", bMembers, err)
	}

	// Prune account a keeping only f1; f2 is deleted while b's f1 remains.
	if err := s.PruneFolders(ctx, "a", []string{"f1"}); err != nil {
		t.Fatal(err)
	}
	foldersA, err := s.Folders(ctx, "a")
	if err != nil || len(foldersA) != 1 || foldersA[0].ID != "f1" || foldersA[0].Title != "First Renamed" {
		t.Fatalf("folders after prune = %+v, err=%v", foldersA, err)
	}
	f2Members, err = s.FolderMembers(ctx, "a", "f2")
	if err != nil || len(f2Members) != 0 {
		t.Fatalf("pruned folder members still present: %v, err=%v", f2Members, err)
	}
	foldersB, err := s.Folders(ctx, "b")
	if err != nil || len(foldersB) != 1 {
		t.Fatalf("other account folders affected: %+v, err=%v", foldersB, err)
	}

	// Prune with empty keepIDs deletes all folders for account a only.
	if err := s.PruneFolders(ctx, "a", nil); err != nil {
		t.Fatal(err)
	}
	foldersA, err = s.Folders(ctx, "a")
	if err != nil || len(foldersA) != 0 {
		t.Fatalf("expected 0 folders for a, got %+v, err=%v", foldersA, err)
	}
	foldersB, err = s.Folders(ctx, "b")
	if err != nil || len(foldersB) != 1 {
		t.Fatalf("other account folders affected on empty prune: %+v, err=%v", foldersB, err)
	}
}
