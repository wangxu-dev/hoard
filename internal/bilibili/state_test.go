package bilibili

import "testing"

func TestPersonalState(t *testing.T) {
	for _, body := range []string{`{"code":0,"data":null}`, `{"code":0,"data":{}}`, `{"code":-101}`} {
		if value, err := ParseLiked([]byte(body)); err == nil || value != nil {
			t.Fatal("unknown like became known")
		}
		if value, err := ParseFavorited([]byte(body)); err == nil || value != nil {
			t.Fatal("unknown favorite became known")
		}
		if value, err := ParseCoins([]byte(body)); err == nil || value != nil {
			t.Fatal("unknown coins became known")
		}
	}
	liked, err := ParseLiked([]byte(`{"code":0,"data":0}`))
	if err != nil || liked == nil || *liked {
		t.Fatal("explicit false")
	}
	fav, err := ParseFavorited([]byte(`{"code":0,"data":{"favoured":true,"count":500}}`))
	if err != nil || fav == nil || !*fav {
		t.Fatal("personal favorite")
	}
	coins, err := ParseCoins([]byte(`{"code":0,"data":{"multiply":2}}`))
	if err != nil || coins == nil || *coins != 2 {
		t.Fatal("personal coins")
	}
}
func TestWatchLater(t *testing.T) {
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{`{"code":0,"data":{"count":0,"list":null}}`, true},
		{`{"code":0,"data":{"count":2,"list":[]}}`, false},
		{`{"code":0,"data":{"list":[]}}`, false},
		{`{"code":0,"data":{"count":1,"list":[{"bvid":"BV1abcdefghj","title":"Example","owner":{"mid":42},"duration":3}]}}`, true},
	} {
		_, err := ParseWatchLater([]byte(tc.body))
		if (err == nil) != tc.valid {
			t.Fatalf("unexpected %v", err)
		}
	}
}
