package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/plugins/registry"
)

// TestPluginLifecycleWithoutEventHandlerWritesNothingToStandardStreams
// asserts xcl, both plugins and the external plugin process write nothing of
// their own to stdout or stderr across an apply and destroy when no event
// handler is given
func TestPluginLifecycleWithoutEventHandlerWritesNothingToStandardStreams(t *testing.T) {
	c := newPluginConfig(t, registry.NewPluginRegistry(), nil, t.TempDir(), testStateKey)

	var applyErr, destroyErr error
	captured := testutil.CaptureStandardStreams(t, func() {
		applyErr = c.Apply(pluginConfigDir)
		destroyErr = c.Destroy()
	})

	require.NoError(t, applyErr)
	require.NoError(t, destroyErr)
	require.Empty(t, captured.Stdout)
	require.Empty(t, captured.Stderr)
}

// TestPluginStandardStreamsHoldNoSecret asserts xcl, both plugins and the
// external plugin process write nothing holding either password to the
// process's standard streams while an event handler receives every event of
// an apply and destroy
func TestPluginStandardStreamsHoldNoSecret(t *testing.T) {
	recorder := &testutil.EventRecorder{}
	c := newPluginConfig(t, registry.NewPluginRegistry(), recorder.Record, t.TempDir(), testStateKey)

	var applyErr, destroyErr error
	captured := testutil.CaptureStandardStreams(t, func() {
		applyErr = c.Apply(pluginConfigDir)
		destroyErr = c.Destroy()
	})

	require.NoError(t, applyErr)
	require.NoError(t, destroyErr)
	require.NotContains(t, captured.Stdout, pluginStateVariablePassword)
	require.NotContains(t, captured.Stdout, pluginStateModulePassword)
	require.NotContains(t, captured.Stderr, pluginStateVariablePassword)
	require.NotContains(t, captured.Stderr, pluginStateModulePassword)
}
