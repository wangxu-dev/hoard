package app

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wangxu-dev/hoard/internal/store"
)

type biliActionFake struct {
	actionFake
	later    bool
	selected bool
	paths    []string
}

func (f *biliActionFake) ReadBilibili(_ context.Context, path string, _ map[string]string) ([]byte, error) {
	if f.unknown && f.writes > 0 {
		return nil, errors.New("timeout")
	}
	var data any
	switch path {
	case "/x/web-interface/view":
		data = map[string]any{"aid": 42, "bvid": "BV0000000001"}
	case "/x/web-interface/archive/has/like":
		data = 0
		if f.liked {
			data = 1
		}
	case "/x/v3/fav/folder/created/list-all":
		selected := 0
		if f.selected {
			selected = 1
		}
		data = map[string]any{"count": 1, "list": []any{map[string]any{"id": 1, "title": "Example", "media_count": 1, "fav_state": selected}}}
	case "/x/v2/history/toview":
		count := 0
		list := []any{}
		if f.later {
			count = 1
			list = append(list, map[string]any{"aid": 42, "bvid": "BV0000000001", "title": "Example"})
		}
		data = map[string]any{"count": count, "list": list}
	default:
		return nil, errors.New("unexpected read")
	}
	return json.Marshal(map[string]any{"code": 0, "data": data})
}
func (f *biliActionFake) WriteBilibili(_ context.Context, path string, p map[string]string) ([]byte, error) {
	f.writes++
	f.paths = append(f.paths, path)
	switch path {
	case "/x/web-interface/archive/like":
		f.liked = p["like"] == "1"
	case "/x/v3/fav/resource/deal":
		if p["add_media_ids"] == "1" {
			f.selected = true
		}
		if p["del_media_ids"] == "1" {
			f.selected = false
		}
	case "/x/v2/history/toview/add":
		f.later = true
	case "/x/v2/history/toview/del":
		f.later = false
	default:
		return nil, errors.New("unexpected write")
	}
	if f.timeout {
		return nil, errors.New("timeout")
	}
	return []byte(`{"code":0}`), nil
}
func TestBilibiliActions(t *testing.T) {
	for _, tc := range []struct {
		name, kind           string
		on, timeout, unknown bool
		folders              []string
		wantErr              string
	}{
		{name: "like", kind: "like", on: true}, {name: "unlike", kind: "like"},
		{name: "favorite", kind: "favorite", on: true, folders: []string{"1"}}, {name: "unfavorite", kind: "favorite", folders: []string{}},
		{name: "later", kind: "watch_later", on: true}, {name: "remove later", kind: "watch_later"},
		{name: "timeout confirmed", kind: "like", on: true, timeout: true}, {name: "unknown", kind: "like", on: true, unknown: true, wantErr: "ACTION_UNKNOWN"},
		{name: "invalid folder", kind: "favorite", on: true, folders: []string{"2"}, wantErr: "INVALID_ARGUMENT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := store.Open(filepath.Join(t.TempDir(), "library.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			ctx := context.Background()
			if _, err := db.UpsertAccount(ctx, store.Account{ID: "a", Source: "bilibili", RemoteID: "42", SessionName: "bili-a", Status: "active"}); err != nil {
				t.Fatal(err)
			}
			if _, err := db.SaveWorks(ctx, "a", "watch_later", true, []store.Work{{RemoteID: "BV0000000001", Kind: "video", Title: "Example"}}); err != nil {
				t.Fatal(err)
			}
			f := &biliActionFake{actionFake: actionFake{identity: "42", liked: !tc.on, timeout: tc.timeout, unknown: tc.unknown}, later: !tc.on, selected: !tc.on}
			a := &App{Store: db}
			persist := a.bilibiliPersistence()
			actions := &Actions{app: a, engine: func() actionEngine { return f }}
			if tc.folders != nil {
				_, err = actions.SetFolders("a", "BV0000000001", tc.folders)
			} else {
				_, err = actions.Set("a", "BV0000000001", tc.kind, tc.on)
			}
			if (err == nil) != (tc.wantErr == "") || err != nil && !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err=%v want=%s", err, tc.wantErr)
			}
			wantWrites := 1
			if tc.wantErr == "INVALID_ARGUMENT" {
				wantWrites = 0
			}
			if f.writes != wantWrites {
				t.Fatalf("writes=%d", f.writes)
			}
			if f.session != "bili-a" {
				t.Fatal("wrong session")
			}
			if tc.wantErr == "" {
				state, err := actions.State("a", "BV0000000001")
				if err != nil || state.value(tc.kind) != tc.on {
					t.Fatalf("state=%+v err=%v", state, err)
				}
			}
			if tc.wantErr == "" && persist(ctx, func() error { t.Fatal("stale save called"); return nil }) == nil {
				t.Fatal("stale response accepted")
			}
		})
	}
}
