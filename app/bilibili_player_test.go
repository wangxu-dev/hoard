package app

import (
	"context"
	"testing"
	"time"

	"github.com/wangxu-dev/hoard/internal/store"
)

func (e *bilibiliSyncTestEngine) Source() string { return "bilibili" }

func TestBilibiliPlaybackReturnsBeforePersonalState(t *testing.T) {
	a, _, engine := newAccountTestApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.accountID, a.sessionCtx = "test", ctx
	if _, err := a.Store.UpsertAccount(ctx, store.Account{ID: "test", Source: "bilibili", RemoteID: "42", SessionName: "test", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	started, release, ended := make(chan struct{}), make(chan struct{}), make(chan struct{})
	a.Engine = &bilibiliSyncTestEngine{fakeAccountEngine: engine, read: func(ctx context.Context, path string, _ map[string]string) ([]byte, error) {
		switch path {
		case "/x/web-interface/view":
			return []byte(`{"code":0,"data":{"cid":123}}`), nil
		case "/x/player/playurl":
			return []byte(`{"code":0,"data":{"quality":16,"format":"mp4","durl":[{"url":"https://test.bilivideo.com/video.mp4"}]}}`), nil
		case "/x/web-interface/archive/has/like":
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
			}
			close(ended)
			return nil, context.Canceled
		}
		t.Errorf("unexpected endpoint %s", path)
		return nil, context.Canceled
	}}
	result, err := NewPlayer(a).bilibiliMedia(ctx, store.MediaRecord{Source: "bilibili", RemoteID: "BV0000000001", Extra: []byte(`{"aid":"123"}`)})
	if err != nil || len(result.Videos) != 1 {
		t.Fatalf("playback: %+v %v", result, err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("on-demand refresh did not start")
	}
	close(release)
	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		t.Fatal("refresh did not exit")
	}
}
