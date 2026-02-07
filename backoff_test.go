package retry_test

import (
	"testing"
	"time"

	"github.com/bjaus/retry"
)

func TestConstant(t *testing.T) {
	b := retry.Constant(100 * time.Millisecond)

	for attempt := 1; attempt <= 5; attempt++ {
		d := b.Delay(attempt)
		if d != 100*time.Millisecond {
			t.Errorf("attempt %d: expected 100ms, got %v", attempt, d)
		}
	}
}

func TestLinear(t *testing.T) {
	b := retry.Linear(100 * time.Millisecond)

	cases := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 100 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 300 * time.Millisecond},
		{5, 500 * time.Millisecond},
	}

	for _, tc := range cases {
		d := b.Delay(tc.attempt)
		if d != tc.expected {
			t.Errorf("attempt %d: expected %v, got %v", tc.attempt, tc.expected, d)
		}
	}
}

func TestExponential(t *testing.T) {
	b := retry.Exponential(100 * time.Millisecond)

	cases := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 100 * time.Millisecond},  // 100 * 2^0
		{2, 200 * time.Millisecond},  // 100 * 2^1
		{3, 400 * time.Millisecond},  // 100 * 2^2
		{4, 800 * time.Millisecond},  // 100 * 2^3
		{5, 1600 * time.Millisecond}, // 100 * 2^4
	}

	for _, tc := range cases {
		d := b.Delay(tc.attempt)
		if d != tc.expected {
			t.Errorf("attempt %d: expected %v, got %v", tc.attempt, tc.expected, d)
		}
	}
}

func TestExponential_overflow(t *testing.T) {
	b := retry.Exponential(100 * time.Millisecond)

	// Very high attempt should not overflow or panic
	d := b.Delay(100)
	if d <= 0 {
		t.Error("expected positive duration for high attempt count")
	}
}

func TestExponential_zeroAttempt(t *testing.T) {
	b := retry.Exponential(100 * time.Millisecond)

	// Zero and negative attempts should return base
	if d := b.Delay(0); d != 100*time.Millisecond {
		t.Errorf("expected 100ms for attempt 0, got %v", d)
	}
	if d := b.Delay(-1); d != 100*time.Millisecond {
		t.Errorf("expected 100ms for attempt -1, got %v", d)
	}
}

func TestWithCap(t *testing.T) {
	b := retry.WithCap(500*time.Millisecond, retry.Exponential(100*time.Millisecond))

	cases := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 100 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 400 * time.Millisecond},
		{4, 500 * time.Millisecond},  // capped
		{5, 500 * time.Millisecond},  // capped
		{10, 500 * time.Millisecond}, // capped
	}

	for _, tc := range cases {
		d := b.Delay(tc.attempt)
		if d != tc.expected {
			t.Errorf("attempt %d: expected %v, got %v", tc.attempt, tc.expected, d)
		}
	}
}

func TestWithMin(t *testing.T) {
	// Linear with minimum
	b := retry.WithMin(150*time.Millisecond, retry.Linear(50*time.Millisecond))

	cases := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 150 * time.Millisecond}, // 50ms < 150ms min
		{2, 150 * time.Millisecond}, // 100ms < 150ms min
		{3, 150 * time.Millisecond}, // 150ms = min
		{4, 200 * time.Millisecond}, // 200ms > min
	}

	for _, tc := range cases {
		d := b.Delay(tc.attempt)
		if d != tc.expected {
			t.Errorf("attempt %d: expected %v, got %v", tc.attempt, tc.expected, d)
		}
	}
}

func TestWithJitter(t *testing.T) {
	b := retry.WithJitter(0.2, retry.Constant(100*time.Millisecond))

	// Run multiple times and check that values are within expected range
	for range 100 {
		d := b.Delay(1)
		// With 20% jitter on 100ms: expected range is 80ms to 120ms
		if d < 80*time.Millisecond || d > 120*time.Millisecond {
			t.Errorf("delay %v outside expected range [80ms, 120ms]", d)
		}
	}
}

func TestWithJitter_zeroFactor(t *testing.T) {
	b := retry.WithJitter(0, retry.Constant(100*time.Millisecond))

	for range 10 {
		d := b.Delay(1)
		if d != 100*time.Millisecond {
			t.Errorf("expected 100ms with zero jitter, got %v", d)
		}
	}
}

func TestWithJitter_negativeFactor(t *testing.T) {
	b := retry.WithJitter(-0.5, retry.Constant(100*time.Millisecond))

	for range 10 {
		d := b.Delay(1)
		if d != 100*time.Millisecond {
			t.Errorf("expected 100ms with negative jitter factor, got %v", d)
		}
	}
}

func TestWithJitter_verySmallDelay(t *testing.T) {
	// Test that jitter doesn't produce negative durations
	b := retry.WithJitter(0.9, retry.Constant(1*time.Millisecond))

	for range 100 {
		d := b.Delay(1)
		if d < 0 {
			t.Errorf("jitter produced negative duration: %v", d)
		}
	}
}

func TestWithJitter_largeFactor(t *testing.T) {
	// With factor > 1.0, jitter can exceed the delay itself
	// This tests the negative result guard (result < 0 returns 0)
	b := retry.WithJitter(2.0, retry.Constant(1*time.Nanosecond))

	// Run many times - with factor 2.0 on 1ns, jitter range is ±2ns
	// so some results should be clamped to 0
	var zeroCount int
	for range 1000 {
		d := b.Delay(1)
		if d < 0 {
			t.Errorf("jitter produced negative duration: %v", d)
		}
		if d == 0 {
			zeroCount++
		}
	}
	// We should see at least some zero values due to clamping
	// (but this is probabilistic, so we don't assert on it)
}

func TestComposedBackoff(t *testing.T) {
	// Exponential, capped at 1s, with 10% jitter
	b := retry.WithJitter(0.1,
		retry.WithCap(1*time.Second,
			retry.Exponential(100*time.Millisecond),
		),
	)

	// Attempt 10 would be 100ms * 2^9 = 51.2s, but capped at 1s
	for range 10 {
		d := b.Delay(10)
		// Expected: 1s ± 10% = 900ms to 1100ms
		if d < 900*time.Millisecond || d > 1100*time.Millisecond {
			t.Errorf("delay %v outside expected range [900ms, 1100ms]", d)
		}
	}
}

func TestBackoffFunc(t *testing.T) {
	// Custom backoff using BackoffFunc
	custom := retry.BackoffFunc(func(attempt int) time.Duration {
		return time.Duration(attempt*attempt) * 10 * time.Millisecond
	})

	cases := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 10 * time.Millisecond},
		{2, 40 * time.Millisecond},
		{3, 90 * time.Millisecond},
		{4, 160 * time.Millisecond},
	}

	for _, tc := range cases {
		d := custom.Delay(tc.attempt)
		if d != tc.expected {
			t.Errorf("attempt %d: expected %v, got %v", tc.attempt, tc.expected, d)
		}
	}
}
