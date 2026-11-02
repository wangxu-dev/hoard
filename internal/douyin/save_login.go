package douyin

import (
	"context"
	_ "embed"
	"time"
)

//go:embed save-login.js
var saveLoginScript string

// SaveLogin clicks only the save-login prompt and waits for its dismissal.
func (a *Engine) SaveLogin(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	for attempt := 0; attempt < 2; attempt++ {
		page, err := a.page()
		if err == nil {
			value, err := page.EvalContext(ctx, saveLoginScript)
			if err == nil && value == true {
				return true
			}
		}
		if !sleepContext(ctx, 500*time.Millisecond) {
			return false
		}
	}
	return false
}
