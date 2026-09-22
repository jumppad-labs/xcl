package parser

import "errors"

// ErrEmptyConfiguration is returned by Apply when the configuration declares no
// blocks. Applying it would remove everything, which is what Destroy is for, so
// nothing is destroyed, created, changed or saved.
var ErrEmptyConfiguration = errors.New("the configuration declares no blocks, use Destroy to remove everything")

// errNotReached is returned for a provider step that was skipped because the
// operation's context was cancelled. The resource is treated as never reached:
// nothing is recorded for it, so the state keeps its previous entry. It is
// never returned to callers, a cancelled walk reports the context's error.
var errNotReached = errors.New("not reached, the operation was cancelled")

// reportedError is an error an event has already been emitted for, so the
// code returning it further up does not emit it a second time
type reportedError struct {
	error
}

// Unwrap returns the error that was reported
func (e reportedError) Unwrap() error {
	return e.error
}

// reported returns true when err, or an error it wraps, was already emitted
func reported(err error) bool {
	var r reportedError
	return errors.As(err, &r)
}
