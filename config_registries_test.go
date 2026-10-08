package xcl

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/plugins/example/pkg/person"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/stretchr/testify/require"
)

// A Config gets its plugins from every registry given with WithRegistry,
// loading them on the first operation that needs them. Any plugin that fails
// to start, and any block type provided twice, fails that operation.

// badPluginName is the file name of a plugin binary that exits as soon as it
// starts
const badPluginName = "xcl-plugin-bad"

// namedRegistry gives a registry a name of its own, so two local registries
// can be told apart in events and errors
type namedRegistry struct {
	registry.Registry
	name string
}

func (r namedRegistry) Name() string {
	return r.name
}

// writeBadPlugin writes an executable script called name to dir that exits
// with an error as soon as it runs, and returns its path
func writeBadPlugin(t *testing.T, dir, name string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte("#!/bin/sh\nexit 1\n"), 0755)
	require.NoError(t, err)

	return path
}

// copyExecutable copies the executable at source to dir under name and
// returns the copy's path
func copyExecutable(t *testing.T, source, dir, name string) string {
	t.Helper()

	contents, err := os.ReadFile(source)
	require.NoError(t, err)

	path := filepath.Join(dir, name)
	err = os.WriteFile(path, contents, 0755)
	require.NoError(t, err)

	return path
}

// twoRegistriesFixture returns the absolute path of the fixture declaring a
// person and a network, block types from two different plugins
func twoRegistriesFixture(t *testing.T) string {
	t.Helper()

	path, err := filepath.Abs("./internal/test_fixtures/config/registries/two_registries/main.xcl")
	require.NoError(t, err)

	return path
}

// loadStarts returns the (plugin, registry) of every load start event the
// recorder received, in the order they were emitted
func loadStarts(recorder *eventRecorder) [][2]any {
	started := [][2]any{}
	for _, e := range eventsFor(recorder, events.OperationLoad, events.PhaseStart) {
		started = append(started, [2]any{e.Meta["plugin"], e.Meta["registry"]})
	}

	return started
}

func TestMissingPluginBinaryFailsFirstApplyNamingRegistry(t *testing.T) {
	isolateHome(t)

	local := registry.NewLocal()

	// registering a path that does not exist is not an error
	local.RegisterExternalPlugin(missingPluginPath)

	c, err := NewConfig(WithRegistry(local))
	require.NoError(t, err)

	err = c.Apply(writeConfigFile(t, variableOnlyConfig))
	require.ErrorIs(t, err, ErrPluginLoad)
	require.Contains(t, err.Error(), missingPluginPath)
	require.Contains(t, err.Error(), "local")

	var loadErr *PluginLoadError
	require.True(t, errors.As(err, &loadErr))
	require.Equal(t, "local", loadErr.Registry)
}

func TestPluginsFromTwoRegistriesAreUsable(t *testing.T) {
	isolateHome(t)

	people := registry.NewLocal()
	people.RegisterPlugin(&PersonPlugin{})

	networks := registry.NewLocal()
	networks.RegisterPlugin(&parser.TestPlugin{})

	c, err := NewConfig(WithRegistry(people), WithRegistry(networks))
	require.NoError(t, err)

	err = c.Apply(twoRegistriesFixture(t))
	require.NoError(t, err)

	ada, err := Find[person.Person](c, "resource.person.ada")
	require.NoError(t, err)
	require.Equal(t, "Ada", ada.FirstName)
	require.Equal(t, "person-ada-lovelace", ada.PersonID)

	network, err := Find[structs.Network](c, "resource.network.one")
	require.NoError(t, err)
	require.Equal(t, "10.0.1.0/24", network.Subnet)
}

func TestPluginLoadEventsFollowRegistryOrder(t *testing.T) {
	isolateHome(t)

	first := registry.NewLocal()
	first.RegisterPlugin(&parser.TestPlugin{})
	first.RegisterPlugin(&serverPlugin{})

	second := registry.NewLocal()
	second.RegisterPlugin(&PersonPlugin{})

	recorder := &eventRecorder{}

	c, err := NewConfig(
		WithRegistry(namedRegistry{Registry: first, name: "first"}),
		WithRegistry(namedRegistry{Registry: second, name: "second"}),
		WithEventHandler(recorder.Record),
	)
	require.NoError(t, err)

	err = c.Validate(writeConfigFile(t, variableOnlyConfig))
	require.NoError(t, err)

	require.Equal(t, [][2]any{
		{"TestPlugin", "first"},
		{"serverPlugin", "first"},
		{"PersonPlugin", "second"},
	}, loadStarts(recorder))
}

func TestFailingDiscoveredPluginFailsTheLoad(t *testing.T) {
	isolateHome(t)

	dir := t.TempDir()
	writeBadPlugin(t, dir, badPluginName)

	local := registry.NewLocal()
	local.RegisterPluginDirectory(dir)

	c, err := NewConfig(WithRegistry(local))
	require.NoError(t, err)
	stopPluginHosts(t, c)

	err = c.Apply(writeConfigFile(t, variableOnlyConfig))
	require.ErrorIs(t, err, ErrPluginLoad)

	var loadErr *PluginLoadError
	require.True(t, errors.As(err, &loadErr))
	require.Equal(t, badPluginName, loadErr.Plugin)
	require.Equal(t, "local", loadErr.Registry)
}

func TestFailingRegisteredPluginFailsTheLoad(t *testing.T) {
	isolateHome(t)

	path := writeBadPlugin(t, t.TempDir(), badPluginName)

	local := registry.NewLocal()
	local.RegisterExternalPlugin(path)

	c, err := NewConfig(WithRegistry(local))
	require.NoError(t, err)
	stopPluginHosts(t, c)

	err = c.Validate(writeConfigFile(t, variableOnlyConfig))
	require.ErrorIs(t, err, ErrPluginLoad)

	var loadErr *PluginLoadError
	require.True(t, errors.As(err, &loadErr))
	require.Equal(t, badPluginName, loadErr.Plugin)
	require.Equal(t, "local", loadErr.Registry)
}

func TestCustomPatternLoadsOnlyMatchingPlugins(t *testing.T) {
	// built before HOME is isolated, so the build uses the real module cache
	binary := buildSubtypelessPlugin(t)

	isolateHome(t)

	// the real plugin matches the custom pattern, the bad one only matches
	// the default pattern and so is never started
	dir := t.TempDir()
	copyExecutable(t, binary, dir, "acme-widget")
	writeBadPlugin(t, dir, badPluginName)

	local := registry.NewLocal(registry.PluginPattern("acme-*"))
	local.RegisterPluginDirectory(dir)

	recorder := &eventRecorder{}

	c, err := NewConfig(WithRegistry(local), WithEventHandler(recorder.Record))
	require.NoError(t, err)
	stopPluginHosts(t, c)

	err = c.Apply(subtypelessFixture(t, "subtypeless"))
	require.NoError(t, err)

	require.Equal(t, [][2]any{{"acme-widget", "local"}}, loadStarts(recorder))

	widget, err := Find[subtypelessWidget](c, "widget.main")
	require.NoError(t, err)
	require.Equal(t, 3, widget.Size)
	require.Equal(t, "wheel,axle", widget.PartNames)
}

func TestDuplicateTypeAcrossRegistriesFailsLoad(t *testing.T) {
	isolateHome(t)

	c, err := NewConfig(
		WithRegistry(namedRegistry{Registry: inProcessPersonRegistry(), name: "first"}),
		WithRegistry(namedRegistry{Registry: inProcessPersonRegistry(), name: "second"}),
	)
	require.NoError(t, err)

	err = c.Validate(writeConfigFile(t, variableOnlyConfig))
	require.Error(t, err)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "resource.person", clash.Name)
	require.Equal(t, inProcessPersonPluginName, clash.Provider)
	require.Equal(t, "second", clash.Registry)
	require.Equal(t, inProcessPersonPluginName, clash.Existing)
	require.Equal(t, "first", clash.ExistingRegistry)
}

func TestDuplicateTypeWithinRegistryFailsLoad(t *testing.T) {
	isolateHome(t)

	local := registry.NewLocal()
	local.RegisterPlugin(&PersonPlugin{})
	local.RegisterPlugin(&PersonPlugin{})

	c, err := NewConfig(WithRegistry(local))
	require.NoError(t, err)

	err = c.Validate(writeConfigFile(t, variableOnlyConfig))
	require.Error(t, err)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "resource.person", clash.Name)
	require.Equal(t, inProcessPersonPluginName, clash.Provider)
	require.Equal(t, "local", clash.Registry)
	require.Equal(t, inProcessPersonPluginName, clash.Existing)
	require.Equal(t, "local", clash.ExistingRegistry)
}

func TestPluginTypeClashingWithDeclaredTypeFailsLoad(t *testing.T) {
	isolateHome(t)

	c, err := NewConfig(
		WithType(&registered.Database{}, "resource", "person"),
		WithRegistry(inProcessPersonRegistry()),
	)
	require.NoError(t, err)

	err = c.Validate(writeConfigFile(t, variableOnlyConfig))
	require.Error(t, err)
	require.Contains(t, err.Error(), inProcessPersonPluginName)
	require.Contains(t, err.Error(), `"resource.person"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "resource.person", clash.Name)
	require.Equal(t, inProcessPersonPluginName, clash.Provider)
	require.Equal(t, "local", clash.Registry)
	require.Equal(t, "type *registered.Database", clash.Existing)
	require.Empty(t, clash.ExistingRegistry)
}

func TestPluginTypeClashingWithTypeDeclaredAfterRegistryFailsLoad(t *testing.T) {
	isolateHome(t)

	c, err := NewConfig(
		WithRegistry(inProcessPersonRegistry()),
		WithType(&registered.Database{}, "resource", "person"),
	)
	require.NoError(t, err)

	err = c.Validate(writeConfigFile(t, variableOnlyConfig))
	require.Error(t, err)
	require.Contains(t, err.Error(), inProcessPersonPluginName)
	require.Contains(t, err.Error(), `"resource.person"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "resource.person", clash.Name)
	require.Equal(t, inProcessPersonPluginName, clash.Provider)
	require.Equal(t, "local", clash.Registry)
	require.Equal(t, "type *registered.Database", clash.Existing)
	require.Empty(t, clash.ExistingRegistry)
}
