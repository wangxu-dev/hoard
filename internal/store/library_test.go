package store

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveWorksKeepsListOrder(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.UpsertAccount(ctx, Account{ID: "a", Source: "douyin", RemoteID: "r", SessionName: "s", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	work := func(id, title string, kind string) Work {
		return Work{RemoteID: id, Kind: kind, Title: title, Author: &Author{RemoteID: "u" + id, Nickname: "author " + id}}
	}
	if n, err := s.SaveWorks(ctx, "a", "like", false, []Work{work("3", "third", "video"), work("2", "second", "gallery")}); err != nil || n != 2 {
		t.Fatalf("backfill: %d %v", n, err)
	}
	if _, err := s.SaveWorks(ctx, "a", "like", false, []Work{work("1", "first", "video")}); err != nil {
		t.Fatal(err)
	}
	if n, err := s.SaveWorks(ctx, "a", "like", true, []Work{work("5", "newest", "video"), work("4", "newer", "video")}); err != nil || n != 2 {
		t.Fatalf("head: %d %v", n, err)
	}
	if n, _ := s.SaveWorks(ctx, "a", "like", false, []Work{work("1", "first", "video")}); n != 0 {
		t.Fatalf("duplicate counted as new")
	}
	items, total, err := s.Library(ctx, LibraryQuery{AccountID: "a", Kind: "like", Sort: "recent", Limit: 10})
	if err != nil || total != 5 {
		t.Fatalf("list: %d %v", total, err)
	}
	order := ""
	for _, it := range items {
		order += it.RemoteID
	}
	if order != "54321" {
		t.Fatalf("order %s", order)
	}
	if _, total, _ := s.Library(ctx, LibraryQuery{AccountID: "a", Kind: "like", Type: "gallery", Limit: 10}); total != 1 {
		t.Fatalf("type filter total %d", total)
	}
	if items, total, _ := s.Library(ctx, LibraryQuery{Kind: "like", Query: "author 4", Limit: 10}); total != 1 || items[0].RemoteID != "4" {
		t.Fatalf("search by author failed")
	}
	known, _ := s.KnownWorks(ctx, "a", "like", []string{"4", "9"})
	if !known["4"] || known["9"] {
		t.Fatalf("known works %v", known)
	}
	if err := s.PutSyncRecord(ctx, "a", SyncRecord{Kind: "like", Phase: "backfill", Status: "running", BackfillCursor: "123"}); err != nil {
		t.Fatal(err)
	}
	records, err := s.SyncRecords(ctx, "a")
	if err != nil || len(records) != 1 || records[0].Items != 5 || records[0].BackfillCursor != "123" {
		t.Fatalf("records %+v %v", records, err)
	}
}

func TestTimeline(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.UpsertAccount(ctx, Account{ID: "a", Source: "douyin", RemoteID: "r", SessionName: "s", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	empty, err := s.Timeline(ctx, LibraryQuery{AccountID: "a", Kind: "like", Sort: "published"})
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty library: %v %v", empty, err)
	}
	// Mid-month times stay in their month in any time zone.
	mid := func(year, month int) int64 {
		return time.Date(year, time.Month(month), 15, 12, 0, 0, 0, time.UTC).Unix()
	}
	works := []Work{}
	add := func(n int, at int64, kind, title string) {
		for i := 0; i < n; i++ {
			id := fmt.Sprintf("%s%d-%d", kind, at, i)
			works = append(works, Work{RemoteID: id, Kind: kind, Title: title, PublishedAt: at, Author: &Author{RemoteID: "u" + id, Nickname: "author"}})
		}
	}
	add(3, mid(2026, 10), "video", "cat")
	add(2, mid(2026, 10), "gallery", "dog")
	add(4, mid(2026, 3), "video", "dog")
	add(1, mid(2025, 12), "gallery", "cat")
	add(2, 0, "video", "cat")
	if _, err := s.SaveWorks(ctx, "a", "like", false, works); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		q    LibraryQuery
		want string
	}{
		{"all", LibraryQuery{Kind: "like", Sort: "published"}, "2026-10:5@0 2026-3:4@5 2025-12:1@9 0-0:2@10"},
		{"one account", LibraryQuery{AccountID: "a", Kind: "like", Sort: "published"}, "2026-10:5@0 2026-3:4@5 2025-12:1@9 0-0:2@10"},
		{"videos", LibraryQuery{Kind: "like", Type: "video", Sort: "published"}, "2026-10:3@0 2026-3:4@3 0-0:2@7"},
		{"search", LibraryQuery{Kind: "like", Query: "cat", Sort: "published"}, "2026-10:3@0 2025-12:1@3 0-0:2@4"},
		{"recent sort", LibraryQuery{Kind: "like", Sort: "recent"}, ""},
		{"music", LibraryQuery{Kind: "music", Sort: "published"}, ""},
		{"other list", LibraryQuery{Kind: "favorite", Sort: "published"}, ""},
	} {
		months, err := s.Timeline(ctx, c.q)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		got, sum := []string{}, 0
		for _, m := range months {
			got = append(got, fmt.Sprintf("%d-%d:%d@%d", m.Year, m.Month, m.Count, m.Start))
			sum += m.Count
		}
		if strings.Join(got, " ") != c.want {
			t.Fatalf("%s: %s", c.name, strings.Join(got, " "))
		}
		if c.q.Sort != "published" || c.q.Kind == "music" {
			continue
		}
		// One account's lists read through its relations give the same items in the same
		// order as the general query over every account.
		for _, sort := range []string{"published", "recent"} {
			one, all := c.q, c.q
			one.AccountID, all.AccountID, one.Sort, all.Sort, one.Limit, all.Limit, one.Offset, all.Offset = "a", "", sort, sort, 4, 4, 1, 1
			a, ta, err1 := s.Library(ctx, one)
			b, tb, err2 := s.Library(ctx, all)
			if err1 != nil || err2 != nil || ta != tb || fmt.Sprint(a) != fmt.Sprint(b) {
				t.Fatalf("%s %s: account %d %v, all %d %v (%v %v)", c.name, sort, ta, a, tb, b, err1, err2)
			}
		}
		// The months add up to the list, and each starts where the list reaches it.
		q := c.q
		q.Limit = 100
		items, total, err := s.Library(ctx, q)
		if err != nil || sum != total {
			t.Fatalf("%s: sum %d total %d %v", c.name, sum, total, err)
		}
		for _, m := range months {
			at := items[m.Start].PublishedAt
			year, month := 0, 0
			if at > 0 {
				d := time.Unix(at, 0)
				year, month = d.Year(), int(d.Month())
			}
			if year != m.Year || month != m.Month {
				t.Fatalf("%s: month %d-%d starts at an item of %d-%d", c.name, m.Year, m.Month, year, month)
			}
		}
	}
}
