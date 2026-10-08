package registry

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/stretchr/testify/require"
)

// testPluginSetup contains helper functions for plugin discovery tests
type testPluginSetup struct {
	t       *testing.T
	testDir string
}

// newTestPluginSetup creates a new test setup in a temporary directory that is
// removed when the test ends
func newTestPluginSetup(t *testing.T) *testPluginSetup {
	return &testPluginSetup{
		t:       t,
		testDir: t.TempDir(),
	}
}

// createPluginDir creates a plugin directory and returns its path
func (s *testPluginSetup) createPluginDir(name string) string {
	dir := filepath.Join(s.testDir, name)
	err := os.MkdirAll(dir, 0755)
	require.NoError(s.t, err, "failed to create plugin directory %s", dir)

	return dir
}

// buildExamplePlugin builds the example plugin binary and returns its path
func (s *testPluginSetup) buildExamplePlugin(outputName string) string {
	outputPath := filepath.Join(s.testDir, outputName)
	if runtime.GOOS == "windows" {
		outputPath += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", outputPath, "./plugins/example")
	cmd.Dir = getRootDir(s.t)

	output, err := cmd.CombinedOutput()
	require.NoError(s.t, err, "failed to build example plugin: %s", output)

	return outputPath
}

// getRootDir finds the project root directory, the first parent holding go.mod
func getRootDir(t *testing.T) string {
	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "could not find project root")
		dir = parent
	}
}

// copyPlugin copies a plugin binary to a new location with a new name
func (s *testPluginSetup) copyPlugin(src, dstDir, dstName string) string {
	dst := filepath.Join(dstDir, dstName)
	if runtime.GOOS == "windows" && filepath.Ext(dst) != ".exe" {
		dst += ".exe"
	}

	srcData, err := os.ReadFile(src)
	require.NoError(s.t, err, "failed to read source plugin %s", src)

	err = os.WriteFile(dst, srcData, 0755)
	require.NoError(s.t, err, "failed to write plugin to %s", dst)

	return dst
}

// createNonPlugin creates an executable file that is not a plugin
func (s *testPluginSetup) createNonPlugin(dir, name string) string {
	path := filepath.Join(dir, name)

	content := "#!/bin/sh\necho 'This is not a plugin'\n"
	if runtime.GOOS == "windows" {
		path += ".bat"
		content = "@echo off\necho This is not a plugin\n"
	}

	err := os.WriteFile(path, []byte(content), 0755)
	require.NoError(s.t, err, "failed to create non-plugin %s", path)

	return path
}

// createNonExecutable creates a file without execute permission
func (s *testPluginSetup) createNonExecutable(dir, name string) string {
	path := filepath.Join(dir, name)

	err := os.WriteFile(path, []byte("This is not an executable file"), 0644)
	require.NoError(s.t, err, "failed to create non-executable %s", path)

	return path
}

// logsWithMessage returns the log events in recorded with the given message
func logsWithMessage(recorded []events.Event, message string) []events.Event {
	found := []events.Event{}
	for _, e := range recorded {
		if e.Phase == events.PhaseLog && e.Meta[events.KeyMessage] == message {
			found = append(found, e)
		}
	}

	return found
}

// requireAbsoluteExistingPaths fails the test unless every path is absolute
// and exists
func requireAbsoluteExistingPaths(t *testing.T, paths []string) {
	t.Helper()

	for _, path := range paths {
		require.True(t, filepath.IsAbs(path), "plugin path is not absolute: %s", path)
		require.FileExists(t, path)
	}
}

func TestPluginDiscoverySingleValidPlugin(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, validDir, "xcl-plugin-test")

	plugins, err := newPluginDiscovery([]string{validDir}, "xcl-plugin-*", nil).discoverPlugins()
	require.NoError(t, err)

	require.Len(t, plugins, 1)
	requireAbsoluteExistingPaths(t, plugins)
}

func TestPluginDiscoveryMultipleValidPlugins(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, validDir, "xcl-plugin-one")
	setup.copyPlugin(examplePlugin, validDir, "xcl-plugin-two")

	plugins, err := newPluginDiscovery([]string{validDir}, "xcl-plugin-*", nil).discoverPlugins()
	require.NoError(t, err)

	require.Len(t, plugins, 2)
	requireAbsoluteExistingPaths(t, plugins)
}

func TestPluginDiscoveryPluginNotMatchingPattern(t *testing.T) {
	setup := newTestPluginSetup(t)
	invalidDir := setup.createPluginDir("invalid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, invalidDir, "not-a-plugin")

	plugins, err := newPluginDiscovery([]string{invalidDir}, "xcl-plugin-*", nil).discoverPlugins()
	require.NoError(t, err)

	require.Empty(t, plugins)
}

func TestPluginDiscoveryNonExecutableFile(t *testing.T) {
	setup := newTestPluginSetup(t)
	invalidDir := setup.createPluginDir("invalid")

	setup.createNonExecutable(invalidDir, "xcl-plugin-fake")

	plugins, err := newPluginDiscovery([]string{invalidDir}, "xcl-plugin-*", nil).discoverPlugins()
	require.NoError(t, err)

	require.Empty(t, plugins)
}

func TestPluginDiscoveryMixedDirectory(t *testing.T) {
	setup := newTestPluginSetup(t)
	mixedDir := setup.createPluginDir("mixed")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, mixedDir, "xcl-plugin-good")
	setup.createNonPlugin(mixedDir, "xcl-plugin-bad")
	setup.createNonExecutable(mixedDir, "xcl-plugin-text.txt")
	setup.copyPlugin(examplePlugin, mixedDir, "wrong-pattern")

	plugins, err := newPluginDiscovery([]string{mixedDir}, "xcl-plugin-*", nil).discoverPlugins()
	require.NoError(t, err)

	// xcl-plugin-good and xcl-plugin-bad, both are executables
	require.Len(t, plugins, 2)
	requireAbsoluteExistingPaths(t, plugins)
}

func TestPluginDiscoveryEmptyDirectory(t *testing.T) {
	setup := newTestPluginSetup(t)
	emptyDir := setup.createPluginDir("empty")

	plugins, err := newPluginDiscovery([]string{emptyDir}, "xcl-plugin-*", nil).discoverPlugins()
	require.NoError(t, err)

	require.Empty(t, plugins)
}

func TestPluginDiscoveryNonExistentDirectory(t *testing.T) {
	setup := newTestPluginSetup(t)
	nonExistentDir := filepath.Join(setup.testDir, "non-existent")

	plugins, err := newPluginDiscovery([]string{nonExistentDir}, "xcl-plugin-*", nil).discoverPlugins()
	require.NoError(t, err)

	require.Empty(t, plugins)
}

func TestPluginDiscoveryMultipleDirectories(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	mixedDir := setup.createPluginDir("mixed")
	emptyDir := setup.createPluginDir("empty")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, validDir, "xcl-plugin-dir1")
	setup.copyPlugin(examplePlugin, mixedDir, "xcl-plugin-dir2")

	plugins, err := newPluginDiscovery([]string{validDir, mixedDir, emptyDir}, "xcl-plugin-*", nil).discoverPlugins()
	require.NoError(t, err)

	require.Len(t, plugins, 2)
	requireAbsoluteExistingPaths(t, plugins)
}

func TestPluginDiscoveryCustomPattern(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, validDir, "my-custom-plugin-test")
	setup.copyPlugin(examplePlugin, validDir, "xcl-plugin-ignored")

	plugins, err := newPluginDiscovery([]string{validDir}, "my-custom-plugin-*", nil).discoverPlugins()
	require.NoError(t, err)

	require.Len(t, plugins, 1)
	requireAbsoluteExistingPaths(t, plugins)
}

func TestPluginDiscoveryEmptyPatternUsesDefault(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, validDir, "xcl-plugin-default")
	setup.copyPlugin(examplePlugin, validDir, "other-plugin")

	plugins, err := newPluginDiscovery([]string{validDir}, "", nil).discoverPlugins()
	require.NoError(t, err)

	require.Len(t, plugins, 1)
	require.Equal(t, "xcl-plugin-default", filepath.Base(plugins[0]))
}

func TestPluginDiscoveryDuplicateDirectories(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, validDir, "xcl-plugin-unique")

	plugins, err := newPluginDiscovery([]string{validDir, validDir, validDir}, "xcl-plugin-*", nil).discoverPlugins()
	require.NoError(t, err)

	// the directory is searched once
	require.Len(t, plugins, 1)
	requireAbsoluteExistingPaths(t, plugins)
}

func TestPluginDiscoveryWindowsExecutables(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	setup := newTestPluginSetup(t)
	dir := setup.createPluginDir("windows")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	// with a .exe extension, found
	exePath := setup.copyPlugin(examplePlugin, dir, "xcl-plugin-test.exe")

	// without a .exe extension, not found on Windows
	nonExePath := filepath.Join(dir, "xcl-plugin-noext")
	err := os.WriteFile(nonExePath, []byte("fake"), 0755)
	require.NoError(t, err)

	plugins, err := newPluginDiscovery([]string{dir}, "xcl-plugin-*", nil).discoverPlugins()
	require.NoError(t, err)

	require.Equal(t, []string{exePath}, plugins)
}

func TestExpandPluginDirectoriesExpandHomeDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	result := expandPluginDirectories([]string{"~/plugins", "~/.config/plugins"})

	require.Equal(t, []string{
		filepath.Join(home, "plugins"),
		filepath.Join(home, ".config", "plugins"),
	}, result)
}

func TestExpandPluginDirectoriesExpandEnvironmentVariables(t *testing.T) {
	t.Setenv("TEST_PLUGIN_DIR", "/test/plugins")

	result := expandPluginDirectories([]string{"$TEST_PLUGIN_DIR", "${TEST_PLUGIN_DIR}/sub"})

	require.Equal(t, []string{"/test/plugins", "/test/plugins/sub"}, result)
}

func TestExpandPluginDirectoriesNoExpansionNeeded(t *testing.T) {
	t.Setenv("TEST_PLUGIN_DIR", "/test/plugins")

	result := expandPluginDirectories([]string{"/absolute/path", "./relative/path"})

	require.Equal(t, []string{"/absolute/path", "./relative/path"}, result)
}

func TestExpandPluginDirectoriesMixedPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TEST_PLUGIN_DIR", "/test/plugins")

	result := expandPluginDirectories([]string{"~/plugins", "$TEST_PLUGIN_DIR", "/absolute"})

	require.Equal(t, []string{filepath.Join(home, "plugins"), "/test/plugins", "/absolute"}, result)
}

func TestPluginDiscoveryReportsWhatItFindsAsDiscoverLogEvents(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")
	pluginPath := setup.copyPlugin(examplePlugin, validDir, "xcl-plugin-test")

	recorder := &testutil.EventRecorder{}

	_, err := newPluginDiscovery([]string{validDir}, "xcl-plugin-*", recorder.Record).discoverPlugins()
	require.NoError(t, err)

	found := logsWithMessage(recorder.Events(), "Found plugin")
	require.Len(t, found, 1)
	require.Equal(t, events.SourceCore, found[0].Source)
	require.Equal(t, events.OperationDiscover, found[0].Operation)
	require.Equal(t, pluginPath, found[0].Meta["path"])
}

func TestPluginDiscoveryWithANilEmitDoesNotPanic(t *testing.T) {
	setup := newTestPluginSetup(t)
	emptyDir := setup.createPluginDir("empty")

	require.NotPanics(t, func() {
		_, err := newPluginDiscovery([]string{emptyDir}, "xcl-plugin-*", nil).discoverPlugins()
		require.NoError(t, err)
	})
}
