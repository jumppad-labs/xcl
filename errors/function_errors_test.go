package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// FunctionNameClashError. A function name offered twice has no namespace and
// no precedence between its providers, so the message has to name the
// function and both sides of the clash, with the registry each came from,
// for a user to decide which to drop.

func TestFunctionNameClashErrorMessageNamesTheFunctionBothProvidersAndBothRegistries(t *testing.T) {
	err := &FunctionNameClashError{
		Name:             "slugify",
		Provider:         "function func(string) (string, error)",
		Registry:         "second",
		Existing:         "function func(string) (string, error)",
		ExistingRegistry: "first",
	}

	require.Equal(t, `function "slugify" is provided by both function func(string) (string, error) (registry first) and function func(string) (string, error) (registry second)`, err.Error())
}

func TestFunctionNameClashErrorMessageForAClashWithABuiltinNamesTheBuiltin(t *testing.T) {
	err := &FunctionNameClashError{
		Name:     "upper",
		Provider: "function func(string) (string, error)",
		Registry: "local",
		Existing: "builtin",
	}

	require.Equal(t, `function "upper" is provided by both builtin and function func(string) (string, error) (registry local)`, err.Error())
}

func TestFunctionNameClashErrorMessageWithoutAProvider(t *testing.T) {
	err := &FunctionNameClashError{
		Name:     "upper",
		Existing: "builtin",
	}

	require.Equal(t, `function "upper" is already provided by builtin`, err.Error())
}

func TestFunctionNameClashErrorDetailIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("creating config: %w", &FunctionNameClashError{
		Name:             "slugify",
		Provider:         "function func(string) (string, error)",
		Registry:         "second",
		Existing:         "slug-plugin",
		ExistingRegistry: "first",
	})

	var detail *FunctionNameClashError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, "slugify", detail.Name)
	require.Equal(t, "function func(string) (string, error)", detail.Provider)
	require.Equal(t, "second", detail.Registry)
	require.Equal(t, "slug-plugin", detail.Existing)
	require.Equal(t, "first", detail.ExistingRegistry)
}

// InvalidFunctionError. A function configuration cannot call is reported
// with its name, the registry it came from and the problem with its
// signature, so the programmer who registered it can find it.

func TestInvalidFunctionErrorMessageNamesTheFunctionRegistryAndProblem(t *testing.T) {
	err := &InvalidFunctionError{
		Name:     "broken",
		Registry: "local",
		Err:      errors.New("functions must return a result and an error"),
	}

	require.Equal(t, `function "broken" (registry local) cannot be called from configuration: functions must return a result and an error`, err.Error())
}

func TestInvalidFunctionErrorMessageWithoutARegistry(t *testing.T) {
	err := &InvalidFunctionError{
		Name: "broken",
		Err:  errors.New("variadic functions are not supported"),
	}

	require.Equal(t, `function "broken" cannot be called from configuration: variadic functions are not supported`, err.Error())
}

func TestInvalidFunctionErrorUnwrapsTheProblem(t *testing.T) {
	problem := errors.New("parameter 0: type struct {} is not supported")
	wrapped := fmt.Errorf("creating config: %w", &InvalidFunctionError{
		Name:     "broken",
		Registry: "local",
		Err:      problem,
	})

	var invalid *InvalidFunctionError
	require.True(t, errors.As(wrapped, &invalid))
	require.Equal(t, "broken", invalid.Name)
	require.Equal(t, "local", invalid.Registry)
	require.ErrorIs(t, wrapped, problem)
}
