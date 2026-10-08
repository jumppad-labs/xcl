package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// pluginLoadErrCause stands in for the reason a plugin could not be loaded,
// so a test can find it again through the error that carries it.
var pluginLoadErrCause = errors.New("exec format error")

// PluginLoadError. A plugin that fails to load has to say which plugin it was
// and which registry it came from, because the same plugin name can be offered
// by more than one registry, and still answer for the sentinel and its cause.

func TestPluginLoadErrorMessageNamesThePluginAndItsRegistry(t *testing.T) {
	err := &PluginLoadError{Plugin: "postgres", Registry: "default", Err: pluginLoadErrCause}

	require.Equal(t, "plugin postgres from registry default failed to load: exec format error", err.Error())
}

func TestPluginLoadErrorMessageForARegistryThatFailedToLoadItsPlugins(t *testing.T) {
	err := &PluginLoadError{Registry: "default", Err: pluginLoadErrCause}

	require.Equal(t, "registry default failed to load its plugins: exec format error", err.Error())
}

func TestPluginLoadErrorMessageWithoutARegistryNamesThePlugin(t *testing.T) {
	err := &PluginLoadError{Plugin: "postgres", Err: pluginLoadErrCause}

	require.Equal(t, "plugin postgres failed to load: exec format error", err.Error())
}

func TestPluginLoadErrorMatchesErrPluginLoad(t *testing.T) {
	err := &PluginLoadError{Plugin: "postgres", Registry: "default", Err: pluginLoadErrCause}

	require.ErrorIs(t, err, ErrPluginLoad)
}

func TestPluginLoadErrorMatchesItsCause(t *testing.T) {
	err := &PluginLoadError{Plugin: "postgres", Registry: "default", Err: pluginLoadErrCause}

	require.ErrorIs(t, err, pluginLoadErrCause)
}

func TestPluginLoadErrorDetailIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("validating configuration: %w", &PluginLoadError{Plugin: "postgres", Registry: "default", Err: pluginLoadErrCause})

	var detail *PluginLoadError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, "postgres", detail.Plugin)
	require.Equal(t, "default", detail.Registry)
}
