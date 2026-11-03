package app

import (
	"context"
	"errors"
)

// A background response started before an action must not undo that action.
func (a *App) bilibiliPersistence() func(context.Context, func() error) error {
	revision := a.actionRevision.Load()
	return func(ctx context.Context, save func() error) error {
		a.mu.Lock()
		defer a.mu.Unlock()
		if err := ctx.Err(); err != nil {
			return err
		}
		if a.actionRevision.Load() != revision {
			return errors.New("ACTIONS_CHANGED")
		}
		return save()
	}
}
