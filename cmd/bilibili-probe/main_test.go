package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSummaryExcludesPrivateFields(t *testing.T) {
	raw := `{"state":"complete","cookie":"secret","results":[{"kind":"items","count":2,"mid":"private","url":"https://example.invalid/private","nickname":"Private"}]}`
	var result probeResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"secret", "private", "nickname", "cookie"} {
		if strings.Contains(string(out), private) {
			t.Fatalf("leaked field %s", private)
		}
	}
	if result.Results[0].Count != 2 {
		t.Fatal("missing summary")
	}
}
