package bilibili

import (
	"errors"
	"testing"
)

func TestPlayback(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"mp4", `{"code":0,"data":{"quality":16,"format":"mp4","accept_quality":[16],"durl":[{"url":"https://media.bilivideo.com/video.mp4","backup_url":["https://backup.bilivideo.com/video.mp4"]}]}}`, true},
		{"segments", `{"code":0,"data":{"format":"mp4","durl":[{"url":"a"},{"url":"b"}]}}`, false},
		{"dash", `{"code":0,"data":{"dash":{"video":[]}}}`, false},
		{"flv", `{"code":0,"data":{"format":"flv","durl":[{"url":"a"}]}}`, false},
		{"auth", `{"code":-101}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := ParsePlayback([]byte(tc.body))
			if (err == nil) != tc.valid {
				t.Fatal(err)
			}
			if tc.valid && (p.Quality != 16 || len(p.URLs) != 2) {
				t.Fatal("lost stream candidates")
			}
		})
	}
	cid, err := ParseCID([]byte(`{"code":0,"data":{"cid":123456}}`))
	if err != nil || cid != "123456" {
		t.Fatal("invalid cid")
	}
}

func TestPaidPlaybackAndParts(t *testing.T) {
	if _, err := ParsePlayback([]byte(`{"code":87008,"message":"87008"}`)); !errors.Is(err, ErrPaid) {
		t.Fatalf("locked part: %v", err)
	}
	for _, tc := range []struct {
		name, body string
		paid       bool
	}{
		{"normal", `{"code":0,"data":{"rights":{"pay":0,"ugc_pay":0}}}`, false},
		{"upower", `{"code":0,"data":{"is_upower_exclusive":true}}`, true},
		{"ugc", `{"code":0,"data":{"rights":{"ugc_pay":1}}}`, true},
		{"vip", `{"code":0,"data":{"rights":{"pay":1}}}`, true},
		{"auth", `{"code":-101}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if PaidVideo([]byte(tc.body)) != tc.paid {
				t.Fatal("payment flag")
			}
		})
	}
	for _, flag := range []string{"1", "true"} {
		p, err := ParsePlayback([]byte(`{"code":0,"data":{"quality":16,"format":"mp4","is_preview":` + flag + `,"timelength":30000,"durl":[{"url":"https://test.bilivideo.com/preview.mp4"}]}}`))
		if err != nil || p.PreviewMS != 30000 || len(p.URLs) != 1 {
			t.Fatalf("preview: %+v %v", p, err)
		}
	}
	parts, err := ParseParts([]byte(`{"code":0,"data":{"pages":[{"cid":123,"part":"One","duration":60},{"cid":124,"part":"Two","duration":120}]}}`))
	if err != nil || len(parts) != 2 || parts[1].ID != "124" || parts[1].DurationMS != 120000 {
		t.Fatalf("parts: %+v %v", parts, err)
	}
	if _, err := ParseParts([]byte(`{"code":0,"data":{"pages":[{"cid":"invalid"}]}}`)); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}
