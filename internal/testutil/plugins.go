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

// FunctionFixturePluginName is the file name of the fixture plugin binary
// that offers the function greet and provides the block type greeting
const FunctionFixturePluginName = "xcl-plugin-greeting"

// FunctionClashFixturePluginName is the file name of the fixture plugin
// binary that also offers greet, from the provider of the block type salute
const FunctionClashFixturePluginName = "xcl-plugin-salute"

// builtPlugin is one fixture plugin built once per test binary
type builtPlugin struct {
	once   sync.Once
	path   string
	output string
	err    error
}

// fixturePlugins holds every fixture plugin built so far, keyed by the
// package it is built from
var (
	fixturePluginsMu sync.Mutex
	fixturePlugins   = map[string]*builtPlugin{}
)

// BuildFixturePlugin builds the self-contained external test plugin in
// internal/test_fixtures/plugins/subtypeless, which provides the block type
// widget, and returns the path of the binary, named FixturePluginName.
// moduleRoot is the path of the xcl module root from the test's package
// directory. The plugin is built once per test binary into a directory under
// the OS temporary directory, since no single test owns it.
func BuildFixturePlugin(t testing.TB, moduleRoot string) string {
	t.Helper()

	return buildFixturePlugin(t, moduleRoot, "./internal/test_fixtures/plugins/subtypeless", FixturePluginName)
}

// BuildFunctionFixturePlugin builds the external test plugin in
// internal/test_fixtures/plugins/functions, which provides the block type
// greeting and offers the function greet(name), and returns the path of the
// binary, named FunctionFixturePluginName.
func BuildFunctionFixturePlugin(t testing.TB, moduleRoot string) string {
	t.Helper()

	return buildFixturePlugin(t, moduleRoot, "./internal/test_fixtures/plugins/functions", FunctionFixturePluginName)
}

// BuildFunctionClashFixturePlugin builds the external test plugin in
// internal/test_fixtures/plugins/functions_clash, which provides the block
// type salute and also offers greet, and returns the path of the binary,
// named FunctionClashFixturePluginName.
func BuildFunctionClashFixturePlugin(t testing.TB, moduleRoot string) string {
	t.Helper()

	return buildFixturePlugin(t, moduleRoot, "./internal/test_fixtures/plugins/functions_clash", FunctionClashFixturePluginName)
}

// buildFixturePlugin builds the plugin main package at pkg, relative to
// moduleRoot, into a binary called name under the OS temporary directory,
// once per test binary, and returns its path
func buildFixturePlugin(t testing.TB, moduleRoot, pkg, name string) string {
	t.Helper()

	fixturePluginsMu.Lock()
	built, ok := fixturePlugins[pkg]
	if !ok {
		built = &builtPlugin{}
		fixturePlugins[pkg] = built
	}
	fixturePluginsMu.Unlock()

	built.once.Do(func() {
		dir, err := os.MkdirTemp("", "xcl-fixture-plugin-*")
		if err != nil {
			built.err = err
			return
		}

		path := filepath.Join(dir, name)

		build := exec.Command("go", "build", "-o", path, pkg)
		build.Dir = moduleRoot

		output, err := build.CombinedOutput()
		built.path, built.output, built.err = path, string(output), err
	})

	require.NoError(t, built.err, "unable to build the fixture plugin %s: %s", pkg, built.output)

	return built.path
}
