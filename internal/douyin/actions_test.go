package douyin

import "testing"

func TestActionStateRequiresExplicitValues(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		ok         bool
	}{
		{"explicit", `{"aweme_id":"123","aweme_type":0,"user_digged":1,"collect_stat":"0"}`, true},
		{"missing like", `{"aweme_id":"123","aweme_type":0,"collect_stat":0}`, false},
		{"aggregate count", `{"aweme_id":"123","aweme_type":0,"user_digged":0,"collect_stat":{"collect_count":10}}`, false},
		{"different work", `{"aweme_id":"456","aweme_type":0,"user_digged":1,"collect_stat":0}`, false},
		{"missing type", `{"aweme_id":"123","user_digged":1,"collect_stat":0}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseActionState(PageReply{Status: 200, Body: `{"status_code":0,"aweme_detail":` + tc.body + `}`}, "123")
			if (err == nil) != tc.ok {
				t.Fatalf("error %v", err)
			}
		})
	}
	for _, id := range []string{"", "123&action=1", "-1", "1.2"} {
		if ValidWorkID(id) {
			t.Fatalf("accepted %q", id)
		}
	}
}
