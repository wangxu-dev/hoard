package app

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/wangxu-dev/hoard/internal/bilibili"
	"github.com/wangxu-dev/hoard/internal/store"
	"github.com/wangxu-dev/hoard/internal/syncpolicy"
)

const bilibiliSyncGap = 800 * time.Millisecond

func bilibiliSyncFailure(err error) (string, string) {
	switch {
	case errors.Is(err, bilibili.ErrAuth):
		return "signed_out", "AUTH_REQUIRED"
	case errors.Is(err, bilibili.ErrVerification):
		return "blocked", "VERIFICATION_REQUIRED"
	case errors.Is(err, bilibili.ErrRiskControl):
		return "blocked", "RISK_CONTROL"
	case errors.Is(err, bilibili.ErrRateLimited):
		return "cooling", "RATE_LIMITED"
	default:
		return "paused", "REQUEST_FAILED"
	}
}

// Bilibili's adapter owns envelope classification; the lane only schedules calls.
func (s *Sync) readBilibiliSync(ctx context.Context, accountID string, r *store.SyncRecord, read func() ([]byte, error)) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		if !s.resumeCooling(ctx, accountID, r) {
			return nil, ctx.Err()
		}
		if err := s.lockBackground(ctx, syncpolicy.Gap(r.DelayMS, bilibiliSyncGap)); err != nil {
			return nil, err
		}
		started := time.Now()
		body, err := read()
		s.app.lastRequest = time.Now()
		s.app.requestMu.Unlock()
		if err == nil {
			err = bilibili.CheckResponse(body)
		}
		log.Printf("[hoard:sync] source=bilibili kind=%s duration_ms=%d ok=%t", r.Kind, time.Since(started).Milliseconds(), err == nil)
		if err == nil {
			recoverPace(r, bilibiliSyncGap)
			return body, nil
		}
		if errors.Is(err, bilibili.ErrRateLimited) || errors.Is(err, bilibili.ErrRiskControl) {
			r.ErrorCode = "RATE_LIMITED"
			if errors.Is(err, bilibili.ErrRiskControl) {
				r.ErrorCode = "RISK_CONTROL"
			}
			s.slow(accountID, r, bilibiliSyncGap)
			if errors.Is(err, bilibili.ErrRiskControl) || attempt >= 2 {
				return nil, err
			}
			continue
		}
		// Only transport failures are replayed. Business rejections require a fresh run.
		if attempt >= 1 || errors.Is(err, bilibili.ErrAuth) || errors.Is(err, bilibili.ErrVerification) || errors.Is(err, bilibili.ErrInvalid) || errors.Is(err, bilibili.ErrUnavailable) {
			return nil, err
		}
		if !wait(ctx, 3*time.Second) {
			return nil, ctx.Err()
		}
	}
}
