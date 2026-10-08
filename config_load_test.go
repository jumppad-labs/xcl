package xcl

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/state"
)

// newLoadingConfig returns a Config reading the state in store, with its own
// registry and TestPlugin, as a second run of a program would build it
func newLoadingConfig(t *testing.T, store state.StateStore, opts ...ConfigOption) (*Config, *parser.TestPlugin) {
	t.Helper()

	local := registry.NewLocal()

	testPlugin := &parser.TestPlugin{}
	local.RegisterPlugin(testPlugin)

	opts = append([]ConfigOption{WithRegistry(local), WithStateStore(store)}, opts...)

	c, err := NewConfig(opts...)
	require.NoError(t, err)

	return c, testPlugin
}

func TestConfigLoadReadsTheSavedStateIntoANewConfig(t *testing.T) {
	f := setupDestroyConfig(t, logger.Nop())
	applyDestroyFixture(t, f)

	c, _ := newLoadingConfig(t, f.store)

	err := c.Load()
	require.NoError(t, err)

	require.Equal(t, f.config.EntityCount(), c.EntityCount())

	_, err = c.FindResource("resource.network.first")
	require.NoError(t, err)

	_, err = c.FindResource("resource.container.third")
	require.NoError(t, err)
}

func TestConfigLoadNeedsNoConfiguration(t *testing.T) {
	f := setupDestroyConfig(t, logger.Nop())
	applyDestroyFixture(t, f)

	err := os.RemoveAll(f.configDir)
	require.NoError(t, err)

	c, _ := newLoadingConfig(t, f.store)

	err = c.Load()
	require.NoError(t, err)

	_, err = c.FindResource("resource.container.second")
	require.NoError(t, err)
}

func TestConfigLoadCallsNoProvider(t *testing.T) {
	f := setupDestroyConfig(t, logger.Nop())
	applyDestroyFixture(t, f)

	c, testPlugin := newLoadingConfig(t, f.store)

	err := c.Load()
	require.NoError(t, err)

	require.Empty(t, testPlugin.GetCalls())
}

func TestConfigLoadLeavesTheSavedStateUnchanged(t *testing.T) {
	f := setupDestroyConfig(t, logger.Nop())
	applyDestroyFixture(t, f)

	before, err := os.ReadFile(f.statePath)
	require.NoError(t, err)

	c, _ := newLoadingConfig(t, f.store)

	err = c.Load()
	require.NoError(t, err)

	after, err := os.ReadFile(f.statePath)
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestConfigLoadWithNothingSavedHoldsNothing(t *testing.T) {
	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	c, _ := newLoadingConfig(t, store)

	err = c.Load()
	require.NoError(t, err)

	require.Equal(t, 0, c.EntityCount())
}

func TestConfigLoadWithNoStateStoreSucceeds(t *testing.T) {
	c, err := NewConfig()
	require.NoError(t, err)

	err = c.Load()
	require.NoError(t, err)

	require.Equal(t, 0, c.EntityCount())
}

func TestConfigLoadReportsStartThenSuccess(t *testing.T) {
	f := setupDestroyConfig(t, logger.Nop())
	applyDestroyFixture(t, f)

	recorder := &eventRecorder{}
	c, _ := newLoadingConfig(t, f.store, WithEventHandler(recorder.Record))

	err := c.Load()
	require.NoError(t, err)

	require.Len(t, recorder.find("", "load_state", "start"), 1)
	require.Len(t, recorder.find("", "load_state", "success"), 1)
	require.Less(t, eventIndex(recorder, "", "load_state", "start"), eventIndex(recorder, "", "load_state", "success"))
}

func TestConfigLoadFailsForUnreadableState(t *testing.T) {
	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	err = os.WriteFile(store.Path(), []byte("not state"), 0644)
	require.NoError(t, err)

	c, _ := newLoadingConfig(t, store)

	err = c.Load()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to load state")
}
