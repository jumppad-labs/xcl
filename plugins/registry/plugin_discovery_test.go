package registry

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

func TestPluginDiscoverySingleValidPlugin(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, validDir, "xcl-plugin-test")

	pd := NewPluginDiscovery([]string{validDir}, "xcl-plugin-*", nil)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Errorf("DiscoverPlugins() error = %v, wantErr false", err)
		return
	}

	if len(plugins) != 1 {
		t.Errorf("DiscoverPlugins() found %d plugins, want 1", len(plugins))
		t.Logf("Found plugins: %v", plugins)
	}

	for _, plugin := range plugins {
		if !filepath.IsAbs(plugin) {
			t.Errorf("Plugin path is not absolute: %s", plugin)
		}
		if _, err := os.Stat(plugin); err != nil {
			t.Errorf("Plugin file does not exist: %s", plugin)
		}
	}
}

func TestPluginDiscoveryMultipleValidPlugins(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, validDir, "xcl-plugin-one")
	setup.copyPlugin(examplePlugin, validDir, "xcl-plugin-two")

	pd := NewPluginDiscovery([]string{validDir}, "xcl-plugin-*", nil)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Errorf("DiscoverPlugins() error = %v, wantErr false", err)
		return
	}

	if len(plugins) != 2 {
		t.Errorf("DiscoverPlugins() found %d plugins, want 2", len(plugins))
		t.Logf("Found plugins: %v", plugins)
	}

	for _, plugin := range plugins {
		if !filepath.IsAbs(plugin) {
			t.Errorf("Plugin path is not absolute: %s", plugin)
		}
		if _, err := os.Stat(plugin); err != nil {
			t.Errorf("Plugin file does not exist: %s", plugin)
		}
	}
}

func TestPluginDiscoveryPluginNotMatchingPattern(t *testing.T) {
	setup := newTestPluginSetup(t)
	invalidDir := setup.createPluginDir("invalid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, invalidDir, "not-a-plugin")

	pd := NewPluginDiscovery([]string{invalidDir}, "xcl-plugin-*", nil)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Errorf("DiscoverPlugins() error = %v, wantErr false", err)
		return
	}

	if len(plugins) != 0 {
		t.Errorf("DiscoverPlugins() found %d plugins, want 0", len(plugins))
		t.Logf("Found plugins: %v", plugins)
	}
}

func TestPluginDiscoveryNonExecutableFile(t *testing.T) {
	setup := newTestPluginSetup(t)
	invalidDir := setup.createPluginDir("invalid")

	setup.createNonExecutable(invalidDir, "xcl-plugin-fake")

	pd := NewPluginDiscovery([]string{invalidDir}, "xcl-plugin-*", nil)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Errorf("DiscoverPlugins() error = %v, wantErr false", err)
		return
	}

	if len(plugins) != 0 {
		t.Errorf("DiscoverPlugins() found %d plugins, want 0", len(plugins))
		t.Logf("Found plugins: %v", plugins)
	}
}

func TestPluginDiscoveryMixedDirectory(t *testing.T) {
	setup := newTestPluginSetup(t)
	mixedDir := setup.createPluginDir("mixed")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, mixedDir, "xcl-plugin-good")
	setup.createNonPlugin(mixedDir, "xcl-plugin-bad")
	setup.createNonExecutable(mixedDir, "xcl-plugin-text.txt")
	setup.copyPlugin(examplePlugin, mixedDir, "wrong-pattern")

	pd := NewPluginDiscovery([]string{mixedDir}, "xcl-plugin-*", nil)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Errorf("DiscoverPlugins() error = %v, wantErr false", err)
		return
	}

	// plugin-good and plugin-bad (both executables)
	if len(plugins) != 2 {
		t.Errorf("DiscoverPlugins() found %d plugins, want 2", len(plugins))
		t.Logf("Found plugins: %v", plugins)
	}

	for _, plugin := range plugins {
		if !filepath.IsAbs(plugin) {
			t.Errorf("Plugin path is not absolute: %s", plugin)
		}
		if _, err := os.Stat(plugin); err != nil {
			t.Errorf("Plugin file does not exist: %s", plugin)
		}
	}
}

func TestPluginDiscoveryEmptyDirectory(t *testing.T) {
	setup := newTestPluginSetup(t)
	emptyDir := setup.createPluginDir("empty")

	pd := NewPluginDiscovery([]string{emptyDir}, "xcl-plugin-*", nil)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Errorf("DiscoverPlugins() error = %v, wantErr false", err)
		return
	}

	if len(plugins) != 0 {
		t.Errorf("DiscoverPlugins() found %d plugins, want 0", len(plugins))
		t.Logf("Found plugins: %v", plugins)
	}
}

func TestPluginDiscoveryNonExistentDirectory(t *testing.T) {
	setup := newTestPluginSetup(t)
	nonExistentDir := filepath.Join(setup.testDir, "non-existent")

	pd := NewPluginDiscovery([]string{nonExistentDir}, "xcl-plugin-*", nil)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Errorf("DiscoverPlugins() error = %v, wantErr false", err)
		return
	}

	if len(plugins) != 0 {
		t.Errorf("DiscoverPlugins() found %d plugins, want 0", len(plugins))
		t.Logf("Found plugins: %v", plugins)
	}
}

func TestPluginDiscoveryMultipleDirectories(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	mixedDir := setup.createPluginDir("mixed")
	emptyDir := setup.createPluginDir("empty")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, validDir, "xcl-plugin-dir1")
	setup.copyPlugin(examplePlugin, mixedDir, "xcl-plugin-dir2")

	pd := NewPluginDiscovery([]string{validDir, mixedDir, emptyDir}, "xcl-plugin-*", nil)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Errorf("DiscoverPlugins() error = %v, wantErr false", err)
		return
	}

	if len(plugins) != 2 {
		t.Errorf("DiscoverPlugins() found %d plugins, want 2", len(plugins))
		t.Logf("Found plugins: %v", plugins)
	}

	for _, plugin := range plugins {
		if !filepath.IsAbs(plugin) {
			t.Errorf("Plugin path is not absolute: %s", plugin)
		}
		if _, err := os.Stat(plugin); err != nil {
			t.Errorf("Plugin file does not exist: %s", plugin)
		}
	}
}

func TestPluginDiscoveryCustomPattern(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, validDir, "my-custom-plugin-test")
	setup.copyPlugin(examplePlugin, validDir, "xcl-plugin-ignored")

	pd := NewPluginDiscovery([]string{validDir}, "my-custom-plugin-*", nil)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Errorf("DiscoverPlugins() error = %v, wantErr false", err)
		return
	}

	if len(plugins) != 1 {
		t.Errorf("DiscoverPlugins() found %d plugins, want 1", len(plugins))
		t.Logf("Found plugins: %v", plugins)
	}

	for _, plugin := range plugins {
		if !filepath.IsAbs(plugin) {
			t.Errorf("Plugin path is not absolute: %s", plugin)
		}
		if _, err := os.Stat(plugin); err != nil {
			t.Errorf("Plugin file does not exist: %s", plugin)
		}
	}
}

func TestPluginDiscoveryDuplicateDirectories(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, validDir, "xcl-plugin-unique")

	pd := NewPluginDiscovery([]string{validDir, validDir, validDir}, "xcl-plugin-*", nil)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Errorf("DiscoverPlugins() error = %v, wantErr false", err)
		return
	}

	// Should deduplicate
	if len(plugins) != 1 {
		t.Errorf("DiscoverPlugins() found %d plugins, want 1", len(plugins))
		t.Logf("Found plugins: %v", plugins)
	}

	for _, plugin := range plugins {
		if !filepath.IsAbs(plugin) {
			t.Errorf("Plugin path is not absolute: %s", plugin)
		}
		if _, err := os.Stat(plugin); err != nil {
			t.Errorf("Plugin file does not exist: %s", plugin)
		}
	}
}

func TestPluginDiscovery_WindowsExecutables(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	setup := newTestPluginSetup(t)
	dir := setup.createPluginDir("windows")

	// Build example plugin
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	// Copy with .exe extension (should be found)
	exePath := setup.copyPlugin(examplePlugin, dir, "xcl-plugin-test.exe")

	// Create without .exe extension (should not be found on Windows)
	nonExePath := filepath.Join(dir, "xcl-plugin-noext")
	if err := os.WriteFile(nonExePath, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}

	pd := NewPluginDiscovery([]string{dir}, "xcl-plugin-*", nil)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(plugins) != 1 {
		t.Fatalf("Expected 1 plugin, found %d", len(plugins))
	}

	if plugins[0] != exePath {
		t.Errorf("Expected plugin path %s, got %s", exePath, plugins[0])
	}
}

func TestExpandPluginDirectoriesExpandHomeDirectory(t *testing.T) {
	// Save original env
	originalHome := os.Getenv("HOME")
	originalTestVar := os.Getenv("TEST_PLUGIN_DIR")
	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("TEST_PLUGIN_DIR", originalTestVar)
	}()

	// Set test environment
	os.Setenv("TEST_PLUGIN_DIR", "/test/plugins")
	homeDir, _ := os.UserHomeDir()

	input := []string{"~/plugins", "~/.config/plugins"}
	expected := []string{filepath.Join(homeDir, "plugins"), filepath.Join(homeDir, ".config/plugins")}

	result := ExpandPluginDirectories(input)

	if len(result) != len(expected) {
		t.Fatalf("Expected %d paths, got %d", len(expected), len(result))
	}

	for i, path := range result {
		// Normalize paths for comparison
		expectedPath := filepath.Clean(expected[i])
		got := filepath.Clean(path)

		if got != expectedPath {
			t.Errorf("Path %d: expected %s, got %s", i, expectedPath, got)
		}
	}
}

func TestExpandPluginDirectoriesExpandEnvironmentVariables(t *testing.T) {
	// Save original env
	originalHome := os.Getenv("HOME")
	originalTestVar := os.Getenv("TEST_PLUGIN_DIR")
	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("TEST_PLUGIN_DIR", originalTestVar)
	}()

	// Set test environment
	os.Setenv("TEST_PLUGIN_DIR", "/test/plugins")

	input := []string{"$TEST_PLUGIN_DIR", "${TEST_PLUGIN_DIR}/sub"}
	expected := []string{"/test/plugins", "/test/plugins/sub"}

	result := ExpandPluginDirectories(input)

	if len(result) != len(expected) {
		t.Fatalf("Expected %d paths, got %d", len(expected), len(result))
	}

	for i, path := range result {
		// Normalize paths for comparison
		expectedPath := filepath.Clean(expected[i])
		got := filepath.Clean(path)

		if got != expectedPath {
			t.Errorf("Path %d: expected %s, got %s", i, expectedPath, got)
		}
	}
}

func TestExpandPluginDirectoriesNoExpansionNeeded(t *testing.T) {
	// Save original env
	originalHome := os.Getenv("HOME")
	originalTestVar := os.Getenv("TEST_PLUGIN_DIR")
	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("TEST_PLUGIN_DIR", originalTestVar)
	}()

	// Set test environment
	os.Setenv("TEST_PLUGIN_DIR", "/test/plugins")

	input := []string{"/absolute/path", "./relative/path"}
	expected := []string{"/absolute/path", "./relative/path"}

	result := ExpandPluginDirectories(input)

	if len(result) != len(expected) {
		t.Fatalf("Expected %d paths, got %d", len(expected), len(result))
	}

	for i, path := range result {
		// Normalize paths for comparison
		expectedPath := filepath.Clean(expected[i])
		got := filepath.Clean(path)

		if got != expectedPath {
			t.Errorf("Path %d: expected %s, got %s", i, expectedPath, got)
		}
	}
}

func TestExpandPluginDirectoriesMixedPaths(t *testing.T) {
	// Save original env
	originalHome := os.Getenv("HOME")
	originalTestVar := os.Getenv("TEST_PLUGIN_DIR")
	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("TEST_PLUGIN_DIR", originalTestVar)
	}()

	// Set test environment
	os.Setenv("TEST_PLUGIN_DIR", "/test/plugins")
	homeDir, _ := os.UserHomeDir()

	input := []string{"~/plugins", "$TEST_PLUGIN_DIR", "/absolute"}
	expected := []string{filepath.Join(homeDir, "plugins"), "/test/plugins", "/absolute"}

	result := ExpandPluginDirectories(input)

	if len(result) != len(expected) {
		t.Fatalf("Expected %d paths, got %d", len(expected), len(result))
	}

	for i, path := range result {
		// Normalize paths for comparison
		expectedPath := filepath.Clean(expected[i])
		got := filepath.Clean(path)

		if got != expectedPath {
			t.Errorf("Path %d: expected %s, got %s", i, expectedPath, got)
		}
	}
}

func TestPluginDiscoveryReportsWhatItFindsAsDiscoverLogEvents(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")
	pluginPath := setup.copyPlugin(examplePlugin, validDir, "xcl-plugin-test")

	recorder := &eventRecorder{}

	pd := NewPluginDiscovery([]string{validDir}, "xcl-plugin-*", recorder.emit)
	_, err := pd.DiscoverPlugins()
	require.NoError(t, err)

	found := logsWithMessage(recorder.recorded(), "Found plugin")
	require.Len(t, found, 1)
	require.Equal(t, events.SourceCore, found[0].Source)
	require.Equal(t, events.OperationDiscover, found[0].Operation)
	require.Equal(t, pluginPath, found[0].Meta["path"])
}

func TestPluginDiscoveryWithANilEmitDoesNotPanic(t *testing.T) {
	setup := newTestPluginSetup(t)
	emptyDir := setup.createPluginDir("empty")

	require.NotPanics(t, func() {
		_, err := NewPluginDiscovery([]string{emptyDir}, "xcl-plugin-*", nil).DiscoverPlugins()
		require.NoError(t, err)
	})
}

func TestDiscoverPluginsStartsNothingBeforeLoad(t *testing.T) {
	setup := newTestPluginSetup(t)
	pluginDir := setup.createPluginDir("plugins")

	examplePlugin := setup.buildExamplePlugin("test-plugin")
	setup.copyPlugin(examplePlugin, pluginDir, "xcl-plugin-example")

	r := NewPluginRegistry()

	r.DiscoverPlugins([]string{pluginDir}, "xcl-plugin-*")

	require.Empty(t, r.GetPluginHosts())
	require.False(t, r.Loaded())
}

func TestLoadStartsDiscoveredPlugins(t *testing.T) {
	setup := newTestPluginSetup(t)
	pluginDir := setup.createPluginDir("plugins")

	examplePlugin := setup.buildExamplePlugin("test-plugin")
	setup.copyPlugin(examplePlugin, pluginDir, "xcl-plugin-example")

	r := NewPluginRegistry()
	t.Cleanup(func() { stopHosts(r) })

	r.DiscoverPlugins([]string{pluginDir}, "xcl-plugin-*")

	err := r.Load(nil)
	require.NoError(t, err)

	require.Len(t, r.GetPluginHosts(), 1)
}

func TestLoadStartsNothingWhenNoDiscoveredPluginMatches(t *testing.T) {
	setup := newTestPluginSetup(t)
	pluginDir := setup.createPluginDir("plugins")

	examplePlugin := setup.buildExamplePlugin("test-plugin")
	setup.copyPlugin(examplePlugin, pluginDir, "not-a-matching-name")

	r := NewPluginRegistry()

	r.DiscoverPlugins([]string{pluginDir}, "xcl-plugin-*")

	err := r.Load(nil)
	require.NoError(t, err)

	require.Empty(t, r.GetPluginHosts())
}

// Person is a plain Go resource type registered under the name the example
// plugin provides, "person"
type Person struct {
	types.ResourceBase `xcl:",remain"`

	FirstName string `xcl:"first_name" json:"first_name"`
}

func TestLoadRejectsDiscoveredPluginClashingWithRegisteredType(t *testing.T) {
	setup := newTestPluginSetup(t)
	pluginDir := setup.createPluginDir("plugins")

	examplePlugin := setup.buildExamplePlugin("test-plugin")
	setup.copyPlugin(examplePlugin, pluginDir, "xcl-plugin-test")

	r := NewPluginRegistry()
	t.Cleanup(func() { stopHosts(r) })

	err := r.RegisterType("person", &Person{})
	require.NoError(t, err)

	r.DiscoverPlugins([]string{pluginDir}, "xcl-plugin-*")

	err = r.Load(nil)
	require.Error(t, err)
	require.ErrorContains(t, err, `"person"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "person", clash.Name)
	require.Equal(t, "registered type", clash.Existing)

	require.Empty(t, r.GetPluginHosts())
	require.True(t, r.IsRegisteredType("person"))
}

func TestLoadRejectsSecondDiscoveredPluginProvidingSameType(t *testing.T) {
	setup := newTestPluginSetup(t)
	pluginDir := setup.createPluginDir("plugins")

	examplePlugin := setup.buildExamplePlugin("test-plugin")
	setup.copyPlugin(examplePlugin, pluginDir, "xcl-plugin-one")
	setup.copyPlugin(examplePlugin, pluginDir, "xcl-plugin-two")

	r := NewPluginRegistry()
	t.Cleanup(func() { stopHosts(r) })

	r.DiscoverPlugins([]string{pluginDir}, "xcl-plugin-*")

	err := r.Load(nil)
	require.Error(t, err)
	require.ErrorContains(t, err, `"person"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "person", clash.Name)
	require.Equal(t, "plugin", clash.Existing)

	require.Len(t, r.GetPluginHosts(), 1)
}

func TestLoadFailsWhenEveryDiscoveredPluginFailsToStart(t *testing.T) {
	setup := newTestPluginSetup(t)
	pluginDir := setup.createPluginDir("plugins")

	setup.createNonPlugin(pluginDir, "xcl-plugin-bad")

	r := NewPluginRegistry()

	r.DiscoverPlugins([]string{pluginDir}, "xcl-plugin-*")

	err := r.Load(nil)
	require.Error(t, err)
	require.ErrorContains(t, err, "all plugin loads failed")

	require.Empty(t, r.GetPluginHosts())
}

func TestLoadRejectsExplicitPluginPathClashingWithRegisteredType(t *testing.T) {
	setup := newTestPluginSetup(t)
	examplePlugin := setup.buildExamplePlugin("test-plugin")

	r := NewPluginRegistry()
	t.Cleanup(func() { stopHosts(r) })

	err := r.RegisterType("person", &Person{})
	require.NoError(t, err)

	err = r.RegisterPluginWithPath(examplePlugin)
	require.NoError(t, err)

	err = r.Load(nil)
	require.Error(t, err)
	require.ErrorContains(t, err, `"person"`)
	require.ErrorContains(t, err, examplePlugin)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "person", clash.Name)
	require.Equal(t, "registered type", clash.Existing)

	require.Empty(t, r.GetPluginHosts())
}

func TestLoadStartsExplicitPluginPathWithoutClash(t *testing.T) {
	setup := newTestPluginSetup(t)
	examplePlugin := setup.buildExamplePlugin("test-plugin")

	r := NewPluginRegistry()
	t.Cleanup(func() { stopHosts(r) })

	err := r.RegisterType("thing", &Thing{})
	require.NoError(t, err)

	err = r.RegisterPluginWithPath(examplePlugin)
	require.NoError(t, err)

	err = r.Load(nil)
	require.NoError(t, err)

	require.Len(t, r.GetPluginHosts(), 1)
}

func TestDiscoveryReportsDiscoverLoadAndRejectEvents(t *testing.T) {
	setup := newTestPluginSetup(t)
	pluginDir := setup.createPluginDir("plugins")

	examplePlugin := setup.buildExamplePlugin("test-plugin")
	setup.copyPlugin(examplePlugin, pluginDir, "xcl-plugin-good")
	badPath := setup.createNonPlugin(pluginDir, "xcl-plugin-bad")

	recorder := &eventRecorder{}
	r := NewPluginRegistry()
	t.Cleanup(func() { stopHosts(r) })

	r.DiscoverPlugins([]string{pluginDir}, "xcl-plugin-*")

	err := r.Load(recorder.emit)
	require.NoError(t, err)

	discovered := lifecycleEvents(recorder.recorded(), events.OperationDiscover)
	require.Len(t, discovered, 2)

	require.Equal(t, events.SourceCore, discovered[0].Source)
	require.Equal(t, events.PhaseStart, discovered[0].Phase)
	require.Equal(t, map[string]any{"dirs": []string{pluginDir}}, discovered[0].Meta)

	require.Equal(t, events.SourceCore, discovered[1].Source)
	require.Equal(t, events.PhaseSuccess, discovered[1].Phase)
	require.Equal(t, map[string]any{"dirs": []string{pluginDir}, "count": 2}, discovered[1].Meta)

	loads := lifecycleEvents(recorder.recorded(), events.OperationLoad)

	succeeded := eventsWithPhase(loads, events.PhaseSuccess)
	require.Len(t, succeeded, 1)
	require.Equal(t, events.SourceCore, succeeded[0].Source)
	require.Equal(t, map[string]any{"plugin": "xcl-plugin-good", "block_types": "person"}, succeeded[0].Meta)

	rejected := eventsWithPhase(loads, events.PhaseError)
	require.Len(t, rejected, 1)
	require.Equal(t, events.SourceCore, rejected[0].Source)
	require.Error(t, rejected[0].Error)
	require.Equal(t, map[string]any{"plugin": "xcl-plugin-bad", "path": badPath, "rejected": true}, rejected[0].Meta)
}
