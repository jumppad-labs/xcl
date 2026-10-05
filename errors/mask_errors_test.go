package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// UnrecoverableError. A masked value that cannot be read back has to say
// where it was and which masker produced it, so a caller knows which key or
// masker to look for.

func TestUnrecoverableErrorMatchesErrUnrecoverable(t *testing.T) {
	err := &UnrecoverableError{ID: "resource.database.main", MaskedBy: "aes-256-gcm", Reason: "it does not open"}

	require.ErrorIs(t, err, ErrUnrecoverable)
}

func TestUnrecoverableErrorDetailIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("reading state: %w", &UnrecoverableError{ID: "resource.database.main", MaskedBy: "aes-256-gcm"})

	var detail *UnrecoverableError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, "resource.database.main", detail.ID)
	require.Equal(t, "aes-256-gcm", detail.MaskedBy)
}

func TestUnrecoverableErrorMessageNamesTheIDAndTheMasker(t *testing.T) {
	err := &UnrecoverableError{ID: "resource.database.main", MaskedBy: "aes-256-gcm", Reason: "it does not open"}

	require.Equal(t, `masked value in "resource.database.main", masked by "aes-256-gcm", cannot be recovered: it does not open`, err.Error())
	require.Contains(t, err.Error(), "resource.database.main")
	require.Contains(t, err.Error(), "aes-256-gcm")
}

// MaskNotReversibleError. A one-way masker configured for state is named, so
// the caller can see which masker to replace.

func TestMaskNotReversibleErrorMessageNamesTheMasker(t *testing.T) {
	err := &MaskNotReversibleError{Masker: "hmac-sha256"}

	require.Equal(t, `the state masker must be reversible: "hmac-sha256" cannot recover the values it masks`, err.Error())
}

func TestMaskNotReversibleErrorMatchesErrMaskNotReversible(t *testing.T) {
	err := &MaskNotReversibleError{Masker: "hmac-sha256"}

	require.ErrorIs(t, err, ErrMaskNotReversible)
}
