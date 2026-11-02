package app

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wangxu-dev/hoard/internal/douyin"
	"github.com/wangxu-dev/hoard/internal/relay"
	"github.com/wangxu-dev/hoard/internal/store"
)

func TestCachedLikesRoundTrip(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	media, err := relay.New()
	if err != nil {
		t.Fatal(err)
	}
	writer := &App{Store: db}
	if err := rememberForTest(writer, douyin.LoginState{LoggedIn: true, UID: "test-account", Nickname: "Test account"}); err != nil {
		t.Fatal(err)
	}
	want := douyin.LikesResult{
		OK: true, Cursor: "99", HasMore: true,
		Items: []douyin.LikeItem{{
			ID: "1", Desc: "Test work", Author: "Test author",
			Cover: "https://example.test/cover.jpg", Duration: 19000, CreatedAt: 1700000000, AwemeType: 4,
			SourceURL: "https://v.douyinvod.com/test.mp4", Images: []string{"https://example.test/image.jpg"},
		}},
	}
	if err := writer.persist(want, true); err != nil {
		t.Fatal(err)
	}
	if err := writer.persist(douyin.LikesResult{OK: false}, false); err != nil {
		t.Fatal(err)
	}
	if err := rememberForTest(writer, douyin.LoginState{}); err != nil {
		t.Fatal(err)
	}

	reader := &App{Store: db, Media: media}
	got, err := reader.CachedLikes()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || !strings.HasPrefix(got.Items[0].PlayURL, "http://127.0.0.1:") || !strings.Contains(got.Items[0].PlayURL, "/video/") {
		t.Fatal("cached playback address was not registered")
	}
	want.Items[0].PlayURL = got.Items[0].PlayURL
	if !reflect.DeepEqual(got, want) || reader.uid != writer.uid || reader.nickname != writer.nickname {
		t.Fatal("cached library or remembered account changed")
	}
}

func TestCachedLikesEmpty(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, tt := range []struct {
		name string
		app  *App
	}{
		{"without store", &App{}},
		{"without account", &App{Store: db}},
		{"without cached works", &App{Store: db, uid: "test-account"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.app.CachedLikes()
			if err != nil || !reflect.DeepEqual(got, douyin.LikesResult{OK: true, Items: []douyin.LikeItem{}}) {
				t.Fatal("empty library result changed")
			}
		})
	}
}
