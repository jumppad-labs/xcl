package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// FixturePluginName is the file name of the fixture plugin binary, so a fake
// release of a repository with this name serves it under the contract's
// names
const FixturePluginName = "xcl-plugin-widget"

// fixturePlugin is the fixture plugin built once per test binary
var fixturePlugin struct {
	once   sync.Once
	path   string
	output string
	err    error
}

// BuildFixturePlugin builds the self-contained external test plugin in
// internal/test_fixtures/plugins/subtypeless, which provides the block type
// widget, and returns the path of the binary, named FixturePluginName.
// moduleRoot is the path of the xcl module root from the test's package
// directory. The plugin is built once per test binary into a directory under
// the OS temporary directory, since no single test owns it.
func BuildFixturePlugin(t testing.TB, moduleRoot string) string {
	t.Helper()

	fixturePlugin.once.Do(func() {
		dir, err := os.MkdirTemp("", "xcl-fixture-plugin-*")
		if err != nil {
			fixturePlugin.err = err
			return
		}

		path := filepath.Join(dir, FixturePluginName)

		build := exec.Command("go", "build", "-o", path, "./internal/test_fixtures/plugins/subtypeless")
		build.Dir = moduleRoot

		output, err := build.CombinedOutput()
		fixturePlugin.path, fixturePlugin.output, fixturePlugin.err = path, string(output), err
	})

	require.NoError(t, fixturePlugin.err, "unable to build the fixture plugin: %s", fixturePlugin.output)

	return fixturePlugin.path
}
