package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/e2e/fixtures/inprocess"
	"github.com/jumppad-labs/xcl/registry"
)

// TestMissingExternalPluginFailsApply asserts a missing external plugin
// binary fails the first Apply. Registering the path only records it, the
// plugin is started by the Apply, which fails with ErrPluginLoad naming the
// path and the local registry it came from
func TestMissingExternalPluginFailsApply(t *testing.T) {
	missing := "./does-not-exist"

	local := registry.NewLocal()
	local.RegisterPlugin(&inprocess.Plugin{})
	local.RegisterExternalPlugin(missing)

	c := newConfig(t, nil, t.TempDir(), testStateKey, xcl.WithRegistry(local))

	err := c.Apply(pluginConfigDir)
	require.Error(t, err)
	require.ErrorIs(t, err, xcl.ErrPluginLoad)
	require.Contains(t, err.Error(), missing)
	require.Contains(t, err.Error(), "from registry local")
}
