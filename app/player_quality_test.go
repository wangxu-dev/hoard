package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wangxu-dev/hoard/internal/douyin"
	"github.com/wangxu-dev/hoard/internal/store"
)

type qualityTestEngine struct {
	*fakeAccountEngine
	reply func(douyin.Operation, map[string]string) (douyin.PageReply, error)
}

func (e *qualityTestEngine) Request(_ context.Context, op douyin.Operation, params map[string]string) (douyin.PageReply, error) {
	return e.reply(op, params)
}

func TestPlayerMediaReturnsQualitiesSortedAndExcludesHEVC(t *testing.T) {
	a, _, _ := newAccountTestApp(t)
	ctx := context.Background()
	if _, err := a.Store.UpsertAccount(ctx, store.Account{
		ID:          "acc",
		Source:      "douyin",
		RemoteID:    "test-owner",
		SessionName: "owner-session",
		Status:      "active",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Store.SaveWorks(ctx, "acc", "like", false, []store.Work{{
		RemoteID: "7000000000000000101",
		Kind:     "video",
	}}); err != nil {
		t.Fatal(err)
	}
	a.accountID = "acc"

	future := time.Now().Add(2 * time.Hour).Unix()
	url540 := "https://test.douyinvod.com/540.mp4"
	url720 := "https://test.douyinvod.com/720.mp4"
	url1080 := fmt.Sprintf("https://test.douyinvod.com/1080.mp4?x-expires=%d", future)
	urlHEVC := "https://test.douyinvod.com/hevc.mp4"

	p := NewPlayer(a)
	t.Cleanup(func() { p.closeAccount("") })
	p.engine = func() AccountEngine {
		return &qualityTestEngine{
			fakeAccountEngine: &fakeAccountEngine{sessions: map[string]*fakeSession{}},
			reply: func(op douyin.Operation, params map[string]string) (douyin.PageReply, error) {
				if op != douyin.OpDetail || params["aweme_id"] != "7000000000000000101" {
					t.Fatalf("unexpected request: op=%s params=%v", op, params)
				}
				return douyin.PageReply{
					Status: 200,
					Body: fmt.Sprintf(`{"status_code":0,"aweme_detail":{"video":{"bit_rate":[`+
						`{"gear_name":"normal_540_0","bit_rate":500000,"play_addr":{"url_list":[%q]}},`+
						`{"gear_name":"normal_1080_bytevc1","is_bytevc1":1,"bit_rate":2000000,"play_addr":{"url_list":[%q]}},`+
						`{"gear_name":"normal_1080_0","bit_rate":1800000,"play_addr":{"url_list":[%q]}},`+
						`{"gear_name":"normal_720_0","bit_rate":900000,"play_addr":{"url_list":[%q]}}`+
						`]}}}`, url540, urlHEVC, url1080, url720),
				}, nil
			},
		}
	}

	res, err := p.Media("video", "7000000000000000101")
	if err != nil {
		t.Fatalf("Media failed: %v", err)
	}
	if len(res.Qualities) != 3 {
		t.Fatalf("qualities len = %d, want 3: %+v", len(res.Qualities), res.Qualities)
	}
	wantOrder := []struct {
		id    string
		label string
		raw   string
	}{
		{id: "1080", label: "1080P", raw: url1080},
		{id: "720", label: "720P", raw: url720},
		{id: "540", label: "540P", raw: url540},
	}
	for i, w := range wantOrder {
		got := res.Qualities[i]
		if got.ID != w.id || got.Label != w.label {
			t.Fatalf("qualities[%d] = (%q, %q), want (%q, %q)", i, got.ID, got.Label, w.id, w.label)
		}
		if len(got.Videos) != 1 || !strings.HasPrefix(got.Videos[0], "http://127.0.0.1:") {
			t.Fatalf("qualities[%d].Videos = %v", i, got.Videos)
		}
		if got.Videos[0] != a.Media.Register(w.raw) {
			t.Fatalf("qualities[%d].Videos[0] = %q, want relay for %q", i, got.Videos[0], w.raw)
		}
	}
	hevcRelay := a.Media.Register(urlHEVC)
	for _, u := range res.Videos {
		if u == hevcRelay {
			t.Fatalf("HEVC URL leaked into Videos: %q", u)
		}
	}
	if res.ExpiresAt == 0 {
		t.Fatal("expected ExpiresAt to reflect quality URL expiry")
	}
}
