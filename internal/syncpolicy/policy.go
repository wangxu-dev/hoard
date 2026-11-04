package syncpolicy

import "time"

const MaxGap = 5 * time.Second

// Gap recovers a saved slowdown without allowing it to exceed the policy bounds.
func Gap(savedMS int, base time.Duration) time.Duration {
	return min(max(time.Duration(savedMS)*time.Millisecond, base), MaxGap)
}

func Slow(gap time.Duration) time.Duration { return min(max(gap*2, time.Second), MaxGap) }

// Recover gradually returns to the normal cadence after successful pages.
func Recover(gap, base time.Duration) time.Duration {
	return max(base, gap-10*time.Millisecond)
}

func Cooldown(failures int) time.Duration {
	return min(time.Minute<<min(max(failures-1, 0), 4), 10*time.Minute)
}
