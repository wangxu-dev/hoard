package bilibili

import (
	"context"
	"fmt"
	"github.com/wangxu-dev/hoard/internal/store"
	"path/filepath"
	"testing"
)

func TestIncrementalFolderStopsAtKnownPage(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.UpsertAccount(ctx, store.Account{ID: "test", Source: "bilibili", RemoteID: "42", SessionName: "test", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	pages := 0
	count := 2
	fail := false
	c := Client{Read: func(_ context.Context, path string, params map[string]string) ([]byte, error) {
		switch path {
		case "/x/v2/history/toview":
			return []byte(`{"code":0,"data":{"count":0,"list":[]}}`), nil
		case "/x/v3/fav/folder/created/list-all":
			return []byte(fmt.Sprintf(`{"code":0,"data":{"count":1,"list":[{"id":1,"title":"Test","media_count":%d}]}}`, count)), nil
		case "/x/v3/fav/resource/list":
			pages++
			if fail {
				return nil, fmt.Errorf("offline")
			}
			return []byte(fmt.Sprintf(`{"code":0,"data":{"has_more":%t,"medias":[{"id":%s,"bvid":"BV000000000%s","type":2,"fav_time":100}]}}`, params["pn"] == "1", params["pn"], params["pn"])), nil
		case opusFeed:
			return []byte(`{"code":0,"data":{"has_more":false,"items":[]}}`), nil
		}
		return nil, fmt.Errorf("unexpected endpoint")
	}}
	if _, err = c.Index(ctx, db, "test"); err != nil {
		t.Fatal(err)
	}
	if pages != 2 {
		t.Fatalf("initial pages %d", pages)
	}
	c.Incremental = true
	pages = 0
	if _, err = c.Index(ctx, db, "test"); err != nil {
		t.Fatal(err)
	}
	if pages != 0 {
		t.Fatalf("incremental pages %d", pages)
	}
	count = 3
	fail = true
	if _, err = c.Index(ctx, db, "test"); err == nil {
		t.Fatal("failure hidden")
	}
	folders, err := db.Folders(ctx, "test")
	if err != nil || folders[0].Count != 2 {
		t.Fatalf("failed traversal advanced count: %+v %v", folders, err)
	}
	fail = false
	pages = 0
	if _, err = c.Index(ctx, db, "test"); err != nil {
		t.Fatal(err)
	}
	if pages != 1 {
		t.Fatalf("changed count should check first page: %d", pages)
	}
}
