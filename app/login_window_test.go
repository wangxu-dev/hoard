package app

import (
	"context"
	"testing"
)

type loginSaveEngine struct {
	*fakeAccountEngine
	saved bool
}

func (e *loginSaveEngine) SaveLogin(context.Context) bool { return e.saved }

func TestLoginWindowHidesOnlyAfterSaveConfirmation(t *testing.T) {
	for _, saved := range []bool{false, true} {
		for _, current := range []bool{false, true} {
			a, _, engine := newAccountTestApp(t)
			ctx, cancel := context.WithCancel(context.Background())
			a.sessionCtx = ctx
			if !current {
				cancel()
			}
			engine.shown = true
			a.Engine = &loginSaveEngine{fakeAccountEngine: engine, saved: saved}
			a.saveLogin(ctx)
			if engine.shown != (!current || !saved) {
				t.Fatalf("saved=%t current=%t shown=%t", saved, current, engine.shown)
			}
			cancel()
		}
	}
}
