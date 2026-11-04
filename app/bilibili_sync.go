package app

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/wangxu-dev/hoard/internal/bilibili"
	"github.com/wangxu-dev/hoard/internal/store"
)

func (s *Sync) runBilibili(ctx context.Context, accountID string) {
	engine, ok := s.app.Engine.(interface {
		ReadBilibili(context.Context, string, map[string]string) ([]byte, error)
	})
	if !ok {
		return
	}
	previous, err := s.app.Store.SyncRecords(ctx, accountID)
	if err != nil {
		log.Print("[hoard:sync] Bilibili state read failed")
		return
	}
	incremental := false
	for _, r := range previous {
		if r.Kind == "favorite" && r.LastFullAt != nil {
			incremental = true
		}
	}
	kinds := []string{"watch_later", "favorite"}
	records := map[string]*store.SyncRecord{}
	for _, kind := range kinds {
		r := &store.SyncRecord{Kind: kind, Phase: "head", Status: "running"}
		for _, old := range previous {
			if old.Kind == kind {
				*r = old
			}
		}
		if !s.resumeCooling(ctx, accountID, r) {
			return
		}
		r.Phase, r.Status = "head", "running"
		records[kind] = r
		s.save(accountID, r)
	}
	defer func() {
		for _, kind := range kinds {
			r := records[kind]
			if r.Status != "checking" {
				continue
			}
			r.Status, r.ErrorCode = "idle", ""
			if err != nil {
				r.Status, r.ErrorCode = bilibiliSyncFailure(err)
			}
			if ctx.Err() != nil {
				r.Status, r.ErrorCode = "paused", "CANCELED"
			}
			s.save(accountID, r)
		}
	}()
	if !s.waitReady(ctx) {
		for _, r := range records {
			r.Status, r.ErrorCode = "paused", "NOT_READY"
			s.save(accountID, r)
		}
		return
	}
	lastPath := ""
	client := bilibili.Client{Persist: s.app.bilibiliPersistence(), Incremental: incremental, Read: func(ctx context.Context, path string, params map[string]string) ([]byte, error) {
		lastPath = path
		kind := "favorite"
		if path == "/x/v2/history/toview" {
			kind = "watch_later"
		}
		return s.readBilibiliSync(ctx, accountID, records[kind], func() ([]byte, error) { return engine.ReadBilibili(ctx, path, params) })
	}, Changed: func(kind string) {
		if ctx.Err() != nil {
			return
		}
		r := records[kind]
		r.Pages++
		s.save(accountID, r)
		if err := libraryChanged.Broadcast(LibraryChange{AccountID: accountID, Kind: kind}); err != nil {
			log.Print("[hoard:sync] library event delivery failed")
		}
	}, Completed: func(kind string) {
		r := records[kind]
		now := time.Now().Unix()
		r.Phase, r.Status, r.LastFullAt = "done", "checking", &now
		s.save(accountID, r)
	}}
	videos, err := client.Index(ctx, s.app.Store, accountID)
	for _, kind := range kinds {
		r := records[kind]
		if r.Phase == "done" {
			continue
		}
		if err != nil {
			r.Status, r.ErrorCode = bilibiliSyncFailure(err)
			if ctx.Err() != nil {
				r.Status, r.ErrorCode = "paused", "CANCELED"
			}
		} else {
			now := time.Now().Unix()
			r.Phase, r.Status, r.LastFullAt = "done", "idle", &now
		}
		s.save(accountID, r)
	}
	if err != nil {
		log.Printf("[hoard:sync] Bilibili indexing stopped path=%s invalid=%t auth=%t canceled=%t", lastPath, errors.Is(err, bilibili.ErrInvalid), errors.Is(err, bilibili.ErrAuth), ctx.Err() != nil)
		return
	}
	log.Printf("[hoard:sync] Bilibili indexing complete videos=%d", len(videos))
}
