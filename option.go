package retry

import "time"

// config holds all retry configuration.
type config struct {
	// Policy-level options
	maxAttempts int
	maxDuration time.Duration
	backoff     Backoff
	clock       Clock

	// Call-level options
	condition   Condition
	onRetry     OnRetryFunc
	onSuccess   OnSuccessFunc
	onExhausted OnExhaustedFunc
	allErrors   bool
}

// Option configures retry behavior.
type Option func(*config)

// WithMaxAttempts sets the maximum number of attempts.
func WithMaxAttempts(n int) Option {
	return func(c *config) {
		c.maxAttempts = n
	}
}

// WithMaxDuration sets the maximum total duration for all attempts.
// Retries stop when this duration is exceeded, even if attempts remain.
func WithMaxDuration(d time.Duration) Option {
	return func(c *config) {
		c.maxDuration = d
	}
}

// WithBackoff sets the backoff strategy.
func WithBackoff(b Backoff) Option {
	return func(c *config) {
		c.backoff = b
	}
}

// WithClock sets the clock for time operations. Useful for testing.
func WithClock(clock Clock) Option {
	return func(c *config) {
		c.clock = clock
	}
}

// If sets the condition that determines whether an error should be retried.
// If the condition returns false, the retry loop stops immediately.
func If(cond Condition) Option {
	return func(c *config) {
		c.condition = cond
	}
}

// IfNot sets a condition where matching errors are NOT retried.
// This is equivalent to If(Not(cond)).
func IfNot(cond Condition) Option {
	return If(Not(cond))
}

// Not inverts a condition.
func Not(cond Condition) Condition {
	return func(err error) bool {
		return !cond(err)
	}
}

// OnRetry sets a hook that is called before each retry sleep.
func OnRetry(fn OnRetryFunc) Option {
	return func(c *config) {
		c.onRetry = fn
	}
}

// OnSuccess sets a hook that is called when the function succeeds.
func OnSuccess(fn OnSuccessFunc) Option {
	return func(c *config) {
		c.onSuccess = fn
	}
}

// OnExhausted sets a hook that is called when all retry attempts are exhausted.
func OnExhausted(fn OnExhaustedFunc) Option {
	return func(c *config) {
		c.onExhausted = fn
	}
}

// WithAllErrors configures the retry to collect all errors from each attempt.
// When enabled, the final error is an errors.Join of all attempt errors.
// By default, only the last error is returned.
func WithAllErrors() Option {
	return func(c *config) {
		c.allErrors = true
	}
}
