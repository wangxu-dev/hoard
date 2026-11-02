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
	page, err := a.page()
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	value, err := page.EvalContext(ctx, saveLoginScript)
	return err == nil && value == true
}
