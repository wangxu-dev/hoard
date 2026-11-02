package app

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wangxu-dev/hoard/internal/douyin"
	"github.com/wangxu-dev/hoard/internal/relay"
	"github.com/wangxu-dev/hoard/internal/store"
)

func TestCachedLikesAccounts(t *testing.T) {
	for _, tt := range []struct {
		name         string
		secondUID    string
		wantAccounts int
	}{
		{"same identity", "test-a", 1},
		{"switch identity", "test-b", 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db, err := store.Open(filepath.Join(t.TempDir(), "library.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			media, err := relay.New()
			if err != nil {
				t.Fatal(err)
			}
			a := &App{Store: db, Media: media}
			if err := rememberForTest(a, douyin.LoginState{LoggedIn: true, UID: "test-a", Nickname: "Test A"}); err != nil {
				t.Fatal(err)
			}
			firstID := a.accountID
			if err := a.persist(douyin.LikesResult{OK: true, Cursor: "99", HasMore: true, Items: []douyin.LikeItem{{ID: "work-a", Desc: "Work A", Images: []string{}}}}, true); err != nil {
				t.Fatal(err)
			}
			if err := rememberForTest(a, douyin.LoginState{LoggedIn: true, UID: tt.secondUID, Nickname: "Updated account"}); err != nil {
				t.Fatal(err)
			}
			if (a.accountID == firstID) != (tt.wantAccounts == 1) {
				t.Fatal("account identity was not reused or isolated")
			}
			reader := &App{Store: db, Media: media}
			result, err := reader.CachedLikes()
			if err != nil {
				t.Fatal(err)
			}
			wantItems := 1
			if tt.wantAccounts == 2 {
				wantItems = 0
			}
			if len(result.Items) != wantItems || reader.uid != tt.secondUID || reader.nickname != "Updated account" {
				t.Fatal("cached account selection changed")
			}
			if n, err := db.CountWorks(context.Background(), firstID, "like"); err != nil || n != 1 {
				t.Fatal("switch discarded the previous account's library")
			}
		})
	}
}

func TestCachedLikesReopen(t *testing.T) {
	for _, complete := range []bool{false, true} {
		t.Run(fmt.Sprintf("complete %t", complete), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "library.db")
			db, err := store.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = db.Close() }()
			writer := &App{Store: db}
			if err := rememberForTest(writer, douyin.LoginState{LoggedIn: true, UID: "test-a", Nickname: "Test account"}); err != nil {
				t.Fatal(err)
			}
			items := make([]douyin.LikeItem, 205)
			for i := range items {
				items[i] = douyin.LikeItem{ID: fmt.Sprintf("work-%03d", i), Desc: "Test work", CreatedAt: int64(i), Images: []string{}}
			}
			if err := writer.persist(douyin.LikesResult{OK: true, Cursor: "head-cursor", HasMore: true, Items: items[:100]}, true); err != nil {
				t.Fatal(err)
			}
			if err := writer.persist(douyin.LikesResult{OK: true, Cursor: "backfill-cursor", HasMore: !complete, Items: items[100:]}, false); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = store.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			media, err := relay.New()
			if err != nil {
				t.Fatal(err)
			}
			reader := &App{Store: db, Media: media}
			got, err := reader.CachedLikes()
			if err != nil || len(got.Items) != len(items) || got.Cursor != "backfill-cursor" || got.HasMore == complete {
				t.Fatalf("cached page: count=%d cursor=%q hasMore=%t err=%v", len(got.Items), got.Cursor, got.HasMore, err)
			}
			for i, item := range got.Items {
				if item.ID != items[len(items)-1-i].ID {
					t.Fatal("cached publication order changed")
				}
			}
		})
	}
}
