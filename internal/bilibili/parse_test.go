package bilibili

import (
	"errors"
	"testing"
)

func TestParseFolders(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		count      int
		err        error
	}{
		{"empty", `{"code":0,"data":{"count":0,"list":null}}`, 0, nil},
		{"folder", `{"code":0,"data":{"count":1,"list":[{"id":1234567890123456789,"title":"Example","media_count":2}]}}`, 1, nil},
		{"auth", `{"code":-101}`, 0, ErrAuth},
		{"missing code", `{"data":{}}`, 0, ErrInvalid},
		{"truncated", `{"code":0,"data":{"count":2,"list":[]}}`, 0, ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseFolders([]byte(tc.body))
			if !errors.Is(err, tc.err) || len(got) != tc.count {
				t.Fatalf("count=%d err=%v", len(got), err)
			}
			if tc.name == "folder" && got[0].ID != "1234567890123456789" {
				t.Fatal("lost ID precision")
			}
		})
	}
}
func TestParsePage(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		count      int
		err        error
	}{
		{"empty", `{"code":0,"data":{"has_more":false,"medias":null}}`, 0, nil},
		{"missing end", `{"code":0,"data":{"medias":[]}}`, 0, ErrInvalid},
		{"empty more", `{"code":0,"data":{"has_more":true,"medias":[]}}`, 0, ErrInvalid},
		{"other resource", `{"code":0,"data":{"has_more":true,"medias":[{"type":12}]}}`, 0, nil},
		{"video", `{"code":0,"data":{"has_more":false,"medias":[{"type":2,"bvid":"BV1abcdefghj","title":"Example","cover":"https://example.invalid/cover","duration":3,"upper":{"mid":42,"name":"Author"},"attr":9}]}}`, 1, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParsePage([]byte(tc.body))
			if !errors.Is(err, tc.err) || len(got.Videos) != tc.count {
				t.Fatalf("%+v %v", got, err)
			}
			if tc.count > 0 && (got.Videos[0].Available || got.Videos[0].Cover != "" || got.Videos[0].DurationMS != 3000) {
				t.Fatal("invalid metadata mapping")
			}
		})
	}
}
