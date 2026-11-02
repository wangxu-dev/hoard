package douyin

import "testing"

func TestClassifyResponseByOperation(t *testing.T) {
	for _, tt := range []struct {
		name     string
		op       string
		http     int
		code     int64
		msg      string
		body     string
		envelope bool
		want     ResponseClass
	}{
		{"self risk code is not risk", "self", 200, 8, "", `{"status_code":8}`, false, ResponseNotReady},
		{"self success", "self", 200, 0, "", `{"status_code":0}`, false, ResponseOK},
		{"self missing identity", "self", 200, 0, "", `{"status_code":0}`, true, ResponseNotReady},
		{"auth", "list", 401, 0, "", "", false, ResponseAuthRequired},
		{"rate", "list", 429, 0, "", "", false, ResponseRateLimited},
		{"html", "list", 200, 0, "", "<html>", false, ResponseVerificationRequired},
		{"captcha", "list", 200, 0, "captcha", "", false, ResponseVerificationRequired},
		{"risk", "list", 200, 2154, "", "", false, ResponseRiskControl},
		{"envelope", "list", 200, 0, "", "", true, ResponseRiskControl},
		{"rejected", "list", 200, 42, "bad", "", false, ResponseAPIRejected},
		{"ok", "list", 200, 0, "", "{}", false, ResponseOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyResponse(tt.op, tt.http, tt.code, tt.msg, tt.body, tt.envelope); got != tt.want {
				t.Fatalf("got %s want %s", got, tt.want)
			}
		})
	}
}
