package xcl

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// A plugin's providers may offer functions to configuration, called by their
// bare names as a builtin is. Configuration calls them through the plugin
// host, in process or over gRPC, and the plugin's providers see the value a
// call produced, never the call itself. These tests apply the greeting
// fixture through an in-process plugin and through the external plugin in
// internal/test_fixtures/plugins/functions, which share the shape below.

// greetingEntity matches the fixture plugin's Greeting, it is the in-process
// plugin's type and the type an external greeting is read back into
type greetingEntity struct {
	types.ResourceBase `xcl:",remain"`

	// Target is the value to greet, configuration sets it from greet(...)
	Target string `xcl:"target" json:"target"`

	// Received is computed, the provider copies the target it was given
	Received string `xcl:"received,optional,computed" json:"received,omitempty"`
}

// greetingFunctionProvider offers the functions it is built with, copies
// the target of every greeting it creates into received, and records the
// target of every greeting it was asked to create, so a test can see the
// value the plugin received
type greetingFunctionProvider struct {
	plugins.DefaultChanged[*greetingEntity]

	functions plugins.ProviderFunctions

	mu      sync.Mutex
	created []string
}

// newGreetingFunctionProvider returns a provider offering fns
func newGreetingFunctionProvider(fns map[string]plugins.ProviderFunction) *greetingFunctionProvider {
	return &greetingFunctionProvider{functions: plugins.NewFunctions(fns)}
}

func (p *greetingFunctionProvider) Init(state plugins.State, functions plugins.ProviderFunctions, log logger.Logger) error {
	return nil
}

func (p *greetingFunctionProvider) Create(ctx context.Context, g *greetingEntity) (*greetingEntity, error) {
	p.mu.Lock()
	p.created = append(p.created, g.Target)
	p.mu.Unlock()

	g.Received = g.Target
	return g, nil
}

func (p *greetingFunctionProvider) Destroy(ctx context.Context, g *greetingEntity, force bool) error {
	return nil
}

func (p *greetingFunctionProvider) Read(ctx context.Context, old *greetingEntity, new *greetingEntity) (*greetingEntity, error) {
	return new, nil
}

func (p *greetingFunctionProvider) Update(ctx context.Context, g *greetingEntity, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*greetingEntity, error) {
	g.Received = g.Target
	return g, nil
}

func (p *greetingFunctionProvider) Functions() plugins.ProviderFunctions {
	return p.functions
}

// createdTargets returns the target of every greeting Create received, in
// order
func (p *greetingFunctionProvider) createdTargets() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]string{}, p.created...)
}

// greetFunction returns the function greet, which prefixes its argument
// with hello, the same as the fixture plugin's
func greetFunction(t *testing.T) plugins.ProviderFunction {
	t.Helper()

	greet, err := plugins.NewFunction(func(name string) (string, error) {
		return "hello " + name, nil
	})
	require.NoError(t, err)

	return greet
}

// upperFunction returns a function named like the builtin upper
func upperFunction(t *testing.T) plugins.ProviderFunction {
	t.Helper()

	upper, err := plugins.NewFunction(func(s string) (string, error) {
		return strings.ToUpper(s), nil
	})
	require.NoError(t, err)

	return upper
}

// GreetingFunctionPlugin is an in-process plugin that registers greeting
// with no subtype, through a provider offering greet
type GreetingFunctionPlugin struct {
	plugins.PluginBase

	provider *greetingFunctionProvider
}

func (p *GreetingFunctionPlugin) Init(l logger.Logger, s plugins.State) error {
	return plugins.RegisterResourceProvider(&p.PluginBase, l, s, "greeting", "", &greetingEntity{}, p.provider)
}

// UpperFunctionPlugin is an in-process plugin that registers greeting with
// no subtype, through a provider offering upper, the name of a builtin
type UpperFunctionPlugin struct {
	plugins.PluginBase

	provider *greetingFunctionProvider
}

func (p *UpperFunctionPlugin) Init(l logger.Logger, s plugins.State) error {
	return plugins.RegisterResourceProvider(&p.PluginBase, l, s, "greeting", "", &greetingEntity{}, p.provider)
}

// greetingFunctionConfig sets a greeting's target from the plugin function
// greet
const greetingFunctionConfig = `
greeting "hi" {
  target = greet("Nic")
}
`

// inProcessGreetingRegistry returns a registry holding a
// GreetingFunctionPlugin backed by provider
func inProcessGreetingRegistry(provider *greetingFunctionProvider) *registry.Local {
	local := registry.NewLocal()
	local.RegisterPlugin(&GreetingFunctionPlugin{provider: provider})

	return local
}

// Criterion 1: configuration calls a plugin's function and the entity holds
// the result, for a plugin registered in process and for a compiled one
// called over gRPC. Criterion 2: the plugin's create step computes received
// from the target it was given, so received holds the result too.

func TestApplyCallsInProcessPluginFunction(t *testing.T) {
	isolateHome(t)

	provider := newGreetingFunctionProvider(map[string]plugins.ProviderFunction{"greet": greetFunction(t)})
	f := setupPersonConfig(t, inProcessGreetingRegistry(provider), t.TempDir(), greetingFunctionConfig)

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	greeting, err := Find[greetingEntity](f.config, "greeting.hi")
	require.NoError(t, err)
	require.Equal(t, "greeting.hi", greeting.Meta.ID)
	require.Equal(t, "hello Nic", greeting.Target)
	require.Equal(t, "hello Nic", greeting.Received)
}

func TestApplyCallsExternalPluginFunction(t *testing.T) {
	// built before HOME is isolated, so the build uses the real module cache
	binary := testutil.BuildFunctionFixturePlugin(t, ".")
	isolateHome(t)

	local := registry.NewLocal()
	local.RegisterExternalPlugin(binary)
	f := setupPersonConfig(t, local, t.TempDir(), greetingFunctionConfig)

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	greeting, err := Find[greetingEntity](f.config, "greeting.hi")
	require.NoError(t, err)
	require.Equal(t, "greeting.hi", greeting.Meta.ID)
	require.Equal(t, "hello Nic", greeting.Target)

	// the plugin copies the target it received into received, so this
	// proves the function's result crossed the process boundary as a value
	require.Equal(t, "hello Nic", greeting.Received)
}

// Criterion 2: the plugin's create step receives the function's result,
// never the text of the call.

func TestPluginCreateReceivesFunctionResult(t *testing.T) {
	isolateHome(t)

	provider := newGreetingFunctionProvider(map[string]plugins.ProviderFunction{"greet": greetFunction(t)})
	f := setupPersonConfig(t, inProcessGreetingRegistry(provider), t.TempDir(), greetingFunctionConfig)

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	require.Equal(t, []string{"hello Nic"}, provider.createdTargets())
}

// Criterion 3: an application function named like a loaded plugin's function
// makes the first operation fail, naming the function, the application
// function and the plugin.

func TestFirstOperationReportsApplicationAndPluginFunctionClash(t *testing.T) {
	binary := testutil.BuildFunctionFixturePlugin(t, ".")
	isolateHome(t)

	local := registry.NewLocal()
	local.RegisterFunction("greet", func(name string) (string, error) { return "application " + name, nil })
	local.RegisterExternalPlugin(binary)

	// the clash is only known once the plugin loads, so NewConfig succeeds
	f := setupPersonConfig(t, local, t.TempDir(), variableOnlyConfig)

	err := f.config.Validate(f.configFile)
	require.Error(t, err)

	var clash *FunctionNameClashError
	require.True(t, errors.As(err, &clash), "expected a *FunctionNameClashError, got %T: %v", err, err)
	require.Equal(t, "greet", clash.Name)
	require.Equal(t, testutil.FunctionFixturePluginName, clash.Provider)
	require.Equal(t, "local", clash.Registry)
	require.True(t, strings.HasPrefix(clash.Existing, "function func("), "Existing should name the Go func, got %q", clash.Existing)
	require.Equal(t, "local", clash.ExistingRegistry)

	require.Contains(t, err.Error(), `"greet"`)
	require.Contains(t, err.Error(), "function func(string) (string, error)")
	require.Contains(t, err.Error(), testutil.FunctionFixturePluginName)
}

// Criterion 4: two loaded plugins offering the same function name make the
// first operation fail, naming both plugins and their registries.

func TestFirstOperationReportsFunctionClashBetweenPlugins(t *testing.T) {
	greeting := testutil.BuildFunctionFixturePlugin(t, ".")
	salute := testutil.BuildFunctionClashFixturePlugin(t, ".")
	isolateHome(t)

	local := registry.NewLocal()
	local.RegisterExternalPlugin(greeting)
	local.RegisterExternalPlugin(salute)

	f := setupPersonConfig(t, local, t.TempDir(), variableOnlyConfig)

	err := f.config.Validate(f.configFile)
	require.Error(t, err)

	var clash *FunctionNameClashError
	require.True(t, errors.As(err, &clash), "expected a *FunctionNameClashError, got %T: %v", err, err)
	require.Equal(t, "greet", clash.Name)
	require.Equal(t, testutil.FunctionClashFixturePluginName, clash.Provider)
	require.Equal(t, "local", clash.Registry)
	require.Equal(t, testutil.FunctionFixturePluginName, clash.Existing)
	require.Equal(t, "local", clash.ExistingRegistry)

	require.Contains(t, err.Error(), `"greet"`)
	require.Contains(t, err.Error(), testutil.FunctionFixturePluginName)
	require.Contains(t, err.Error(), testutil.FunctionClashFixturePluginName)
	require.Contains(t, err.Error(), "registry local")
}

// Criterion 5: a plugin function named like a builtin fails the plugin's
// load, naming the builtin and the plugin.

func TestFirstOperationReportsPluginFunctionClashWithBuiltin(t *testing.T) {
	isolateHome(t)

	provider := newGreetingFunctionProvider(map[string]plugins.ProviderFunction{"upper": upperFunction(t)})
	local := registry.NewLocal()
	local.RegisterPlugin(&UpperFunctionPlugin{provider: provider})

	f := setupPersonConfig(t, local, t.TempDir(), variableOnlyConfig)

	err := f.config.Validate(f.configFile)
	require.Error(t, err)

	var clash *FunctionNameClashError
	require.True(t, errors.As(err, &clash), "expected a *FunctionNameClashError, got %T: %v", err, err)
	require.Equal(t, "upper", clash.Name)
	require.Equal(t, "UpperFunctionPlugin", clash.Provider)
	require.Equal(t, "local", clash.Registry)
	require.Equal(t, "builtin", clash.Existing)
	require.Empty(t, clash.ExistingRegistry)

	require.Contains(t, err.Error(), `"upper"`)
	require.Contains(t, err.Error(), "builtin")
	require.Contains(t, err.Error(), "UpperFunctionPlugin")
}

// Criterion 6: a plugin that offers no functions loads and runs its
// lifecycle as before. The widget fixture answers the function RPCs with an
// empty list; a binary built before the RPCs existed answers Unimplemented,
// which the host treats the same way, see the loopback tests in
// plugins/grpc_server_test.go.

func TestPluginOfferingNoFunctionsStillLoadsAndRuns(t *testing.T) {
	binary := testutil.BuildFixturePlugin(t, ".")
	isolateHome(t)

	local := registry.NewLocal()
	local.RegisterExternalPlugin(binary)
	f := newPersonConfig(t, local, subtypelessFixture(t, "subtypeless"), t.TempDir())

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	widget, err := Find[subtypelessWidget](f.config, "widget.main")
	require.NoError(t, err)
	require.Equal(t, "widget.main", widget.Meta.ID)
	require.Equal(t, "wheel,axle", widget.PartNames)
}
