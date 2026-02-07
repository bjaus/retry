package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

type immediateClock struct{}

func (immediateClock) Now() time.Time                             { return time.Now() }
func (immediateClock) Sleep(context.Context, time.Duration) error { return nil }

func BenchmarkDo_ImmediateSuccess(b *testing.B) {
	ctx := context.Background()
	clockOpt := WithClock(immediateClock{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Do(ctx, func(ctx context.Context) error {
			return nil
		}, clockOpt)
	}
}

func BenchmarkDo_OneRetry(b *testing.B) {
	ctx := context.Background()
	errTest := errors.New("test")
	clockOpt := WithClock(immediateClock{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		attempt := 0
		Do(ctx, func(ctx context.Context) error {
			attempt++
			if attempt < 2 {
				return errTest
			}
			return nil
		}, clockOpt)
	}
}

func BenchmarkDo_Exhausted(b *testing.B) {
	ctx := context.Background()
	errTest := errors.New("test")
	opts := []Option{WithMaxAttempts(3), WithClock(immediateClock{})}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Do(ctx, func(ctx context.Context) error {
			return errTest
		}, opts...)
	}
}

func BenchmarkPolicy_Do(b *testing.B) {
	ctx := context.Background()
	policy := New(WithMaxAttempts(3), WithClock(immediateClock{}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		policy.Do(ctx, func(ctx context.Context) error {
			return nil
		})
	}
}

func BenchmarkBackoff_Exponential(b *testing.B) {
	backoff := Exponential(100 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff.Delay(i % 10)
	}
}

func BenchmarkBackoff_ExponentialWithJitter(b *testing.B) {
	backoff := WithJitter(0.2, Exponential(100*time.Millisecond))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff.Delay(i % 10)
	}
}
