// Package retry provides flexible, composable retry logic with dependency injection support.
//
// retry is a retry package that provides:
//
//   - Dependency Injection: Inject policies at wire-up, customize behavior at call sites
//   - Composable Backoff: Chain strategies like Exponential, WithCap, and WithJitter
//   - Injectable Clock: Control time in tests without real sleeps
//   - Lifecycle Hooks: OnRetry, OnSuccess, OnExhausted for observability
//   - Error Aggregation: Collect all errors or just the last one
//   - Zero Dependencies: Only the Go standard library
//
// # Quick Start
//
// Using the global Do function for one-off retries:
//
//	err := retry.Do(ctx, func(ctx context.Context) error {
//	    return client.Call(ctx)
//	})
//
// Creating a reusable policy for dependency injection:
//
//	// At wire-up time (e.g., in main or a DI container)
//	policy := retry.New(
//	    retry.WithMaxAttempts(5),
//	    retry.WithBackoff(retry.Exponential(100*time.Millisecond)),
//	)
//
//	// At call site
//	err := policy.Do(ctx, func(ctx context.Context) error {
//	    return client.Call(ctx)
//	},
//	    retry.If(isTransient),
//	    retry.OnRetry(func(ctx context.Context, attempt int, err error, delay time.Duration) {
//	        log.Warn("retrying", "attempt", attempt, "error", err, "delay", delay)
//	    }),
//	)
//
// # Design Philosophy
//
// The package separates configuration into two categories:
//
// Policy-Level (set at wire-up, injected via DI):
//   - MaxAttempts: How many times to try
//   - MaxDuration: Total time budget across all attempts
//   - Backoff: Delay strategy between attempts
//   - Clock: Time abstraction for testing
//
// Call-Level (set at each call site):
//   - If: Condition to determine if an error should be retried
//   - OnRetry: Hook called before each retry sleep
//   - OnSuccess: Hook called when the function succeeds
//   - OnExhausted: Hook called when all attempts are exhausted
//   - WithAllErrors: Collect all errors instead of just the last
//
// This separation allows:
//   - Infrastructure to control retry budgets (how many, how fast)
//   - Application code to control retry behavior (which errors, what to log)
//   - Clean dependency injection without coupling to configuration
//
// # Terminal Errors
//
// Use Stop to signal that an error should not be retried:
//
//	func fetchUser(ctx context.Context, id string) (*User, error) {
//	    user, err := db.Get(ctx, id)
//	    if errors.Is(err, sql.ErrNoRows) {
//	        return nil, retry.Stop(ErrNotFound)  // Don't retry "not found"
//	    }
//	    return user, err  // Other errors will be retried
//	}
//
// # Backoff Strategies
//
// The package provides three base strategies:
//
//	retry.Constant(100*time.Millisecond)    // Always 100ms
//	retry.Linear(100*time.Millisecond)      // 100ms, 200ms, 300ms, ...
//	retry.Exponential(100*time.Millisecond) // 100ms, 200ms, 400ms, 800ms, ...
//
// Strategies can be composed with wrappers:
//
//	// Exponential backoff, capped at 10s, with ±20% jitter
//	backoff := retry.WithJitter(0.2,
//	    retry.WithCap(10*time.Second,
//	        retry.Exponential(100*time.Millisecond),
//	    ),
//	)
//
// Available wrappers:
//
//   - WithCap(max, b): Caps delay at max duration
//   - WithMin(min, b): Ensures delay is at least min duration
//   - WithJitter(factor, b): Adds random jitter (±factor * delay)
//
// Custom backoff strategies can be created using BackoffFunc:
//
//	custom := retry.BackoffFunc(func(attempt int) time.Duration {
//	    return time.Duration(attempt*attempt) * 100 * time.Millisecond
//	})
//
// # Time Budgets
//
// Use both MaxAttempts and MaxDuration for precise control:
//
//	policy := retry.New(
//	    retry.WithMaxAttempts(10),               // Stop after 10 attempts
//	    retry.WithMaxDuration(30*time.Second),   // OR stop after 30s total
//	)
//
// The retry loop stops when either limit is reached first.
//
// # Lifecycle Hooks
//
// Hooks provide observability without coupling to a specific logger or metrics system:
//
//	err := policy.Do(ctx, fn,
//	    retry.OnRetry(func(ctx context.Context, attempt int, err error, delay time.Duration) {
//	        logger.Warn("retrying", "attempt", attempt, "delay", delay)
//	        metrics.Increment("retries")
//	    }),
//	    retry.OnSuccess(func(ctx context.Context, attempts int) {
//	        if attempts > 1 {
//	            logger.Info("recovered", "attempts", attempts)
//	        }
//	    }),
//	    retry.OnExhausted(func(ctx context.Context, attempts int, err error) {
//	        logger.Error("gave up", "attempts", attempts, "error", err)
//	        alerting.Notify("retry exhausted")
//	    }),
//	)
//
// # Error Aggregation
//
// By default, only the last error is returned. Use WithAllErrors to collect all:
//
//	err := retry.Do(ctx, fn, retry.WithAllErrors())
//	// err contains all attempt errors via errors.Join
//	// errors.Is/As work through the chain
//
// # Testing
//
// Inject a fake clock to control time in tests:
//
//	type fakeClock struct {
//	    now    time.Time
//	    sleeps []time.Duration
//	}
//
//	func (c *fakeClock) Now() time.Time { return c.now }
//	func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
//	    c.sleeps = append(c.sleeps, d)
//	    c.now = c.now.Add(d)
//	    return ctx.Err()
//	}
//
//	func TestRetry(t *testing.T) {
//	    clock := &fakeClock{now: time.Now()}
//	    policy := retry.New(
//	        retry.WithMaxAttempts(3),
//	        retry.WithClock(clock),
//	    )
//
//	    attempts := 0
//	    _ = policy.Do(ctx, func(ctx context.Context) error {
//	        attempts++
//	        return errors.New("fail")
//	    })
//
//	    assert.Equal(t, 3, attempts)
//	    assert.Len(t, clock.sleeps, 2) // 2 sleeps between 3 attempts
//	}
//
// # Pre-Built Policies
//
// The package provides convenience functions for common configurations:
//
//	retry.Never()   // No retries, just run once
//	retry.Default() // Sensible defaults (3 attempts, exponential backoff with jitter)
//
// # Best Practices
//
// 1. Inject policies, customize at call sites:
//
//	// Wire-up
//	policy := retry.New(retry.WithMaxAttempts(5))
//
//	// Call site
//	err := policy.Do(ctx, fn, retry.If(isTransient))
//
// 2. Use Stop for non-retryable errors:
//
//	if errors.Is(err, ErrNotFound) {
//	    return retry.Stop(err)
//	}
//
// 3. Add jitter to prevent thundering herd:
//
//	retry.WithJitter(0.2, retry.Exponential(100*time.Millisecond))
//
// 4. Cap exponential backoff to prevent excessive delays:
//
//	retry.WithCap(30*time.Second, retry.Exponential(100*time.Millisecond))
//
// 5. Use hooks for observability instead of wrapping:
//
//	retry.OnRetry(func(...) { logger.Warn(...) })
package retry
