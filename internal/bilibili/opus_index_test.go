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
	c := Client{Completed: func(kind string) { completed = append(completed, kind) }, Read: func(_ context.Context, path string, params map[string]string) ([]byte, error) {
		switch path {
		case "/x/v2/history/toview":
			return []byte(`{"code":0,"data":{"count":0,"list":[]}}`), nil
		case "/x/v3/fav/folder/created/list-all":
			return []byte(`{"code":0,"data":{"count":0,"list":[]}}`), nil
		case opusFeed:
			page := params["page"]
			return []byte(fmt.Sprintf(`{"code":0,"data":{"has_more":%t,"items":[{"opus_id":"%s","content":"Example","cover":{"url":"https://i0.hdslb.com/example.jpg"}}]}}`, page == "1", page)), nil
		}
		t.Fatalf("unexpected path %s", path)
		return nil, nil
	}}
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
}
