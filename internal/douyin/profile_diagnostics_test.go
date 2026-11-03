package douyin

import (
	"strings"
	"testing"
)

func TestProfileDiagnostics(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"empty", "", "bytes=0 json=false"},
		{"envelope", `{"status_code":0,"extra":{}}`, "code=0 envelope=true user=false user_id=false"},
		{"rejected", `{"status_code":8}`, "code=8 envelope=true"},
		{"profile", `{"status_code":0,"user":{"sec_uid":"private-test-id","nickname":"private-test-name"}}`, "user=true user_id=true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ProfileDiagnostics(PageReply{Status: 200, Body: tc.body})
			if !strings.Contains(got, tc.want) {
				t.Fatalf("summary = %s", got)
			}
			if strings.Contains(got, "private-test") || strings.Contains(got, "nickname") {
				t.Fatal("diagnostics exposed identity")
			}
		})
	}
}
