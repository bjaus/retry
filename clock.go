package retry

import (
	"context"
	"time"
)

// Clock abstracts time operations for testing.
type Clock interface {
	Now() time.Time
	Sleep(ctx context.Context, d time.Duration) error
}

// realClock implements Clock using the standard time package.
type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now()
}

func (realClock) Sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
		return ctx.Err()
	}
}
