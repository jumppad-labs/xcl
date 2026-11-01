package xcl

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/registry"
)

// A local external plugin can be registered as a command, a program and its
// arguments, rather than a single binary. These tests start the fixture
// widget plugin through the launcher fixture, a program that runs the plugin
// named by its argument as an interpreter runs a script.

// buildLauncher builds the launcher fixture and returns its path. It builds
// with the test's own HOME, so call it before isolateHome.
func buildLauncher(t *testing.T) string {
	t.Helper()

	return testutil.BuildProgram(t, "./internal/test_fixtures/plugins/launcher")
}

func TestApplyWithPluginStartedFromACommandStoresItsResource(t *testing.T) {
	plugin := testutil.BuildFixturePlugin(t, ".")
	launcher := buildLauncher(t)
	isolateHome(t)

	local := registry.NewLocal()
	local.RegisterExternalPlugin(launcher, plugin)
	c := newGitHubConfig(t, t.TempDir(), local)

	err := c.Apply(subtypelessFixture(t, "github"))
	require.NoError(t, err)

	widget, err := Find[subtypelessWidget](c, "widget.app")
	require.NoError(t, err)

	// the plugin computes part_names, so this proves the plugin started
	// through the launcher created the resource
	require.Equal(t, "a", widget.PartNames)
}

func TestDestroyAfterApplyWithPluginStartedFromACommandSucceeds(t *testing.T) {
	plugin := testutil.BuildFixturePlugin(t, ".")
	launcher := buildLauncher(t)
	isolateHome(t)

	local := registry.NewLocal()
	local.RegisterExternalPlugin(launcher, plugin)
	c := newGitHubConfig(t, t.TempDir(), local)

	err := c.Apply(subtypelessFixture(t, "github"))
	require.NoError(t, err)

	err = c.Destroy()
	require.NoError(t, err)

	require.Equal(t, 0, c.ResourceCount())

	_, err = Find[subtypelessWidget](c, "widget.app")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestPluginStartedFromACommandIsNamedByItsCommandLineInLoadEvents(t *testing.T) {
	plugin := testutil.BuildFixturePlugin(t, ".")
	launcher := buildLauncher(t)
	isolateHome(t)

	local := registry.NewLocal()
	local.RegisterExternalPlugin(launcher, plugin)

	recorder := &eventRecorder{}

	c, err := NewConfig(WithRegistry(local), WithEventHandler(recorder.Record))
	require.NoError(t, err)
	stopPluginHosts(t, c)

	err = c.Validate(writeConfigFile(t, variableOnlyConfig))
	require.NoError(t, err)

	require.Equal(t, [][2]any{{launcher + " " + plugin, "local"}}, loadStarts(recorder))
}

func TestPluginFromACommandThatFailsIsNamedByItsCommandLineInTheError(t *testing.T) {
	isolateHome(t)

	local := registry.NewLocal()
	local.RegisterExternalPlugin(missingPluginPath, "plugin.py")

	c, err := NewConfig(WithRegistry(local))
	require.NoError(t, err)

	err = c.Apply(writeConfigFile(t, variableOnlyConfig))
	require.ErrorIs(t, err, ErrPluginLoad)

	var loadErr *PluginLoadError
	require.True(t, errors.As(err, &loadErr))
	require.Equal(t, missingPluginPath+" plugin.py", loadErr.Plugin)
}
