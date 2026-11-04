package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/wangxu-dev/hoard/internal/douyin"
	"github.com/wangxu-dev/hoard/internal/store"
)

func TestSyncConfirmsEndBeforeAdvancing(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprint(changed), func(t *testing.T) {
			a, _, eng := newAccountTestApp(t)
			ctx := context.Background()
			if _, err := a.Store.UpsertAccount(ctx, store.Account{ID: "test", Source: "douyin", RemoteID: "42", SessionName: "test", Status: "active"}); err != nil {
				t.Fatal(err)
			}
			calls := 0
			a.Engine = &syncTestEngine{fakeAccountEngine: eng, request: func(_ douyin.Operation, p map[string]string) (douyin.PageReply, error) {
				calls++
				a.lastRequest = time.Time{}
				id, cursor, more := "1", "next", 1
				if p["max_cursor"] == "next" {
					id, cursor, more = "2", "end", 0
					if changed && calls == 3 {
						id = "3"
					}
				}
				return douyin.PageReply{Status: 200, Body: fmt.Sprintf(`{"status_code":0,"has_more":%d,"max_cursor":"%s","aweme_list":[{"aweme_id":"%s"}]}`, more, cursor, id)}, nil
			}}
			s := NewSync(a)
			r := store.SyncRecord{Kind: "like", Phase: "idle"}
			s.syncKind(ctx, "test", "fixture", &r, 10, false)
			records, err := a.Store.SyncRecords(ctx, "test")
			if err != nil {
				t.Fatal(err)
			}
			n, err := a.Store.CountWorks(ctx, "test", "like")
			if err != nil {
				t.Fatal(err)
			}
			if changed {
				if n != 1 || records[0].BackfillCursor != "next" || records[0].Phase == "done" {
					t.Fatalf("unconfirmed end advanced: %d %+v", n, records)
				}
			} else {
				if n != 2 || records[0].Phase != "done" || records[0].BackfillCursor != "" {
					t.Fatalf("confirmed end not committed: %d %+v", n, records)
				}
			}
			if calls != 3 {
				t.Fatal(calls)
			}
		})
	}
}
