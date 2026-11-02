package bilibili

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/wangxu-dev/hoard/internal/store"
)

func TestIndexStopsOnRepeatedPage(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.UpsertAccount(ctx, store.Account{ID: "test", Source: "bilibili", RemoteID: "42", SessionName: "test", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	pages := 0
	c := Client{Read: func(_ context.Context, path string, _ map[string]string) ([]byte, error) {
		switch path {
		case "/x/v2/history/toview":
			return []byte(`{"code":0,"data":{"count":0,"list":[]}}`), nil
		case "/x/v3/fav/folder/created/list-all":
			return []byte(`{"code":0,"data":{"count":1,"list":[{"id":1,"title":"Example","media_count":40}]}}`), nil
		default:
			pages++
			return []byte(`{"code":0,"data":{"has_more":true,"medias":[{"id":123,"bvid":"BV0000000001","type":2,"title":"Example","upper":{"mid":42}}]}}`), nil
		}
	}}
	if _, err = c.Index(ctx, db, "test"); err == nil || pages != 2 {
		t.Fatalf("pagination did not stop: %d %v", pages, err)
	}
	count, err := db.CountWorks(ctx, "test", "favorite")
	if err != nil || count != 1 {
		t.Fatalf("saved page lost: %d %v", count, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	before := pages
	if _, err = c.Index(canceled, db, "test"); err == nil || pages != before {
		t.Fatal("canceled index made a request")
	}
}
