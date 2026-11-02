package relay

import "testing"

func TestBilibiliMedia(t *testing.T) {
	for _, tc := range []struct {
		url     string
		allowed bool
	}{
		{"https://media.bilivideo.com/video.mp4", true},
		{"https://media.bilivideo.cn/video.mp4", true},
		{"https://upos-hz-mirrorakam.akamaized.net/video.mp4", true},
		{"https://evil.akamaized.net/video.mp4", false},
		{"https://bilivideo.com.example.test/video.mp4", false},
		{"http://media.bilivideo.com/video.mp4", false},
	} {
		if allowedMedia(tc.url) != tc.allowed {
			t.Fatal("incorrect host boundary")
		}
		if tc.allowed && mediaReferer(tc.url) != "https://www.bilibili.com/" {
			t.Fatal("incorrect referer")
		}
	}
}
