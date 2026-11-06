package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func explorationFixture(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "fixture.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	ctx := context.Background()
	for _, a := range []Account{{ID: "a", Source: "douyin", RemoteID: "a", SessionName: "a", Status: "active"}, {ID: "b", Source: "douyin", RemoteID: "b", SessionName: "b", Status: "active"}, {ID: "c", Source: "bilibili", RemoteID: "c", SessionName: "c", Status: "active"}} {
		if _, err = s.UpsertAccount(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	return s
}
func saveExploration(t *testing.T, s *Store, account, kind string, works ...Work) {
	t.Helper()
	if _, err := s.SaveWorks(context.Background(), account, kind, false, works); err != nil {
		t.Fatal(err)
	}
}
func TestExploreScopeIsolationAndUnion(t *testing.T) {
	s := explorationFixture(t)
	w := func(id string) Work {
		return Work{RemoteID: id, Kind: "video", Title: "摄影", Author: &Author{RemoteID: "author-" + id, Nickname: "same nickname"}}
	}
	saveExploration(t, s, "a", "like", w("shared"))
	saveExploration(t, s, "a", "favorite", w("shared"))
	saveExploration(t, s, "b", "favorite", w("shared"), w("only-b"))
	saveExploration(t, s, "c", "watch_later", w("shared"))
	for _, q := range []struct {
		s ExploreScope
		n int
	}{{ExploreScope{}, 3}, {ExploreScope{Source: "douyin"}, 2}, {ExploreScope{AccountID: "a"}, 1}, {ExploreScope{AccountID: "b"}, 2}, {ExploreScope{AccountID: "c", Source: "bilibili"}, 1}} {
		got, err := s.ExploreInput(context.Background(), q.s)
		if err != nil || len(got) != q.n {
			t.Fatalf("scope %v count %d err %v", q.s, len(got), err)
		}
		for _, r := range got {
			if r.AuthorID == "" {
				t.Fatal("author ID missing")
			}
		}
	}
	if _, err := s.ExploreInput(context.Background(), ExploreScope{AccountID: "a", Source: "bilibili"}); err == nil {
		t.Fatal("mixed account/source accepted")
	}
	if _, err := s.ExploreInput(context.Background(), ExploreScope{AccountID: "missing"}); err == nil {
		t.Fatal("missing account accepted")
	}
}
func TestExploreReadTransactionConsistentDuringSyncWrite(t *testing.T) {
	s := explorationFixture(t)
	saveExploration(t, s, "a", "like", Work{RemoteID: "1", Kind: "video", Title: "before"})
	got, err := s.exploreInput(context.Background(), ExploreScope{AccountID: "a"}, func() {
		saveExploration(t, s, "a", "like", Work{RemoteID: "1", Kind: "video", Title: "after"}, Work{RemoteID: "2", Kind: "video", Title: "new"})
	})
	if err != nil || len(got) != 1 || got[0].Title != "before" {
		t.Fatalf("mixed snapshot %v %v", got, err)
	}
	next, err := s.ExploreInput(context.Background(), ExploreScope{AccountID: "a"})
	if err != nil || len(next) != 2 {
		t.Fatal("writer did not commit independently")
	}
	titles := ""
	for _, r := range next {
		titles += r.Title
	}
	if !strings.Contains(titles, "after") {
		t.Fatal("next snapshot missing updated title")
	}
}
func TestExploreReadCancellation(t *testing.T) {
	s := explorationFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.ExploreInput(ctx, ExploreScope{}); err == nil {
		t.Fatal("cancelled query succeeded")
	}
}
