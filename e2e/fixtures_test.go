package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/plugins/registry"
)

func TestKubeFixtureApplies(t *testing.T) {
	t.Setenv("DB_PASSWORD", testPassword)

	r := registry.NewPluginRegistry()
	c := newKubeConfig(t, r, nil, t.TempDir(), testStateKey)

	require.NoError(t, c.Apply(kubeConfigDir))

	_, err := testutil.EntityByID(c.Entities(), "deployment.api")
	require.NoError(t, err)
}

func TestPluginFixtureApplies(t *testing.T) {
	r := registry.NewPluginRegistry()
	c := newPluginConfig(t, r, nil, t.TempDir(), testStateKey)

	require.NoError(t, c.Apply(pluginConfigDir))

	_, err := testutil.EntityByID(c.Entities(), "resource.app.web")
	require.NoError(t, err)
}
