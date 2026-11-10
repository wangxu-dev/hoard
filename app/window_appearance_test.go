package app

import "testing"

func TestWindowAppearanceColorValidation(t *testing.T) {
	for _, c := range []string{"", "#fff", "white", "#12345z", "rgba(0,0,0,1)", "#12345600"} {
		if _, _, _, err := parseWindowColor(c); err == nil {
			t.Fatalf("accepted %q", c)
		}
	}
	r, g, b, err := parseWindowColor("#0b131d")
	if err != nil || r != 11 || g != 19 || b != 29 {
		t.Fatal("incorrect galaxy RGB")
	}
}
