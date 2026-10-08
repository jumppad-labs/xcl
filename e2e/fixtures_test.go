package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/internal/testutil"
)

func TestKubeFixtureApplies(t *testing.T) {
	t.Setenv("DB_PASSWORD", testPassword)

	c := newKubeConfig(t, nil, t.TempDir(), testStateKey)

	require.NoError(t, c.Apply(kubeConfigDir))

	_, err := testutil.EntityByID(c.Entities(), "deployment.api")
	require.NoError(t, err)
}

func TestPluginFixtureApplies(t *testing.T) {
	c := newPluginConfig(t, nil, t.TempDir(), testStateKey)

	require.NoError(t, c.Apply(pluginConfigDir))

	_, err := testutil.EntityByID(c.Entities(), "resource.app.web")
	require.NoError(t, err)
}
