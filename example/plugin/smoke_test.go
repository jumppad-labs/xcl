package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// buildExample builds xcl-docker and the Docker plugin side by side into a
// temporary directory, as `make build` does, and returns the path of
// xcl-docker, which finds the plugin next to itself
func buildExample(t *testing.T) string {
	t.Helper()

	buildDir := t.TempDir()
	binary := filepath.Join(buildDir, "xcl-docker")

	build := exec.Command("go", "build", "-o", binary, ".")
	output, err := build.CombinedOutput()
	require.NoError(t, err, string(output))

	buildPlugin := exec.Command("go", "build", "-o", filepath.Join(buildDir, dockerPluginName), "./cmd/docker")
	buildPlugin.Dir = filepath.Join("plugins", "docker")
	output, err = buildPlugin.CombinedOutput()
	require.NoError(t, err, string(output))

	return binary
}

// runExample runs the xcl-docker binary with args from this directory, adding
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

func TestSmokeApplyStatusDestroyShareTheSavedState(t *testing.T) {
	requireDocker(t)

	binary := buildExample(t)
	env := []string{"XCL_VAR_output_dir=" + t.TempDir()}
	stateDir := t.TempDir()

	stdout, stderr, err := runExample(t, binary, env, "apply", "--state", stateDir, "./config/full")
	t.Cleanup(func() {
		// remove what apply made even when a later step fails
		runExample(t, binary, env, "destroy", "--state", stateDir)
	})
	require.NoError(t, err, "stdout:\n%s\nstderr:\n%s", stdout, stderr)
	require.Empty(t, stdout, "apply prints nothing of its own")
	require.Contains(t, stderr, "apply success")

	stdout, stderr, err = runExample(t, binary, env, "status", "--state", stateDir)
	require.NoError(t, err, "stdout:\n%s\nstderr:\n%s", stdout, stderr)
	require.Empty(t, stderr, "status prints only its tree")
	require.Contains(t, stdout, "● docker.network.app")
	require.Contains(t, stdout, "└── ● template.init")
	require.Contains(t, stdout, "    └── ● docker.container.web")
	require.Contains(t, stdout, "        └── ● template.welcome")

	stdout, stderr, err = runExample(t, binary, env, "inspect", "--state", stateDir, "docker.container.web")
	require.NoError(t, err, "stdout:\n%s\nstderr:\n%s", stdout, stderr)
	require.Empty(t, stderr, "inspect prints only the configuration")
	require.Contains(t, stdout, `docker "container" "web" {`)
	require.Contains(t, stdout, "# set by the provider")

	stdout, stderr, err = runExample(t, binary, env, "destroy", "--state", stateDir)
	require.NoError(t, err, "stdout:\n%s\nstderr:\n%s", stdout, stderr)
	require.Empty(t, stdout, "destroy prints nothing of its own")
	require.Contains(t, stderr, "destroy success")
	require.NotContains(t, stderr, "error:")

	stdout, _, err = runExample(t, binary, env, "status", "--state", stateDir)
	require.NoError(t, err)
	require.Equal(t, "nothing applied\n", stdout)
}

func TestSmokeApplyFailsForMissingPlugin(t *testing.T) {
	// apply checks for a Docker engine before it loads any plugin
	requireDocker(t)

	binary := buildExample(t)
	env := []string{"XCL_VAR_output_dir=" + t.TempDir()}

	_, stderr, err := runExample(t, binary, env, "apply", "--state", t.TempDir(), "--plugin", "/nonexistent/docker-plugin", "./config/full")
	require.Error(t, err)
	require.Contains(t, stderr, "failed to load")
	require.True(t, strings.HasSuffix(stderr, "build it with `make build` in example/plugin\n"), "the error is its message on one line: %s", stderr)
}

// TestSmokePlanPrintsAReportForAMistake runs plan against a configuration
// whose network has a block the Docker plugin does not take, and checks
// stderr holds the report with the faulty lines numbered
func TestSmokePlanPrintsAReportForAMistake(t *testing.T) {
	// plan checks for a Docker engine before it reads the configuration
	requireDocker(t)

	binary := buildExample(t)

	_, stderr, err := runExample(t, binary, nil, "plan", "--state", t.TempDir(), "./testdata/broken")
	require.Error(t, err)
	require.Contains(t, stderr, "Error: Unsupported block type\n")
	require.Contains(t, stderr, " line 7, in docker.network.broken, field timeout[0]:\n")
	require.Contains(t, stderr, "     5 |   subnet = \"10.99.0.0/24\"\n")
	require.Contains(t, stderr, "     7 |   timeout \"http\" {\n")
	require.Contains(t, stderr, "     9 |   }\n")
	require.Contains(t, stderr, "\nBlocks of type \"timeout\" are not expected here.\n")
}

func TestSmokePlanDoesNotPrintAMistakeAsOneLine(t *testing.T) {
	requireDocker(t)

	binary := buildExample(t)

	_, stderr, err := runExample(t, binary, nil, "plan", "--state", t.TempDir(), "./testdata/broken")
	require.Error(t, err)
	require.NotContains(t, stderr, "error: ")
}

func TestSmokeApplyFailsWithoutDocker(t *testing.T) {
	binary := buildExample(t)
	socket := "unix://" + filepath.Join(t.TempDir(), "none.sock")

	_, stderr, err := runExample(t, binary, []string{"DOCKER_HOST=" + socket}, "apply", "--state", t.TempDir(), "./config/full")
	require.Error(t, err)
	require.Contains(t, stderr, "no Docker engine reachable")
}
