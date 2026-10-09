package xcl

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// A plugin may register a block type with no subtype, written
// widget "<name>" {} and addressed widget.<name>. These tests apply the
// subtypeless fixture through an in-process plugin and through the external
// plugin in internal/test_fixtures/plugins/subtypeless, which share the shape
// below.

// subtypelessWidget matches the fixture plugin's Widget, it is the in-process
// plugin's type and the type an external widget is read back into
type subtypelessWidget struct {
	types.ResourceBase `xcl:",remain"`

	Size  int               `xcl:"size,optional" json:"size,omitempty"`
	Parts []subtypelessPart `xcl:"part,block" json:"part,omitempty"`

	PartNames string `xcl:"part_names,optional,computed" json:"part_names,omitempty"`
}

// subtypelessPart is the nested part block of a widget
type subtypelessPart struct {
	Name string `xcl:"name" json:"name"`
}

// subtypelessWidgetProvider joins the names of a widget's parts into
// part_names on create and records the address of every widget it destroys
type subtypelessWidgetProvider struct {
	plugins.DefaultChanged[*subtypelessWidget]

	mu        sync.Mutex
	destroyed []string
}

func (p *subtypelessWidgetProvider) Init(state plugins.State, functions plugins.ProviderFunctions, log logger.Logger) error {
	log.Debug("provider ready")
	return nil
}

func (p *subtypelessWidgetProvider) Create(ctx context.Context, w *subtypelessWidget) (*subtypelessWidget, error) {
	names := []string{}
	for _, part := range w.Parts {
		names = append(names, part.Name)
	}

	w.PartNames = strings.Join(names, ",")
	return w, nil
}

func (p *subtypelessWidgetProvider) Destroy(ctx context.Context, w *subtypelessWidget, force bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.destroyed = append(p.destroyed, w.Meta.ID)
	return nil
}

func (p *subtypelessWidgetProvider) Read(ctx context.Context, old *subtypelessWidget, new *subtypelessWidget) (*subtypelessWidget, error) {
	return new, nil
}

func (p *subtypelessWidgetProvider) Update(ctx context.Context, w *subtypelessWidget, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*subtypelessWidget, error) {
	return w, nil
}

func (p *subtypelessWidgetProvider) Functions() plugins.ProviderFunctions {
	return nil
}

// destroyedWidgets returns the addresses of the widgets destroyed so far
func (p *subtypelessWidgetProvider) destroyedWidgets() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]string{}, p.destroyed...)
}

// SubtypelessWidgetPlugin is an in-process plugin that registers widget with
// no subtype
type SubtypelessWidgetPlugin struct {
	plugins.PluginBase

	provider *subtypelessWidgetProvider
}

func (p *SubtypelessWidgetPlugin) Init(l logger.Logger, s plugins.State) error {
	return plugins.RegisterResourceProvider(&p.PluginBase, l, s, "widget", "", &subtypelessWidget{}, p.provider)
}

// subtypelessFixture returns the absolute path of the main.xcl of the named
// fixture directory under internal/test_fixtures/config
func subtypelessFixture(t *testing.T, name string) string {
	t.Helper()

	path, err := filepath.Abs(filepath.Join("internal", "test_fixtures", "config", name, "main.xcl"))
	require.NoError(t, err)

	return path
}

// inProcessWidgetRegistry returns a registry holding a
// SubtypelessWidgetPlugin backed by provider
func inProcessWidgetRegistry(provider *subtypelessWidgetProvider) *registry.Local {
	local := registry.NewLocal()
	local.RegisterPlugin(&SubtypelessWidgetPlugin{provider: provider})

	return local
}

// buildSubtypelessPlugin builds the external subtypeless fixture plugin into
// a temporary directory and returns the path of the binary
func buildSubtypelessPlugin(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "xcl-plugin-subtypeless")

	build := exec.Command("go", "build", "-o", binary, "./internal/test_fixtures/plugins/subtypeless")
	output, err := build.CombinedOutput()
	require.NoError(t, err, "unable to build the subtypeless plugin: %s", output)

	return binary
}

// externalWidgetRegistry returns a registry holding the external subtypeless
// plugin binary
func externalWidgetRegistry(binary string) *registry.Local {
	local := registry.NewLocal()
	local.RegisterExternalPlugin(binary)

	return local
}

func TestApplyCreatesInProcessPluginTypeWithoutSubtype(t *testing.T) {
	isolateHome(t)

	provider := &subtypelessWidgetProvider{}
	f := newPersonConfig(t, inProcessWidgetRegistry(provider), subtypelessFixture(t, "subtypeless"), t.TempDir())

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	widget, err := Find[subtypelessWidget](f.config, "widget.main")
	require.NoError(t, err)
	require.Equal(t, "widget.main", widget.Meta.ID)
	require.Equal(t, "widget", widget.Meta.Type)
	require.Equal(t, "main", widget.Meta.Name)
	require.Equal(t, 3, widget.Size)
	require.Equal(t, []subtypelessPart{{Name: "wheel"}, {Name: "axle"}}, widget.Parts)
	require.Equal(t, "wheel,axle", widget.PartNames)
}

func TestDestroyRemovesInProcessPluginTypeWithoutSubtype(t *testing.T) {
	isolateHome(t)

	provider := &subtypelessWidgetProvider{}
	f := newPersonConfig(t, inProcessWidgetRegistry(provider), subtypelessFixture(t, "subtypeless"), t.TempDir())

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	err = f.config.Destroy()
	require.NoError(t, err)

	require.Equal(t, []string{"widget.main"}, provider.destroyedWidgets())
	require.Equal(t, 0, f.config.ResourceCount())

	_, err = Find[subtypelessWidget](f.config, "widget.main")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestInProcessPluginTypeWithoutSubtypeRejectsSubtypeLabel(t *testing.T) {
	isolateHome(t)

	provider := &subtypelessWidgetProvider{}
	f := newPersonConfig(t, inProcessWidgetRegistry(provider), subtypelessFixture(t, "subtypeless_with_subtype"), t.TempDir())

	err := f.config.Apply(f.configFile)
	require.ErrorContains(t, err, "invalid format for 'widget', it is declared with only a name")
	require.Equal(t, 0, f.config.ResourceCount())
}

func TestInProcessPluginTypeWithoutSubtypeTagsInitLogsWithType(t *testing.T) {
	isolateHome(t)

	recorder := &eventRecorder{}
	provider := &subtypelessWidgetProvider{}
	f := newPersonConfig(t, inProcessWidgetRegistry(provider), subtypelessFixture(t, "subtypeless"), t.TempDir(), WithEventHandler(recorder.Record))

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	ready := pluginLogsWithMessage(recorder, "provider ready")
	require.Len(t, ready, 1)
	require.Equal(t, "SubtypelessWidgetPlugin", ready[0].Source)
	require.Equal(t, "widget", ready[0].Meta["provider"])
}

func TestApplyCreatesExternalPluginTypeWithoutSubtype(t *testing.T) {
	// built before HOME is isolated, so the build uses the real module cache
	binary := buildSubtypelessPlugin(t)
	isolateHome(t)

	f := newPersonConfig(t, externalWidgetRegistry(binary), subtypelessFixture(t, "subtypeless"), t.TempDir())

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	widget, err := Find[subtypelessWidget](f.config, "widget.main")
	require.NoError(t, err)
	require.Equal(t, "widget.main", widget.Meta.ID)
	require.Equal(t, "widget", widget.Meta.Type)
	require.Equal(t, "main", widget.Meta.Name)
	require.Equal(t, 3, widget.Size)
	require.Equal(t, []subtypelessPart{{Name: "wheel"}, {Name: "axle"}}, widget.Parts)

	// the plugin computes part_names from the parts it received, so this
	// proves the repeated nested block crossed the process boundary
	require.Equal(t, "wheel,axle", widget.PartNames)
}

func TestDestroyRemovesExternalPluginTypeWithoutSubtype(t *testing.T) {
	binary := buildSubtypelessPlugin(t)
	isolateHome(t)

	recorder := &eventRecorder{}
	f := newPersonConfig(t, externalWidgetRegistry(binary), subtypelessFixture(t, "subtypeless"), t.TempDir(), WithEventHandler(recorder.Record))

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	err = f.config.Destroy()
	require.NoError(t, err)

	destroyed := pluginLogsWithMessage(recorder, "destroyed widget")
	require.Len(t, destroyed, 1)
	require.Equal(t, "widget.main", destroyed[0].ResourceID)
	require.Equal(t, events.OperationDestroy, destroyed[0].Operation)

	require.Equal(t, 0, f.config.ResourceCount())

	_, err = Find[subtypelessWidget](f.config, "widget.main")
	require.ErrorIs(t, err, ErrNotFound)
}
