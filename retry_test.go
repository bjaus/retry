package retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bjaus/retry"
)

var errTest = errors.New("test error")

// fakeClock is a test clock that tracks sleep calls without actually sleeping.
type fakeClock struct {
	now    time.Time
	sleeps []time.Duration
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Now()}
}

func (c *fakeClock) Now() time.Time {
	return c.now
}

func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		c.sleeps = append(c.sleeps, d)
		c.now = c.now.Add(d)
		return nil
	}
}

func (c *fakeClock) Advance(d time.Duration) {
	c.now = c.now.Add(d)
}

func TestDo(t *testing.T) {
	t.Run("succeeds on first attempt", func(t *testing.T) {
		attempts := 0
		err := retry.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			return nil
		}, retry.WithClock(newFakeClock()))

		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if attempts != 1 {
			t.Fatalf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("succeeds after retries", func(t *testing.T) {
		attempts := 0
		err := retry.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			if attempts < 3 {
				return errTest
			}
			return nil
		}, retry.WithClock(newFakeClock()))

		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if attempts != 3 {
			t.Fatalf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("exhausts max attempts", func(t *testing.T) {
		attempts := 0
		err := retry.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			return errTest
		},
			retry.WithMaxAttempts(5),
			retry.WithClock(newFakeClock()),
		)

		if !errors.Is(err, errTest) {
			t.Fatalf("expected errTest, got %v", err)
		}
		if attempts != 5 {
			t.Fatalf("expected 5 attempts, got %d", attempts)
		}
	})

	t.Run("stops immediately with Stop error", func(t *testing.T) {
		attempts := 0
		err := retry.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			return retry.Stop(errTest)
		},
			retry.WithMaxAttempts(5),
			retry.WithClock(newFakeClock()),
		)

		if !errors.Is(err, errTest) {
			t.Fatalf("expected errTest, got %v", err)
		}
		if attempts != 1 {
			t.Fatalf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0

		err := retry.Do(ctx, func(ctx context.Context) error {
			attempts++
			if attempts == 2 {
				cancel()
			}
			return errTest
		},
			retry.WithMaxAttempts(10),
			retry.WithClock(newFakeClock()),
		)

		if !errors.Is(err, errTest) {
			t.Fatalf("expected errTest, got %v", err)
		}
		if attempts != 2 {
			t.Fatalf("expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("respects condition", func(t *testing.T) {
		attempts := 0
		nonRetryable := errors.New("non-retryable")

		err := retry.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			if attempts == 2 {
				return nonRetryable
			}
			return errTest
		},
			retry.WithMaxAttempts(10),
			retry.WithClock(newFakeClock()),
			retry.If(func(err error) bool {
				return !errors.Is(err, nonRetryable)
			}),
		)

		if !errors.Is(err, nonRetryable) {
			t.Fatalf("expected nonRetryable, got %v", err)
		}
		if attempts != 2 {
			t.Fatalf("expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("IfNot skips matching errors", func(t *testing.T) {
		attempts := 0
		skipThis := errors.New("skip this error")

		err := retry.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			if attempts == 2 {
				return skipThis
			}
			return errTest
		},
			retry.WithMaxAttempts(10),
			retry.WithClock(newFakeClock()),
			retry.IfNot(func(err error) bool {
				return errors.Is(err, skipThis)
			}),
		)

		if !errors.Is(err, skipThis) {
			t.Fatalf("expected skipThis, got %v", err)
		}
		if attempts != 2 {
			t.Fatalf("expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("Not inverts condition", func(t *testing.T) {
		alwaysTrue := func(err error) bool { return true }
		alwaysFalse := func(err error) bool { return false }

		inverted := retry.Not(alwaysTrue)
		if inverted(errTest) != false {
			t.Fatal("expected Not(alwaysTrue) to return false")
		}

		inverted = retry.Not(alwaysFalse)
		if inverted(errTest) != true {
			t.Fatal("expected Not(alwaysFalse) to return true")
		}
	})
}

func TestPolicy(t *testing.T) {
	t.Run("reuses configuration", func(t *testing.T) {
		clock := newFakeClock()
		policy := retry.New(
			retry.WithMaxAttempts(2),
			retry.WithClock(clock),
		)

		// First call
		attempts1 := 0
		_ = policy.Do(context.Background(), func(ctx context.Context) error {
			attempts1++
			return errTest
		})

		// Second call
		attempts2 := 0
		_ = policy.Do(context.Background(), func(ctx context.Context) error {
			attempts2++
			return errTest
		})

		if attempts1 != 2 {
			t.Fatalf("expected 2 attempts for first call, got %d", attempts1)
		}
		if attempts2 != 2 {
			t.Fatalf("expected 2 attempts for second call, got %d", attempts2)
		}
	})

	t.Run("Never policy does not retry", func(t *testing.T) {
		attempts := 0
		err := retry.Never().Do(context.Background(), func(ctx context.Context) error {
			attempts++
			return errTest
		}, retry.WithClock(newFakeClock()))

		if !errors.Is(err, errTest) {
			t.Fatalf("expected errTest, got %v", err)
		}
		if attempts != 1 {
			t.Fatalf("expected 1 attempt, got %d", attempts)
		}
	})
}

func TestHooks(t *testing.T) {
	t.Run("OnRetry called before each retry", func(t *testing.T) {
		var retryAttempts []int
		err := retry.Do(context.Background(), func(ctx context.Context) error {
			return errTest
		},
			retry.WithMaxAttempts(3),
			retry.WithClock(newFakeClock()),
			retry.OnRetry(func(ctx context.Context, attempt int, err error, delay time.Duration) {
				retryAttempts = append(retryAttempts, attempt)
			}),
		)

		if !errors.Is(err, errTest) {
			t.Fatalf("expected errTest, got %v", err)
		}
		// OnRetry is called after attempts 1 and 2 (before retry)
		if len(retryAttempts) != 2 {
			t.Fatalf("expected 2 OnRetry calls, got %d", len(retryAttempts))
		}
		if retryAttempts[0] != 1 || retryAttempts[1] != 2 {
			t.Fatalf("expected attempts [1, 2], got %v", retryAttempts)
		}
	})

	t.Run("OnSuccess called with attempt count", func(t *testing.T) {
		var successAttempts int
		attempts := 0
		err := retry.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			if attempts < 3 {
				return errTest
			}
			return nil
		},
			retry.WithClock(newFakeClock()),
			retry.OnSuccess(func(ctx context.Context, a int) {
				successAttempts = a
			}),
		)

		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if successAttempts != 3 {
			t.Fatalf("expected success on attempt 3, got %d", successAttempts)
		}
	})

	t.Run("OnExhausted called when attempts exhausted", func(t *testing.T) {
		var exhaustedAttempts int
		var exhaustedErr error

		err := retry.Do(context.Background(), func(ctx context.Context) error {
			return errTest
		},
			retry.WithMaxAttempts(3),
			retry.WithClock(newFakeClock()),
			retry.OnExhausted(func(ctx context.Context, a int, e error) {
				exhaustedAttempts = a
				exhaustedErr = e
			}),
		)

		if !errors.Is(err, errTest) {
			t.Fatalf("expected errTest, got %v", err)
		}
		if exhaustedAttempts != 3 {
			t.Fatalf("expected exhausted on attempt 3, got %d", exhaustedAttempts)
		}
		if !errors.Is(exhaustedErr, errTest) {
			t.Fatalf("expected exhaustedErr to be errTest, got %v", exhaustedErr)
		}
	})
}

func TestMaxDuration(t *testing.T) {
	t.Run("stops when duration exceeded", func(t *testing.T) {
		clock := newFakeClock()
		attempts := 0

		err := retry.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			// Simulate time passing during each attempt
			clock.Advance(100 * time.Millisecond)
			return errTest
		},
			retry.WithMaxAttempts(100),
			retry.WithMaxDuration(250*time.Millisecond),
			retry.WithBackoff(retry.Constant(50*time.Millisecond)),
			retry.WithClock(clock),
		)

		if !errors.Is(err, errTest) {
			t.Fatalf("expected errTest, got %v", err)
		}
		// With 250ms budget, 100ms per attempt + 50ms sleep:
		// Attempt 1: 100ms elapsed, sleep 50ms = 150ms total
		// Attempt 2: 100ms elapsed = 250ms total, then check deadline
		if attempts < 2 || attempts > 3 {
			t.Fatalf("expected 2-3 attempts, got %d", attempts)
		}
	})

	t.Run("coexists with max attempts", func(t *testing.T) {
		clock := newFakeClock()
		attempts := 0

		// Max attempts will trigger first
		err := retry.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			return errTest
		},
			retry.WithMaxAttempts(2),
			retry.WithMaxDuration(1*time.Hour),
			retry.WithClock(clock),
		)

		if !errors.Is(err, errTest) {
			t.Fatalf("expected errTest, got %v", err)
		}
		if attempts != 2 {
			t.Fatalf("expected 2 attempts, got %d", attempts)
		}
	})
}

func TestWithAllErrors(t *testing.T) {
	t.Run("collects all errors", func(t *testing.T) {
		err1 := errors.New("error 1")
		err2 := errors.New("error 2")
		err3 := errors.New("error 3")

		attempts := 0
		err := retry.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			switch attempts {
			case 1:
				return err1
			case 2:
				return err2
			default:
				return err3
			}
		},
			retry.WithMaxAttempts(3),
			retry.WithClock(newFakeClock()),
			retry.WithAllErrors(),
		)

		if !errors.Is(err, err1) {
			t.Fatal("expected err to contain err1")
		}
		if !errors.Is(err, err2) {
			t.Fatal("expected err to contain err2")
		}
		if !errors.Is(err, err3) {
			t.Fatal("expected err to contain err3")
		}
	})

	t.Run("default returns only last error", func(t *testing.T) {
		err1 := errors.New("error 1")
		err2 := errors.New("error 2")

		attempts := 0
		err := retry.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			if attempts == 1 {
				return err1
			}
			return err2
		},
			retry.WithMaxAttempts(2),
			retry.WithClock(newFakeClock()),
		)

		if !errors.Is(err, err2) {
			t.Fatal("expected err to be err2")
		}
		if errors.Is(err, err1) {
			t.Fatal("expected err to NOT contain err1")
		}
	})
}

func TestStop(t *testing.T) {
	t.Run("nil error returns nil", func(t *testing.T) {
		err := retry.Stop(nil)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("preserves error chain", func(t *testing.T) {
		inner := errors.New("inner")
		wrapped := errors.New("wrapped: " + inner.Error())

		err := retry.Do(context.Background(), func(ctx context.Context) error {
			return retry.Stop(wrapped)
		},
			retry.WithMaxAttempts(5),
			retry.WithClock(newFakeClock()),
		)

		if err.Error() != wrapped.Error() {
			t.Fatalf("expected %q, got %q", wrapped.Error(), err.Error())
		}
	})

	t.Run("error method returns wrapped error message", func(t *testing.T) {
		inner := errors.New("the inner error")
		stopped := retry.Stop(inner)

		if stopped.Error() != "the inner error" {
			t.Fatalf("expected %q, got %q", "the inner error", stopped.Error())
		}
	})
}

func TestDefault(t *testing.T) {
	t.Run("default policy uses sensible defaults", func(t *testing.T) {
		policy := retry.Default()
		clock := newFakeClock()

		attempts := 0
		_ = policy.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			return errTest
		}, retry.WithClock(clock))

		if attempts != 3 {
			t.Fatalf("expected 3 attempts (default), got %d", attempts)
		}
	})
}

func TestMaxDurationEdgeCases(t *testing.T) {
	t.Run("delay exceeds remaining time budget", func(t *testing.T) {
		clock := newFakeClock()
		attempts := 0
		var exhaustedCalled bool

		_ = retry.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			// Advance clock so remaining time is less than backoff delay
			clock.Advance(90 * time.Millisecond)
			return errTest
		},
			retry.WithMaxAttempts(10),
			retry.WithMaxDuration(100*time.Millisecond),
			retry.WithBackoff(retry.Constant(50*time.Millisecond)),
			retry.WithClock(clock),
			retry.OnExhausted(func(ctx context.Context, a int, e error) {
				exhaustedCalled = true
			}),
		)

		if !exhaustedCalled {
			t.Fatal("expected OnExhausted to be called")
		}
	})

	t.Run("delay capped to remaining budget", func(t *testing.T) {
		clock := newFakeClock()
		var delays []time.Duration

		_ = retry.Do(context.Background(), func(ctx context.Context) error {
			clock.Advance(80 * time.Millisecond)
			return errTest
		},
			retry.WithMaxAttempts(10),
			retry.WithMaxDuration(100*time.Millisecond),
			retry.WithBackoff(retry.Constant(50*time.Millisecond)),
			retry.WithClock(clock),
			retry.OnRetry(func(ctx context.Context, attempt int, err error, delay time.Duration) {
				delays = append(delays, delay)
			}),
		)

		// First retry should have delay capped to remaining budget (20ms)
		if len(delays) > 0 && delays[0] > 20*time.Millisecond {
			t.Fatalf("expected delay to be capped, got %v", delays[0])
		}
	})

	t.Run("zero remaining budget stops immediately", func(t *testing.T) {
		clock := newFakeClock()
		attempts := 0

		_ = retry.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			clock.Advance(100 * time.Millisecond)
			return errTest
		},
			retry.WithMaxAttempts(10),
			retry.WithMaxDuration(100*time.Millisecond),
			retry.WithBackoff(retry.Constant(10*time.Millisecond)),
			retry.WithClock(clock),
		)

		if attempts != 1 {
			t.Fatalf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("zero remaining budget calls OnExhausted", func(t *testing.T) {
		clock := newFakeClock()
		var exhaustedCalled bool
		var exhaustedAttempts int

		_ = retry.Do(context.Background(), func(ctx context.Context) error {
			// Advance exactly to the deadline so remaining = 0
			// After(deadline) is false when Now() == deadline
			// but remaining = deadline - Now() = 0
			clock.Advance(100 * time.Millisecond)
			return errTest
		},
			retry.WithMaxAttempts(10),
			retry.WithMaxDuration(100*time.Millisecond),
			retry.WithBackoff(retry.Constant(10*time.Millisecond)),
			retry.WithClock(clock),
			retry.OnExhausted(func(ctx context.Context, a int, e error) {
				exhaustedCalled = true
				exhaustedAttempts = a
			}),
		)

		if !exhaustedCalled {
			t.Fatal("expected OnExhausted to be called")
		}
		if exhaustedAttempts < 1 {
			t.Fatalf("expected at least 1 attempt, got %d", exhaustedAttempts)
		}
	})
}

func TestZeroMaxAttempts(t *testing.T) {
	t.Run("zero max attempts uses default", func(t *testing.T) {
		attempts := 0
		_ = retry.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			return errTest
		},
			retry.WithMaxAttempts(0),
			retry.WithClock(newFakeClock()),
		)

		if attempts != 3 {
			t.Fatalf("expected 3 attempts (default), got %d", attempts)
		}
	})
}

func TestRealClock(t *testing.T) {
	t.Run("uses real clock when not injected", func(t *testing.T) {
		attempts := 0
		start := time.Now()

		err := retry.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			if attempts < 2 {
				return errTest
			}
			return nil
		},
			retry.WithMaxAttempts(3),
			retry.WithBackoff(retry.Constant(5*time.Millisecond)),
		)

		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if attempts != 2 {
			t.Fatalf("expected 2 attempts, got %d", attempts)
		}
		// Should have slept at least once (5ms)
		if elapsed < 5*time.Millisecond {
			t.Fatalf("expected elapsed >= 5ms, got %v", elapsed)
		}
	})

	t.Run("real clock respects context cancellation during sleep", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0

		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		start := time.Now()
		_ = retry.Do(ctx, func(ctx context.Context) error {
			attempts++
			return errTest
		},
			retry.WithMaxAttempts(100),
			retry.WithBackoff(retry.Constant(1*time.Second)), // Long sleep
		)
		elapsed := time.Since(start)

		// Should have been cancelled during sleep, not waited full 1s
		if elapsed > 500*time.Millisecond {
			t.Fatalf("expected early cancellation, but took %v", elapsed)
		}
	})

	t.Run("real clock with max duration", func(t *testing.T) {
		attempts := 0
		start := time.Now()

		_ = retry.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			return errTest
		},
			retry.WithMaxAttempts(100),
			retry.WithMaxDuration(50*time.Millisecond),
			retry.WithBackoff(retry.Constant(10*time.Millisecond)),
		)

		elapsed := time.Since(start)

		// Should have stopped within ~50ms due to max duration
		if elapsed > 200*time.Millisecond {
			t.Fatalf("expected to stop within max duration, took %v", elapsed)
		}
	})
}
