package bilibili

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wangxu-dev/hoard/internal/store"
)

func TestOpusResumeAfterReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "library.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.UpsertAccount(ctx, store.Account{ID: "test", Source: "bilibili", RemoteID: "42", SessionName: "test", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	fail := true
	pages := []string{}
	c := Client{Read: func(_ context.Context, path string, p map[string]string) ([]byte, error) {
		switch path {
		case opusFeed:
			page := p["page"]
			pages = append(pages, page)
			if page == "2" && fail {
				return nil, errors.New("offline")
			}
			return []byte(fmt.Sprintf(`{"code":0,"data":{"has_more":%t,"items":[{"opus_id":"%s","content":"Example"}]}}`, page == "1", page)), nil
		case "/x/polymer/web-dynamic/v1/opus/detail":
			return []byte(fmt.Sprintf(`{"code":0,"data":{"item":{"id_str":"%s","modules":[{"module_author":{"mid":42,"name":"Example","pub_ts":1700000000}}]}}}`, p["id"])), nil
		}
		return nil, fmt.Errorf("unexpected endpoint")
	}}
	if err = c.indexOpus(ctx, db, "test"); err == nil {
		t.Fatal("failure hidden")
	}
	checkpoint, err := db.SyncCheckpoint(ctx, "test", "opus")
	if err != nil || checkpoint.NextPage != 2 {
		t.Fatalf("checkpoint %+v %v", checkpoint, err)
	}
	db.Close()
	db, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	fail = false
	pages = nil
	if err = c.indexOpus(ctx, db, "test"); err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 || pages[0] != "2" || pages[1] != "2" {
		t.Fatal(pages)
	}
	n, err := db.CountWorks(ctx, "test", "favorite")
	if err != nil || n != 2 {
		t.Fatalf("count %d %v", n, err)
	}
}
