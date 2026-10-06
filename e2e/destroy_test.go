package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
)

// TestDestroyLeavesSavedStateEmpty asserts the state saved after a destroy is
// empty, everything the apply saved has been destroyed
func TestDestroyLeavesSavedStateEmpty(t *testing.T) {
	t.Setenv("DB_PASSWORD", testPassword)

	stateDir := t.TempDir()
	c := newKubeConfig(t, registry.NewPluginRegistry(), nil, stateDir, testStateKey)

	require.NoError(t, c.Apply(kubeConfigDir))
	require.NoError(t, c.Destroy())

	// the state file is still there, emptied rather than removed
	_, err := os.Stat(filepath.Join(stateDir, state.StateFileName))
	require.NoError(t, err)

	store, err := state.NewFileStateStore(stateDir)
	require.NoError(t, err)

	saved, err := store.Load()
	require.NoError(t, err)
	require.Empty(t, saved)
}
