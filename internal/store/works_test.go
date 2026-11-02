package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestSharedWorksAndCleanup(t *testing.T) {
	for _, tt := range []struct {
		name         string
		secondSource string
		wantWorks    int
	}{
		{"same source", "alpha", 1},
		{"different sources", "beta", 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestStore(t)
			ctx := context.Background()
			a := addTestAccount(t, s, "account-a", "alpha")
			b := addTestAccount(t, s, "account-b", tt.secondSource)
			for _, id := range []string{a, b} {
				if err := s.WriteWorkPage(ctx, WorkPage{AccountID: id, Kind: "like", Phase: "head", Cursor: "next", Works: []Work{testWork("shared")}}); err != nil {
					t.Fatal(err)
				}
			}
			assertRows(t, s, "works", tt.wantWorks)
			assertRows(t, s, "authors", tt.wantWorks)
			for _, id := range []string{a, b} {
				if n, err := s.CountWorks(ctx, id, "like"); err != nil || n != 1 {
					t.Fatalf("count: %d %v", n, err)
				}
			}
			if n, err := s.CountWorks(ctx, "other-account", "like"); err != nil || n != 0 {
				t.Fatalf("isolated count: %d %v", n, err)
			}
			if err := s.RemoveRelation(ctx, a, "like", "alpha:shared"); err != nil {
				t.Fatal(err)
			}
			assertRows(t, s, "works", tt.wantWorks)
			// Cleanup preserves every work that another account still references.
			removed, err := s.CleanupOrphanWorks(ctx)
			wantRemoved := tt.wantWorks - 1
			if err != nil || removed != int64(wantRemoved) {
				t.Fatalf("first cleanup: %d %v", removed, err)
			}
			assertRows(t, s, "works", 1)
			if err := s.RemoveRelation(ctx, b, "like", tt.secondSource+":shared"); err != nil {
				t.Fatal(err)
			}
			assertRows(t, s, "works", 1)
			if removed, err := s.CleanupOrphanWorks(ctx); err != nil || removed != 1 {
				t.Fatalf("last cleanup: %d %v", removed, err)
			}
			assertRows(t, s, "works", 0)
			if removed, err := s.CleanupOrphanWorks(ctx); err != nil || removed != 0 {
				t.Fatalf("repeat cleanup: %d %v", removed, err)
			}
		})
	}
}

func assertRows(t *testing.T, s *Store, table string, want int) {
	t.Helper()
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil || n != want {
		t.Fatalf("%s rows=%d want=%d err=%v", table, n, want, err)
	}
}

func TestWorkPagination(t *testing.T) {
	for _, tt := range []struct {
		name  string
		limit int
		ties  bool
	}{
		{"single", 1, false}, {"partial last page", 3, false}, {"exact page", 4, false}, {"one page", 100, false}, {"equal sequences", 3, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestStore(t)
			ctx := context.Background()
			id := addTestAccount(t, s, "account-a", "alpha")
			addTestAccount(t, s, "account-b", "alpha")
			for _, page := range []WorkPage{
				{AccountID: id, Kind: "like", Phase: "head", Cursor: "c1", Works: []Work{testWork("3"), testWork("4")}},
				{AccountID: id, Kind: "like", Phase: "backfill", Cursor: "c2", Works: []Work{testWork("5"), testWork("6")}},
				{AccountID: id, Kind: "like", Phase: "head", Cursor: "c3", Works: []Work{testWork("1"), testWork("2")}},
				{AccountID: id, Kind: "like", Phase: "backfill", Cursor: "c4", Complete: true, Works: []Work{testWork("7"), testWork("8")}},
				{AccountID: id, Kind: "like", Phase: "backfill", Cursor: "c4", Complete: true, Works: []Work{testWork("7"), testWork("8")}},
				{AccountID: "account-b", Kind: "like", Phase: "head", Cursor: "c1", Works: []Work{testWork("private")}},
				{AccountID: id, Kind: "favorite", Phase: "head", Cursor: "c1", Works: []Work{testWork("favorite")}},
			} {
				if err := s.WriteWorkPage(ctx, page); err != nil {
					t.Fatal(err)
				}
			}
			if tt.ties {
				if _, err := s.db.Exec(`UPDATE relations SET seq=0 WHERE account_id=? AND kind='like'`, id); err != nil {
					t.Fatal(err)
				}
			}
			var after *WorkCursor
			var got []string
			seen := map[string]bool{}
			var previous *Work
			for {
				works, next, err := s.ListWorks(ctx, id, "like", after, tt.limit)
				if err != nil {
					t.Fatal(err)
				}
				if len(works) > tt.limit {
					t.Fatal("page exceeds limit")
				}
				for _, w := range works {
					if seen[w.ID] {
						t.Fatal("duplicate work")
					}
					if previous != nil && (previous.Seq < w.Seq || (previous.Seq == w.Seq && previous.ID >= w.ID)) {
						t.Fatal("invalid keyset order")
					}
					seen[w.ID] = true
					got = append(got, w.RemoteID)
					copy := w
					previous = &copy
				}
				if next == nil {
					break
				}
				if len(got) > 8 {
					t.Fatal("pagination did not terminate")
				}
				after = next
			}
			if !reflect.DeepEqual(got, []string{"1", "2", "3", "4", "5", "6", "7", "8"}) {
				t.Fatalf("page order: %v", got)
			}
			if n, err := s.CountWorks(ctx, id, "like"); err != nil || n != 8 {
				t.Fatalf("count: %d %v", n, err)
			}
			state, err := s.SyncState(ctx, id, "like")
			if err != nil || state.Phase != "done" || state.HeadCursor != "c3" || state.BackfillCursor != "c4" {
				t.Fatalf("state: %+v %v", state, err)
			}
		})
	}
}

func TestInitialSyncPhase(t *testing.T) {
	for _, tt := range []struct {
		phase        string
		wantHead     string
		wantBackfill string
		wantHeadSeq  int64
	}{
		{"head", "next", "", 1},
		{"backfill", "0", "next", 0},
	} {
		t.Run(tt.phase, func(t *testing.T) {
			s := newTestStore(t)
			ctx := context.Background()
			id := addTestAccount(t, s, "account-a", "alpha")
			if err := s.WriteWorkPage(ctx, WorkPage{AccountID: id, Kind: "like", Phase: tt.phase, Cursor: "next", Works: []Work{testWork("1")}}); err != nil {
				t.Fatal(err)
			}
			state, err := s.SyncState(ctx, id, "like")
			if err != nil || state.HeadCursor != tt.wantHead || state.BackfillCursor != tt.wantBackfill || state.HeadSeq != tt.wantHeadSeq {
				t.Fatalf("initial state: %+v %v", state, err)
			}
		})
	}
}

func TestWorkPageRollback(t *testing.T) {
	for _, tt := range []struct {
		name    string
		initial bool
		failAt  string
	}{
		{"first page", false, "relation"},
		{"existing cursor", true, "relation"},
		{"cursor update", true, "cursor"},
		{"invalid JSON", true, "json"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestStore(t)
			ctx := context.Background()
			id := addTestAccount(t, s, "account-a", "alpha")
			var before SyncState
			if tt.initial {
				if err := s.WriteWorkPage(ctx, WorkPage{AccountID: id, Kind: "like", Phase: "head", Cursor: "old-cursor", Works: []Work{testWork("original")}}); err != nil {
					t.Fatal(err)
				}
				var err error
				before, err = s.SyncState(ctx, id, "like")
				if err != nil {
					t.Fatal(err)
				}
			}
			works := []Work{testWork("first"), testWork("fail")}
			works[0].Author = &Author{RemoteID: "new-author", Nickname: "New author"}
			switch tt.failAt {
			case "relation":
				if _, err := s.db.Exec(`CREATE TRIGGER reject_relation BEFORE INSERT ON relations WHEN NEW.work_id='alpha:fail' BEGIN SELECT RAISE(ABORT,'test relation failure'); END`); err != nil {
					t.Fatal(err)
				}
			case "cursor":
				if _, err := s.db.Exec(`CREATE TRIGGER reject_cursor BEFORE UPDATE ON sync_state BEGIN SELECT RAISE(ABORT,'test cursor failure'); END`); err != nil {
					t.Fatal(err)
				}
			case "json":
				works[1].Extra = []byte(`invalid`)
			}
			if err := s.WriteWorkPage(ctx, WorkPage{AccountID: id, Kind: "like", Phase: "backfill", Cursor: "new-cursor", Works: works}); err == nil {
				t.Fatal("expected page failure")
			}
			state, err := s.SyncState(ctx, id, "like")
			wantRows := 0
			if tt.initial {
				wantRows = 1
				if err != nil || state != before {
					t.Fatalf("cursor advanced: %+v %v", state, err)
				}
			} else if !errors.Is(err, sql.ErrNoRows) {
				t.Fatalf("failed first page saved state: %v", err)
			}
			assertRows(t, s, "works", wantRows)
			assertRows(t, s, "authors", wantRows)
			assertRows(t, s, "relations", wantRows)
		})
	}
}

func TestRelationKindsAndAccountCascade(t *testing.T) {
	for _, removeAccount := range []bool{false, true} {
		t.Run(fmt.Sprintf("delete account %t", removeAccount), func(t *testing.T) {
			s := newTestStore(t)
			ctx := context.Background()
			id := addTestAccount(t, s, "account-a", "alpha")
			for _, kind := range []string{"like", "favorite"} {
				if err := s.WriteWorkPage(ctx, WorkPage{AccountID: id, Kind: kind, Phase: "head", Works: []Work{testWork("1")}}); err != nil {
					t.Fatal(err)
				}
			}
			assertRows(t, s, "works", 1)
			if err := s.RemoveRelation(ctx, id, "like", "alpha:1"); err != nil {
				t.Fatal(err)
			}
			if n, err := s.CleanupOrphanWorks(ctx); err != nil || n != 0 {
				t.Fatalf("favorite lost: %d %v", n, err)
			}
			if removeAccount {
				if _, err := s.db.Exec(`DELETE FROM accounts WHERE id=?`, id); err != nil {
					t.Fatal(err)
				}
				assertRows(t, s, "relations", 0)
				assertRows(t, s, "sync_state", 0)
			} else if err := s.RemoveRelation(ctx, id, "favorite", "alpha:1"); err != nil {
				t.Fatal(err)
			}
			if n, err := s.CleanupOrphanWorks(ctx); err != nil || n != 1 {
				t.Fatalf("cleanup: %d %v", n, err)
			}
		})
	}
}
