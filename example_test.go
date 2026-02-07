package retry_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bjaus/retry"
)

// ExampleDo demonstrates the simplest usage with the global Do function.
func ExampleDo() {
	attempts := 0
	err := retry.Do(context.Background(), func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary failure")
		}
		return nil
	},
		retry.WithMaxAttempts(5),
		retry.WithBackoff(retry.Constant(time.Millisecond)),
	)

	fmt.Println("Error:", err)
	fmt.Println("Attempts:", attempts)

	// Output:
	// Error: <nil>
	// Attempts: 3
}

// ExampleNew demonstrates creating a reusable policy.
func ExampleNew() {
	policy := retry.New(
		retry.WithMaxAttempts(3),
		retry.WithBackoff(retry.Constant(time.Millisecond)),
	)

	attempts := 0
	err := policy.Do(context.Background(), func(ctx context.Context) error {
		attempts++
		return errors.New("always fails")
	})

	fmt.Println("Error:", err)
	fmt.Println("Attempts:", attempts)

	// Output:
	// Error: always fails
	// Attempts: 3
}

// ExampleNever demonstrates a policy that does not retry.
func ExampleNever() {
	policy := retry.Never()

	attempts := 0
	_ = policy.Do(context.Background(), func(ctx context.Context) error {
		attempts++
		return errors.New("fail")
	})

	fmt.Println("Attempts:", attempts)

	// Output:
	// Attempts: 1
}

// ExampleStop demonstrates signaling a non-retryable error.
func ExampleStop() {
	notFound := errors.New("not found")

	attempts := 0
	err := retry.Do(context.Background(), func(ctx context.Context) error {
		attempts++
		return retry.Stop(notFound)
	},
		retry.WithMaxAttempts(5),
		retry.WithBackoff(retry.Constant(time.Millisecond)),
	)

	fmt.Println("Error:", err)
	fmt.Println("Attempts:", attempts)

	// Output:
	// Error: not found
	// Attempts: 1
}

// ExampleIf demonstrates conditional retry based on error type.
func ExampleIf() {
	transient := errors.New("transient error")
	permanent := errors.New("permanent error")

	attempts := 0
	err := retry.Do(context.Background(), func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return transient
		}
		return permanent
	},
		retry.WithMaxAttempts(10),
		retry.WithBackoff(retry.Constant(time.Millisecond)),
		retry.If(func(err error) bool {
			return errors.Is(err, transient)
		}),
	)

	fmt.Println("Error:", err)
	fmt.Println("Attempts:", attempts)

	// Output:
	// Error: permanent error
	// Attempts: 3
}

// ExampleOnRetry demonstrates the retry hook for logging.
func ExampleOnRetry() {
	attempts := 0
	retryCount := 0

	_ = retry.Do(context.Background(), func(ctx context.Context) error {
		attempts++
		return errors.New("fail")
	},
		retry.WithMaxAttempts(3),
		retry.WithBackoff(retry.Constant(time.Millisecond)),
		retry.OnRetry(func(ctx context.Context, attempt int, err error, delay time.Duration) {
			retryCount++
			fmt.Printf("Retry %d: %v\n", attempt, err)
		}),
	)

	fmt.Println("Total retries:", retryCount)

	// Output:
	// Retry 1: fail
	// Retry 2: fail
	// Total retries: 2
}

// ExampleOnSuccess demonstrates the success hook.
func ExampleOnSuccess() {
	attempts := 0

	_ = retry.Do(context.Background(), func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("not yet")
		}
		return nil
	},
		retry.WithMaxAttempts(5),
		retry.WithBackoff(retry.Constant(time.Millisecond)),
		retry.OnSuccess(func(ctx context.Context, attempts int) {
			fmt.Printf("Succeeded on attempt %d\n", attempts)
		}),
	)

	// Output:
	// Succeeded on attempt 3
}

// ExampleOnExhausted demonstrates the exhausted hook.
func ExampleOnExhausted() {
	_ = retry.Do(context.Background(), func(ctx context.Context) error {
		return errors.New("always fails")
	},
		retry.WithMaxAttempts(3),
		retry.WithBackoff(retry.Constant(time.Millisecond)),
		retry.OnExhausted(func(ctx context.Context, attempts int, err error) {
			fmt.Printf("Exhausted after %d attempts: %v\n", attempts, err)
		}),
	)

	// Output:
	// Exhausted after 3 attempts: always fails
}

// ExampleWithAllErrors demonstrates collecting all errors.
func ExampleWithAllErrors() {
	attempt := 0
	err := retry.Do(context.Background(), func(ctx context.Context) error {
		attempt++
		return fmt.Errorf("error %d", attempt)
	},
		retry.WithMaxAttempts(3),
		retry.WithBackoff(retry.Constant(time.Millisecond)),
		retry.WithAllErrors(),
	)

	fmt.Println("Contains error 1:", errors.Is(err, fmt.Errorf("error 1")))
	fmt.Println("Error string contains all:", err != nil)

	// Output:
	// Contains error 1: false
	// Error string contains all: true
}

// ExampleConstant demonstrates constant backoff.
func ExampleConstant() {
	b := retry.Constant(100 * time.Millisecond)

	fmt.Println("Attempt 1:", b.Delay(1))
	fmt.Println("Attempt 2:", b.Delay(2))
	fmt.Println("Attempt 5:", b.Delay(5))

	// Output:
	// Attempt 1: 100ms
	// Attempt 2: 100ms
	// Attempt 5: 100ms
}

// ExampleLinear demonstrates linear backoff.
func ExampleLinear() {
	b := retry.Linear(100 * time.Millisecond)

	fmt.Println("Attempt 1:", b.Delay(1))
	fmt.Println("Attempt 2:", b.Delay(2))
	fmt.Println("Attempt 5:", b.Delay(5))

	// Output:
	// Attempt 1: 100ms
	// Attempt 2: 200ms
	// Attempt 5: 500ms
}

// ExampleExponential demonstrates exponential backoff.
func ExampleExponential() {
	b := retry.Exponential(100 * time.Millisecond)

	fmt.Println("Attempt 1:", b.Delay(1))
	fmt.Println("Attempt 2:", b.Delay(2))
	fmt.Println("Attempt 3:", b.Delay(3))
	fmt.Println("Attempt 4:", b.Delay(4))

	// Output:
	// Attempt 1: 100ms
	// Attempt 2: 200ms
	// Attempt 3: 400ms
	// Attempt 4: 800ms
}

// ExampleWithCap demonstrates capping backoff delays.
func ExampleWithCap() {
	b := retry.WithCap(500*time.Millisecond, retry.Exponential(100*time.Millisecond))

	fmt.Println("Attempt 1:", b.Delay(1))
	fmt.Println("Attempt 2:", b.Delay(2))
	fmt.Println("Attempt 3:", b.Delay(3))
	fmt.Println("Attempt 4:", b.Delay(4)) // Would be 800ms, capped to 500ms
	fmt.Println("Attempt 5:", b.Delay(5)) // Would be 1.6s, capped to 500ms

	// Output:
	// Attempt 1: 100ms
	// Attempt 2: 200ms
	// Attempt 3: 400ms
	// Attempt 4: 500ms
	// Attempt 5: 500ms
}

// ExampleWithMin demonstrates minimum backoff delays.
func ExampleWithMin() {
	b := retry.WithMin(150*time.Millisecond, retry.Linear(50*time.Millisecond))

	fmt.Println("Attempt 1:", b.Delay(1)) // 50ms -> 150ms (min)
	fmt.Println("Attempt 2:", b.Delay(2)) // 100ms -> 150ms (min)
	fmt.Println("Attempt 3:", b.Delay(3)) // 150ms (at min)
	fmt.Println("Attempt 4:", b.Delay(4)) // 200ms (above min)

	// Output:
	// Attempt 1: 150ms
	// Attempt 2: 150ms
	// Attempt 3: 150ms
	// Attempt 4: 200ms
}

// ExampleBackoffFunc demonstrates creating a custom backoff strategy.
func ExampleBackoffFunc() {
	// Quadratic backoff: delay = base * attempt^2
	b := retry.BackoffFunc(func(attempt int) time.Duration {
		return time.Duration(attempt*attempt) * 10 * time.Millisecond
	})

	fmt.Println("Attempt 1:", b.Delay(1))
	fmt.Println("Attempt 2:", b.Delay(2))
	fmt.Println("Attempt 3:", b.Delay(3))
	fmt.Println("Attempt 4:", b.Delay(4))

	// Output:
	// Attempt 1: 10ms
	// Attempt 2: 40ms
	// Attempt 3: 90ms
	// Attempt 4: 160ms
}

// Example_composedBackoff demonstrates composing multiple backoff wrappers.
func Example_composedBackoff() {
	// Exponential backoff, capped at 1s, with minimum 50ms
	b := retry.WithMin(50*time.Millisecond,
		retry.WithCap(1*time.Second,
			retry.Exponential(10*time.Millisecond),
		),
	)

	fmt.Println("Attempt 1:", b.Delay(1))   // 10ms -> 50ms (min)
	fmt.Println("Attempt 2:", b.Delay(2))   // 20ms -> 50ms (min)
	fmt.Println("Attempt 5:", b.Delay(5))   // 160ms
	fmt.Println("Attempt 10:", b.Delay(10)) // 5.12s -> 1s (cap)

	// Output:
	// Attempt 1: 50ms
	// Attempt 2: 50ms
	// Attempt 5: 160ms
	// Attempt 10: 1s
}

// Example_dependencyInjection demonstrates the recommended DI pattern.
func Example_dependencyInjection() {
	// === Wire-up time (e.g., in main or DI container) ===
	policy := retry.New(
		retry.WithMaxAttempts(5),
		retry.WithBackoff(retry.Constant(time.Millisecond)),
	)

	// === Call site (in application code) ===
	// The caller doesn't know or care about the retry budget.
	// It only controls which errors to retry and what to log.
	attempts := 0
	var retried bool

	err := policy.Do(context.Background(), func(ctx context.Context) error {
		attempts++
		if attempts < 2 {
			return errors.New("transient")
		}
		return nil
	},
		retry.If(func(err error) bool {
			return err.Error() == "transient"
		}),
		retry.OnRetry(func(ctx context.Context, attempt int, err error, delay time.Duration) {
			retried = true
		}),
	)

	fmt.Println("Error:", err)
	fmt.Println("Retried:", retried)

	// Output:
	// Error: <nil>
	// Retried: true
}
