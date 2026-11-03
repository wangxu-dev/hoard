package app

import (
	"github.com/wangxu-dev/hoard/internal/store"
	"testing"
)

func TestAutomaticSyncDue(t *testing.T) {
	for _, tc := range []struct {
		name          string
		age           int64
		phase, status string
		missing       bool
		want          bool
	}{
		{"recent", 21, "done", "idle", false, false},
		{"exact boundary", 180, "done", "idle", false, false},
		{"older", 181, "done", "idle", false, true},
		{"unfinished", 21, "backfill", "paused", false, true},
		{"never synced", 0, "idle", "idle", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			last := int64(1000) - tc.age
			records := []store.SyncRecord{}
			for _, kind := range syncKinds {
				r := store.SyncRecord{Kind: kind, Phase: tc.phase, Status: tc.status, LastFullAt: &last}
				if tc.missing {
					r.LastFullAt = nil
				}
				records = append(records, r)
			}
			if got := automaticSyncDue("douyin", records, 1000); got != tc.want {
				t.Fatalf("due=%t want=%t", got, tc.want)
			}
		})
	}
	last := int64(1000)
	if automaticSyncDue("bilibili", []store.SyncRecord{{Kind: "favorite", Phase: "done", LastFullAt: &last}, {Kind: "watch_later", Phase: "done", LastFullAt: &last}}, 1000) {
		t.Fatal("Bilibili required Douyin lists")
	}
	if !automaticSyncDue("douyin", nil, 1000) {
		t.Fatal("missing lists skipped")
	}
}
