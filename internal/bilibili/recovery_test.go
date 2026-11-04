package bilibili

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/wangxu-dev/hoard/internal/store"
)

func TestFolderResumeAfterReopen(t *testing.T) {
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
	pages := []int{}
	c := Client{Read: func(_ context.Context, path string, p map[string]string) ([]byte, error) {
		switch path {
		case "/x/v2/history/toview":
			return []byte(`{"code":0,"data":{"count":0,"list":[]}}`), nil
		case "/x/v3/fav/folder/created/list-all":
			return []byte(`{"code":0,"data":{"count":1,"list":[{"id":1,"title":"Test","media_count":3}]}}`), nil
		case "/x/v3/fav/resource/list":
			page, _ := strconv.Atoi(p["pn"])
			pages = append(pages, page)
			if page == 2 && fail {
				return nil, errors.New("offline")
			}
			return []byte(fmt.Sprintf(`{"code":0,"data":{"has_more":%t,"medias":[{"id":%d,"bvid":"BV000000000%d","type":2,"fav_time":%d}]}}`, page < 3, page, page, page)), nil
		case opusFeed:
			return []byte(`{"code":0,"data":{"has_more":false,"items":[]}}`), nil
		}
		return nil, fmt.Errorf("unexpected endpoint")
	}}
	if _, err = c.Index(ctx, db, "test"); err == nil {
		t.Fatal("failure hidden")
	}
	cp, err := db.SyncCheckpoint(ctx, "test", "folder:1")
	if err != nil || cp.NextPage != 2 {
		t.Fatalf("checkpoint %+v %v", cp, err)
	}
	db.Close()
	db, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	fail = false
	pages = nil
	if _, err = c.Index(ctx, db, "test"); err != nil {
		t.Fatal(err)
	}
	if len(pages) != 3 || pages[0] != 2 || pages[1] != 3 || pages[2] != 3 {
		t.Fatalf("resumed requests %v", pages)
	}
	n, err := db.CountWorks(ctx, "test", "favorite")
	if err != nil || n != 3 {
		t.Fatalf("count %d %v", n, err)
	}
	cp, err = db.SyncCheckpoint(ctx, "test", "folder:1")
	if err != nil || cp.NextPage != 0 {
		t.Fatal("completed checkpoint retained")
	}
	members, err := db.FolderMembers(ctx, "test", "1")
	if err != nil || len(members) != 3 {
		t.Fatalf("members %d %v", len(members), err)
	}
}

func TestReadRestrictionCodes(t *testing.T) {
	for _, tc := range []struct {
		code int
		want error
	}{{-101, ErrAuth}, {-509, ErrRateLimited}, {-412, ErrRiskControl}, {-352, ErrVerification}} {
		if err := CheckResponse([]byte(fmt.Sprintf(`{"code":%d}`, tc.code))); !errors.Is(err, tc.want) {
			t.Fatalf("%d: %v", tc.code, err)
		}
	}
}
