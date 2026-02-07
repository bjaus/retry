# retry

[![Go Reference](https://pkg.go.dev/badge/github.com/bjaus/retry.svg)](https://pkg.go.dev/github.com/bjaus/retry)
[![Go Report Card](https://goreportcard.com/badge/github.com/bjaus/retry)](https://goreportcard.com/report/github.com/bjaus/retry)
[![CI](https://github.com/bjaus/retry/actions/workflows/ci.yml/badge.svg)](https://github.com/bjaus/retry/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/bjaus/retry/branch/main/graph/badge.svg)](https://codecov.io/gh/bjaus/retry)

Flexible, composable retry logic for Go with dependency injection support.

## Features

- **Dependency Injection** — Inject policies at wire-up, customize behavior at call sites
- **Composable Backoff** — Chain strategies like Exponential, WithCap, and WithJitter
- **Injectable Clock** — Control time in tests without real sleeps
- **Lifecycle Hooks** — OnRetry, OnSuccess, OnExhausted for observability
- **Time Budgets** — Limit by attempts, total duration, or both
- **Error Aggregation** — Collect all errors or just the last one
- **Zero Dependencies** — Only the Go standard library

## Installation

```bash
go get github.com/bjaus/retry
```

Requires Go 1.23 or later.

## Quick Start

```go
package main

import (
    "context"
    "errors"
    "log"
    "time"

    "github.com/bjaus/retry"
)

func main() {
    // Simple one-off retry
    err := retry.Do(context.Background(), func(ctx context.Context) error {
        return callExternalAPI(ctx)
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

## Usage

### Creating a Reusable Policy

Policies are created at wire-up time and injected where needed:

```go
// In main or DI container
policy := retry.New(
    retry.WithMaxAttempts(5),
    retry.WithBackoff(retry.Exponential(100*time.Millisecond)),
)

// Inject into services
svc := NewUserService(policy, db)
```

### Customizing at Call Sites

Each call site controls its own retry behavior:

```go
err := policy.Do(ctx, func(ctx context.Context) error {
    return client.Fetch(ctx, id)
},
    retry.If(isTransient),
    retry.OnRetry(func(ctx context.Context, attempt int, err error, delay time.Duration) {
        logger.Warn("retrying", "attempt", attempt, "error", err)
    }),
)
```

### Terminal Errors

Use `Stop` to signal errors that should not be retried:

```go
func fetchUser(ctx context.Context, id string) (*User, error) {
    user, err := db.Get(ctx, id)
    if errors.Is(err, sql.ErrNoRows) {
        return nil, retry.Stop(ErrNotFound)  // Don't retry
    }
    return user, err  // Retry other errors
}
```

### Backoff Strategies

Three base strategies:

```go
retry.Constant(100*time.Millisecond)    // Always 100ms
retry.Linear(100*time.Millisecond)      // 100ms, 200ms, 300ms, ...
retry.Exponential(100*time.Millisecond) // 100ms, 200ms, 400ms, 800ms, ...
```

Compose with wrappers:

```go
// Exponential, capped at 10s, with ±20% jitter
backoff := retry.WithJitter(0.2,
    retry.WithCap(10*time.Second,
        retry.Exponential(100*time.Millisecond),
    ),
)
```

| Wrapper | Description |
|---------|-------------|
| `WithCap(max, b)` | Caps delay at max duration |
| `WithMin(min, b)` | Ensures delay is at least min |
| `WithJitter(factor, b)` | Adds random jitter (±factor × delay) |

### Time Budgets

Combine attempt limits with duration limits:

```go
policy := retry.New(
    retry.WithMaxAttempts(10),              // Stop after 10 attempts
    retry.WithMaxDuration(30*time.Second),  // OR stop after 30s total
)
```

### Lifecycle Hooks

```go
err := policy.Do(ctx, fn,
    retry.OnRetry(func(ctx context.Context, attempt int, err error, delay time.Duration) {
        logger.Warn("retrying", "attempt", attempt)
        metrics.Increment("retries")
    }),
    retry.OnSuccess(func(ctx context.Context, attempts int) {
        if attempts > 1 {
            logger.Info("recovered", "attempts", attempts)
        }
    }),
    retry.OnExhausted(func(ctx context.Context, attempts int, err error) {
        alerting.Notify("retry exhausted")
    }),
)
```

### Error Aggregation

By default, only the last error is returned:

```go
err := retry.Do(ctx, fn)  // Returns last error only
```

Collect all errors:

```go
err := retry.Do(ctx, fn, retry.WithAllErrors())
// err contains all attempt errors via errors.Join
// errors.Is/As work through the chain
```

### Pre-Built Policies

```go
retry.Never()   // No retries (max attempts = 1)
retry.Default() // 3 attempts, exponential backoff with jitter
```

## Testing

Inject a fake clock to control time:

```go
type fakeClock struct {
    now    time.Time
    sleeps []time.Duration
}

func (c *fakeClock) Now() time.Time { return c.now }
func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
    c.sleeps = append(c.sleeps, d)
    c.now = c.now.Add(d)
    return ctx.Err()
}

func TestRetry(t *testing.T) {
    clock := &fakeClock{now: time.Now()}
    policy := retry.New(
        retry.WithMaxAttempts(3),
        retry.WithClock(clock),
    )

    attempts := 0
    _ = policy.Do(ctx, func(ctx context.Context) error {
        attempts++
        return errors.New("fail")
    })

    assert.Equal(t, 3, attempts)
    assert.Len(t, clock.sleeps, 2)  // 2 sleeps between 3 attempts
}
```

## API Reference

### Policy Options (set at wire-up)

| Option | Description |
|--------|-------------|
| `WithMaxAttempts(n)` | Maximum number of attempts |
| `WithMaxDuration(d)` | Maximum total duration |
| `WithBackoff(b)` | Backoff strategy |
| `WithClock(c)` | Clock for time operations (testing) |

### Call Options (set at each call site)

| Option | Description |
|--------|-------------|
| `If(cond)` | Retry if condition returns true |
| `IfNot(cond)` | Skip retry if condition returns true |
| `Not(cond)` | Inverts a condition (helper for composing) |
| `OnRetry(fn)` | Hook called before each retry sleep |
| `OnSuccess(fn)` | Hook called when function succeeds |
| `OnExhausted(fn)` | Hook called when all attempts exhausted |
| `WithAllErrors()` | Collect all errors instead of just the last |

## Design Philosophy

This package separates configuration into two layers:

**Policy-Level** (infrastructure controls the budget):
- How many attempts
- How long to wait between attempts
- Total time budget

**Call-Level** (application controls behavior):
- Which errors to retry
- What to log/metric on retry
- What to do on success/failure

This separation enables clean dependency injection without coupling application code to retry configuration.

## License

MIT License - see [LICENSE](LICENSE) for details.
