package store

import (
	"context"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}

func addTestAccount(t *testing.T, s *Store, id, source string) string {
	t.Helper()
	got, err := s.UpsertAccount(context.Background(), Account{ID: id, Source: source, RemoteID: "remote-" + id, Nickname: "Test account", SessionName: "session-" + id, Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func testWork(id string) Work {
	return Work{RemoteID: id, Kind: "video", Title: "Test work", PublishedAt: 1700000000, DurationMS: 19000,
		CoverURL: "https://example.test/cover.jpg", Author: &Author{RemoteID: "test-author", Nickname: "Test author"}}
}

func TestWorkPageRoundTrip(t *testing.T) {
	for _, kind := range []string{"like", "favorite"} {
		t.Run(kind, func(t *testing.T) {
			s := newTestStore(t)
			ctx := context.Background()
			id := addTestAccount(t, s, "account-a", "test")
			page := WorkPage{AccountID: id, Kind: kind, Phase: "head", Cursor: "99", Works: []Work{testWork("1")}}
			if err := s.WriteWorkPage(ctx, page); err != nil {
				t.Fatal(err)
			}
			works, next, err := s.ListWorks(ctx, id, kind, nil, 10)
			if err != nil || next != nil || len(works) != 1 {
				t.Fatalf("list: %d %v %v", len(works), next, err)
			}
			w := works[0]
			if w.ID != "test:1" || w.Source != "test" || w.RemoteID != "1" || w.Title != "Test work" || w.Author == nil || w.Author.Nickname != "Test author" || w.Seq != 1 || w.Available == nil || !*w.Available || string(w.Extra) != "{}" {
				t.Fatalf("work: %+v", w)
			}
			page.Works[0].Title = "Updated work"
			page.Works[0].Author.Nickname = "Updated author"
			page.Phase = "backfill"
			page.Cursor = "100"
			if err := s.WriteWorkPage(ctx, page); err != nil {
				t.Fatal(err)
			}
			works, _, err = s.ListWorks(ctx, id, kind, nil, 10)
			if err != nil || len(works) != 1 || works[0].Title != "Updated work" || works[0].Author.Nickname != "Updated author" || works[0].Seq != 1 {
				t.Fatalf("upsert: %+v %v", works, err)
			}
			state, err := s.SyncState(ctx, id, kind)
			if err != nil || state.HeadCursor != "99" || state.BackfillCursor != "100" || state.Pages != 2 || state.Items != 2 {
				t.Fatalf("state: %+v %v", state, err)
			}
			if err := s.SetSetting(ctx, "test", "value"); err != nil {
				t.Fatal(err)
			}
			if value, err := s.Setting(ctx, "test"); err != nil || value != "value" {
				t.Fatalf("setting: %q %v", value, err)
			}
		})
	}
}
