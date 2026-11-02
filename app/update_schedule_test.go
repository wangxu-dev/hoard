package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestUpdateRetryUntilSuccess(t *testing.T) {
	calls := 0
	healthy := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if !healthy {
			http.Error(w, "offline", 503)
			return
		}
		fmt.Fprint(w, `{"tag_name":"v0.2.0"}`)
	}))
	defer server.Close()
	u := testUpdates(t, server.URL)
	now := time.Unix(1800000000, 0)
	u.now = func() time.Time { return now }
	for attempt := 1; attempt <= 3; attempt++ {
		info, _ := u.Check("0.1.0", false)
		if calls != attempt || info.Error == "" || info.NextCheckAt != now.Add(5*time.Minute).UnixMilli() {
			t.Fatalf("attempt %d: calls=%d %+v", attempt, calls, info)
		}
		now = now.Add(5*time.Minute - time.Millisecond)
		cached, _ := u.Check("0.1.0", false)
		if calls != attempt || cached.NextCheckAt != info.NextCheckAt {
			t.Fatal("failure retried before its deadline")
		}
		now = now.Add(time.Millisecond)
	}
	healthy = true
	recovered, _ := u.Check("0.1.0", false)
	if calls != 4 || recovered.Error != "" || recovered.NextCheckAt != now.Add(time.Hour).UnixMilli() {
		t.Fatalf("recovery: calls=%d %+v", calls, recovered)
	}
	now = now.Add(5 * time.Minute)
	_, _ = u.Check("0.1.0", false)
	if calls != 4 {
		t.Fatal("successful retry did not resume hourly cache")
	}
}
