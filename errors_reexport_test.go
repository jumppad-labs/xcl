package xcl

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	xclerrors "github.com/jumppad-labs/xcl/errors"
)

// The errors a plugin load reports are built in the errors package, but a
// caller only imports xcl, so the root package's names have to recover them
// with the standard error-matching functions.

func TestReexportedTypeNameClashErrorIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("validating configuration: %w", &xclerrors.TypeNameClashError{
		Name:             "resource.postgres",
		Provider:         "postgres-ng",
		Registry:         "community",
		Existing:         "postgres",
		ExistingRegistry: "default",
	})

	var clash *TypeNameClashError
	require.True(t, errors.As(wrapped, &clash))
	require.Equal(t, "resource.postgres", clash.Name)
	require.Equal(t, "community", clash.Registry)
	require.Equal(t, "default", clash.ExistingRegistry)
}

func TestReexportedTypeFormErrorIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("validating configuration: %w", &xclerrors.TypeFormError{Type: "server", TakesSubtype: true})

	var form *TypeFormError
	require.True(t, errors.As(wrapped, &form))
	require.Equal(t, "server", form.Type)
	require.True(t, form.TakesSubtype)
}

func TestReexportedPluginLoadErrorIsRecoverableThroughAWrap(t *testing.T) {
	cause := errors.New("exec format error")
	wrapped := fmt.Errorf("validating configuration: %w", &xclerrors.PluginLoadError{Plugin: "postgres", Registry: "default", Err: cause})

	var loadErr *PluginLoadError
	require.True(t, errors.As(wrapped, &loadErr))
	require.Equal(t, "postgres", loadErr.Plugin)
	require.Equal(t, "default", loadErr.Registry)
}

func TestReexportedPluginLoadErrorMatchesErrPluginLoadThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("validating configuration: %w", &xclerrors.PluginLoadError{Plugin: "postgres", Registry: "default", Err: errors.New("exec format error")})

	require.ErrorIs(t, wrapped, ErrPluginLoad)
}

func TestReexportedPluginInstallErrorIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("installing plugins: %w", &xclerrors.PluginInstallError{
		Repository: "o/r",
		Version:    "v1.0.0",
		Platform:   "linux/amd64",
		Err:        fmt.Errorf("no build: %w", xclerrors.ErrPluginNotFound),
	})

	var installErr *PluginInstallError
	require.True(t, errors.As(wrapped, &installErr))
	require.Equal(t, "o/r", installErr.Repository)
	require.Equal(t, "v1.0.0", installErr.Version)
	require.Equal(t, "linux/amd64", installErr.Platform)
}

func TestReexportedErrPluginNotFoundMatchesThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("installing plugins: %w", &xclerrors.PluginInstallError{
		Repository: "o/r",
		Err:        fmt.Errorf("no build: %w", xclerrors.ErrPluginNotFound),
	})

	require.ErrorIs(t, wrapped, ErrPluginNotFound)
}

func TestReexportedErrPluginVerificationMatchesThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("installing plugins: %w", &xclerrors.PluginInstallError{
		Repository: "o/r",
		Err:        fmt.Errorf("checksum mismatch: %w", xclerrors.ErrPluginVerification),
	})

	require.ErrorIs(t, wrapped, ErrPluginVerification)
}

func TestReexportedErrPluginNotFoundDoesNotMatchAVerificationFailure(t *testing.T) {
	wrapped := fmt.Errorf("installing plugins: %w", &xclerrors.PluginInstallError{
		Repository: "o/r",
		Err:        fmt.Errorf("checksum mismatch: %w", xclerrors.ErrPluginVerification),
	})

	require.NotErrorIs(t, wrapped, ErrPluginNotFound)
}

func TestReexportedErrPluginNotFoundMatchesThroughAPluginLoadError(t *testing.T) {
	install := &xclerrors.PluginInstallError{
		Repository: "o/r",
		Err:        fmt.Errorf("no build: %w", xclerrors.ErrPluginNotFound),
	}
	wrapped := fmt.Errorf("validating configuration: %w", &xclerrors.PluginLoadError{Registry: "github", Err: install})

	require.ErrorIs(t, wrapped, ErrPluginNotFound)
}

func TestReexportedInvalidTypeErrorIsRecoverableThroughAWrap(t *testing.T) {
	problem := errors.New("field Listener of main.Postgres cannot hold blocks")
	wrapped := fmt.Errorf("validating configuration: %w", &xclerrors.InvalidTypeError{
		Type:     "resource.postgres",
		Provider: "postgres",
		Registry: "default",
		Err:      problem,
	})

	var invalid *InvalidTypeError
	require.True(t, errors.As(wrapped, &invalid))
	require.Equal(t, "resource.postgres", invalid.Type)
	require.Equal(t, "postgres", invalid.Provider)
	require.Equal(t, "default", invalid.Registry)
	require.ErrorIs(t, wrapped, problem)
}

func TestReexportedFunctionNameClashErrorIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("creating config: %w", &xclerrors.FunctionNameClashError{
		Name:             "slugify",
		Provider:         "function func(string) (string, error)",
		Registry:         "second",
		Existing:         "function func(string) (string, error)",
		ExistingRegistry: "first",
	})

	var clash *FunctionNameClashError
	require.True(t, errors.As(wrapped, &clash))
	require.Equal(t, "slugify", clash.Name)
	require.Equal(t, "function func(string) (string, error)", clash.Provider)
	require.Equal(t, "second", clash.Registry)
	require.Equal(t, "function func(string) (string, error)", clash.Existing)
	require.Equal(t, "first", clash.ExistingRegistry)
}

func TestReexportedInvalidFunctionErrorIsRecoverableThroughAWrap(t *testing.T) {
	problem := errors.New("functions must return a result and an error")
	wrapped := fmt.Errorf("creating config: %w", &xclerrors.InvalidFunctionError{
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
