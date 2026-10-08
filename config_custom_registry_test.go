package xcl_test

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/plugins/example/pkg/person"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/stretchr/testify/require"
)

// A third party can write its own registry and its own way of starting a
// plugin against the registry interfaces, and a Config uses both.

// personPlugin provides resource.person through the example person provider
type personPlugin struct {
	plugins.PluginBase
}

func (p *personPlugin) Init(l logger.Logger, s plugins.State) error {
	return plugins.RegisterResourceProvider(&p.PluginBase, l, s, "resource", "person", &person.Person{}, &person.ExampleProvider{})
}

// recordingPlugin starts personPlugin in process and records that it was
// started
type recordingPlugin struct {
	started atomic.Bool
}

func (p *recordingPlugin) Name() string {
	return "recording-person"
}

func (p *recordingPlugin) Start(emit events.Emit) (plugins.PluginHost, error) {
	p.started.Store(true)
	return plugins.NewDirectPluginHost(emit, nil, &personPlugin{})
}

// customRegistry provides one recordingPlugin, when it has one, and declares
// types
type customRegistry struct {
	plugin *recordingPlugin
	types  []registry.Type
}

func (r *customRegistry) Name() string {
	return "custom"
}

func (r *customRegistry) Types() []registry.Type {
	return r.types
}

func (r *customRegistry) Plugins(ctx context.Context, emit events.Emit) ([]registry.Plugin, error) {
	if r.plugin == nil {
		return nil, nil
	}

	return []registry.Plugin{r.plugin}, nil
}

func TestCustomRegistryAndPluginStarterAreUsed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	plugin := &recordingPlugin{}

	c, err := xcl.NewConfig(xcl.WithRegistry(&customRegistry{plugin: plugin}))
	require.NoError(t, err)

	path, err := filepath.Abs("./internal/test_fixtures/config/registries/custom/main.xcl")
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	require.True(t, plugin.started.Load())

	ada, err := xcl.Find[person.Person](c, "resource.person.ada")
	require.NoError(t, err)
	require.Equal(t, "Ada", ada.FirstName)
	require.Equal(t, "person-ada-lovelace", ada.PersonID)
}

func TestCustomRegistryTypesAreDeclared(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	custom := &customRegistry{
		types: []registry.Type{
			{Type: registered.TypeCache, Prototype: &registered.Cache{}},
			{Type: "resource", Subtype: registered.TypeDatabase, Prototype: &registered.Database{}},
		},
	}

	c, err := xcl.NewConfig(xcl.WithRegistry(custom))
	require.NoError(t, err)

	path, err := filepath.Abs("./internal/test_fixtures/config/registered/bare/main.xcl")
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	database, err := xcl.Find[registered.Database](c, "resource.database.main")
	require.NoError(t, err)
	require.Equal(t, "us-east", database.Location)
	require.Equal(t, 5432, database.Port)

	cache, err := xcl.Find[registered.Cache](c, "cache.main")
	require.NoError(t, err)
	require.Equal(t, "us-east", cache.Location)
}

func TestCustomRegistryMalformedTypeFailsNewConfig(t *testing.T) {
	custom := &customRegistry{
		types: []registry.Type{
			{Type: registered.TypeCache, Prototype: registered.Cache{}},
		},
	}

	c, err := xcl.NewConfig(xcl.WithRegistry(custom))
	require.Nil(t, c)
	require.ErrorContains(t, err, "must be a pointer to a struct that embeds types.ResourceBase")
}
