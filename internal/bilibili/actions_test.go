package bilibili

import "testing"

func TestActionParsers(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"selected", `{"code":0,"data":{"count":1,"list":[{"id":1,"title":"Example","media_count":2,"fav_state":1}]}}`, true},
		{"missing state", `{"code":0,"data":{"count":1,"list":[{"id":1,"media_count":2}]}}`, false},
		{"invalid state", `{"code":0,"data":{"count":1,"list":[{"id":1,"media_count":2,"fav_state":2}]}}`, false},
		{"auth", `{"code":-101}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			folders, err := ParseActionFolders([]byte(tc.body))
			if (err == nil) != tc.valid {
				t.Fatalf("folders=%v err=%v", folders, err)
			}
			if tc.valid && !folders[0].Selected {
				t.Fatal("selected state lost")
			}
		})
	}
	for _, tc := range []struct {
		id    string
		valid bool
	}{{"BV0000000001", true}, {"opus:123", true}, {"opus:0", false}, {"123", false}, {"https://example.test", false}} {
		if ValidWorkID(tc.id) != tc.valid {
			t.Fatalf("id %s", tc.id)
		}
	}
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{`{"code":0,"data":{"item":{"id_str":"123","modules":[{"module_stat":{"like":{"status":true},"favorite":{"status":false}}}]}}}`, true},
		{`{"code":0,"data":{"item":{"id_str":"123","modules":[{"module_stat":{"like":{"count":100},"favorite":{"count":2}}}]}}}`, false},
	} {
		liked, fav, err := ParseOpusState([]byte(tc.body), "123")
		if (err == nil) != tc.valid || tc.valid && (!liked || fav) {
			t.Fatalf("opus=%t,%t err=%v", liked, fav, err)
		}
	}
	if _, err := ParseAID([]byte(`{"code":0,"data":{"aid":42,"bvid":"BV0000000001"}}`), "BV0000000002"); err == nil {
		t.Fatal("accepted wrong target")
	}
}
