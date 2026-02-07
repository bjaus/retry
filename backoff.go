package retry

import (
	"math"
	"math/rand/v2"
	"time"
)

// Backoff calculates the delay between retry attempts.
type Backoff interface {
	Delay(attempt int) time.Duration
}

// BackoffFunc is an adapter that allows a function to be used as a Backoff.
type BackoffFunc func(attempt int) time.Duration

// Delay implements Backoff.
func (f BackoffFunc) Delay(attempt int) time.Duration {
	return f(attempt)
}

// Constant returns a backoff that always waits the same duration.
func Constant(d time.Duration) Backoff {
	return BackoffFunc(func(attempt int) time.Duration {
		return d
	})
}

// Linear returns a backoff that increases linearly with each attempt.
// delay = base * attempt
func Linear(base time.Duration) Backoff {
	return BackoffFunc(func(attempt int) time.Duration {
		return base * time.Duration(attempt)
	})
}

// Exponential returns a backoff that doubles with each attempt.
// delay = base * 2^(attempt-1)
func Exponential(base time.Duration) Backoff {
	return BackoffFunc(func(attempt int) time.Duration {
		if attempt <= 0 {
			return base
		}
		// Prevent overflow
		if attempt > 62 {
			return time.Duration(math.MaxInt64)
		}
		return base * time.Duration(1<<uint(attempt-1))
	})
}

// WithCap wraps a backoff and caps the delay at a maximum value.
func WithCap(max time.Duration, b Backoff) Backoff {
	return BackoffFunc(func(attempt int) time.Duration {
		d := b.Delay(attempt)
		if d > max {
			return max
		}
		return d
	})
}

// WithMin wraps a backoff and ensures the delay is at least a minimum value.
func WithMin(min time.Duration, b Backoff) Backoff {
	return BackoffFunc(func(attempt int) time.Duration {
		d := b.Delay(attempt)
		if d < min {
			return min
		}
		return d
	})
}

// WithJitter wraps a backoff and adds random jitter to the delay.
// The jitter is a factor between 0 and 1, where 0.2 means ±20%.
func WithJitter(factor float64, b Backoff) Backoff {
	return BackoffFunc(func(attempt int) time.Duration {
		d := b.Delay(attempt)
		if factor <= 0 {
			return d
		}
		// Calculate jitter range: delay * factor
		jitterRange := float64(d) * factor
		// Random value between -jitterRange and +jitterRange
		jitter := (rand.Float64()*2 - 1) * jitterRange
		result := time.Duration(float64(d) + jitter)
		if result < 0 {
			return 0
		}
		return result
	})
}
