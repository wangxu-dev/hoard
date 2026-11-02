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
				r.LastFullAt = old.LastFullAt
			}
		}
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
				r.Status, r.ErrorCode = "paused", "REQUEST_FAILED"
				if errors.Is(err, bilibili.ErrAuth) {
					r.Status, r.ErrorCode = "signed_out", "AUTH_REQUIRED"
				}
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
	client := bilibili.Client{Incremental: incremental, Read: func(ctx context.Context, path string, params map[string]string) ([]byte, error) {
		lastPath = path
		for s.app.frontRequests.Load() > 0 {
			if !wait(ctx, 20*time.Millisecond) {
				return nil, ctx.Err()
			}
		}
		s.app.requestMu.Lock()
		defer s.app.requestMu.Unlock()
		if delay := time.Until(s.app.lastRequest.Add(pageGap)); delay > 0 && !wait(ctx, delay) {
			return nil, ctx.Err()
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		body, err := engine.ReadBilibili(ctx, path, params)
		s.app.lastRequest = time.Now()
		return body, err
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
			r.Status, r.ErrorCode = "paused", "REQUEST_FAILED"
			if errors.Is(err, bilibili.ErrAuth) {
				r.Status, r.ErrorCode = "signed_out", "AUTH_REQUIRED"
			}
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
