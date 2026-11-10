package app

import (
	"context"
	"github.com/wangxu-dev/hoard/internal/explore"
	"github.com/wangxu-dev/hoard/internal/store"
	"path/filepath"
	"testing"
	"time"
)

func TestExplorationOfflineSingleSourceAndEmpty(t *testing.T) {
	for _, source := range []string{"", "douyin", "bilibili"} {
		t.Run("only-"+source, func(t *testing.T) {
			db, err := store.Open(filepath.Join(t.TempDir(), "fixture.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			ctx := context.Background()
			if source != "" {
				if _, err = db.UpsertAccount(ctx, store.Account{ID: "fixture-account", Source: source, RemoteID: "fixture-owner", SessionName: "fixture-session", Status: "expired"}); err != nil {
					t.Fatal(err)
				}
				if _, err = db.SaveWorks(ctx, "fixture-account", "like", false, []store.Work{{RemoteID: "fixture-work", Kind: "video", Title: "fixture"}}); err != nil {
					t.Fatal(err)
				}
			}
			svc := NewExploration(func() (*store.Store, string) { return db, t.TempDir() })
			defer CloseExploration(svc)
			task, err := svc.Start(explore.Scope{}, "fixture-view")
			if err != nil {
				t.Fatal(err)
			}
			m, _ := svc.get()
			wait, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			snap, err := m.Wait(wait, task.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(snap.Scopes) != 2 {
				t.Fatal("missing empty source snapshot")
			}
			total := 0
			for _, s := range snap.Scopes {
				total += len(s.Works)
				if s.Source != source && len(s.Works) != 0 {
					t.Fatal("unavailable source invented works")
				}
			}
			want := 0
			if source != "" {
				want = 1
			}
			if total != want {
				t.Fatal("local input lost without a login session")
			}
			bad, err := svc.Start(explore.Scope{AccountID: "nonexistent-account"}, "unknown-account")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = m.Wait(wait, bad.ID); err == nil {
				t.Fatal("unknown account borrowed union")
			}
			state, _ := svc.Status(bad.ID)
			if state.Error != "INVALID_SCOPE" {
				t.Fatal("scope error lost")
			}
			db.Close()
			broken, err := svc.Start(explore.Scope{}, "closed-store")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = m.Wait(wait, broken.ID); err == nil {
				t.Fatal("store failure disguised as an empty snapshot")
			}
		})
	}
}
