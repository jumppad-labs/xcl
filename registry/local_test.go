package registry

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// Thing is a plain Go resource type provided by thingPlugin
type Thing struct {
	types.ResourceBase `xcl:",remain"`

	Size int `xcl:"size" json:"size"`
}

// thingProvider is a no-op provider for Thing resources
type thingProvider struct {
	plugins.DefaultChanged[*Thing]
}

var _ plugins.ResourceProvider[*Thing] = (*thingProvider)(nil)

func (p *thingProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
	return nil
}

func (p *thingProvider) Create(ctx context.Context, resource *Thing) (*Thing, error) {
	return resource, nil
}

func (p *thingProvider) Destroy(ctx context.Context, resource *Thing, force bool) error {
	return nil
}

func (p *thingProvider) Read(ctx context.Context, old *Thing, new *Thing) (*Thing, error) {
	return new, nil
}

func (p *thingProvider) Update(ctx context.Context, resource *Thing, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*Thing, error) {
	return resource, nil
}

func (p *thingProvider) Functions() plugins.ProviderFunctions {
	return nil
}

// thingPlugin is an in-process plugin that provides the resource type "thing"
type thingPlugin struct {
	plugins.PluginBase
}

var _ plugins.Plugin = (*thingPlugin)(nil)

func (p *thingPlugin) Init(logger logger.Logger, state plugins.State) error {
	return plugins.RegisterResourceProvider(&p.PluginBase, logger, state, "resource", "thing", &Thing{}, &thingProvider{})
}

// writeExecutable writes a fake executable file named name into dir and
// returns its path. It is never started, registries only list it.
func writeExecutable(t *testing.T, dir, name string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0755)
	require.NoError(t, err)

	return path
}

// writeNonExecutable writes a file without execute permission named name
// into dir and returns its path
func writeNonExecutable(t *testing.T, dir, name string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte("not an executable"), 0644)
	require.NoError(t, err)

	return path
}

// pluginNames returns the Name of every plugin, in order
func pluginNames(found []Plugin) []string {
	names := []string{}
	for _, p := range found {
		names = append(names, p.Name())
	}

	return names
}

// discoverLifecycleEvents returns the discover events in recorded that are
// not log messages
func discoverLifecycleEvents(recorded []events.Event) []events.Event {
	found := []events.Event{}
	for _, e := range recorded {
		if e.Operation == events.OperationDiscover && e.Phase != events.PhaseLog {
			found = append(found, e)
		}
	}

	return found
}

func TestLocalPluginsKeepRegistrationOrder(t *testing.T) {
	pluginDir := t.TempDir()
	writeExecutable(t, pluginDir, "xcl-plugin-alpha")
	writeExecutable(t, pluginDir, "xcl-plugin-beta")

	local := NewLocal()
	local.RegisterExternalPlugin("/opt/plugins/xcl-plugin-first")
	local.RegisterPluginDirectory(pluginDir)
	local.RegisterPlugin(&thingPlugin{})
	local.RegisterExternalPlugin("/opt/plugins/xcl-plugin-last")

	found, err := local.Plugins(context.Background(), nil)
	require.NoError(t, err)

	require.Equal(t, []string{
		"xcl-plugin-first",
		"xcl-plugin-alpha",
		"xcl-plugin-beta",
		"thingPlugin",
		"xcl-plugin-last",
	}, pluginNames(found))
}

func TestLocalCustomPatternFindsOnlyMatchingExecutables(t *testing.T) {
	pluginDir := t.TempDir()
	writeExecutable(t, pluginDir, "acme-plugin-a")
	writeExecutable(t, pluginDir, "xcl-plugin-b")
	writeNonExecutable(t, pluginDir, "acme-plugin-c")

	local := NewLocal(PluginPattern("acme-plugin-*"))
	local.RegisterPluginDirectory(pluginDir)

	found, err := local.Plugins(context.Background(), nil)
	require.NoError(t, err)

	require.Equal(t, []string{"acme-plugin-a"}, pluginNames(found))
}

func TestLocalDefaultPatternIsXclPlugin(t *testing.T) {
	pluginDir := t.TempDir()
	writeExecutable(t, pluginDir, "xcl-plugin-b")
	writeExecutable(t, pluginDir, "acme-plugin-a")

	local := NewLocal()
	local.RegisterPluginDirectory(pluginDir)

	found, err := local.Plugins(context.Background(), nil)
	require.NoError(t, err)

	require.Equal(t, "xcl-plugin-*", DefaultPluginPattern)
	require.Equal(t, []string{"xcl-plugin-b"}, pluginNames(found))
}

func TestLocalEmptyPluginPatternKeepsDefault(t *testing.T) {
	pluginDir := t.TempDir()
	writeExecutable(t, pluginDir, "xcl-plugin-b")
	writeExecutable(t, pluginDir, "acme-plugin-a")

	local := NewLocal(PluginPattern(""))
	local.RegisterPluginDirectory(pluginDir)

	found, err := local.Plugins(context.Background(), nil)
	require.NoError(t, err)

	require.Equal(t, []string{"xcl-plugin-b"}, pluginNames(found))
}

func TestLocalRegisterNilPluginPanics(t *testing.T) {
	local := NewLocal()

	require.PanicsWithValue(t, "xcl: registry local: in-process plugin must not be nil", func() {
		local.RegisterPlugin(nil)
	})
}

// Gadget is a second plain Go resource type, declared alongside Thing
type Gadget struct {
	types.ResourceBase `xcl:",remain"`

	Colour string `xcl:"colour" json:"colour"`
}

func TestLocalTypesKeepRegistrationOrder(t *testing.T) {
	local := NewLocal()
	local.RegisterType(&Thing{}, "resource", "thing")
	local.RegisterType(&Gadget{}, "gadget")
	local.RegisterType(&Thing{}, "server", "big")

	declared := local.Types()

	require.Len(t, declared, 3)
	require.Equal(t, Type{Type: "resource", Subtype: "thing", Prototype: &Thing{}}, declared[0])
	require.Equal(t, Type{Type: "gadget", Prototype: &Gadget{}}, declared[1])
	require.Equal(t, Type{Type: "server", Subtype: "big", Prototype: &Thing{}}, declared[2])
}

// A registry with no Go types returns nil, as the Registry interface asks
func TestLocalTypesIsEmptyWithoutRegistrations(t *testing.T) {
	local := NewLocal()
	local.RegisterPlugin(&thingPlugin{})

	require.Nil(t, local.Types())
}

// A clash between declarations is returned by NewConfig, which sees every
// registry, so registering the same type twice only records it twice
func TestLocalRegisterDuplicateTypeDoesNotPanic(t *testing.T) {
	local := NewLocal()

	require.NotPanics(t, func() {
		local.RegisterType(&Thing{}, "resource", "thing")
		local.RegisterType(&Gadget{}, "resource", "thing")
	})

	require.Len(t, local.Types(), 2)
}

func TestLocalRegisterTypePanicsOnEmptyName(t *testing.T) {
	local := NewLocal()

	require.PanicsWithValue(t, "xcl: registry local: an entity type must be named", func() {
		local.RegisterType(&Thing{}, "")
	})
}

func TestLocalRegisterTypePanicsOnMoreThanOneSubtype(t *testing.T) {
	local := NewLocal()

	require.PanicsWithValue(t, `xcl: registry local: type "server" takes at most one subtype, got 2`, func() {
		local.RegisterType(&Thing{}, "server", "big", "small")
	})
}

func TestLocalRegisterTypePanicsOnEmptySubtype(t *testing.T) {
	local := NewLocal()

	require.PanicsWithValue(t,
		`xcl: registry local: type "server" was given an empty subtype, leave it out to register the type without one`,
		func() {
			local.RegisterType(&Thing{}, "server", "")
		},
	)
}

func TestLocalRegisterTypePanicsOnNonEntityType(t *testing.T) {
	local := NewLocal()

	require.PanicsWithValue(t,
		`xcl: registry local: type "resource.thing" must be a pointer to a struct that embeds types.ResourceBase`,
		func() {
			local.RegisterType(Thing{}, "resource", "thing")
		},
	)
}

func TestLocalRegisterTypePanicRecordsNothing(t *testing.T) {
	local := NewLocal()

	require.Panics(t, func() {
		local.RegisterType(&Thing{}, "")
	})

	require.Nil(t, local.Types())
}

func TestLocalRegisterMissingPathDoesNotFail(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "xcl-plugin-missing")

	local := NewLocal()
	require.NotPanics(t, func() {
		local.RegisterExternalPlugin(missing)
	})

	found, err := local.Plugins(context.Background(), nil)
	require.NoError(t, err)

	require.Equal(t, []string{"xcl-plugin-missing"}, pluginNames(found))
}

func TestLocalMissingPluginDirectoryProvidesNoPlugins(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-dir")

	local := NewLocal()
	local.RegisterPluginDirectory(missing)

	found, err := local.Plugins(context.Background(), nil)
	require.NoError(t, err)

	require.Empty(t, found)
}

func TestLocalExpandsHomeInPluginDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	pluginDir := filepath.Join(home, "plugins")
	err := os.MkdirAll(pluginDir, 0755)
	require.NoError(t, err)
	writeExecutable(t, pluginDir, "xcl-plugin-home")

	local := NewLocal()
	local.RegisterPluginDirectory("~/plugins")

	found, err := local.Plugins(context.Background(), nil)
	require.NoError(t, err)

	require.Equal(t, []string{"xcl-plugin-home"}, pluginNames(found))
}

func TestLocalExpandsEnvironmentVariableInPluginDirectory(t *testing.T) {
	pluginDir := t.TempDir()
	t.Setenv("XCL_TEST_PLUGIN_DIR", pluginDir)
	writeExecutable(t, pluginDir, "xcl-plugin-env")

	local := NewLocal()
	local.RegisterPluginDirectory("$XCL_TEST_PLUGIN_DIR")

	found, err := local.Plugins(context.Background(), nil)
	require.NoError(t, err)

	require.Equal(t, []string{"xcl-plugin-env"}, pluginNames(found))
}

func TestLocalNameIsLocal(t *testing.T) {
	local := NewLocal()

	require.Equal(t, "local", LocalName)
	require.Equal(t, "local", local.Name())
}

func TestLocalIsARegistry(t *testing.T) {
	var registry Registry = NewLocal()

	require.NotNil(t, registry)
}

func TestLocalEmitsDiscoverEventsNamingTheRegistry(t *testing.T) {
	pluginDir := t.TempDir()
	writeExecutable(t, pluginDir, "xcl-plugin-one")
	writeExecutable(t, pluginDir, "xcl-plugin-two")

	recorder := &testutil.EventRecorder{}

	local := NewLocal()
	local.RegisterPluginDirectory(pluginDir)

	_, err := local.Plugins(context.Background(), recorder.Record)
	require.NoError(t, err)

	discovered := discoverLifecycleEvents(recorder.Events())
	require.Len(t, discovered, 2)

	require.Equal(t, events.SourceCore, discovered[0].Source)
	require.Equal(t, events.PhaseStart, discovered[0].Phase)
	require.Equal(t, map[string]any{
		"dirs":     []string{pluginDir},
		"registry": "local",
	}, discovered[0].Meta)

	require.Equal(t, events.SourceCore, discovered[1].Source)
	require.Equal(t, events.PhaseSuccess, discovered[1].Phase)
	require.Equal(t, map[string]any{
		"dirs":     []string{pluginDir},
		"registry": "local",
		"count":    2,
	}, discovered[1].Meta)
}

func TestLocalEmitsDiscoverErrorEventWhenDirectoryCannotBeSearched(t *testing.T) {
	notADir := writeNonExecutable(t, t.TempDir(), "a-file")

	recorder := &testutil.EventRecorder{}

	local := NewLocal()
	local.RegisterPluginDirectory(notADir)

	found, err := local.Plugins(context.Background(), recorder.Record)
	require.Error(t, err)
	require.ErrorContains(t, err, "plugin discovery failed")
	require.Nil(t, found)

	discovered := discoverLifecycleEvents(recorder.Events())
	require.Len(t, discovered, 2)

	require.Equal(t, events.PhaseStart, discovered[0].Phase)

	require.Equal(t, events.SourceCore, discovered[1].Source)
	require.Equal(t, events.PhaseError, discovered[1].Phase)
	require.Error(t, discovered[1].Error)
	require.Equal(t, map[string]any{
		"dirs":     []string{notADir},
		"registry": "local",
	}, discovered[1].Meta)
}

func TestLocalPluginsWithANilEmitDoesNotPanic(t *testing.T) {
	local := NewLocal()
	local.RegisterPluginDirectory(t.TempDir())

	require.NotPanics(t, func() {
		_, err := local.Plugins(context.Background(), nil)
		require.NoError(t, err)
	})
}

func TestInProcessPanicsOnNil(t *testing.T) {
	require.PanicsWithValue(t, "xcl: registry: in-process plugin must not be nil", func() {
		InProcess(nil)
	})
}

func TestInProcessNameIsTheGoTypeName(t *testing.T) {
	plugin := InProcess(&thingPlugin{})

	require.Equal(t, "thingPlugin", plugin.Name())
}

func TestInProcessStartReturnsHostWithPluginTypes(t *testing.T) {
	plugin := InProcess(&thingPlugin{})

	host, err := plugin.Start(nil)
	require.NoError(t, err)
	require.NotNil(t, host)

	registered := host.GetTypes()
	require.Len(t, registered, 1)
	require.Equal(t, "resource", registered[0].Type)
	require.Equal(t, "thing", registered[0].SubType)
}

func TestExecutableNameIsTheBinaryName(t *testing.T) {
	plugin := Executable("/opt/plugins/xcl-plugin-docker")

	require.Equal(t, "xcl-plugin-docker", plugin.Name())
}

func TestExecutableStartFailsForMissingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "xcl-plugin-missing")

	host, err := Executable(missing).Start(nil)
	require.Error(t, err)
	require.ErrorContains(t, err, missing)
	require.Nil(t, host)
}
