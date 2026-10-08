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
