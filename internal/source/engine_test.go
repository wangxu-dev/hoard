package source

import "testing"

func TestBilibiliIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, body            string
		logged, auth, invalid bool
	}{
		{"active", `{"code":0,"data":{"isLogin":true,"mid":"42","uname":"Example","face":""}}`, true, false, false},
		{"signed out", `{"code":-101}`, false, true, false},
		{"guest", `{"code":0,"data":{"isLogin":false}}`, false, true, false},
		{"missing identity", `{"code":0,"data":{"isLogin":true}}`, false, false, true},
		{"invalid identity", `{"code":0,"data":{"isLogin":true,"mid":"not-an-id"}}`, false, false, true},
		{"missing envelope", `{}`, false, false, true},
		{"server error", `{"code":-500}`, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseBilibiliIdentity([]byte(tc.body))
			if (err != nil) != tc.invalid || got.LoggedIn != tc.logged || got.AuthRequired != tc.auth {
				t.Fatalf("identity flags: %+v %v", got, err)
			}
		})
	}
}
func TestBilibiliNavigation(t *testing.T) {
	for _, tc := range []struct {
		url     string
		allowed bool
	}{
		{"https://www.bilibili.com/", true}, {"https://passport.bilibili.com/login", true},
		{"https://bilibili.com.example.test/", false}, {"https://evilbilibili.com/", false},
		{"http://www.bilibili.com/", false}, {"https://user@www.bilibili.com/", false},
	} {
		if bilibiliURL(tc.url) != tc.allowed {
			t.Fatal("incorrect navigation boundary")
		}
	}
}
