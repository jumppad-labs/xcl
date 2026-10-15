package xcl

import (
	"context"
	"errors"
	"testing"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// A Go type with a field configuration cannot be decoded into is the
// program's mistake, so it is reported where the type is declared, as an
// error naming the type and the field, and never as a panic while a file is
// read. A declared type is reported by NewConfig, a plugin's type by the
// first operation that loads plugins.

// badType is a declared type whose block field is a string, which cannot
// hold blocks
type badType struct {
	Meta types.Meta

	Listener string `xcl:"listener,block"`
}

// badPluginType is the same shape, provided by an in-process plugin
type badPluginType struct {
	Meta types.Meta

	Listener string `xcl:"listener,block"`
}

// badPluginProvider is a no-op provider for badPluginType
type badPluginProvider struct {
	plugins.DefaultChanged[*badPluginType]
}

func (p *badPluginProvider) Init(state plugins.State, functions plugins.ProviderFunctions, log logger.Logger) error {
	return nil
}

func (p *badPluginProvider) Create(ctx context.Context, r *badPluginType) (*badPluginType, error) {
	return r, nil
}

func (p *badPluginProvider) Destroy(ctx context.Context, r *badPluginType, force bool) error {
	return nil
}

func (p *badPluginProvider) Read(ctx context.Context, old *badPluginType, new *badPluginType) (*badPluginType, error) {
	return new, nil
}

func (p *badPluginProvider) Update(ctx context.Context, r *badPluginType, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*badPluginType, error) {
	return r, nil
}

func (p *badPluginProvider) Functions() plugins.ProviderFunctions {
	return nil
}

// BadBlockPlugin is an in-process plugin that provides resource.bad, a type
// with a string block field
type BadBlockPlugin struct {
	plugins.PluginBase
}

func (p *BadBlockPlugin) Init(l logger.Logger, s plugins.State) error {
	return plugins.RegisterResourceProvider(&p.PluginBase, l, s, "resource", "bad", &badPluginType{}, &badPluginProvider{})
}

// badBlockPluginName is the name the in-process plugin loads under, its Go
// type name
const badBlockPluginName = "BadBlockPlugin"

func TestNewConfigReturnsErrorForStringBlockField(t *testing.T) {
	local := registry.NewLocal()
	require.NotPanics(t, func() {
		local.RegisterType(&badType{}, "resource", "bad")
	})

	var c *Config
	var err error
	require.NotPanics(t, func() {
		c, err = NewConfig(WithRegistry(local))
	})

	require.Nil(t, c)
	require.Error(t, err)

	var invalid *InvalidTypeError
	require.True(t, errors.As(err, &invalid))
	require.Equal(t, "resource.bad", invalid.Type)
	require.Equal(t, "type *xcl.badType", invalid.Provider)
	require.Equal(t, "local", invalid.Registry)
	require.ErrorContains(t, invalid.Err, "Listener")
	require.ErrorContains(t, invalid.Err, "cannot hold blocks")
}

func TestNewConfigErrorForStringBlockFieldNamesTheTypeAndField(t *testing.T) {
	local := registry.NewLocal()
	local.RegisterType(&badType{}, "resource", "bad")

	_, err := NewConfig(WithRegistry(local))
	require.Error(t, err)

	require.Contains(t, err.Error(), `"resource.bad"`)
	require.Contains(t, err.Error(), "type *xcl.badType")
	require.Contains(t, err.Error(), "Listener")
}

func TestFirstOperationReturnsErrorForPluginTypeWithBadBlockField(t *testing.T) {
	isolateHome(t)

	local := registry.NewLocal()
	local.RegisterPlugin(&BadBlockPlugin{})

	var c *Config
	var err error
	require.NotPanics(t, func() {
		c, err = NewConfig(WithRegistry(local))
	})
	require.NoError(t, err)

	require.NotPanics(t, func() {
		err = c.Validate(writeConfigFile(t, variableOnlyConfig))
	})
	require.Error(t, err)

	// the plugin cannot describe the type, so it fails to load
	var load *PluginLoadError
	require.True(t, errors.As(err, &load), "error chain was: %v", err)
	require.ErrorContains(t, err, "resource.bad")
	require.ErrorContains(t, err, "Listener")
	require.ErrorContains(t, err, "a block must hold a struct")
}

func TestFirstOperationErrorForPluginTypeNamesThePluginTypeAndField(t *testing.T) {
	isolateHome(t)

	local := registry.NewLocal()
	local.RegisterPlugin(&BadBlockPlugin{})

	c, err := NewConfig(WithRegistry(local))
	require.NoError(t, err)

	err = c.Validate(writeConfigFile(t, variableOnlyConfig))
	require.Error(t, err)

	require.Contains(t, err.Error(), badBlockPluginName)
	require.Contains(t, err.Error(), "resource.bad")
	require.Contains(t, err.Error(), "Listener")
}
