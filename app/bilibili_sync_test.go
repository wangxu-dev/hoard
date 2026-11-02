package app

import (
	"context"
	"errors"
	"testing"

	"github.com/wangxu-dev/hoard/internal/store"
)

type bilibiliSyncTestEngine struct {
	*fakeAccountEngine
	read func(context.Context, string, map[string]string) ([]byte, error)
}

func (e *bilibiliSyncTestEngine) ReadBilibili(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	return e.read(ctx, path, params)
}

func TestBilibiliSyncBusyThroughObservation(t *testing.T) {
	for _, outcome := range []string{"success", "failure", "cancel"} {
		t.Run(outcome, func(t *testing.T) {
			a, _, engine := newAccountTestApp(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if _, err := a.Store.UpsertAccount(ctx, store.Account{ID: "test", Source: "bilibili", RemoteID: "42", SessionName: "test", Status: "active"}); err != nil {
				t.Fatal(err)
			}
			a.accountID, a.sessionCtx = "test", ctx
			engine.open = true
			syncer := NewSync(a)
			observed := false
			a.Engine = &bilibiliSyncTestEngine{fakeAccountEngine: engine, read: func(ctx context.Context, path string, _ map[string]string) ([]byte, error) {
				switch path {
				case "/x/v2/history/toview":
					return []byte(`{"code":0,"data":{"count":1,"list":[{"aid":123,"bvid":"BV0000000001","title":"Example","owner":{"mid":42}}]}}`), nil
				case "/x/v3/fav/folder/created/list-all":
					return []byte(`{"code":0,"data":{"count":0,"list":[]}}`), nil
				case "/x/polymer/web-dynamic/v1/opus/feed/fav":
					return []byte(`{"code":0,"data":{"has_more":false,"items":[]}}`), nil
				case "/x/web-interface/archive/has/like":
					observed = true
					records, err := a.Store.SyncRecords(ctx, "test")
					if err != nil {
						t.Fatal(err)
					}
					for _, r := range records {
						if r.Status != "checking" || r.Phase != "done" {
							t.Fatalf("premature completion: %+v", r)
						}
					}
					if outcome == "failure" {
						return nil, errors.New("offline")
					}
					if outcome == "cancel" {
						cancel()
						return nil, ctx.Err()
					}
					return []byte(`{"code":0,"data":0}`), nil
				case "/x/v2/fav/video/favoured":
					return []byte(`{"code":0,"data":{"favoured":false}}`), nil
				case "/x/web-interface/archive/coins":
					return []byte(`{"code":0,"data":{"multiply":0}}`), nil
				}
				t.Fatalf("unexpected endpoint %s", path)
				return nil, nil
			}}
			syncer.runBilibili(ctx, "test")
			if !observed {
				t.Fatal("observation not reached")
			}
			records, err := a.Store.SyncRecords(context.Background(), "test")
			if err != nil {
				t.Fatal(err)
			}
			want := "idle"
			if outcome != "success" {
				want = "paused"
			}
			for _, r := range records {
				if r.Status != want {
					t.Fatalf("final state: %+v want %s", r, want)
				}
			}
		})
	}
}
