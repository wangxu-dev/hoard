package explore

import (
	"context"
	"testing"
	"time"

	"github.com/wangxu-dev/hoard/internal/store"
)

func TestPublicationSnapshotAndCache(t *testing.T) {
	at := time.Date(2024, 3, 1, 0, 30, 0, 0, time.Local).Unix()
	rows := []store.ExploreRecord{{Source: "douyin", ID: "dated", Title: "摄影", PublishedAt: at}, {Source: "douyin", ID: "unknown", PublishedAt: 0}}
	s, err := Analyze(context.Background(), Scope{}, rows)
	if err != nil {
		t.Fatal(err)
	}
	sc := s.Scopes[0]
	if !sc.TimeCapabilities.Views["source_time"] || sc.TimeCapabilities.Views["observed_time"] {
		t.Fatal("incorrect time capabilities")
	}
	for _, w := range sc.Works {
		if w.ID == "dated" && (w.PublishedAt != at || w.MonthKeys.SourceTime == nil || *w.MonthKeys.SourceTime != "2024-03") {
			t.Fatal("publication month lost")
		}
		if w.ID == "unknown" && (w.PublishedAt != 0 || w.MonthKeys.SourceTime != nil) {
			t.Fatal("invented missing date")
		}
	}
	cache := Cache{Dir: t.TempDir()}
	if err := cache.Save(s); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Load(s.Scope, s.InputRevision, RuleHash); err != nil {
		t.Fatal(err)
	}
	old := s.InputRevision
	rows[0].PublishedAt++
	if old == Revision(Scope{}, rows) {
		t.Fatal("time update did not invalidate input")
	}
	for i := range s.Scopes[0].Works {
		if s.Scopes[0].Works[i].ID == "dated" {
			wrong := "2020-01"
			s.Scopes[0].Works[i].MonthKeys.SourceTime = &wrong
		}
	}
	if err := cache.Save(s); err == nil {
		t.Fatal("mismatched month accepted")
	}
}
