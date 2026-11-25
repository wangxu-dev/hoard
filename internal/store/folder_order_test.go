package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestFolderOrderDefaultThenDescendingNumericID(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.UpsertAccount(ctx, Account{ID: "a", Source: "bilibili", RemoteID: "123", SessionName: "a", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	for _, folder := range []Folder{{ID: "9", Title: "Older"}, {ID: "100", Title: "Newest"}, {ID: "20", Title: "Recent"}, {ID: "1", Title: "默认收藏夹"}} {
		if err := s.SaveFolderMembership(ctx, "a", folder, nil); err != nil {
			t.Fatal(err)
		}
	}
	folders, err := s.Folders(ctx, "a")
	if err != nil || len(folders) != 4 {
		t.Fatalf("folders: %v %v", folders, err)
	}
	for i, id := range []string{"1", "100", "20", "9"} {
		if folders[i].ID != id {
			t.Fatalf("position %d: got %s, want %s", i, folders[i].ID, id)
		}
	}
}
