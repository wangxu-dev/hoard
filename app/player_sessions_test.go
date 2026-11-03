package app

import (
	"context"
	"testing"

	"github.com/wangxu-dev/hoard/internal/douyin"
	"github.com/wangxu-dev/hoard/internal/store"
)

func TestPlaybackAccountSelection(t *testing.T) {
	accounts := []store.Account{{ID: "a", Status: "signed_out"}, {ID: "b", Status: "active"}, {ID: "c", Status: "active"}}
	for _, tc := range []struct{ name, requested, active, want, code string }{
		{"active owner", "", "c", "c", ""}, {"other platform active", "", "other", "b", ""},
		{"explicit owner", "b", "c", "b", ""}, {"signed out owner", "a", "c", "", "AUTH_REQUIRED"},
		{"non owner", "other", "c", "", "UNAVAILABLE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := playbackAccount(accounts, tc.requested, tc.active)
			if tc.code != "" {
				if err == nil || err.Error() != tc.code {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil || got.ID != tc.want {
				t.Fatalf("account = %s, error = %v", got.ID, err)
			}
		})
	}
}

type playbackTestEngine struct{ *fakeAccountEngine }

func (e *playbackTestEngine) Request(context.Context, douyin.Operation, map[string]string) (douyin.PageReply, error) {
	return douyin.PageReply{Status: 200, Body: `{"status_code":0,"aweme_detail":{"video":{"play_addr":{"url_list":["https://test.douyinvod.com/test.mp4"]}}}}`}, nil
}

func TestPlaybackUsesOwnerSessionWithoutSwitchingSync(t *testing.T) {
	a, _, syncEngine := newAccountTestApp(t)
	ctx := context.Background()
	for _, account := range []store.Account{
		{ID: "owner", Source: "douyin", RemoteID: "test-owner", SessionName: "owner-session", Status: "active"},
		{ID: "sync", Source: "bilibili", RemoteID: "test-sync", SessionName: "sync-session", Status: "active"},
	} {
		if _, err := a.Store.UpsertAccount(ctx, account); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.Store.SaveWorks(ctx, "owner", "favorite", false, []store.Work{{RemoteID: "1234567890123456789", Kind: "video"}}); err != nil {
		t.Fatal(err)
	}
	a.accountID = "sync"
	syncCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	a.sessionCtx = syncCtx
	if err := syncEngine.UseSourceSession("bilibili", "sync-session", false); err != nil {
		t.Fatal(err)
	}
	p := NewPlayer(a)
	opened := 0
	var playback *playbackTestEngine
	p.engine = func() AccountEngine {
		opened++
		playback = &playbackTestEngine{&fakeAccountEngine{sessions: map[string]*fakeSession{}}}
		return playback
	}
	for range 2 {
		media, err := p.Resolve("video", "1234567890123456789", "", true, "")
		if err != nil || len(media.Videos) != 1 {
			t.Fatalf("playback videos=%d err=%v", len(media.Videos), err)
		}
	}
	if p.sessions["owner"].player.requestApp() != a {
		t.Fatal("playback bypassed the shared request gate")
	}
	if opened != 1 || playback.current != "owner-session" {
		t.Fatalf("opened=%d session=%s", opened, playback.current)
	}
	if a.accountID != "sync" || syncEngine.current != "sync-session" || syncCtx.Err() != nil {
		t.Fatal("playback switched the sync session")
	}
	p.closeAccount("owner")
	if playback.open {
		t.Fatal("playback window was not closed")
	}
}

func TestRestoringWithoutActiveSettingSelectsOnlyAccount(t *testing.T) {
	a, _, engine := newAccountTestApp(t)
	if _, err := a.Store.UpsertAccount(context.Background(), store.Account{ID: "only", Source: "douyin", RemoteID: "test-owner", SessionName: "only-session", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if err := RestoreAccounts(a); err != nil {
		t.Fatal(err)
	}
	if a.accountID != "only" || engine.current != "only-session" {
		t.Fatal("single account was not restored")
	}
}

func TestStartupVerificationKeepsSavedCursor(t *testing.T) {
	a, _, _ := newAccountTestApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := a.Store.UpsertAccount(ctx, store.Account{ID: "a", Source: "douyin", RemoteID: "test-owner", SessionName: "session", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if err := a.Store.PutSyncRecord(ctx, "a", store.SyncRecord{Kind: "like", Phase: "backfill", Status: "running", BackfillCursor: "saved-cursor"}); err != nil {
		t.Fatal(err)
	}
	a.accountID, a.sessionCtx = "a", ctx
	sync := NewSync(a)
	a.loginBlocked(ctx)
	views, err := sync.States()
	if err != nil {
		t.Fatal(err)
	}
	for _, view := range views {
		if view.Kind == "like" && (view.Status != "blocked" || view.ErrorCode != "NEED_VERIFY") {
			t.Fatalf("view = %+v", view)
		}
	}
	records, err := a.Store.SyncRecords(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if record.Kind == "like" && record.BackfillCursor != "saved-cursor" {
			t.Fatal("saved cursor was changed")
		}
	}
}
