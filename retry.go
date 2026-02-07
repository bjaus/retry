package retry

import (
	"context"
	"errors"
	"time"
)

// Func is the function signature for retryable operations.
type Func func(ctx context.Context) error

// Condition determines whether an error should be retried.
type Condition func(error) bool

// OnRetryFunc is called before each retry sleep.
type OnRetryFunc func(ctx context.Context, attempt int, err error, delay time.Duration)

// OnSuccessFunc is called when the function succeeds.
type OnSuccessFunc func(ctx context.Context, attempts int)

// OnExhaustedFunc is called when all retry attempts are exhausted.
type OnExhaustedFunc func(ctx context.Context, attempts int, err error)

// Policy defines retry behavior. Safe for concurrent use.
type Policy struct {
	maxAttempts int
	maxDuration time.Duration
	backoff     Backoff
	clock       Clock
}

// Default values.
const (
	DefaultMaxAttempts = 3
)

// package-level defaults to avoid allocation
var (
	defaultBackoff = Exponential(100 * time.Millisecond)
	defaultClock   = realClock{}
)

// New creates a Policy with the given options.
func New(opts ...Option) *Policy {
	cfg := &config{
		maxAttempts: DefaultMaxAttempts,
		backoff:     Exponential(100 * time.Millisecond),
		clock:       realClock{},
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return &Policy{
		maxAttempts: cfg.maxAttempts,
		maxDuration: cfg.maxDuration,
		backoff:     cfg.backoff,
		clock:       cfg.clock,
	}
}

// Never returns a policy that does not retry.
func Never() *Policy {
	return New(WithMaxAttempts(1))
}

// Default returns a policy with sensible defaults.
func Default() *Policy {
	return New(
		WithMaxAttempts(3),
		WithBackoff(WithJitter(0.2, WithCap(10*time.Second, Exponential(100*time.Millisecond)))),
	)
}

// Do executes fn with retry using the default policy.
func Do(ctx context.Context, fn Func, opts ...Option) error {
	cfg := config{
		maxAttempts: DefaultMaxAttempts,
		backoff:     defaultBackoff,
		clock:       defaultClock,
		condition:   defaultCondition,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return execute(ctx, fn, cfg)
}

// Do executes fn with retry using this policy's configuration.
func (p *Policy) Do(ctx context.Context, fn Func, opts ...Option) error {
	cfg := config{
		maxAttempts: p.maxAttempts,
		maxDuration: p.maxDuration,
		backoff:     p.backoff,
		clock:       p.clock,
		condition:   defaultCondition,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return execute(ctx, fn, cfg)
}

func execute(ctx context.Context, fn Func, cfg config) error {
	var lastErr error
	var errs []error
	var deadline time.Time

	if cfg.maxDuration > 0 {
		deadline = cfg.clock.Now().Add(cfg.maxDuration)
	}

	maxAttempts := cfg.maxAttempts
	if maxAttempts <= 0 {
		maxAttempts = DefaultMaxAttempts
	}

	for attempt := 1; ; attempt++ {
		err := fn(ctx)
		if err == nil {
			if cfg.onSuccess != nil {
				cfg.onSuccess(ctx, attempt)
			}
			return nil
		}

		// Check for terminal error
		var stopped *stopError
		if errors.As(err, &stopped) {
			return stopped.Unwrap()
		}

		// Collect or replace error
		if cfg.allErrors {
			errs = append(errs, err)
		} else {
			lastErr = err
		}

		// Check if we've exhausted attempts
		if attempt >= maxAttempts {
			if cfg.onExhausted != nil {
				cfg.onExhausted(ctx, attempt, err)
			}
			if cfg.allErrors {
				return joinErrors(errs)
			}
			return lastErr
		}

		// Check condition
		if cfg.condition != nil && !cfg.condition(err) {
			if cfg.allErrors {
				return joinErrors(errs)
			}
			return lastErr
		}

		// Check time budget
		if cfg.maxDuration > 0 && cfg.clock.Now().After(deadline) {
			if cfg.onExhausted != nil {
				cfg.onExhausted(ctx, attempt, err)
			}
			if cfg.allErrors {
				return joinErrors(errs)
			}
			return lastErr
		}

		// Calculate delay
		delay := cfg.backoff.Delay(attempt)

		// Check if delay would exceed deadline
		if cfg.maxDuration > 0 {
			remaining := deadline.Sub(cfg.clock.Now())
			if delay > remaining {
				delay = remaining
			}
			if delay <= 0 {
				if cfg.onExhausted != nil {
					cfg.onExhausted(ctx, attempt, err)
				}
				if cfg.allErrors {
					return joinErrors(errs)
				}
				return lastErr
			}
		}

		if cfg.onRetry != nil {
			cfg.onRetry(ctx, attempt, err, delay)
		}

		if err := cfg.clock.Sleep(ctx, delay); err != nil {
			if cfg.allErrors {
				return joinErrors(errs)
			}
			return lastErr
		}
	}
}

func joinErrors(errs []error) error {
	if len(errs) == 1 {
		return errs[0]
	}
	return errors.Join(errs...)
}

func defaultCondition(err error) bool {
	return err != nil
}
