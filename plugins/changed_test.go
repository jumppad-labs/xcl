package plugins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/types"
)

// testResource is a resource shaped like a real provider resource, it embeds
// types.ResourceBase with the remain tag and has a couple of plain fields.
type testResource struct {
	Meta types.Meta

	Name  string `xcl:"name" json:"name"`
	Count int    `xcl:"count,optional" json:"count,omitempty"`
}

// providerWithDefaultChanged embeds DefaultChanged and does not define its own
// Changed method.
type providerWithDefaultChanged struct {
	DefaultChanged[*testResource]
}

func (p *providerWithDefaultChanged) Init(state State, functions ProviderFunctions, log logger.Logger) error {
	return nil
}

func (p *providerWithDefaultChanged) Create(ctx context.Context, resource *testResource) (*testResource, error) {
	return resource, nil
}

func (p *providerWithDefaultChanged) Destroy(ctx context.Context, resource *testResource, force bool) error {
	return nil
}

func (p *providerWithDefaultChanged) Read(ctx context.Context, old *testResource, new *testResource) (*testResource, error) {
	return new, nil
}

func (p *providerWithDefaultChanged) Update(ctx context.Context, resource *testResource, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*testResource, error) {
	return resource, nil
}

func (p *providerWithDefaultChanged) Functions() ProviderFunctions {
	return nil
}

// A provider that embeds DefaultChanged and defines no Changed satisfies the
// provider interface.
var _ ResourceProvider[*testResource] = (*providerWithDefaultChanged)(nil)

func TestDefaultChangedReportsNoChangeForIdenticalResources(t *testing.T) {
	old := &testResource{Name: "web", Count: 2}
	new := &testResource{Name: "web", Count: 2}

	change, err := DefaultChanged[*testResource]{}.Changed(context.Background(), old, new, nil, nil)
	require.NoError(t, err)
	require.Equal(t, entity.NoChange, change)
}

func TestDefaultChangedReportsChangeWhenFieldDiffers(t *testing.T) {
	old := &testResource{Name: "web", Count: 2}
	new := &testResource{Name: "web", Count: 3}

	change, err := DefaultChanged[*testResource]{}.Changed(context.Background(), old, new, nil, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Update, change)
}

func TestDefaultChangedIgnoresMetaDifferences(t *testing.T) {
	old := &testResource{Name: "web", Count: 2}
	old.Meta.File = "/old/main.hcl"
	old.Meta.Line = 10
	old.Meta.Status = types.StatusCreated

	new := &testResource{Name: "web", Count: 2}
	new.Meta.File = "/new/main.hcl"
	new.Meta.Line = 42
	new.Meta.Status = types.StatusFailed

	change, err := DefaultChanged[*testResource]{}.Changed(context.Background(), old, new, nil, nil)
	require.NoError(t, err)
	require.Equal(t, entity.NoChange, change)
}

func TestDefaultChangedIgnoresDependsOnAndDisabled(t *testing.T) {
	old := &testResource{Name: "web", Count: 2}
	old.Meta.DependsOn = []string{"resource.network.main"}
	old.Meta.Disabled = false

	new := &testResource{Name: "web", Count: 2}
	new.Meta.DependsOn = []string{"resource.network.other", "resource.volume.data"}
	new.Meta.Disabled = true

	change, err := DefaultChanged[*testResource]{}.Changed(context.Background(), old, new, nil, nil)
	require.NoError(t, err)
	require.Equal(t, entity.NoChange, change)
}

func TestDefaultChangedReportsNoChangeForTwoNilResources(t *testing.T) {
	var old *testResource
	var new *testResource

	change, err := DefaultChanged[*testResource]{}.Changed(context.Background(), old, new, nil, nil)
	require.NoError(t, err)
	require.Equal(t, entity.NoChange, change)
}

func TestDefaultChangedReportsChangeWhenOneResourceIsNil(t *testing.T) {
	var old *testResource
	new := &testResource{Name: "web", Count: 2}

	change, err := DefaultChanged[*testResource]{}.Changed(context.Background(), old, new, nil, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Update, change)
}

func TestDefaultChangedIgnoresDependencies(t *testing.T) {
	old := &testResource{Name: "web", Count: 2}
	new := &testResource{Name: "web", Count: 2}
	dependencies := []entity.DependencyChange{
		{Address: "resource.network.main", Change: entity.Replace},
	}

	change, err := DefaultChanged[*testResource]{}.Changed(context.Background(), old, new, nil, dependencies)
	require.NoError(t, err)
	require.Equal(t, entity.NoChange, change)
}

// replaceOnName replaces a resource whose name changes
var replaceOnName = DefaultChanged[*testResource]{
	ReplaceOn: []entity.Path{entity.Path{}.Attribute("name")},
}

func TestDefaultChangedReplacesWhenAChangeIsAReplaceOnSetting(t *testing.T) {
	old := &testResource{Name: "web", Count: 2}
	new := &testResource{Name: "api", Count: 2}
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("name"), Before: "web", After: "api"},
	}

	change, err := replaceOnName.Changed(context.Background(), old, new, changes, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Replace, change)
}

func TestDefaultChangedReplacesWhenAChangeIsUnderAReplaceOnSetting(t *testing.T) {
	provider := DefaultChanged[*testResource]{
		ReplaceOn: []entity.Path{entity.Path{}.Attribute("network")},
	}
	old := &testResource{Name: "web", Count: 2}
	new := &testResource{Name: "web", Count: 2}
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("network").Index(0).Attribute("name"), Before: "a", After: "b"},
	}

	change, err := provider.Changed(context.Background(), old, new, changes, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Replace, change)
}

func TestDefaultChangedReplacesWhenAReplaceOnSettingIsUnknown(t *testing.T) {
	old := &testResource{Name: "web", Count: 2}
	new := &testResource{Name: "web", Count: 2}
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("name"), Before: "web", Unknown: true},
	}

	change, err := replaceOnName.Changed(context.Background(), old, new, changes, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Replace, change)
}

func TestDefaultChangedUpdatesWhenNoChangeIsAReplaceOnSetting(t *testing.T) {
	old := &testResource{Name: "web", Count: 2}
	new := &testResource{Name: "web", Count: 3}
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("count"), Before: 2, After: 3},
	}

	change, err := replaceOnName.Changed(context.Background(), old, new, changes, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Update, change)
}

func TestDefaultChangedWithoutReplaceOnUpdatesARenamedResource(t *testing.T) {
	old := &testResource{Name: "web", Count: 2}
	new := &testResource{Name: "api", Count: 2}
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("name"), Before: "web", After: "api"},
	}

	change, err := DefaultChanged[*testResource]{}.Changed(context.Background(), old, new, changes, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Update, change)
}

func TestDefaultChangedWithReplaceOnIgnoresAReplacedDependency(t *testing.T) {
	old := &testResource{Name: "web", Count: 2}
	new := &testResource{Name: "web", Count: 2}
	dependencies := []entity.DependencyChange{
		{Address: "resource.network.main", Change: entity.Replace},
	}

	change, err := replaceOnName.Changed(context.Background(), old, new, nil, dependencies)
	require.NoError(t, err)
	require.Equal(t, entity.NoChange, change)
}
