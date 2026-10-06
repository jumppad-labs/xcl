package main

import (
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/stretchr/testify/require"
)

// TestPluginExampleSmokeRunsAgainstItsConfig builds the example and runs it
// the way `make run` does, against the external plugin TestMain built,
// exercising main rather than run
func TestPluginExampleSmokeRunsAgainstItsConfig(t *testing.T) {
	run := testutil.RunProgram(t, testutil.BuildProgram(t, "."), configDir, externalPlugin)

	require.NoError(t, run.Err, "stderr: %s", run.Stderr)
	require.Contains(t, run.Stdout, "## Resources")
	require.Contains(t, run.Stdout, "## Databases")
	require.Contains(t, run.Stdout, "## Caches")
	require.Contains(t, run.Stdout, "resource.postgres.main")
	require.NotContains(t, run.Stderr, "error:")
}

func TestPluginExampleSmokeFailsForMissingExternalPlugin(t *testing.T) {
	run := testutil.RunProgram(t, testutil.BuildProgram(t, "."), configDir, filepath.Join(t.TempDir(), "missing"))

	require.Error(t, run.Err)
	require.Contains(t, run.Stderr, "error:")
}
