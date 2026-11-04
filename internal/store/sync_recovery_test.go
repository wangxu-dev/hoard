package store

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestSyncPageProcessExit(t *testing.T) {
	if path := os.Getenv("HOARD_SYNC_CRASH_DB"); path != "" {
		ctx := context.Background()
		s, err := Open(path)
		if err != nil {
			os.Exit(30)
		}
		retry := time.Now().Add(5 * time.Minute).Unix()
		r := SyncRecord{Kind: "like", Phase: "backfill", Status: "cooling", BackfillCursor: "next", Pages: 1, RetryAt: &retry, Failures: 2, DelayMS: 1200}
		if os.Getenv("HOARD_SYNC_CRASH_COMMIT") == "1" {
			if s.SaveSyncPage(ctx, "test", false, []Work{{RemoteID: "1", Kind: "video"}}, nil, r) != nil {
				os.Exit(31)
			}
		} else {
			tx, e := s.db.BeginTx(ctx, nil)
			if e != nil {
				os.Exit(32)
			}
			if _, e = saveWorks(ctx, tx, "test", "like", false, []Work{{RemoteID: "1", Kind: "video"}}); e != nil {
				os.Exit(33)
			}
			if putSyncRecord(ctx, tx, "test", r) != nil {
				os.Exit(34)
			}
		}
		// Exit without Close/Rollback, as with an abruptly terminated process.
		os.Exit(23)
	}
	for _, commit := range []string{"0", "1"} {
		t.Run(commit, func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "library.db")
			s, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.UpsertAccount(ctx, Account{ID: "test", Source: "douyin", RemoteID: "42", SessionName: "test", Status: "active"}); err != nil {
				t.Fatal(err)
			}
			if err = s.PutSyncRecord(ctx, "test", SyncRecord{Kind: "like", Phase: "backfill", Status: "running", BackfillCursor: "0"}); err != nil {
				t.Fatal(err)
			}
			s.Close()
			cmd := exec.Command(os.Args[0], "-test.run=^TestSyncPageProcessExit$")
			cmd.Env = append(os.Environ(), "HOARD_SYNC_CRASH_DB="+path, "HOARD_SYNC_CRASH_COMMIT="+commit)
			err = cmd.Run()
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 23 {
				t.Fatalf("child failed: %v", err)
			}
			s, err = Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			var integrity string
			if err = s.db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
				t.Fatalf("integrity %q %v", integrity, err)
			}
			records, err := s.SyncRecords(ctx, "test")
			if err != nil {
				t.Fatal(err)
			}
			n, err := s.CountWorks(ctx, "test", "like")
			if err != nil {
				t.Fatal(err)
			}
			if commit == "0" {
				if n != 0 || records[0].BackfillCursor != "0" {
					t.Fatalf("uncommitted page advanced %+v count=%d", records, n)
				}
			} else {
				r := records[0]
				if n != 1 || r.BackfillCursor != "next" || r.DelayMS != 1200 || r.Failures != 2 || r.RetryAt == nil || *r.RetryAt <= time.Now().Unix() {
					t.Fatalf("recovery lost %+v count=%d", r, n)
				}
			}
			if err = s.SaveSyncPage(ctx, "test", false, []Work{{RemoteID: "1", Kind: "video"}}, nil, SyncRecord{Kind: "like", Phase: "backfill", Status: "running", BackfillCursor: "next"}); err != nil {
				t.Fatal(err)
			}
			n, err = s.CountWorks(ctx, "test", "like")
			if err != nil || n != 1 {
				t.Fatalf("replay count=%d %v", n, err)
			}
		})
	}
}

func TestSyncPageRollbackAndMusic(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	addTestAccount(t, s, "test", "douyin")
	r := SyncRecord{Kind: "like", Phase: "backfill", Status: "running", BackfillCursor: "next"}
	if err := s.SaveSyncPage(ctx, "test", false, []Work{{RemoteID: "1", Kind: "video"}, {Kind: "invalid"}}, nil, r); err == nil {
		t.Fatal("invalid page accepted")
	}
	n, _ := s.CountWorks(ctx, "test", "like")
	records, _ := s.SyncRecords(ctx, "test")
	if n != 0 || len(records) != 0 {
		t.Fatal("partial page committed")
	}
	r.Kind = "music"
	if err := s.SaveSyncPage(ctx, "test", false, nil, []Music{{RemoteID: "1"}}, r); err != nil {
		t.Fatal(err)
	}
	records, _ = s.SyncRecords(ctx, "test")
	if records[0].Items != 1 || records[0].BackfillCursor != "next" {
		t.Fatal(records)
	}
}
