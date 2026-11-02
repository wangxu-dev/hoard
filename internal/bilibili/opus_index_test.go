package bilibili

import (
	"context"
	"fmt"
	"github.com/wangxu-dev/hoard/internal/store"
	"path/filepath"
	"testing"
)

func TestOpusIndexAndCompletion(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.UpsertAccount(ctx, store.Account{ID: "test", Source: "bilibili", RemoteID: "42", SessionName: "test", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	completed := []string{}
	detailCalls, feedCalls := 0, 0
	c := Client{Completed: func(kind string) { completed = append(completed, kind) }, Read: func(_ context.Context, path string, params map[string]string) ([]byte, error) {
		switch path {
		case "/x/v2/history/toview":
			return []byte(`{"code":0,"data":{"count":0,"list":[]}}`), nil
		case "/x/v3/fav/folder/created/list-all":
			return []byte(`{"code":0,"data":{"count":0,"list":[]}}`), nil
		case "/x/polymer/web-dynamic/v1/opus/detail":
			detailCalls++
			return []byte(fmt.Sprintf(`{"code":0,"data":{"item":{"id_str":"%s","basic":{"title":"Example"},"modules":[{"module_author":{"mid":42,"name":"Example author","pub_ts":"1700000000"}}]}}}`, params["id"])), nil
		case opusFeed:
			feedCalls++
			page := params["page"]
			return []byte(fmt.Sprintf(`{"code":0,"data":{"has_more":%t,"items":[{"opus_id":"%s","content":"Example","cover":{"url":"https://i0.hdslb.com/example.jpg"}}]}}`, page == "1", page)), nil
		}
		t.Fatalf("unexpected path %s", path)
		return nil, nil
	}}
	if _, err = db.SaveWorks(ctx, "test", "favorite", false, []store.Work{{RemoteID: "opus:1", Kind: "gallery", Title: "Old summary"}}); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Index(ctx, db, "test"); err != nil {
		t.Fatal(err)
	}
	if len(completed) != 2 || completed[0] != "watch_later" || completed[1] != "favorite" {
		t.Fatalf("completion %v", completed)
	}
	works, _, err := db.ListWorks(ctx, "test", "favorite", nil, 10)
	if err != nil || len(works) != 2 || works[0].Kind != "gallery" {
		t.Fatalf("gallery index: %+v %v", works, err)
	}
	months, err := db.Timeline(ctx, store.LibraryQuery{AccountID: "test", Kind: "favorite", Sort: "published"})
	if err != nil || len(months) != 1 || months[0].Year != 2023 {
		t.Fatalf("resynced timeline: %+v %v", months, err)
	}
	for _, w := range works {
		if w.Author == nil || w.Author.Nickname != "Example author" || w.PublishedAt != 1700000000 {
			t.Fatalf("metadata missing: %+v", w)
		}
	}
	c.Incremental = true
	detailCalls, feedCalls = 0, 0
	if _, err = c.Index(ctx, db, "test"); err != nil {
		t.Fatal(err)
	}
	if detailCalls != 0 || feedCalls != 1 {
		t.Fatalf("incremental detail=%d feed=%d", detailCalls, feedCalls)
	}
}
