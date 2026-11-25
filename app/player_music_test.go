package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wangxu-dev/hoard/internal/douyin"
	"github.com/wangxu-dev/hoard/internal/store"
)

type musicTestEngine struct {
	*fakeAccountEngine
	calls  int
	lastOp douyin.Operation
	lastID string
	reply  func(douyin.Operation, map[string]string) (douyin.PageReply, error)
}

func (e *musicTestEngine) Request(_ context.Context, op douyin.Operation, params map[string]string) (douyin.PageReply, error) {
	e.calls++
	e.lastOp = op
	e.lastID = params["music_id"]
	return e.reply(op, params)
}

func setupMusicTest(t *testing.T, id string, extra json.RawMessage) (*App, *Player, *musicTestEngine) {
	t.Helper()
	a, _, base := newAccountTestApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if _, err := a.Store.UpsertAccount(ctx, store.Account{ID: "acc", Source: "douyin", RemoteID: "r1", SessionName: "s1", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Store.SaveMusics(ctx, "acc", true, []store.Music{{
		RemoteID: id,
		Title:    "Demo Song",
		Author:   "Artist",
		Extra:    extra,
	}}); err != nil {
		t.Fatal(err)
	}
	eng := &musicTestEngine{fakeAccountEngine: base}
	a.Engine = eng
	a.accountID = "acc"
	a.sessionCtx = ctx
	return a, NewPlayer(a), eng
}

func TestPlayerMusicRefresh(t *testing.T) {
	t.Run("unsigned valid music resolves from store", func(t *testing.T) {
		storedURL := "https://test.douyinvod.com/stored.mp3"
		a, p, eng := setupMusicTest(t, "7000000000000000001", json.RawMessage(fmt.Sprintf(`{"play_url":%q,"covers":["https://test.douyinpic.com/c.jpg"]}`, storedURL)))
		eng.reply = func(douyin.Operation, map[string]string) (douyin.PageReply, error) {
			t.Fatal("OpMusicDetail should not be called for valid unsigned music")
			return douyin.PageReply{}, nil
		}
		res, err := p.Media("music", "7000000000000000001")
		if err != nil {
			t.Fatalf("Media failed: %v", err)
		}
		if eng.calls != 0 || res.Kind != "music" || res.Complete || res.ExpiresAt != 0 ||
			!strings.HasPrefix(res.Music.URL, "http://127.0.0.1:") || res.Music.URL != a.Media.Register(storedURL) {
			t.Fatalf("unexpected result: %+v calls=%d", res, eng.calls)
		}
	})

	t.Run("expired signed music refreshes and persists play_url", func(t *testing.T) {
		past := time.Now().Add(-10 * time.Minute).Unix()
		future := time.Now().Add(2 * time.Hour).Unix()
		oldURL := fmt.Sprintf("https://test.douyinvod.com/expired.mp3?x-expires=%d", past)
		newURL := fmt.Sprintf("https://test.douyinvod.com/fresh.mp3?x-expires=%d", future)
		a, p, eng := setupMusicTest(t, "7000000000000000002", json.RawMessage(fmt.Sprintf(`{"play_url":%q,"covers":["https://test.douyinpic.com/c.jpg"]}`, oldURL)))
		eng.reply = func(op douyin.Operation, params map[string]string) (douyin.PageReply, error) {
			return douyin.PageReply{
				Status: 200,
				Body:   fmt.Sprintf(`{"status_code":0,"music_info":{"id_str":"7000000000000000002","title":"Demo Song","author":"Artist","play_url":{"url_list":[%q]}}}`, newURL),
			}, nil
		}
		res, err := p.Media("music", "7000000000000000002")
		if err != nil {
			t.Fatalf("Media failed: %v", err)
		}
		if eng.calls != 1 || eng.lastOp != douyin.OpMusicDetail || eng.lastID != "7000000000000000002" {
			t.Fatalf("unexpected engine call: calls=%d op=%s id=%s", eng.calls, eng.lastOp, eng.lastID)
		}
		if !res.Complete || res.ExpiresAt == 0 ||
			!strings.HasPrefix(res.Music.URL, "http://127.0.0.1:") || res.Music.URL != a.Media.Register(newURL) {
			t.Fatalf("unexpected result: %+v", res)
		}
		rec, err := a.Store.MediaRecord(context.Background(), "music", "7000000000000000002")
		if err != nil {
			t.Fatal(err)
		}
		var stored struct {
			PlayURL string   `json:"play_url"`
			Covers  []string `json:"covers"`
		}
		if err := json.Unmarshal(rec.Extra, &stored); err != nil {
			t.Fatal(err)
		}
		if stored.PlayURL != newURL || len(stored.Covers) != 1 || stored.Covers[0] != "https://test.douyinpic.com/c.jpg" {
			t.Fatalf("unexpected persisted extra: %s", string(rec.Extra))
		}
	})

	t.Run("explicit refresh calls OpMusicDetail even when unsigned", func(t *testing.T) {
		storedURL := "https://test.douyinvod.com/stored.mp3"
		refreshedURL := "https://test.douyinvod.com/refreshed.mp3"
		a, p, eng := setupMusicTest(t, "7000000000000000003", json.RawMessage(fmt.Sprintf(`{"play_url":%q,"covers":["https://test.douyinpic.com/c.jpg"]}`, storedURL)))
		eng.reply = func(op douyin.Operation, params map[string]string) (douyin.PageReply, error) {
			return douyin.PageReply{
				Status: 200,
				Body:   fmt.Sprintf(`{"status_code":0,"music_info":{"id_str":"7000000000000000003","title":"Demo Song","author":"Artist","play_url":{"url_list":[%q]}}}`, refreshedURL),
			}, nil
		}
		res, err := p.Refresh("music", "7000000000000000003")
		if err != nil {
			t.Fatalf("Refresh failed: %v", err)
		}
		if eng.calls != 1 || eng.lastOp != douyin.OpMusicDetail || eng.lastID != "7000000000000000003" {
			t.Fatalf("unexpected engine call: calls=%d op=%s id=%s", eng.calls, eng.lastOp, eng.lastID)
		}
		if !res.Complete || !strings.HasPrefix(res.Music.URL, "http://127.0.0.1:") || res.Music.URL != a.Media.Register(refreshedURL) {
			t.Fatalf("unexpected result: %+v", res)
		}
		rec, err := a.Store.MediaRecord(context.Background(), "music", "7000000000000000003")
		if err != nil {
			t.Fatal(err)
		}
		var stored struct {
			PlayURL string `json:"play_url"`
		}
		if err := json.Unmarshal(rec.Extra, &stored); err != nil {
			t.Fatal(err)
		}
		if stored.PlayURL != refreshedURL {
			t.Fatalf("persisted play_url = %q", stored.PlayURL)
		}
	})
}
