package syncpolicy

import (
	"testing"
	"time"
)

func TestPaceAndCooldown(t *testing.T) {
	for _, base := range []time.Duration{300 * time.Millisecond, 800 * time.Millisecond} {
		gap := Gap(0, base)
		for i := 0; i < 20; i++ {
			gap = Slow(gap)
		}
		if gap != MaxGap {
			t.Fatal(gap)
		}
		for i := 0; i < 600; i++ {
			gap = Recover(gap, base)
		}
		if gap != base {
			t.Fatal(gap)
		}
		if Gap(-1, base) != base || Gap(999999, base) != MaxGap {
			t.Fatal("saved gap bounds")
		}
	}
	for i, want := range []time.Duration{time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 10 * time.Minute, 10 * time.Minute} {
		if got := Cooldown(i + 1); got != want {
			t.Fatalf("failure %d: %s", i+1, got)
		}
	}
}
