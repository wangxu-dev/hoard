package store

import (
	"context"
	"slices"
	"testing"
)

func TestReconcileHeadRelations(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T, ctx context.Context, s *Store)
		accountID     string
		kind          string
		seenRemoteIDs []string
		complete      bool
		wantRemoved   int
		wantErr       bool
		verify        func(t *testing.T, ctx context.Context, s *Store)
	}{
		{
			name: "removes item above anchor while keeping anchor older items and works row",
			setup: func(t *testing.T, ctx context.Context, s *Store) {
				addTestAccount(t, s, "acc-1", "douyin")
				// Backfill older-to-newer or head: SaveWorks(head=false) assigns seq -1, -2, -3...
				// SaveWorks(head=true) assigns seq upper+N .. upper+1.
				// Save older item "1" first, then "2" (anchor), then "3" (stale), then "4" (kept head).
				for _, id := range []string{"1", "2", "3", "4"} {
					if _, err := s.SaveWorks(ctx, "acc-1", "recommend", true, []Work{testWork(id)}); err != nil {
						t.Fatal(err)
					}
				}
			},
			accountID:     "acc-1",
			kind:          "recommend",
			seenRemoteIDs: []string{"4", "2", "", "4"},
			complete:      false,
			wantRemoved:   1,
			verify: func(t *testing.T, ctx context.Context, s *Store) {
				works, _, err := s.ListWorks(ctx, "acc-1", "recommend", nil, 10)
				if err != nil {
					t.Fatal(err)
				}
				var got []string
				for _, w := range works {
					got = append(got, w.RemoteID)
				}
				want := []string{"4", "2", "1"}
				if !slices.Equal(got, want) {
					t.Fatalf("works = %v, want %v", got, want)
				}
				// Works row for removed relation "3" must remain intact.
				if _, err := s.MediaRecord(ctx, "video", "3"); err != nil {
					t.Fatalf("works row for 3 was removed: %v", err)
				}
			},
		},
		{
			name: "adds fresh head items and removes stale items in same window",
			setup: func(t *testing.T, ctx context.Context, s *Store) {
				addTestAccount(t, s, "acc-1", "douyin")
				for _, id := range []string{"1", "2", "3"} {
					if _, err := s.SaveWorks(ctx, "acc-1", "recommend", true, []Work{testWork(id)}); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := s.SaveWorks(ctx, "acc-1", "recommend", true, []Work{testWork("5")}); err != nil {
					t.Fatal(err)
				}
			},
			accountID:     "acc-1",
			kind:          "recommend",
			seenRemoteIDs: []string{"5", "2"},
			complete:      false,
			wantRemoved:   1,
			verify: func(t *testing.T, ctx context.Context, s *Store) {
				works, _, err := s.ListWorks(ctx, "acc-1", "recommend", nil, 10)
				if err != nil {
					t.Fatal(err)
				}
				var got []string
				for _, w := range works {
					got = append(got, w.RemoteID)
				}
				want := []string{"5", "2", "1"}
				if !slices.Equal(got, want) {
					t.Fatalf("works = %v, want %v", got, want)
				}
				if _, err := s.MediaRecord(ctx, "video", "3"); err != nil {
					t.Fatalf("works row for 3 missing: %v", err)
				}
			},
		},
		{
			name: "complete true prunes stale items across full list",
			setup: func(t *testing.T, ctx context.Context, s *Store) {
				addTestAccount(t, s, "acc-1", "douyin")
				for _, id := range []string{"1", "2", "3"} {
					if _, err := s.SaveWorks(ctx, "acc-1", "like", true, []Work{testWork(id)}); err != nil {
						t.Fatal(err)
					}
				}
			},
			accountID:     "acc-1",
			kind:          "like",
			seenRemoteIDs: []string{"3"},
			complete:      true,
			wantRemoved:   2,
			verify: func(t *testing.T, ctx context.Context, s *Store) {
				works, _, err := s.ListWorks(ctx, "acc-1", "like", nil, 10)
				if err != nil {
					t.Fatal(err)
				}
				if len(works) != 1 || works[0].RemoteID != "3" {
					t.Fatalf("works = %+v", works)
				}
				for _, id := range []string{"1", "2"} {
					if _, err := s.MediaRecord(ctx, "video", id); err != nil {
						t.Fatalf("works row for %s missing: %v", id, err)
					}
				}
			},
		},
		{
			name: "complete true with empty seenRemoteIDs clears all relations for account and kind",
			setup: func(t *testing.T, ctx context.Context, s *Store) {
				addTestAccount(t, s, "acc-1", "douyin")
				if _, err := s.SaveWorks(ctx, "acc-1", "favorite", true, []Work{testWork("1"), testWork("2")}); err != nil {
					t.Fatal(err)
				}
			},
			accountID:     "acc-1",
			kind:          "favorite",
			seenRemoteIDs: nil,
			complete:      true,
			wantRemoved:   2,
			verify: func(t *testing.T, ctx context.Context, s *Store) {
				n, err := s.CountWorks(ctx, "acc-1", "favorite")
				if err != nil || n != 0 {
					t.Fatalf("count = %d, err = %v", n, err)
				}
				if _, err := s.MediaRecord(ctx, "video", "1"); err != nil {
					t.Fatalf("works row for 1 missing: %v", err)
				}
			},
		},
		{
			name: "complete false with no known anchor removes 0 rows",
			setup: func(t *testing.T, ctx context.Context, s *Store) {
				addTestAccount(t, s, "acc-1", "douyin")
				if _, err := s.SaveWorks(ctx, "acc-1", "recommend", true, []Work{testWork("1"), testWork("2")}); err != nil {
					t.Fatal(err)
				}
			},
			accountID:     "acc-1",
			kind:          "recommend",
			seenRemoteIDs: []string{"99", "100"},
			complete:      false,
			wantRemoved:   0,
			verify: func(t *testing.T, ctx context.Context, s *Store) {
				n, err := s.CountWorks(ctx, "acc-1", "recommend")
				if err != nil || n != 2 {
					t.Fatalf("count = %d, err = %v", n, err)
				}
			},
		},
		{
			name: "complete false with empty seenRemoteIDs removes 0 rows",
			setup: func(t *testing.T, ctx context.Context, s *Store) {
				addTestAccount(t, s, "acc-1", "douyin")
				if _, err := s.SaveWorks(ctx, "acc-1", "recommend", true, []Work{testWork("1")}); err != nil {
					t.Fatal(err)
				}
			},
			accountID:     "acc-1",
			kind:          "recommend",
			seenRemoteIDs: nil,
			complete:      false,
			wantRemoved:   0,
			verify: func(t *testing.T, ctx context.Context, s *Store) {
				n, err := s.CountWorks(ctx, "acc-1", "recommend")
				if err != nil || n != 1 {
					t.Fatalf("count = %d, err = %v", n, err)
				}
			},
		},
		{
			name: "isolates across accounts and kinds",
			setup: func(t *testing.T, ctx context.Context, s *Store) {
				addTestAccount(t, s, "acc-1", "douyin")
				addTestAccount(t, s, "acc-2", "douyin")
				for _, id := range []string{"1", "2", "3"} {
					if _, err := s.SaveWorks(ctx, "acc-1", "recommend", true, []Work{testWork(id)}); err != nil {
						t.Fatal(err)
					}
					if _, err := s.SaveWorks(ctx, "acc-1", "like", true, []Work{testWork(id)}); err != nil {
						t.Fatal(err)
					}
					if _, err := s.SaveWorks(ctx, "acc-2", "recommend", true, []Work{testWork(id)}); err != nil {
						t.Fatal(err)
					}
				}
			},
			accountID:     "acc-1",
			kind:          "recommend",
			seenRemoteIDs: []string{"3", "1"},
			complete:      false,
			wantRemoved:   1,
			verify: func(t *testing.T, ctx context.Context, s *Store) {
				nRec1, _ := s.CountWorks(ctx, "acc-1", "recommend")
				nLike1, _ := s.CountWorks(ctx, "acc-1", "like")
				nRec2, _ := s.CountWorks(ctx, "acc-2", "recommend")
				if nRec1 != 2 || nLike1 != 3 || nRec2 != 3 {
					t.Fatalf("counts acc-1 recommend=%d like=%d acc-2 recommend=%d", nRec1, nLike1, nRec2)
				}
			},
		},
		{
			name: "music kind in music_relations",
			setup: func(t *testing.T, ctx context.Context, s *Store) {
				addTestAccount(t, s, "acc-1", "douyin")
				addTestAccount(t, s, "acc-2", "douyin")
				for _, id := range []string{"m1", "m2", "m3"} {
					m := Music{RemoteID: id, Title: "Track " + id, Author: "Artist"}
					if _, err := s.SaveMusics(ctx, "acc-1", true, []Music{m}); err != nil {
						t.Fatal(err)
					}
					if _, err := s.SaveMusics(ctx, "acc-2", true, []Music{m}); err != nil {
						t.Fatal(err)
					}
				}
			},
			accountID:     "acc-1",
			kind:          "music",
			seenRemoteIDs: []string{"m3", "m1"},
			complete:      false,
			wantRemoved:   1,
			verify: func(t *testing.T, ctx context.Context, s *Store) {
				known1, err := s.KnownWorks(ctx, "acc-1", "music", []string{"m1", "m2", "m3"})
				if err != nil {
					t.Fatal(err)
				}
				if !known1["m1"] || known1["m2"] || !known1["m3"] {
					t.Fatalf("acc-1 music known = %v", known1)
				}
				known2, err := s.KnownWorks(ctx, "acc-2", "music", []string{"m1", "m2", "m3"})
				if err != nil {
					t.Fatal(err)
				}
				if !known2["m1"] || !known2["m2"] || !known2["m3"] {
					t.Fatalf("acc-2 music known = %v", known2)
				}
				if _, err := s.MediaRecord(ctx, "music", "m2"); err != nil {
					t.Fatalf("musics row for m2 missing: %v", err)
				}
			},
		},
		{
			name: "invalid relation kind returns error",
			setup: func(t *testing.T, ctx context.Context, s *Store) {
				addTestAccount(t, s, "acc-1", "douyin")
			},
			accountID:     "acc-1",
			kind:          "invalid",
			seenRemoteIDs: []string{"1"},
			complete:      true,
			wantErr:       true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)
			ctx := context.Background()
			if tc.setup != nil {
				tc.setup(t, ctx, s)
			}
			got, err := s.ReconcileHeadRelations(ctx, tc.accountID, tc.kind, tc.seenRemoteIDs, tc.complete)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if got != tc.wantRemoved {
				t.Fatalf("removed = %d, want %d", got, tc.wantRemoved)
			}
			if tc.verify != nil {
				tc.verify(t, ctx, s)
			}
		})
	}
}
