package bilibili

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/wangxu-dev/hoard/internal/store"
)

func TestIndexAndObserve(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.UpsertAccount(ctx, store.Account{ID: "test", Source: "bilibili", RemoteID: "42", SessionName: "test", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	client := Client{Read: func(ctx context.Context, path string, params map[string]string) ([]byte, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("request has no timeout")
		}
		calls++
		switch path {
		case opusFeed:
			return []byte(`{"code":0,"data":{"items":[],"has_more":false}}`), nil
		case "/x/v2/history/toview":
			return []byte(`{"code":0,"data":{"count":1,"list":[{"aid":123,"bvid":"BV0000000001","title":"Example","owner":{"mid":42}}]}}`), nil
		case "/x/v3/fav/folder/created/list-all":
			return []byte(`{"code":0,"data":{"count":2,"list":[{"id":1,"title":"First","media_count":1},{"id":2,"title":"Second","media_count":1}]}}`), nil
		case "/x/v3/fav/resource/list":
			return []byte(`{"code":0,"data":{"has_more":false,"medias":[{"id":123,"bvid":"BV0000000001","type":2,"attr":1,"title":"Example","upper":{"mid":42}}]}}`), nil
		case "/x/web-interface/archive/has/like":
			return []byte(`{"code":0,"data":0}`), nil
		case "/x/v2/fav/video/favoured":
			if params["aid"] != "123" {
				t.Fatal("missing numeric archive ID")
			}
			return []byte(`{"code":0,"data":{"favoured":true}}`), nil
		case "/x/web-interface/archive/coins":
			return nil, errors.New("offline")
		}
		t.Fatal("unexpected path")
		return nil, nil
	}}
	videos, err := client.Index(ctx, db, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(videos) != 1 || calls != 5 {
		t.Fatalf("deduplication: videos=%d calls=%d", len(videos), calls)
	}
	for _, kind := range []string{"watch_later", "favorite"} {
		count, err := db.CountWorks(ctx, "test", kind)
		if err != nil || count != 1 {
			t.Fatalf("list %s count=%d err=%v", kind, count, err)
		}
		_, total, err := db.Library(ctx, store.LibraryQuery{AccountID: "test", Kind: kind, Type: "all", Limit: 10})
		if err != nil || total != 1 {
			t.Fatalf("library %s: %d %v", kind, total, err)
		}
	}
	for _, folderID := range []string{"1", "2", "missing"} {
		_, total, err := db.Library(ctx, store.LibraryQuery{AccountID: "test", Kind: "favorite", FolderID: folderID, Limit: 10})
		want := 1
		if folderID == "missing" {
			want = 0
		}
		if err != nil || total != want {
			t.Fatalf("folder filter: %d %v", total, err)
		}
	}
	stored, _, err := db.ListWorks(ctx, "test", "favorite", nil, 10)
	if err != nil || len(stored) != 1 || stored[0].Available == nil || *stored[0].Available {
		t.Fatalf("unavailable video lost: %v", err)
	}
	if _, err := db.SaveWorks(ctx, "test", "favorite", false, []store.Work{{RemoteID: videos[0].ID, Kind: "video", Title: "Example"}}); err != nil {
		t.Fatal(err)
	}
	stored, _, err = db.ListWorks(ctx, "test", "favorite", nil, 10)
	if err != nil || stored[0].Available == nil || *stored[0].Available {
		t.Fatal("unknown availability overwrote known false")
	}
	folders, err := db.Folders(ctx, "test")
	if err != nil || len(folders) != 2 {
		t.Fatalf("folders: %v %v", folders, err)
	}
	if err = client.Observe(ctx, db, "test", videos[0]); err == nil {
		t.Fatal("endpoint failure hidden")
	}
	state, err := db.PersonalState(ctx, "test", videos[0].ID)
	if err != nil || state.Liked == nil || *state.Liked || state.Favorited == nil || !*state.Favorited || state.Coins != nil {
		t.Fatalf("partial observations: %+v %v", state, err)
	}
}
