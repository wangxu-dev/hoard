package app

import (
	"context"
	"errors"
	"github.com/wangxu-dev/hoard/internal/bilibili"
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

func TestBilibiliPlaybackMissingSessionIsNotExpiredLogin(t *testing.T) {
	a, _, _ := newAccountTestApp(t)
	_, err := NewPlayer(a).bilibiliMedia(context.Background(), store.MediaRecord{Source: "bilibili", RemoteID: "BV0000000001"})
	if err == nil || err.Error() != "SESSION_NOT_READY" {
		t.Fatalf("missing session: %v", err)
	}
}

func TestBilibiliPlaybackWaitsForPageAndPreservesAuth(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"ready after navigation", `{"code":0,"data":{"quality":16,"format":"mp4","durl":[{"url":"https://test.bilivideo.com/video.mp4"}]}}`, ""},
		{"confirmed API auth", `{"code":-101}`, "AUTH_REQUIRED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _, engine := newAccountTestApp(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			a.accountID, a.sessionCtx = "test", ctx
			reads := 0
			a.Engine = &bilibiliSyncTestEngine{fakeAccountEngine: engine, read: func(_ context.Context, path string, _ map[string]string) ([]byte, error) {
				reads++
				if reads == 1 {
					return nil, errors.New("NOT_READY")
				}
				if path == "/x/web-interface/view" {
					return []byte(`{"code":0,"data":{"cid":123}}`), nil
				}
				return []byte(tc.body), nil
			}}
			result, err := NewPlayer(a).bilibiliMedia(ctx, store.MediaRecord{Source: "bilibili", RemoteID: "BV0000000001"})
			if tc.want == "" {
				if err != nil || len(result.Videos) != 1 {
					t.Fatalf("playback: %+v %v", result, err)
				}
			} else if err == nil || err.Error() != tc.want {
				t.Fatalf("error: %v", err)
			}
			if reads != 3 {
				t.Fatalf("reads = %d", reads)
			}
		})
	}
	if err := playbackError(bilibili.ErrInvalid); err.Error() != "UNAVAILABLE" {
		t.Fatal(err)
	}
}
