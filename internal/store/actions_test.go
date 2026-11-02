package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestConfirmWorkActionIsolation(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, id := range []string{"a", "b"} {
		if _, err := db.UpsertAccount(ctx, Account{ID: id, Source: "douyin", RemoteID: id, SessionName: id, Status: "active"}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.SaveWorks(ctx, id, "like", false, []Work{{RemoteID: "123", Kind: "video", Title: "Preserved"}}); err != nil {
			t.Fatal(err)
		}
	}
	for _, on := range []bool{true, true, false} {
		if err := db.ConfirmWorkAction(ctx, "a", "123", "favorite", on); err != nil {
			t.Fatal(err)
		}
		count, _ := db.CountWorks(ctx, "a", "favorite")
		want := int64(0)
		if on {
			want = 1
		}
		if count != want {
			t.Fatalf("count %d", count)
		}
	}
	if err := db.ConfirmWorkAction(ctx, "a", "123", "like", false); err != nil {
		t.Fatal(err)
	}
	count, _ := db.CountWorks(ctx, "b", "like")
	if count != 1 {
		t.Fatal("other account changed")
	}
	rec, err := db.MediaRecord(ctx, "video", "123")
	if err != nil || rec.Title != "Preserved" {
		t.Fatal("metadata changed")
	}
	if err := db.ConfirmWorkAction(ctx, "a", "123", "music", false); err == nil {
		t.Fatal("invalid kind accepted")
	}
}
