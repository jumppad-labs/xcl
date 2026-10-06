package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/plugins/registry"
)

// TestKubeLifecycleWithoutEventHandlerWritesNothingToStandardStreams asserts
// xcl writes nothing of its own to stdout or stderr across an apply, decode
// and destroy of registered types when no event handler is given
func TestKubeLifecycleWithoutEventHandlerWritesNothingToStandardStreams(t *testing.T) {
	t.Setenv("DB_PASSWORD", testPassword)

	c := newKubeConfig(t, registry.NewPluginRegistry(), nil, t.TempDir(), testStateKey)

	var applyErr, decodeErr, destroyErr error
	captured := testutil.CaptureStandardStreams(t, func() {
		applyErr = c.Apply(kubeConfigDir)

		var cfg kubeAppConfig
		decodeErr = c.Decode(&cfg)

		destroyErr = c.Destroy()
	})

	require.NoError(t, applyErr)
	require.NoError(t, decodeErr)
	require.NoError(t, destroyErr)
	require.Empty(t, captured.Stdout)
	require.Empty(t, captured.Stderr)
}
