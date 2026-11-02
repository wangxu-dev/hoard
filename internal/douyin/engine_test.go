package douyin

import "testing"

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
