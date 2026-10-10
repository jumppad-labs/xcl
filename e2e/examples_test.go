package e2e_test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// exampleTestsError runs `go test ./...` in the module at dir and returns an
// error naming the module, with the command's output, when its tests fail
func exampleTestsError(dir string) error {
	command := exec.Command("go", "test", "./...")
	command.Dir = dir

	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("example %s: tests failed: %w\n%s", filepath.Base(dir), err, output)
	}

	return nil
}

// runExampleTests runs the tests of the example module example/<name>, smoke
// tests included, and fails the test naming the example when they fail. Each
// example is a module of its own, so the root `go test ./...` reaches it only
// through this
func runExampleTests(t testing.TB, name string) {
	t.Helper()

	_, err := exec.LookPath("go")
	require.NoError(t, err, "the go command is needed to run the examples' tests")

	require.NoError(t, exampleTestsError(filepath.Join("..", "example", name)))
}

func TestConfigOnlyExampleTestsPass(t *testing.T) {
	t.Parallel()

	runExampleTests(t, "configonly")
}

func TestPluginExampleTestsPass(t *testing.T) {
	t.Parallel()

	runExampleTests(t, "plugin")
}

func TestDockerPluginExampleTestsPass(t *testing.T) {
	t.Parallel()

	runExampleTests(t, filepath.Join("plugin", "plugins", "docker"))
}

func TestPrettylogExampleTestsPass(t *testing.T) {
	t.Parallel()

	runExampleTests(t, "prettylog")
}

func TestExampleRunnerPassesForAPassingModule(t *testing.T) {
	err := exampleTestsError(filepath.Join("testdata", "passingexample"))
	require.NoError(t, err)
}

func TestExampleRunnerNamesAFailingModule(t *testing.T) {
	err := exampleTestsError(filepath.Join("testdata", "failingexample"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "example failingexample: tests failed")
}
