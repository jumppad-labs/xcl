package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// encodeErrCause stands in for the reason a record could not be read, or a
// value could not be written, wherever a test needs a cause of its own to
// find again through the detail that carries it.
var encodeErrCause = errors.New("the underlying reason")

// UnregisteredTypeError. The type the registry does not know is the whole
// point of the error: it is what a caller has to register, or find a plugin
// for, before the record will read.

func TestUnregisteredTypeErrorMessageNamesTheType(t *testing.T) {
	err := &UnregisteredTypeError{Type: "widget"}

	require.Equal(t, `type "widget" is not known to the registry`, err.Error())
}

func TestUnregisteredTypeErrorMatchesErrUnregisteredType(t *testing.T) {
	err := &UnregisteredTypeError{Type: "widget"}

	require.ErrorIs(t, err, ErrUnregisteredType)
}

func TestUnregisteredTypeErrorMatchesErrUnregisteredTypeThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("reading saved record: %w", &UnregisteredTypeError{Type: "widget"})

	require.ErrorIs(t, wrapped, ErrUnregisteredType)
}

func TestUnregisteredTypeErrorDetailIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("reading saved record: %w", &UnregisteredTypeError{Type: "widget"})

	var detail *UnregisteredTypeError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, "widget", detail.Type)
}

func TestUnregisteredTypeErrorDoesNotMatchTheOtherEncodingSentinels(t *testing.T) {
	err := &UnregisteredTypeError{Type: "widget"}

	require.NotErrorIs(t, err, ErrInvalidSavedData)
	require.NotErrorIs(t, err, ErrNotEncodable)
}

// InvalidSavedDataError. A record that cannot be read has to name itself where
// it was readable enough to, and carry the reason it failed alongside the
// condition, so a caller can match one and report the other.

func TestInvalidSavedDataErrorMessageNamesTheRecordAndTheReason(t *testing.T) {
	err := &InvalidSavedDataError{
		ID:  "resource.database.main",
		Err: errors.New("record has no name"),
	}

	require.Equal(t, `record "resource.database.main" is not a saved entity: record has no name`, err.Error())
}

func TestInvalidSavedDataErrorMessageWithoutAnIDStillReportsTheReason(t *testing.T) {
	err := &InvalidSavedDataError{Err: errors.New("record has no meta")}

	require.Equal(t, "data is not a saved entity: record has no meta", err.Error())
}

func TestInvalidSavedDataErrorMessageWithNoCauseIsJustTheCondition(t *testing.T) {
	err := &InvalidSavedDataError{ID: "resource.database.main"}

	require.Equal(t, `record "resource.database.main" is not a saved entity`, err.Error())
}

func TestInvalidSavedDataErrorMatchesErrInvalidSavedData(t *testing.T) {
	err := &InvalidSavedDataError{ID: "resource.database.main", Err: encodeErrCause}

	require.ErrorIs(t, err, ErrInvalidSavedData)
}

// The nil cause is dropped from the Unwrap slice rather than passed on as a
// nil entry, so this pins that the sentinel is still reachable without one.
func TestInvalidSavedDataErrorWithNoCauseMatchesErrInvalidSavedData(t *testing.T) {
	err := &InvalidSavedDataError{ID: "resource.database.main"}

	require.ErrorIs(t, err, ErrInvalidSavedData)
}

func TestInvalidSavedDataErrorMatchesItsWrappedCause(t *testing.T) {
	err := &InvalidSavedDataError{ID: "resource.database.main", Err: encodeErrCause}

	require.ErrorIs(t, err, encodeErrCause)
}

func TestInvalidSavedDataErrorMatchesBothTheSentinelAndTheCauseThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("loading state: %w", &InvalidSavedDataError{
		ID:  "resource.database.main",
		Err: encodeErrCause,
	})

	require.ErrorIs(t, wrapped, ErrInvalidSavedData)
	require.ErrorIs(t, wrapped, encodeErrCause)
}

func TestInvalidSavedDataErrorDetailIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("loading state: %w", &InvalidSavedDataError{
		ID:  "resource.database.main",
		Err: encodeErrCause,
	})

	var detail *InvalidSavedDataError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, "resource.database.main", detail.ID)
	require.Same(t, encodeErrCause, detail.Err)
}

func TestInvalidSavedDataErrorDoesNotMatchTheOtherEncodingSentinels(t *testing.T) {
	err := &InvalidSavedDataError{ID: "resource.database.main", Err: encodeErrCause}

	require.NotErrorIs(t, err, ErrUnregisteredType)
	require.NotErrorIs(t, err, ErrNotEncodable)
}

func TestInvalidSavedDataErrorWithNoCauseDoesNotMatchAnUnrelatedError(t *testing.T) {
	err := &InvalidSavedDataError{ID: "resource.database.main"}

	require.NotErrorIs(t, err, encodeErrCause)
}

// NotEncodableError. What could not be written, and why, are separate fields
// so a caller can report both without parsing the message back apart.

func TestNotEncodableErrorMessageNamesWhatAndWhyAndTheCause(t *testing.T) {
	err := &NotEncodableError{
		What:   "variable.environment",
		Reason: "builtins have no configuration form",
		Err:    errors.New("no schema for variable"),
	}

	require.Equal(
		t,
		"variable.environment cannot be encoded as configuration: builtins have no configuration form: no schema for variable",
		err.Error(),
	)
}

func TestNotEncodableErrorMessageWithoutAReasonIsJustTheCondition(t *testing.T) {
	err := &NotEncodableError{What: "variable.environment"}

	require.Equal(t, "variable.environment cannot be encoded as configuration", err.Error())
}

func TestNotEncodableErrorMatchesErrNotEncodable(t *testing.T) {
	err := &NotEncodableError{
		What:   "variable.environment",
		Reason: "builtins have no configuration form",
		Err:    encodeErrCause,
	}

	require.ErrorIs(t, err, ErrNotEncodable)
}

// As with InvalidSavedDataError, the nil cause is left out of the Unwrap slice
// entirely, so the sentinel has to remain reachable without one.
func TestNotEncodableErrorWithNoCauseMatchesErrNotEncodable(t *testing.T) {
	err := &NotEncodableError{
		What:   "variable.environment",
		Reason: "builtins have no configuration form",
	}

	require.ErrorIs(t, err, ErrNotEncodable)
}

func TestNotEncodableErrorMatchesItsWrappedCause(t *testing.T) {
	err := &NotEncodableError{What: "variable.environment", Err: encodeErrCause}

	require.ErrorIs(t, err, encodeErrCause)
}

func TestNotEncodableErrorMatchesBothTheSentinelAndTheCauseThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("writing configuration: %w", &NotEncodableError{
		What:   "variable.environment",
		Reason: "builtins have no configuration form",
		Err:    encodeErrCause,
	})

	require.ErrorIs(t, wrapped, ErrNotEncodable)
	require.ErrorIs(t, wrapped, encodeErrCause)
}

func TestNotEncodableErrorDetailIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("writing configuration: %w", &NotEncodableError{
		What:   "variable.environment",
		Reason: "builtins have no configuration form",
		Err:    encodeErrCause,
	})

	var detail *NotEncodableError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, "variable.environment", detail.What)
	require.Equal(t, "builtins have no configuration form", detail.Reason)
	require.Same(t, encodeErrCause, detail.Err)
}

func TestNotEncodableErrorDoesNotMatchTheOtherEncodingSentinels(t *testing.T) {
	err := &NotEncodableError{
		What:   "variable.environment",
		Reason: "builtins have no configuration form",
		Err:    encodeErrCause,
	}

	require.NotErrorIs(t, err, ErrUnregisteredType)
	require.NotErrorIs(t, err, ErrInvalidSavedData)
}

func TestNotEncodableErrorWithNoCauseDoesNotMatchAnUnrelatedError(t *testing.T) {
	err := &NotEncodableError{
		What:   "variable.environment",
		Reason: "builtins have no configuration form",
	}

	require.NotErrorIs(t, err, encodeErrCause)
}
