package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// smokeRun is what one run of the built example produced
type smokeRun struct {
	stdout string
	stderr string
	err    error
}

// buildExample compiles this example into a temporary directory and returns
// the path of the binary, so a smoke test exercises main exactly as a user
// running the example would
func buildExample(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "configonly")

	output, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput()
	require.NoError(t, err, "unable to build the example: %s", output)

	return binary
}

// runExample runs binary with args and returns what it wrote and how it exited
func runExample(binary string, args ...string) smokeRun {
	var stdout, stderr bytes.Buffer

	command := exec.Command(binary, args...)
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()

	return smokeRun{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

// TestConfigOnlyExampleSmokeRunsAgainstItsConfig builds the example and runs
// it the way `make run` does, exercising main rather than run
func TestConfigOnlyExampleSmokeRunsAgainstItsConfig(t *testing.T) {
	run := runExample(buildExample(t), configDir)

	require.NoError(t, run.err, "stderr: %s", run.stderr)
	require.Contains(t, run.stdout, "## Resources")
	require.Contains(t, run.stdout, "## Deployments")
	require.Contains(t, run.stdout, "deployment.api")
	require.NotContains(t, run.stderr, "error:")
}

func TestConfigOnlyExampleSmokeFailsForMissingConfig(t *testing.T) {
	run := runExample(buildExample(t), filepath.Join(t.TempDir(), "missing"))

	require.Error(t, run.err)
	require.Contains(t, run.stderr, "error:")
}
