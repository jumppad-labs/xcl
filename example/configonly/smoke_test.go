package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// configDir is the configuration this example parses
const configDir = "./config"

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

// runExample runs binary with args and returns what it wrote and how it
// exited. DB_PASSWORD is set the same way the Makefile sets it, because the
// configuration reads it with env("DB_PASSWORD")
func runExample(binary string, args ...string) smokeRun {
	var stdout, stderr bytes.Buffer

	command := exec.Command(binary, args...)
	command.Env = append(os.Environ(), "DB_PASSWORD=example-password")
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()

	return smokeRun{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

// TestConfigOnlyExampleSmokeRunsAgainstItsConfig builds the example and runs
// it the way `make run` does, checking the derived ingress route it prints
func TestConfigOnlyExampleSmokeRunsAgainstItsConfig(t *testing.T) {
	run := runExample(buildExample(t), configDir)

	require.NoError(t, run.err, "stderr: %s", run.stderr)
	require.Contains(t, run.stdout, "api.example.com/ -> service.api:80 -> deployment.api container api port http (8080)")
	require.NotContains(t, run.stderr, "error:")
}

func TestConfigOnlyExampleSmokeFailsForMissingConfig(t *testing.T) {
	run := runExample(buildExample(t), filepath.Join(t.TempDir(), "missing"))

	require.Error(t, run.err)
	require.Contains(t, run.stderr, "error:")
}
