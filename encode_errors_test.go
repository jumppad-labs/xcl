package xcl

import (
	"errors"
	"fmt"
	"testing"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/savedentity"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/stretchr/testify/require"
)

// The re-exports exist so an application can match a failure to read saved
// data, or to write configuration, without importing a second package named
// errors beside the standard library's.

func TestReExportedEncodingSentinelsAreTheSameValuesAsTheErrorsPackage(t *testing.T) {
	require.Same(t, xclerrors.ErrUnregisteredType, ErrUnregisteredType)
	require.Same(t, xclerrors.ErrInvalidSavedData, ErrInvalidSavedData)
	require.Same(t, xclerrors.ErrNotEncodable, ErrNotEncodable)
}

func TestReExportedEncodingDetailTypesMatchTheReExportedSentinels(t *testing.T) {
	require.ErrorIs(t, &UnregisteredTypeError{Type: "widget"}, ErrUnregisteredType)
	require.ErrorIs(t, &InvalidSavedDataError{ID: "resource.database.main"}, ErrInvalidSavedData)
	require.ErrorIs(t, &NotEncodableError{What: "variable.environment"}, ErrNotEncodable)
}

func TestReExportedUnregisteredTypeErrorIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("reading saved record: %w", &UnregisteredTypeError{Type: "widget"})

	var detail *UnregisteredTypeError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, "widget", detail.Type)
}

func TestReExportedInvalidSavedDataErrorIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("loading state: %w", &InvalidSavedDataError{ID: "resource.database.main"})

	var detail *InvalidSavedDataError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, "resource.database.main", detail.ID)
}

func TestReExportedNotEncodableErrorIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("writing configuration: %w", &NotEncodableError{
		What:   "variable.environment",
		Reason: "builtins have no configuration form",
	})

	var detail *NotEncodableError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, "variable.environment", detail.What)
	require.Equal(t, "builtins have no configuration form", detail.Reason)
}

// The two tests below match a failure the library actually raised, rather than
// one the test built, which is what the re-exports are for: the reader returns
// the errors package's type and the application names the root package's.
//
// The records are hand written because these are the format's own failures: a
// type nobody registered, and bytes that are not a record at all, cannot be
// produced by an apply.

func TestUnregisteredTypeRaisedByTheLibraryMatchesTheRootPackageSentinel(t *testing.T) {
	reg := registry.NewPluginRegistry()

	record := []byte(`{"meta":{"id":"resource.widget.main","name":"main","type":"resource","subtype":"widget"}}`)

	_, err := savedentity.Decode(reg, record)
	require.Error(t, err)

	require.ErrorIs(t, err, ErrUnregisteredType)

	var detail *UnregisteredTypeError
	require.True(t, errors.As(err, &detail))
	require.Equal(t, "widget", detail.Type)
}

func TestInvalidSavedDataRaisedByTheLibraryMatchesTheRootPackageSentinel(t *testing.T) {
	reg := registry.NewPluginRegistry()

	record := []byte("this is not json")

	_, err := savedentity.Decode(reg, record)
	require.Error(t, err)

	require.ErrorIs(t, err, ErrInvalidSavedData)

	var detail *InvalidSavedDataError
	require.True(t, errors.As(err, &detail))
	require.Empty(t, detail.ID)
}
