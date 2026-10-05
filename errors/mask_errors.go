package errors

import (
	"errors"
	"fmt"
)

// The sentinels below name the ways masking a sensitive value can fail. Each
// is matched by identity with errors.Is, and each is wrapped by a detail type
// recovered with errors.As.
//
// They are declared here, rather than in the mask package that raises them,
// because the shared reader of saved records raises one too, and the public
// package re-exports both.
var (
	// ErrUnrecoverable is returned when a masked value cannot be turned back
	// into its real value: the masker that produced it is one way, it was
	// produced by a different masker, or it does not open, for example under a
	// different key. A masked value is never read back as a wrong value.
	ErrUnrecoverable = errors.New("masked value cannot be recovered")

	// ErrMaskNotReversible is returned when a masker that cannot recover the
	// values it masks is configured for state.
	ErrMaskNotReversible = errors.New("the state masker must be reversible")
)

// UnrecoverableError reports a masked value that cannot be recovered. ID is
// the entity holding it, where that is known, MaskedBy is the masker the
// masked value names, and Reason says why it cannot be recovered.
type UnrecoverableError struct {
	ID       string
	MaskedBy string
	Reason   string
	Err      error
}

func (e *UnrecoverableError) Error() string {
	message := "masked value"
	if e.ID != "" {
		message = fmt.Sprintf("masked value in %q", e.ID)
	}

	if e.MaskedBy != "" {
		message = fmt.Sprintf("%s, masked by %q,", message, e.MaskedBy)
	}

	message = fmt.Sprintf("%s cannot be recovered", message)

	if e.Reason != "" {
		message = fmt.Sprintf("%s: %s", message, e.Reason)
	}

	if e.Err != nil {
		message = fmt.Sprintf("%s: %s", message, e.Err)
	}

	return message
}

// Unwrap returns ErrUnrecoverable, and the underlying failure where there is
// one, so both answer errors.Is
func (e *UnrecoverableError) Unwrap() []error {
	if e.Err == nil {
		return []error{ErrUnrecoverable}
	}

	return []error{ErrUnrecoverable, e.Err}
}

// MaskNotReversibleError reports the masker that was configured for state but
// cannot recover the values it masks.
type MaskNotReversibleError struct {
	Masker string
}

func (e *MaskNotReversibleError) Error() string {
	return fmt.Sprintf("the state masker must be reversible: %q cannot recover the values it masks", e.Masker)
}

func (e *MaskNotReversibleError) Unwrap() error { return ErrMaskNotReversible }
