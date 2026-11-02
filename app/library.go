package app

import (
	"errors"
	"log"
	"time"

	"github.com/wangxu-dev/hoard/internal/covers"
	"github.com/wangxu-dev/hoard/internal/store"
)

// coverKind is the cover cache's kind of a library item: "m" for music, "w" for works.
func coverKind(kind string) string {
	if kind == "music" {
		return "m"
	}
	return "w"
}

// Library reads synced lists for the library page.
type Library struct{ app *App }

func NewLibrary(a *App) *Library { return &Library{app: a} }

// LibraryQuery selects one page of a list.
type LibraryQuery struct {
	AccountID string `json:"accountId"`
	Source    string `json:"source"`
	Kind      string `json:"kind"`
	Type      string `json:"type"`
	Query     string `json:"query"`
	Sort      string `json:"sort"`
	Offset    int    `json:"offset"`
	Limit     int    `json:"limit"`
}

// LibraryItem is one card of the library grid.
type LibraryItem struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Title        string `json:"title"`
	Author       string `json:"author"`
	Cover        string `json:"cover"`
	AuthorAvatar string `json:"authorAvatar"` // cached author avatar; empty for sounds
	PublishedAt  int64  `json:"publishedAt"`
	DurationMS   int    `json:"durationMs"`
	ImageCount   int    `json:"imageCount"`
}

// LibraryPage is one page of a list and the number of matching items.
type LibraryPage struct {
	Items []LibraryItem `json:"items"`
	Total int           `json:"total"`
}

// LibraryCounts is the size of each list in a scope.
type LibraryCounts struct {
	Like     int `json:"like"`
	Favorite int `json:"favorite"`
	Music    int `json:"music"`
}

// List returns one page of a list, filtered and sorted.
func (l *Library) List(q LibraryQuery) (LibraryPage, error) {
	if l.app.Store == nil {
		return LibraryPage{}, errors.New("STORE_FAILED")
	}
	ctx, cancel := accountContext()
	defer cancel()
	items, total, err := l.app.Store.Library(ctx, store.LibraryQuery{AccountID: q.AccountID, Source: q.Source, Kind: q.Kind, Type: q.Type, Query: q.Query, Sort: q.Sort, Offset: q.Offset, Limit: q.Limit})
	if err != nil {
		log.Printf("[hoard:library] list failed kind=%s", q.Kind)
		return LibraryPage{}, errors.New("STORE_FAILED")
	}
	page := LibraryPage{Items: make([]LibraryItem, len(items)), Total: total}
	for i, it := range items {
		cover := it.CoverURL
		if cover != "" && l.app.Covers != nil {
			cover = covers.URL(coverKind(it.Kind), it.RemoteID)
		}
		avatar := ""
		if it.Kind != "music" && l.app.Covers != nil {
			avatar = covers.URL("a", it.RemoteID)
		}
		page.Items[i] = LibraryItem{ID: it.RemoteID, Kind: it.Kind, Title: it.Title, Author: it.Author, Cover: cover, AuthorAvatar: avatar, PublishedAt: it.PublishedAt, DurationMS: it.DurationMS, ImageCount: it.ImageCount}
	}
	return page, nil
}

// TimelineMonth is one month of a list sorted by publish time; start is the offset
// of its first item. Year and month are 0 for items without a publish time.
type TimelineMonth struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Count int `json:"count"`
	Start int `json:"start"`
}

// Timeline returns the months of a list sorted by publish time, newest first;
// other sorts and sounds have none.
func (l *Library) Timeline(q LibraryQuery) ([]TimelineMonth, error) {
	if l.app.Store == nil {
		return nil, errors.New("STORE_FAILED")
	}
	ctx, cancel := accountContext()
	defer cancel()
	began := time.Now()
	months, err := l.app.Store.Timeline(ctx, store.LibraryQuery{AccountID: q.AccountID, Source: q.Source, Kind: q.Kind, Type: q.Type, Query: q.Query, Sort: q.Sort})
	if err != nil {
		log.Printf("[hoard:library] timeline failed kind=%s", q.Kind)
		return nil, errors.New("STORE_FAILED")
	}
	out := make([]TimelineMonth, len(months))
	total := 0
	for i, m := range months {
		out[i] = TimelineMonth{Year: m.Year, Month: m.Month, Count: m.Count, Start: m.Start}
		total += m.Count
	}
	log.Printf("[hoard:library] timeline kind=%s months=%d items=%d took=%s", q.Kind, len(out), total, time.Since(began).Round(100*time.Microsecond))
	return out, nil
}

// Counts returns the size of every list in a scope; empty ids mean all.
func (l *Library) Counts(accountID, source string) (LibraryCounts, error) {
	if l.app.Store == nil {
		return LibraryCounts{}, errors.New("STORE_FAILED")
	}
	ctx, cancel := accountContext()
	defer cancel()
	c, err := l.app.Store.LibraryCounts(ctx, accountID, source)
	if err != nil {
		return LibraryCounts{}, errors.New("STORE_FAILED")
	}
	return LibraryCounts{Like: c["like"], Favorite: c["favorite"], Music: c["music"]}, nil
}
