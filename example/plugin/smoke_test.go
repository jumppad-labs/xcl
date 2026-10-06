package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// buildExample builds the example program into a temporary directory and
// returns the path of the binary
func buildExample(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "plugin")

	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = "."

	output, err := build.CombinedOutput()
	require.NoError(t, err, string(output))

	return binary
}

// runExample runs the example binary with args from this directory, adding
// env to the environment, and returns its stdout, stderr and error
func runExample(t *testing.T, binary string, env []string, args ...string) (string, string, error) {
	t.Helper()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	run := exec.Command(binary, args...)
	run.Dir = "."
	run.Env = append(os.Environ(), env...)
	run.Stdout = stdout
	run.Stderr = stderr

	err := run.Run()

	return stdout.String(), stderr.String(), err
}

func TestPluginExampleSmokeRunsWithDefaultArguments(t *testing.T) {
	requireDocker(t)

	// the example's default Docker plugin path is ./build/docker-plugin
	buildPlugin := exec.Command("go", "build", "-o", "build/docker-plugin", "./cmd/docker-plugin")
	buildPlugin.Dir = "."

	output, err := buildPlugin.CombinedOutput()
	require.NoError(t, err, string(output))

	binary := buildExample(t)
	outputDir := t.TempDir()

	stdout, stderr, err := runExample(t, binary, []string{"HCL_VAR_output_dir=" + outputDir})
	require.NoError(t, err, "stdout:\n%s\nstderr:\n%s", stdout, stderr)

	require.Contains(t, stdout, "## Networks")
	require.Contains(t, stdout, "docker.network.app")
	require.Contains(t, stdout, "## Containers")
	require.Contains(t, stdout, "docker.container.web")
	require.Contains(t, stdout, "## Templates")
	require.Contains(t, stdout, "template.welcome")
	require.Contains(t, stdout, "## Destroyed")
	require.Contains(t, stdout, "0 resources remaining")
	require.NotContains(t, stderr, "error:")
}

func TestPluginExampleSmokeFailsForMissingPlugin(t *testing.T) {
	// the example checks for a Docker engine before it loads any plugin
	requireDocker(t)

	binary := buildExample(t)
	outputDir := t.TempDir()

	_, stderr, err := runExample(t, binary, []string{"HCL_VAR_output_dir=" + outputDir}, "./config", "/nonexistent/docker-plugin")
	require.Error(t, err)
	require.Contains(t, stderr, "error:")
	require.Contains(t, stderr, "make build")
}

func TestPluginExampleSmokeFailsWithoutDocker(t *testing.T) {
	binary := buildExample(t)
	socket := "unix://" + filepath.Join(t.TempDir(), "none.sock")

	_, stderr, err := runExample(t, binary, []string{"DOCKER_HOST=" + socket})
	require.Error(t, err)
	require.Contains(t, stderr, "no Docker engine reachable")
}
