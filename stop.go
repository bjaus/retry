package retry

// Stop wraps an error to signal that it should not be retried.
// The retry loop will immediately return the unwrapped error.
func Stop(err error) error {
	if err == nil {
		return nil
	}
	return &stopError{err: err}
}

// stopError wraps an error that should not be retried.
type stopError struct {
	err error
}

func (e *stopError) Error() string {
	return e.err.Error()
}

func (e *stopError) Unwrap() error {
	return e.err
}
