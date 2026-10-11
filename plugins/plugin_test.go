package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/entity"
	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/logger"
)

// functionProvider is a provider for testResource that offers the functions
// its fns field holds, or none when it is nil. It is the provider every
// function registration test registers.
type functionProvider struct {
	DefaultChanged[*testResource]

	fns map[string]ProviderFunction
}

func (p *functionProvider) Init(state State, functions ProviderFunctions, log logger.Logger) error {
	return nil
}

func (p *functionProvider) Create(ctx context.Context, resource *testResource) (*testResource, error) {
	return resource, nil
}

func (p *functionProvider) Destroy(ctx context.Context, resource *testResource, force bool) error {
	return nil
}

func (p *functionProvider) Read(ctx context.Context, old *testResource, new *testResource) (*testResource, error) {
	return new, nil
}

func (p *functionProvider) Update(ctx context.Context, resource *testResource, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*testResource, error) {
	return resource, nil
}

func (p *functionProvider) Functions() ProviderFunctions {
	if p.fns == nil {
		return nil
	}

	return NewFunctions(p.fns)
}

var _ ResourceProvider[*testResource] = (*functionProvider)(nil)

// greetingProvider returns a provider offering greet, which prefixes its
// argument with hello
func greetingProvider(t *testing.T) *functionProvider {
	t.Helper()

	greet, err := NewFunction(func(name string) (string, error) {
		return "hello " + name, nil
	})
	require.NoError(t, err)

	return &functionProvider{fns: map[string]ProviderFunction{"greet": greet}}
}

// failingProvider returns a provider offering fail, which always returns the
// error boom
func failingProvider(t *testing.T) *functionProvider {
	t.Helper()

	fail, err := NewFunction(func(name string) (string, error) {
		return "", errors.New("boom")
	})
	require.NoError(t, err)

	return &functionProvider{fns: map[string]ProviderFunction{"fail": fail}}
}

func TestRegisterResourceProviderCollectsFunctions(t *testing.T) {
	base := &PluginBase{}

	err := RegisterResourceProvider(base, logger.Nop(), emptyState{}, "resource", "greeter", &testResource{}, greetingProvider(t))
	require.NoError(t, err)

	registered := base.GetFunctions()
	require.Len(t, registered, 1)
	require.Equal(t, "greet", registered[0].Name)

	sig := registered[0].Signature
	require.Len(t, sig.Params, 1)
	require.Equal(t, "arg0", sig.Params[0].Name)
	require.Equal(t, `"string"`, string(sig.Params[0].Type))
	require.Equal(t, `"string"`, string(sig.Return))
	require.Nil(t, sig.Variadic)
}

func TestRegisterResourceProviderSortsFunctionsByName(t *testing.T) {
	zeta, err := NewFunction(func(n int) (int, error) { return n, nil })
	require.NoError(t, err)

	alpha, err := NewFunction(func(n int) (int, error) { return n, nil })
	require.NoError(t, err)

	provider := &functionProvider{fns: map[string]ProviderFunction{"zeta": zeta, "alpha": alpha}}

	base := &PluginBase{}
	err = RegisterResourceProvider(base, logger.Nop(), emptyState{}, "resource", "sorted", &testResource{}, provider)
	require.NoError(t, err)

	registered := base.GetFunctions()
	require.Len(t, registered, 2)
	require.Equal(t, "alpha", registered[0].Name)
	require.Equal(t, "zeta", registered[1].Name)
}

func TestRegisterResourceProviderRejectsDuplicateFunctionInPlugin(t *testing.T) {
	base := &PluginBase{}

	err := RegisterResourceProvider(base, logger.Nop(), emptyState{}, "resource", "first", &testResource{}, greetingProvider(t))
	require.NoError(t, err)

	err = RegisterResourceProvider(base, logger.Nop(), emptyState{}, "resource", "second", &testResource{}, greetingProvider(t))
	require.Error(t, err)

	var clash *xclerrors.FunctionNameClashError
	require.True(t, errors.As(err, &clash), "expected a *FunctionNameClashError, got %T", err)
	require.Equal(t, "greet", clash.Name)
	require.Equal(t, "provider second", clash.Provider)
	require.Equal(t, "provider first", clash.Existing)

	require.Contains(t, err.Error(), "provider first")
	require.Contains(t, err.Error(), "provider second")
	require.Contains(t, err.Error(), `function "greet"`)
}

func TestRegisterResourceProviderAcceptsNilFunctions(t *testing.T) {
	base := &PluginBase{}

	err := RegisterResourceProvider(base, logger.Nop(), emptyState{}, "resource", "plain", &testResource{}, &functionProvider{})
	require.NoError(t, err)

	require.Empty(t, base.GetFunctions())

	registeredTypes := base.GetTypes()
	require.Len(t, registeredTypes, 1)
	require.Equal(t, "resource", registeredTypes[0].Type)
	require.Equal(t, "plain", registeredTypes[0].SubType)

	data, err := base.Create(context.Background(), "resource", "plain", []byte(`{"name":"main","count":3}`))
	require.NoError(t, err)

	var created testResource
	require.NoError(t, json.Unmarshal(data, &created))
	require.Equal(t, "main", created.Name)
	require.Equal(t, 3, created.Count)
}

func TestPluginBaseCallFunctionCallsTheProvidersFunction(t *testing.T) {
	base := &PluginBase{}

	err := RegisterResourceProvider(base, logger.Nop(), emptyState{}, "resource", "greeter", &testResource{}, greetingProvider(t))
	require.NoError(t, err)

	result, err := base.CallFunction(context.Background(), "greet", []byte(`["Nic"]`))
	require.NoError(t, err)

	var greeting string
	require.NoError(t, json.Unmarshal(result, &greeting))
	require.Equal(t, "hello Nic", greeting)
}

func TestPluginBaseCallFunctionReturnsTheFunctionsError(t *testing.T) {
	base := &PluginBase{}

	err := RegisterResourceProvider(base, logger.Nop(), emptyState{}, "resource", "failer", &testResource{}, failingProvider(t))
	require.NoError(t, err)

	_, err = base.CallFunction(context.Background(), "fail", []byte(`["Nic"]`))
	require.EqualError(t, err, "boom")
}

func TestPluginBaseCallFunctionRejectsUnknownFunction(t *testing.T) {
	base := &PluginBase{}

	err := RegisterResourceProvider(base, logger.Nop(), emptyState{}, "resource", "greeter", &testResource{}, greetingProvider(t))
	require.NoError(t, err)

	_, err = base.CallFunction(context.Background(), "missing", []byte(`[]`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "not offered by this plugin")
	require.Contains(t, err.Error(), `"missing"`)
}
