package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/e2e/fixtures/inprocess"
	"github.com/jumppad-labs/xcl/plugins/registry"
)

// TestMissingExternalPluginFailsApply asserts a missing external plugin
// binary fails the first Apply. Registering the path only records it, the
// plugin is started by the Apply, which fails with ErrPluginLoad naming the
// path
func TestMissingExternalPluginFailsApply(t *testing.T) {
	missing := "./does-not-exist"

	r := registry.NewPluginRegistry()
	t.Cleanup(func() {
		for _, host := range r.GetPluginHosts() {
			host.Stop()
		}
	})

	require.NoError(t, r.RegisterPlugin(&inprocess.Plugin{}))
	require.NoError(t, r.RegisterPluginWithPath(missing))

	c := newConfig(t, r, nil, t.TempDir(), testStateKey)

	err := c.Apply(pluginConfigDir)
	require.Error(t, err)
	require.ErrorIs(t, err, xcl.ErrPluginLoad)
	require.Contains(t, err.Error(), missing)
}
