package xcl

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/stretchr/testify/require"
)

// missingPluginPath is an external plugin binary that does not exist
const missingPluginPath = "/nonexistent/xcl-plugin-missing"

// variableOnlyConfig declares nothing a plugin provides, so validating it
// needs no plugin type
const variableOnlyConfig = `
variable "greeting" {
  default = "hello"
}
`

// writeConfigFile writes contents to a configuration file in a temporary
// directory and returns its path
func writeConfigFile(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "main.xcl")
	err := os.WriteFile(path, []byte(contents), 0644)
	require.NoError(t, err)

	return path
}

// isolateHome points HOME at a temporary directory for the test, so nothing
// the test runs reads or writes the real home directory
func isolateHome(t *testing.T) {
	t.Helper()

	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())

	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})
}

// setupMissingPluginConfig returns a Config whose registry holds an external
// plugin path that does not exist, and the recorder its events go to
func setupMissingPluginConfig(t *testing.T) (*Config, *eventRecorder) {
	t.Helper()

	isolateHome(t)

	pr := registry.NewPluginRegistry()

	err := pr.RegisterPluginWithPath(missingPluginPath)
	require.NoError(t, err)

	recorder := &eventRecorder{}

	c := NewConfig(
		WithPluginRegistry(pr),
		WithEventHandler(recorder.handle),
	)

	return c, recorder
}

// buildExamplePlugin builds the external example plugin into a temporary
// directory and returns the binary's path
func buildExamplePlugin(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "xcl-plugin-example")

	build := exec.Command("go", "build", "-o", binary, "./plugins/example")
	output, err := build.CombinedOutput()
	require.NoError(t, err, "unable to build the example plugin: %s", output)

	return binary
}

// eventsFor returns the events the recorder received for operation and phase
// that concern no resource
func eventsFor(recorder *eventRecorder, operation, phase string) []Event {
	return recorder.find("", operation, phase)
}

// indexOf returns the position of the first event matching match, or -1
func indexOf(recorder *eventRecorder, match func(Event) bool) int {
	for i, e := range recorder.snapshot() {
		if match(e) {
			return i
		}
	}

	return -1
}

func TestRegisterPluginWithPathAcceptsMissingBinary(t *testing.T) {
	pr := registry.NewPluginRegistry()

	err := pr.RegisterPluginWithPath(missingPluginPath)
	require.NoError(t, err)
}

func TestFirstValidateFailsNamingMissingPlugin(t *testing.T) {
	c, _ := setupMissingPluginConfig(t)

	err := c.Validate(writeConfigFile(t, variableOnlyConfig))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPluginLoad)
	require.Contains(t, err.Error(), missingPluginPath)

	var loadErr *PluginLoadError
	require.True(t, errors.As(err, &loadErr))
	require.Equal(t, missingPluginPath, loadErr.Plugin)
}

func TestFirstValidateEmitsLoadErrorForMissingPlugin(t *testing.T) {
	c, recorder := setupMissingPluginConfig(t)

	err := c.Validate(writeConfigFile(t, variableOnlyConfig))
	require.Error(t, err)

	loadErrors := eventsFor(recorder, events.OperationLoad, events.PhaseError)
	require.Len(t, loadErrors, 1)
	require.Equal(t, events.SourceCore, loadErrors[0].Source)
	require.ErrorIs(t, loadErrors[0].Error, ErrPluginLoad)

	validateErrors := eventsFor(recorder, events.OperationValidate, events.PhaseError)
	require.Len(t, validateErrors, 1)
	require.ErrorIs(t, validateErrors[0].Error, ErrPluginLoad)
}

func TestFirstApplyFailsNamingMissingPlugin(t *testing.T) {
	c, recorder := setupMissingPluginConfig(t)

	err := c.Apply(writeConfigFile(t, variableOnlyConfig))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPluginLoad)
	require.Contains(t, err.Error(), missingPluginPath)

	require.Len(t, eventsFor(recorder, events.OperationLoad, events.PhaseError), 1)

	applyErrors := eventsFor(recorder, events.OperationApply, events.PhaseError)
	require.Len(t, applyErrors, 1)
	require.ErrorIs(t, applyErrors[0].Error, ErrPluginLoad)
}

// TestFirstDestroyFailsNamingMissingPlugin asserts plugins load before
// Destroy looks for saved state, so a missing plugin fails the first Destroy
// even when there is nothing to destroy
func TestFirstDestroyFailsNamingMissingPlugin(t *testing.T) {
	c, recorder := setupMissingPluginConfig(t)

	err := c.Destroy()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPluginLoad)
	require.Contains(t, err.Error(), missingPluginPath)

	require.Len(t, eventsFor(recorder, events.OperationLoad, events.PhaseError), 1)

	destroyErrors := eventsFor(recorder, events.OperationDestroy, events.PhaseError)
	require.Len(t, destroyErrors, 1)
	require.ErrorIs(t, destroyErrors[0].Error, ErrPluginLoad)
}

// TestSecondValidateReturnsTheSameLoadFailure asserts a failed load is kept,
// a later operation fails with it without trying to load again
func TestSecondValidateReturnsTheSameLoadFailure(t *testing.T) {
	c, recorder := setupMissingPluginConfig(t)
	path := writeConfigFile(t, variableOnlyConfig)

	err := c.Validate(path)
	require.ErrorIs(t, err, ErrPluginLoad)

	err = c.Validate(path)
	require.ErrorIs(t, err, ErrPluginLoad)

	require.Len(t, eventsFor(recorder, events.OperationLoad, events.PhaseStart), 1)
	require.Len(t, eventsFor(recorder, events.OperationValidate, events.PhaseError), 2)
}

func TestSharedRegistryStartsExternalPluginOnce(t *testing.T) {
	// built before HOME is isolated, so the build uses the real module cache
	binary := buildExamplePlugin(t)

	isolateHome(t)

	pr := registry.NewPluginRegistry()
	t.Cleanup(func() {
		for _, host := range pr.GetPluginHosts() {
			host.Stop()
		}
	})

	err := pr.RegisterPluginWithPath(binary)
	require.NoError(t, err)

	firstRecorder := &eventRecorder{}
	first := NewConfig(WithPluginRegistry(pr), WithEventHandler(firstRecorder.handle))

	secondRecorder := &eventRecorder{}
	second := NewConfig(WithPluginRegistry(pr), WithEventHandler(secondRecorder.handle))

	path := writeConfigFile(t, variableOnlyConfig)

	err = first.Validate(path)
	require.NoError(t, err)

	err = second.Validate(path)
	require.NoError(t, err)

	require.Len(t, pr.GetPluginHosts(), 1)

	loaded := append(
		eventsFor(firstRecorder, events.OperationLoad, events.PhaseSuccess),
		eventsFor(secondRecorder, events.OperationLoad, events.PhaseSuccess)...,
	)
	require.Len(t, loaded, 1)
	require.Equal(t, "person", loaded[0].Meta["block_types"])
}

// TestRegisterTypeAcceptsNameOfUnloadedPluginType asserts a name a plugin
// provides is accepted before the plugin has loaded, the clash is reported
// when it loads
func TestRegisterTypeAcceptsNameOfUnloadedPluginType(t *testing.T) {
	pr := registry.NewPluginRegistry()

	err := pr.RegisterPlugin(&parser.TestPlugin{})
	require.NoError(t, err)

	err = pr.RegisterType("network", &registered.Database{})
	require.NoError(t, err)
}

func TestFirstValidateFailsWithClashForPluginType(t *testing.T) {
	isolateHome(t)

	pr := registry.NewPluginRegistry()

	err := pr.RegisterPlugin(&parser.TestPlugin{})
	require.NoError(t, err)

	err = pr.RegisterType("network", &registered.Database{})
	require.NoError(t, err)

	c := NewConfig(WithPluginRegistry(pr))

	err = c.Validate(writeConfigFile(t, variableOnlyConfig))
	require.Error(t, err)

	var clash *registry.TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "network", clash.Name)
	require.Equal(t, "registered type", clash.Existing)
}

func TestValidateEmitsLoadEventsForRegisteredPlugin(t *testing.T) {
	recorder := &eventRecorder{}
	f := setupDeliveryConfig(t, singleNetworkConfig, WithEventHandler(recorder.handle))

	err := f.config.Validate(f.configFile)
	require.NoError(t, err)

	validateStarted := indexOf(recorder, func(e Event) bool {
		return e.Operation == events.OperationValidate && e.Phase == events.PhaseStart
	})
	loadStarted := indexOf(recorder, func(e Event) bool {
		return e.Operation == events.OperationLoad && e.Phase == events.PhaseStart
	})
	loadSucceeded := indexOf(recorder, func(e Event) bool {
		return e.Operation == events.OperationLoad && e.Phase == events.PhaseSuccess
	})

	require.Equal(t, 0, validateStarted)
	require.Greater(t, loadStarted, validateStarted)
	require.Greater(t, loadSucceeded, loadStarted)

	started := eventsFor(recorder, events.OperationLoad, events.PhaseStart)
	require.Len(t, started, 1)
	require.Equal(t, events.SourceCore, started[0].Source)
	require.Equal(t, "TestPlugin", started[0].Meta["plugin"])

	succeeded := eventsFor(recorder, events.OperationLoad, events.PhaseSuccess)
	require.Len(t, succeeded, 1)
	require.Equal(t, events.SourceCore, succeeded[0].Source)
	require.Equal(t, "TestPlugin", succeeded[0].Meta["plugin"])
	require.Equal(t, "container, sidecar, network, template", succeeded[0].Meta["block_types"])
}

// TestSecondValidateEmitsNoLoadEvents asserts plugins load once, the second
// operation on a Config uses the plugins the first loaded
func TestSecondValidateEmitsNoLoadEvents(t *testing.T) {
	recorder := &eventRecorder{}
	f := setupDeliveryConfig(t, singleNetworkConfig, WithEventHandler(recorder.handle))

	err := f.config.Validate(f.configFile)
	require.NoError(t, err)

	err = f.config.Validate(f.configFile)
	require.NoError(t, err)

	require.Len(t, eventsFor(recorder, events.OperationLoad, events.PhaseStart), 1)
	require.Len(t, eventsFor(recorder, events.OperationLoad, events.PhaseSuccess), 1)
}

func TestInProcessPluginInitLogsReachTheLoadingOperation(t *testing.T) {
	recorder := &eventRecorder{}
	f := setupDeliveryConfig(t, singleNetworkConfig, WithEventHandler(recorder.handle))
	f.plugin.SetLogOnInit(parser.LogMessage{Level: "info", Message: "plugin ready", Args: []any{"types", 4}})

	err := f.config.Validate(f.configFile)
	require.NoError(t, err)

	logged := logEvents(recorder)
	require.Len(t, logged, 1)
	require.Equal(t, "TestPlugin", logged[0].Source)
	require.Equal(t, events.OperationLoad, logged[0].Operation)
	require.Equal(t, events.LevelInfo, logged[0].Meta[events.KeyLevel])
	require.Equal(t, "plugin ready", logged[0].Meta[events.KeyMessage])
	require.Equal(t, 4, logged[0].Meta["types"])

	// it is written while the plugin loads, between its load start and success
	loadStarted := indexOf(recorder, func(e Event) bool {
		return e.Operation == events.OperationLoad && e.Phase == events.PhaseStart
	})
	initLogged := indexOf(recorder, func(e Event) bool {
		return e.Phase == events.PhaseLog
	})
	loadSucceeded := indexOf(recorder, func(e Event) bool {
		return e.Operation == events.OperationLoad && e.Phase == events.PhaseSuccess
	})

	require.Greater(t, initLogged, loadStarted)
	require.Greater(t, loadSucceeded, initLogged)
}

// moduleNetworkAddress is a module relative address whose split depends on
// whether "network", a type the TestPlugin provides, is known: the start of the
// body when it is, part of the module name when it is not
const moduleNetworkAddress = "module.web.network.one"

// setupUnloadedPluginConfig returns a Config whose registry holds the
// in-process TestPlugin, not yet loaded
func setupUnloadedPluginConfig(t *testing.T) *Config {
	t.Helper()

	isolateHome(t)

	pr := registry.NewPluginRegistry()

	err := pr.RegisterPlugin(&parser.TestPlugin{})
	require.NoError(t, err)

	return NewConfig(WithPluginRegistry(pr))
}

func TestAddressParserIsNotCachedBeforePluginsLoad(t *testing.T) {
	c := setupUnloadedPluginConfig(t)

	// a lookup before any operation resolves addresses before plugins load
	_, err := c.FindResource("resource.network.one")
	require.ErrorIs(t, err, ErrNotFound)

	require.Nil(t, c.addresses)
}

func TestAddressParserResolvesWithoutPluginTypesBeforePluginsLoad(t *testing.T) {
	c := setupUnloadedPluginConfig(t)

	fqrn, err := c.addressParser().Parse(moduleNetworkAddress)
	require.NoError(t, err)

	require.Equal(t, resources.TypeModule, fqrn.Type)
	require.Equal(t, "web.network", fqrn.Module)
	require.Equal(t, "one", fqrn.Resource)
}

func TestAddressParserKnowsPluginTypesAfterFirstOperation(t *testing.T) {
	c := setupUnloadedPluginConfig(t)

	// resolve an address before plugins load, then run the first operation
	_, err := c.FindResource("resource.network.one")
	require.ErrorIs(t, err, ErrNotFound)

	err = c.Validate(writeConfigFile(t, singleNetworkConfig))
	require.NoError(t, err)

	fqrn, err := c.addressParser().Parse(moduleNetworkAddress)
	require.NoError(t, err)

	require.Equal(t, "network", fqrn.Type)
	require.Equal(t, "web", fqrn.Module)
	require.Equal(t, "one", fqrn.Resource)

	require.NotNil(t, c.addresses)
}
