package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/wangxu-dev/hoard/internal/bilibili"
	"github.com/wangxu-dev/hoard/internal/douyin"
	"github.com/wangxu-dev/hoard/internal/store"
)

type syncTestEngine struct {
	*fakeAccountEngine
	request func(douyin.Operation, map[string]string) (douyin.PageReply, error)
}

func (e *syncTestEngine) Request(_ context.Context, op douyin.Operation, p map[string]string) (douyin.PageReply, error) {
	return e.request(op, p)
}
func (e *syncTestEngine) PageState(context.Context) douyin.PageState {
	return douyin.PageState{Ready: true}
}
func (e *syncTestEngine) Recycle() error      { return nil }
func (e *syncTestEngine) Visible() bool       { return false }
func (e *syncTestEngine) CookiePresent() bool { return true }

func TestSyncRestoresCooldown(t *testing.T) {
	for _, source := range []string{"douyin", "bilibili"} {
		t.Run(source, func(t *testing.T) {
			a, _, eng := newAccountTestApp(t)
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
			defer cancel()
			if _, err := a.Store.UpsertAccount(ctx, store.Account{ID: "test", Source: source, RemoteID: "42", SessionName: "test", Status: "active"}); err != nil {
				t.Fatal(err)
			}
			retry := time.Now().Add(time.Minute).Unix()
			r := store.SyncRecord{Kind: "favorite", Phase: "backfill", Status: "cooling", BackfillCursor: "next", RetryAt: &retry, DelayMS: 1200, Failures: 2}
			if err := a.Store.PutSyncRecord(ctx, "test", r); err != nil {
				t.Fatal(err)
			}
			records, err := a.Store.SyncRecords(ctx, "test")
			if err != nil {
				t.Fatal(err)
			}
			r = records[0]
			calls := 0
			s := NewSync(a)
			if source == "douyin" {
				a.Engine = &syncTestEngine{fakeAccountEngine: eng, request: func(douyin.Operation, map[string]string) (douyin.PageReply, error) {
					calls++
					return douyin.PageReply{}, nil
				}}
				_, out := s.fetch(ctx, "test", &r, "fixture", "next")
				if out != fetchCancel {
					t.Fatal(out)
				}
			} else {
				_, err = s.readBilibiliSync(ctx, "test", &r, func() ([]byte, error) { calls++; return nil, nil })
				if err == nil {
					t.Fatal("expected cancellation")
				}
			}
			if calls != 0 {
				t.Fatal("cooldown bypassed")
			}
			records, err = a.Store.SyncRecords(context.Background(), "test")
			if err != nil {
				t.Fatal(err)
			}
			if records[0].RetryAt == nil || *records[0].RetryAt != retry || records[0].BackfillCursor != "next" || records[0].DelayMS != 1200 {
				t.Fatal(records)
			}
		})
	}
}

func TestSyncRateLimitPersistsSlowdown(t *testing.T) {
	for _, source := range []string{"douyin", "bilibili"} {
		t.Run(source, func(t *testing.T) {
			a, _, eng := newAccountTestApp(t)
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
			defer cancel()
			if _, err := a.Store.UpsertAccount(ctx, store.Account{ID: "test", Source: source, RemoteID: "42", SessionName: "test", Status: "active"}); err != nil {
				t.Fatal(err)
			}
			s := NewSync(a)
			r := store.SyncRecord{Kind: "favorite", Phase: "backfill", BackfillCursor: "next"}
			calls := 0
			if source == "douyin" {
				a.Engine = &syncTestEngine{fakeAccountEngine: eng, request: func(douyin.Operation, map[string]string) (douyin.PageReply, error) {
					calls++
					return douyin.PageReply{Status: 429}, nil
				}}
				s.fetch(ctx, "test", &r, "fixture", "next")
			} else {
				s.readBilibiliSync(ctx, "test", &r, func() ([]byte, error) { calls++; return nil, bilibili.ErrRateLimited })
			}
			records, err := a.Store.SyncRecords(context.Background(), "test")
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 || len(records) != 1 || records[0].RetryAt == nil || records[0].Failures != 1 || records[0].DelayMS < 1000 || records[0].BackfillCursor != "next" {
				t.Fatalf("calls=%d records=%+v", calls, records)
			}
		})
	}
}

func TestSyncIncrementalKeepsNewItemsAfterKnown(t *testing.T) {
	a, _, eng := newAccountTestApp(t)
	ctx := context.Background()
	if _, err := a.Store.UpsertAccount(ctx, store.Account{ID: "test", Source: "douyin", RemoteID: "42", SessionName: "test", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Store.SaveWorks(ctx, "test", "like", false, []store.Work{{RemoteID: "1", Kind: "video"}}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	a.Engine = &syncTestEngine{fakeAccountEngine: eng, request: func(_ douyin.Operation, p map[string]string) (douyin.PageReply, error) {
		calls++
		a.lastRequest = time.Time{}
		ids := `{"aweme_id":"1"},{"aweme_id":"2"}`
		cursor, more := "next", 1
		if p["max_cursor"] != "0" {
			ids = `{"aweme_id":"1"}`
			cursor = "end"
			more = 0
		}
		return douyin.PageReply{Status: 200, Body: fmt.Sprintf(`{"status_code":0,"has_more":%d,"max_cursor":"%s","aweme_list":[%s]}`, more, cursor, ids)}, nil
	}}
	s := NewSync(a)
	r := store.SyncRecord{Kind: "like", Phase: "done", Items: 1}
	s.syncKind(ctx, "test", "fixture", &r, 10, true)
	n, err := a.Store.CountWorks(ctx, "test", "like")
	if err != nil || n != 2 || calls != 2 {
		t.Fatalf("count=%d calls=%d err=%v", n, calls, err)
	}
}

func TestRecoverPaceAndExpiredCooldown(t *testing.T) {
	r := store.SyncRecord{DelayMS: 1000, Failures: 2, Status: "cooling"}
	past := time.Now().Add(-time.Minute).Unix()
	r.RetryAt = &past
	if !(&Sync{}).resumeCooling(context.Background(), "test", &r) || r.RetryAt != nil || r.Status != "running" {
		t.Fatal(r)
	}
	recoverPace(&r, douyinSyncGap)
	if r.DelayMS != 990 || r.Failures != 2 {
		t.Fatal(r)
	}
	for i := 0; i < 100; i++ {
		recoverPace(&r, douyinSyncGap)
	}
	if r.DelayMS != 300 || r.Failures != 0 {
		t.Fatal(r)
	}
}

func TestBackgroundWaitReleasesLane(t *testing.T) {
	a := &App{lastRequest: time.Now()}
	s := &Sync{app: a}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.frontRequests.Store(1)
	done := make(chan error, 1)
	go func() { done <- s.lockBackground(ctx, douyinSyncGap) }()
	acquired := make(chan struct{})
	go func() { a.requestMu.Lock(); a.requestMu.Unlock(); close(acquired) }()
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("background wait held lane")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancellation ignored")
		}
	case <-time.After(time.Second):
		t.Fatal("background stuck")
	}
}
