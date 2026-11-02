package store

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLibraryOwners(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, id := range []string{"a", "b"} {
		if _, err := s.UpsertAccount(ctx, Account{ID: id, Source: "douyin", RemoteID: "remote-" + id, SessionName: "session-" + id, Status: "active"}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.SaveWorks(ctx, id, "like", false, []Work{{RemoteID: "shared", Kind: "video", Title: "Example"}}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.SaveMusics(ctx, id, false, []Music{{RemoteID: "sound", Title: "Example sound"}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.SaveWorks(ctx, "b", "favorite", false, []Work{{RemoteID: "shared", Kind: "video", Title: "Example"}}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		q    LibraryQuery
		id   string
		want []string
	}{
		{"shared likes", LibraryQuery{Kind: "like"}, "shared", []string{"a", "b"}},
		{"independent favorites", LibraryQuery{Kind: "favorite"}, "shared", []string{"b"}},
		{"single account", LibraryQuery{Kind: "like", AccountID: "a"}, "shared", []string{"a"}},
		{"shared music", LibraryQuery{Kind: "music"}, "sound", []string{"a", "b"}},
		{"source isolation", LibraryQuery{Kind: "like", Source: "other"}, "shared", nil},
		{"missing", LibraryQuery{Kind: "like"}, "missing", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.LibraryOwners(ctx, tc.q, []string{tc.id})
			if err != nil || !reflect.DeepEqual(got[tc.id], tc.want) {
				t.Fatalf("owners %v, err %v; want %v", got, err, tc.want)
			}
		})
	}
	if _, err := s.LibraryOwners(ctx, LibraryQuery{Kind: "invalid"}, []string{"shared"}); err == nil {
		t.Fatal("accepted invalid kind")
	}
}
