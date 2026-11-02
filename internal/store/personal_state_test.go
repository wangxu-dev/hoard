package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestPersonalStateIsolation(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, a := range []string{"a", "b"} {
		if _, err := s.UpsertAccount(ctx, Account{ID: a, Source: "bilibili", RemoteID: a, SessionName: a, Status: "active"}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.SaveWorks(ctx, a, "favorite", false, []Work{{RemoteID: "BV1abcdefghj", Kind: "video", Title: "Example"}}); err != nil {
			t.Fatal(err)
		}
	}
	no, yes, coins := false, true, 2
	if err := s.SavePersonalState(ctx, "a", "BV1abcdefghj", PersonalState{Liked: &no, Coins: &coins}); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePersonalState(ctx, "a", "BV1abcdefghj", PersonalState{Favorited: &yes}); err != nil {
		t.Fatal(err)
	}
	state, err := s.PersonalState(ctx, "a", "BV1abcdefghj")
	if err != nil || state.Liked == nil || *state.Liked || state.Coins == nil || *state.Coins != 2 || state.Favorited == nil || !*state.Favorited || state.LikedAt == nil {
		t.Fatalf("bad observations %+v %v", state, err)
	}
	other, err := s.PersonalState(ctx, "b", "BV1abcdefghj")
	if err != nil || other.Liked != nil || other.Coins != nil {
		t.Fatal("other account was affected")
	}
	if err := s.SavePersonalState(ctx, "a", "missing", PersonalState{Liked: &yes}); err == nil {
		t.Fatal("missing target accepted")
	}
}
