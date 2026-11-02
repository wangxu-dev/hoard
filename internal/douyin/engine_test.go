package douyin

import (
	"context"
	"testing"
)

func TestEngineRequiresAccountSession(t *testing.T) {
	for _, tt := range []struct {
		name string
		call func(*Engine) error
	}{
		{"open", func(e *Engine) error { _, err := e.OpenDouyin(); return err }},
		{"ensure", func(e *Engine) error { _, err := e.EnsureSession(); return err }},
		{"identity", func(e *Engine) error { _, err := e.Identity(context.Background()); return err }},
		{"likes", func(e *Engine) error { _, err := e.FetchLikesContext(context.Background(), "0"); return err }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := &Engine{}
			if err := tt.call(e); err == nil {
				t.Fatal("operation accepted without an account session")
			}
			if e.douyin != nil || e.sess != nil {
				t.Fatal("operation created a shared session or window")
			}
		})
	}
}
