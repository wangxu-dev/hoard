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

func TestDouyinNavigationAllowed(t *testing.T) {
	for _, tc := range []struct {
		url  string
		want bool
	}{
		{"about:blank", true}, {"https://www.douyin.com/user/self", true}, {"https://sso.douyin.com/", true},
		{"https://douyin.com.evil.example/", false}, {"http://www.douyin.com/", false}, {"https://user@www.douyin.com/", false}, {"https://www.douyin.com:8443/", false},
	} {
		if got := douyinNavigationAllowed(tc.url); got != tc.want {
			t.Errorf("navigation result %t want %t", got, tc.want)
		}
	}
}

func TestLoginPageClass(t *testing.T) {
	for _, tc := range []struct{ url, want string }{
		{"about:blank", "blank"},
		{"https://www.douyin.com/user/self?private=value", "self"},
		{"https://www.douyin.com/", "home"},
		{"https://www.douyin.com/user/fictional-private-id", "douyin_other"},
		{"https://douyin.com.evil.example/user/self", "external"},
	} {
		if got := loginPageClass(tc.url); got != tc.want {
			t.Errorf("got %s want %s", got, tc.want)
		}
	}
}
