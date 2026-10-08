package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TypeNameClashError. A block type provided twice has no precedence between
// its providers, so the message has to name the type and both sides of the
// clash, with the registry each came from, for a user to decide which to drop.

func TestTypeNameClashErrorMessageNamesTheTypeBothProvidersAndBothRegistries(t *testing.T) {
	err := &TypeNameClashError{
		Name:             "resource.postgres",
		Provider:         "postgres-ng",
		Registry:         "community",
		Existing:         "postgres",
		ExistingRegistry: "default",
	}

	require.Equal(t, `type "resource.postgres" is provided by both postgres (registry default) and postgres-ng (registry community)`, err.Error())
}

func TestTypeNameClashErrorMessageForAClashWithABuiltinNamesTheBuiltin(t *testing.T) {
	err := &TypeNameClashError{
		Name:     "variable",
		Provider: "vars",
		Registry: "default",
		Existing: "builtin",
	}

	require.Equal(t, `type "variable" is provided by both builtin and vars (registry default)`, err.Error())
}

func TestTypeNameClashErrorMessageForAClashWithADeclaredType(t *testing.T) {
	err := &TypeNameClashError{
		Name:             "resource.postgres",
		Provider:         "type *foo.Bar",
		Existing:         "postgres",
		ExistingRegistry: "default",
	}

	require.Equal(t, `type "resource.postgres" is provided by both postgres (registry default) and type *foo.Bar`, err.Error())
}

func TestTypeNameClashErrorMessageWithoutAProviderNamesWhatAlreadyProvidesTheType(t *testing.T) {
	err := &TypeNameClashError{
		Name:     "variable",
		Existing: "builtin",
	}

	require.Equal(t, `type "variable" is already provided by builtin`, err.Error())
}

func TestTypeNameClashErrorDetailIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("loading plugins: %w", &TypeNameClashError{
		Name:             "resource.postgres",
		Provider:         "postgres-ng",
		Registry:         "community",
		Existing:         "postgres",
		ExistingRegistry: "default",
	})

	var detail *TypeNameClashError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, "resource.postgres", detail.Name)
	require.Equal(t, "postgres-ng", detail.Provider)
	require.Equal(t, "community", detail.Registry)
	require.Equal(t, "postgres", detail.Existing)
	require.Equal(t, "default", detail.ExistingRegistry)
}

// TypeFormError. A type keyword is used in one form only, so the message has
// to show the form the keyword already has.

func TestTypeFormErrorMessageForATypeThatTakesASubtype(t *testing.T) {
	err := &TypeFormError{Type: "server", TakesSubtype: true}

	require.Equal(t, `type "server" takes a subtype, i.e. 'server "<subtype>" "<name>" {}', so it must be registered with one`, err.Error())
}

func TestTypeFormErrorMessageForATypeWithoutASubtype(t *testing.T) {
	err := &TypeFormError{Type: "server"}

	require.Equal(t, `type "server" is declared without a subtype, i.e. 'server "<name>" {}', so it can not be registered with one`, err.Error())
}

func TestTypeFormErrorDetailIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("registering type: %w", &TypeFormError{Type: "server", TakesSubtype: true})

	var detail *TypeFormError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, "server", detail.Type)
	require.True(t, detail.TakesSubtype)
}
