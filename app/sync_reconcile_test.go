package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/wangxu-dev/hoard/internal/douyin"
	"github.com/wangxu-dev/hoard/internal/store"
)

func TestSyncHeadReconcileRemovesUnrecommendedWork(t *testing.T) {
	a, _, eng := newAccountTestApp(t)
	ctx := context.Background()
	if _, err := a.Store.UpsertAccount(ctx, store.Account{ID: "test", Source: "douyin", RemoteID: "42", SessionName: "test", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"1", "2", "3"} {
		if _, err := a.Store.SaveWorks(ctx, "test", "recommend", true, []store.Work{{RemoteID: id, Kind: "video", Title: "Work " + id}}); err != nil {
			t.Fatal(err)
		}
	}

	a.Engine = &syncTestEngine{fakeAccountEngine: eng, request: func(op douyin.Operation, p map[string]string) (douyin.PageReply, error) {
		a.lastRequest = time.Time{}
		if op != douyin.OpRecommend {
			return douyin.PageReply{}, fmt.Errorf("unexpected op %s", op)
		}
		// Remote head now has fresh "4", plus known "3" and "1"; "2" was un-recommended.
		return douyin.PageReply{
			Status: 200,
			Body:   `{"status_code":0,"has_more":1,"max_cursor":"next","aweme_list":[{"aweme_id":"4"},{"aweme_id":"3"},{"aweme_id":"1"}]}`,
		}, nil
	}}

	s := NewSync(a)
	r := store.SyncRecord{Kind: "recommend", Phase: "done", Items: 3}
	stop, more := s.syncKind(ctx, "test", "fixture", &r, 10, true)
	if stop || more {
		t.Fatalf("unexpected syncKind result stop=%v more=%v", stop, more)
	}

	n, err := a.Store.CountWorks(ctx, "test", "recommend")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("CountWorks = %d, want 3 (4, 3, 1)", n)
	}
	if r.Items != 3 {
		t.Fatalf("SyncRecord.Items = %d, want 3", r.Items)
	}
	records, err := a.Store.SyncRecords(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Items != 3 {
		t.Fatalf("SyncRecords = %+v, want Items=3", records)
	}
	known, err := a.Store.KnownWorks(ctx, "test", "recommend", []string{"1", "2", "3", "4"})
	if err != nil {
		t.Fatal(err)
	}
	if !known["1"] || known["2"] || !known["3"] || !known["4"] {
		t.Fatalf("known = %v, want 1,3,4 present and 2 removed", known)
	}
	// Works row for un-recommended "2" must still be preserved.
	if _, err := a.Store.MediaRecord(ctx, "video", "2"); err != nil {
		t.Fatalf("works row for 2 was deleted: %v", err)
	}
}
