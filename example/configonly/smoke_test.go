package main

import (
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/stretchr/testify/require"
)

// TestConfigOnlyExampleSmokeRunsAgainstItsConfig builds the example and runs
// it the way `make run` does, exercising main rather than run
func TestConfigOnlyExampleSmokeRunsAgainstItsConfig(t *testing.T) {
	run := testutil.RunProgram(t, testutil.BuildProgram(t, "."), configDir)

	require.NoError(t, run.Err, "stderr: %s", run.Stderr)
	require.Contains(t, run.Stdout, "## Resources")
	require.Contains(t, run.Stdout, "## Deployments")
	require.Contains(t, run.Stdout, "deployment.api")
	require.NotContains(t, run.Stderr, "error:")
}

func TestConfigOnlyExampleSmokeFailsForMissingConfig(t *testing.T) {
	run := testutil.RunProgram(t, testutil.BuildProgram(t, "."), filepath.Join(t.TempDir(), "missing"))

	require.Error(t, run.Err)
	require.Contains(t, run.Stderr, "error:")
}
