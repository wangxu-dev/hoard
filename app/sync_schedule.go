package app

import (
	"context"
	"time"

	"github.com/wangxu-dev/hoard/internal/store"
	"github.com/wangxu-dev/hoard/internal/syncpolicy"
)

// lockBackground yields while waiting so interactive requests can take the lane.
// The caller releases requestMu after its request and lastRequest update.
func (s *Sync) lockBackground(ctx context.Context, gap time.Duration) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.app.requestMu.Lock()
		delay := time.Until(s.app.lastRequest.Add(gap))
		if s.app.frontRequests.Load() == 0 && delay <= 0 {
			return nil
		}
		s.app.requestMu.Unlock()
		if !wait(ctx, min(max(delay, 20*time.Millisecond), 100*time.Millisecond)) {
			return ctx.Err()
		}
	}
}

func (s *Sync) resumeCooling(ctx context.Context, accountID string, r *store.SyncRecord) bool {
	if r.RetryAt != nil && *r.RetryAt > time.Now().Unix() {
		r.Status = "cooling"
		s.save(accountID, r)
		if !wait(ctx, time.Until(time.Unix(*r.RetryAt, 0))) {
			return false
		}
	}
	r.RetryAt = nil
	if r.Status == "cooling" {
		r.Status = "running"
	}
	return ctx.Err() == nil
}

func (s *Sync) slow(accountID string, r *store.SyncRecord, base time.Duration) {
	r.Failures++
	r.DelayMS = int(syncpolicy.Slow(syncpolicy.Gap(r.DelayMS, base)).Milliseconds())
	retry := time.Now().Add(syncpolicy.Cooldown(r.Failures)).Unix()
	r.RetryAt = &retry
	r.Status = "cooling"
	s.save(accountID, r)
}

func recoverPace(r *store.SyncRecord, base time.Duration) {
	gap := syncpolicy.Recover(syncpolicy.Gap(r.DelayMS, base), base)
	r.DelayMS = int(gap.Milliseconds())
	if gap == base {
		r.Failures = 0
	}
}
