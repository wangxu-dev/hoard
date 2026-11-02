package bilibili

import "testing"

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
