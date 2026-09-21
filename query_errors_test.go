package xcl

import (
	"errors"
	"fmt"
	"testing"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/state"
	"github.com/stretchr/testify/require"
)

// A not-found is one condition regardless of where it was raised. The lookup
// surface raises it when no entity is declared at an address, and the state
// layer raises its own type when saved state holds no such record: a caller
// handling "it isn't there" should not need to know which.

func TestNotFoundFromTheLookupSurfaceMatchesErrNotFound(t *testing.T) {
	err := &NotFoundError{Address: "resource.container.web"}

	require.ErrorIs(t, err, ErrNotFound)
}

func TestNotFoundFromSavedStateMatchesErrNotFound(t *testing.T) {
	err := state.ResourceNotFoundError{Resource: "resource.container.web"}

	require.ErrorIs(t, err, ErrNotFound)
}

func TestNotFoundFromSavedStateMatchesErrNotFoundThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("loading state: %w", state.ResourceNotFoundError{Resource: "resource.container.web"})

	require.ErrorIs(t, wrapped, ErrNotFound)
}

// The infrastructure-gone condition shares the words "not found" and nothing
// else. Matching it against the lookup surface's condition, in either
// direction, would make a missing declaration and a deleted machine
// indistinguishable.

func TestPluginNotFoundDoesNotMatchErrNotFound(t *testing.T) {
	require.NotErrorIs(t, plugins.ErrNotFound, ErrNotFound)
}

func TestLookupNotFoundDoesNotMatchPluginNotFound(t *testing.T) {
	err := &NotFoundError{Address: "resource.container.web"}

	require.NotErrorIs(t, err, plugins.ErrNotFound)
}

func TestStateNotFoundDoesNotMatchPluginNotFound(t *testing.T) {
	err := state.ResourceNotFoundError{Resource: "resource.container.web"}

	require.NotErrorIs(t, err, plugins.ErrNotFound)
}

// The re-exports exist so a caller can match and recover without importing a
// second package named errors beside the standard library's.

func TestReExportedSentinelsAreTheSameValuesAsTheErrorsPackage(t *testing.T) {
	require.Same(t, xclerrors.ErrNotFound, ErrNotFound)
	require.Same(t, xclerrors.ErrUnknownType, ErrUnknownType)
	require.Same(t, xclerrors.ErrNotTypeable, ErrNotTypeable)
	require.Same(t, xclerrors.ErrNotRegistered, ErrNotRegistered)
	require.Same(t, xclerrors.ErrTypeMismatch, ErrTypeMismatch)
	require.Same(t, xclerrors.ErrNotAnEntity, ErrNotAnEntity)
	require.Same(t, xclerrors.ErrNotUnique, ErrNotUnique)
}

func TestReExportedDetailTypesMatchTheReExportedSentinels(t *testing.T) {
	require.ErrorIs(t, &NotFoundError{Address: "a"}, ErrNotFound)
	require.ErrorIs(t, &UnknownTypeError{Name: "widget"}, ErrUnknownType)
	require.ErrorIs(t, &NotTypeableError{Segments: []string{"resource"}}, ErrNotTypeable)
	require.ErrorIs(t, &NotRegisteredError{}, ErrNotRegistered)
	require.ErrorIs(t, &TypeMismatchError{Address: "a"}, ErrTypeMismatch)
	require.ErrorIs(t, &NotAnEntityError{}, ErrNotAnEntity)
	require.ErrorIs(t, &NotUniqueError{Count: 2}, ErrNotUnique)
}

func TestReExportedDetailTypesAreRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("running query: %w", &NotUniqueError{
		Segments: []string{"resource", "container"},
		Count:    4,
	})

	var detail *NotUniqueError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, 4, detail.Count)
}
