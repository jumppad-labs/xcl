package xcl

import (
	"errors"
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/stretchr/testify/require"
)

// NewConfig reads every registry's declared Go types, in the order the
// registries were given, and returns an error when they do not fit together.
// A clash comes from how registries are combined, not from one bad line, so
// it is an error rather than a panic. A malformed declaration panics where it
// is registered, see the registry package.

func TestNewConfigFailsOnDuplicateTypeInOneRegistry(t *testing.T) {
	local := registry.NewLocal()
	local.RegisterType(&registered.Database{}, "resource", "database")
	local.RegisterType(&registered.App{}, "resource", "database")

	c, err := NewConfig(WithRegistry(local))
	require.Nil(t, c)
	require.Error(t, err)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "resource.database", clash.Name)
	require.Equal(t, "type *registered.App", clash.Provider)
	require.Equal(t, "local", clash.Registry)
	require.Equal(t, "type *registered.Database", clash.Existing)
	require.Equal(t, "local", clash.ExistingRegistry)
}

func TestNewConfigFailsOnDuplicateTypeAcrossRegistries(t *testing.T) {
	first := registry.NewLocal()
	first.RegisterType(&registered.Database{}, "resource", "database")

	second := registry.NewLocal()
	second.RegisterType(&registered.App{}, "resource", "database")

	c, err := NewConfig(
		WithRegistry(namedRegistry{Registry: first, name: "first"}),
		WithRegistry(namedRegistry{Registry: second, name: "second"}),
	)
	require.Nil(t, c)
	require.Error(t, err)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "resource.database", clash.Name)
	require.Equal(t, "type *registered.App", clash.Provider)
	require.Equal(t, "second", clash.Registry)
	require.Equal(t, "type *registered.Database", clash.Existing)
	require.Equal(t, "first", clash.ExistingRegistry)
}

func TestNewConfigFailsOnTypeInBothForms(t *testing.T) {
	local := registry.NewLocal()
	local.RegisterType(&registered.Cache{}, "server")
	local.RegisterType(&registered.Server{}, "server", "big")

	c, err := NewConfig(WithRegistry(local))
	require.Nil(t, c)
	require.Error(t, err)

	var form *TypeFormError
	require.True(t, errors.As(err, &form))
	require.Equal(t, "server", form.Type)
	require.False(t, form.TakesSubtype)
}

func TestNewConfigFailsOnBuiltinTypeName(t *testing.T) {
	local := registry.NewLocal()
	local.RegisterType(&registered.Database{}, "variable")

	c, err := NewConfig(WithRegistry(local))
	require.Nil(t, c)
	require.Error(t, err)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "variable", clash.Name)
	require.Equal(t, "type *registered.Database", clash.Provider)
	require.Equal(t, "local", clash.Registry)
	require.Equal(t, "builtin", clash.Existing)
	require.Empty(t, clash.ExistingRegistry)
}

func TestWithRegistryPanicsOnNil(t *testing.T) {
	require.PanicsWithValue(t, "xcl: registry must not be nil", func() {
		WithRegistry(nil)
	})
}

func TestNewConfigAcceptsRegistriesInAnyOrder(t *testing.T) {
	first := registry.NewLocal()
	first.RegisterType(&registered.Database{}, "resource", registered.TypeDatabase)

	second := registry.NewLocal()
	second.RegisterType(&registered.Cache{}, registered.TypeCache)

	c, err := NewConfig(
		WithRegistry(second),
		WithVariables(map[string]any{"region": "us-east"}),
		WithRegistry(first),
	)
	require.NoError(t, err)

	require.Equal(t, []registry.Registry{second, first}, c.registries)
	require.True(t, c.catalog.IsRegisteredType("resource", registered.TypeDatabase))
	require.True(t, c.catalog.IsRegisteredType(registered.TypeCache, ""))
}
