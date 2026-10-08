package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/diff"
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

func TestPostgresLocationChangePlansReplace(t *testing.T) {
	scenario := newPluginDiffScenario(t)
	scenario.apply()

	scenario.edit("main.xcl", `location = "replica.localhost"`, `location = "replica2.localhost"`)

	found, _ := scenario.diff()

	require.Equal(t, diff.ActionReplace, diffResource(t, found, "resource.postgres.replica").Action)
}

func TestPostgresOtherChangePlansUpdate(t *testing.T) {
	scenario := newPluginDiffScenario(t)
	scenario.apply()

	scenario.edit("main.xcl", "port     = 5433", "port     = 5434")

	found, _ := scenario.diff()

	require.Equal(t, diff.ActionUpdate, diffResource(t, found, "resource.postgres.replica").Action)
}

func TestIngressHostnameChangePlansReplace(t *testing.T) {
	scenario := newPluginDiffScenario(t)
	scenario.apply()

	scenario.edit("main.xcl", `hostname = "example.com"`, `hostname = "example.org"`)

	found, _ := scenario.diff()

	require.Equal(t, diff.ActionReplace, diffResource(t, found, "resource.ingress.web").Action)
}

func TestIngressOtherChangePlansUpdate(t *testing.T) {
	scenario := newPluginDiffScenario(t)
	scenario.apply()

	scenario.edit("main.xcl", "app_url  = resource.app.web.url", `app_url  = "http://other"`)

	found, _ := scenario.diff()

	require.Equal(t, diff.ActionUpdate, diffResource(t, found, "resource.ingress.web").Action)
}
