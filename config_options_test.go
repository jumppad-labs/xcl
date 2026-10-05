package xcl

import (
	"os"
	"path/filepath"
	"testing"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/stretchr/testify/require"
)

func TestNewConfigWithNoOptions(t *testing.T) {
	cfg, err := NewConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.pluginRegistry)
	require.Nil(t, cfg.stateStore)
	require.NotNil(t, cfg.variables)
	require.Equal(t, 0, len(cfg.variables))
}

func TestNewConfigWithPluginRegistry(t *testing.T) {
	pr := registry.NewPluginRegistry()
	cfg, err := NewConfig(WithPluginRegistry(pr))
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, pr, cfg.pluginRegistry)
}

func TestNewConfigWithVariables(t *testing.T) {
	vars := map[string]any{"foo": "bar", "count": 42}
	cfg, err := NewConfig(WithVariables(vars))
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, vars, cfg.variables)
	require.Equal(t, "bar", cfg.variables["foo"])
	require.Equal(t, 42, cfg.variables["count"])
}

func TestNewConfigWithMultipleOptions(t *testing.T) {
	pr := registry.NewPluginRegistry()
	vars := map[string]any{"env": "test"}

	cfg, err := NewConfig(
		WithPluginRegistry(pr),
		WithVariables(vars),
	)
	require.NoError(t, err)

	require.NotNil(t, cfg)
	require.Equal(t, pr, cfg.pluginRegistry)
	require.Equal(t, vars, cfg.variables)
}

func TestNewConfigOptionsAreComposable(t *testing.T) {
	pr := registry.NewPluginRegistry()
	vars := map[string]any{"env": "test"}

	opts := []ConfigOption{
		WithPluginRegistry(pr),
		WithVariables(vars),
	}

	cfg, err := NewConfig(opts...)
	require.NoError(t, err)

	require.NotNil(t, cfg)
	require.Equal(t, pr, cfg.pluginRegistry)
	require.Equal(t, vars, cfg.variables)
}

func TestNewConfigWithStatePathCreatesFileStateStore(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")

	cfg, err := NewConfig(WithStatePath(dir))
	require.NoError(t, err)

	store, ok := cfg.stateStore.(*state.FileStateStore)
	require.True(t, ok)
	require.Equal(t, filepath.Join(dir, state.StateFileName), store.Path())
	require.FileExists(t, store.Path())
}

func TestNewConfigWithStatePathReturnsErrorWhenStoreCannotBeCreated(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "file")
	err := os.WriteFile(notADir, []byte{}, 0644)
	require.NoError(t, err)

	cfg, err := NewConfig(WithStatePath(filepath.Join(notADir, "state")))

	require.ErrorContains(t, err, "failed to create state store")
	require.Nil(t, cfg)
}

func TestNewConfigWithStateStoreAfterStatePathUsesStateStore(t *testing.T) {
	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	cfg, err := NewConfig(
		WithStatePath(t.TempDir()),
		WithStateStore(store),
	)
	require.NoError(t, err)

	require.Equal(t, store, cfg.stateStore)
}

func TestNewConfigWithNilStateMaskFails(t *testing.T) {
	cfg, err := NewConfig(WithStateMask(nil))

	require.ErrorIs(t, err, xclerrors.ErrMaskNotReversible)
	require.Nil(t, cfg)
}
