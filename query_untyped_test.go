package xcl

import (
	"errors"
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// The untyped lookups answer the same question as FindByType and FindOne,
// over the same applied configuration, without a Go type to convert to. So
// they are exercised against the same harnesses: setupFindConfig for the
// registered types and setupQueryConfig for the plugin provided ones.

func TestFindEntitiesByTypeMatchesFindByType(t *testing.T) {
	c := setupFindConfig(t)

	typed, err := FindByType[registered.Database](c, "resource", "database")
	require.NoError(t, err)
	require.Len(t, typed, 2)

	untyped, err := c.FindEntitiesByType("resource", "database")
	require.NoError(t, err)
	require.Len(t, untyped, 2)

	expected := []string{}
	for _, database := range typed {
		expected = append(expected, database.Meta.ID)
	}

	// each entity comes back exactly as the configuration stores it, which for
	// a registered type is the registered Go type itself
	found := []string{}
	for _, entity := range untyped {
		database, ok := entity.(*registered.Database)
		require.True(t, ok, "expected *registered.Database, got %T", entity)

		found = append(found, database.Meta.ID)
	}

	require.ElementsMatch(t, expected, found)
}

func TestFindEntitiesByTypeUnknownTypeReturnsUnknownTypeError(t *testing.T) {
	c := setupFindConfig(t)

	entities, err := c.FindEntitiesByType("nonsense")
	require.ErrorIs(t, err, ErrUnknownType)
	require.Nil(t, entities)

	var unknown *UnknownTypeError
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, "nonsense", unknown.Name)
}

func TestFindEntitiesByTypeUnknownSubtypeReturnsUnknownTypeError(t *testing.T) {
	c := setupFindConfig(t)

	entities, err := c.FindEntitiesByType("resource", "nonsense")
	require.ErrorIs(t, err, ErrUnknownType)
	require.Nil(t, entities)

	var unknown *UnknownTypeError
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, "nonsense", unknown.Name)
}

func TestFindEntitiesByTypeWithoutSubtypeReturnsNotTypeableError(t *testing.T) {
	c := setupFindConfig(t)

	// resource spans every subtype declared under it, so on its own it does
	// not pin one type, exactly as FindByType refuses it
	entities, err := c.FindEntitiesByType("resource")
	require.ErrorIs(t, err, ErrNotTypeable)
	require.Nil(t, entities)

	var notTypeable *NotTypeableError
	require.True(t, errors.As(err, &notTypeable))
	require.Equal(t, []string{"resource"}, notTypeable.Segments)
	require.Contains(t, notTypeable.Use, "subtype")
}

func TestFindEntitiesByTypeWithoutSegmentsReturnsNotTypeableError(t *testing.T) {
	c := setupFindConfig(t)

	entities, err := c.FindEntitiesByType()
	require.ErrorIs(t, err, ErrNotTypeable)
	require.Nil(t, entities)

	var notTypeable *NotTypeableError
	require.True(t, errors.As(err, &notTypeable))
	require.Equal(t, "All", notTypeable.Use)
}

func TestFindEntitiesByTypeMatchingNothingReturnsEmpty(t *testing.T) {
	c := setupFindConfig(t)

	// cache is registered under resource but the basic fixture declares none,
	// a well formed question with an empty answer rather than a failure
	entities, err := c.FindEntitiesByType("resource", "cache")
	require.NoError(t, err)

	require.NotNil(t, entities)
	require.Empty(t, entities)
}

func TestFindOneEntityReturnsTheEntity(t *testing.T) {
	c := setupFindConfig(t)

	entity, err := c.FindOneEntity("resource", "app")
	require.NoError(t, err)
	require.NotNil(t, entity)

	app, ok := entity.(*registered.App)
	require.True(t, ok, "expected *registered.App, got %T", entity)

	require.Equal(t, "resource.app.web", app.Meta.ID)
	require.Equal(t, "production", app.Environment)
}

func TestFindOneEntityNoneReturnsNotFound(t *testing.T) {
	c := setupFindConfig(t)

	entity, err := c.FindOneEntity("resource", "cache")
	require.ErrorIs(t, err, ErrNotFound)
	require.Nil(t, entity)

	var notFound *NotFoundError
	require.True(t, errors.As(err, &notFound))
	require.Equal(t, "resource.cache", notFound.Address)
}

func TestFindOneEntitySeveralReturnsNotUniqueWithCount(t *testing.T) {
	c := setupFindConfig(t)

	entity, err := c.FindOneEntity("resource", "database")
	require.ErrorIs(t, err, ErrNotUnique)
	require.Nil(t, entity)

	var notUnique *NotUniqueError
	require.True(t, errors.As(err, &notUnique))
	require.Equal(t, 2, notUnique.Count)
	require.Equal(t, []string{"resource", "database"}, notUnique.Segments)
}

func TestFindEntitiesByTypeReturnsPluginEntities(t *testing.T) {
	// setupQueryConfig is the harness with the TestPlugin registered, its
	// network type is declared twice in the fixture
	c := setupQueryConfig(t)

	entities, err := c.FindEntitiesByType("resource", "network")
	require.NoError(t, err)
	require.Len(t, entities, 2)

	// a plugin backed entity is held as the form the plugin's schema produced,
	// not as the plugin's own Go type, so it is returned that way: it is the
	// very entity the configuration stores, and it converts to the plugin's
	// type through As exactly as FindByType would have
	subnets := map[string]string{}
	for _, entity := range entities {
		meta, err := types.GetMeta(entity)
		require.NoError(t, err)

		stored, err := c.FindResource(meta.ID)
		require.NoError(t, err)
		require.Same(t, stored, entity)

		network, err := As[structs.Network](entity)
		require.NoError(t, err)

		subnets[network.Meta.ID] = network.Subnet
	}

	require.Equal(t, map[string]string{
		"resource.network.frontend": "10.0.1.0/24",
		"resource.network.backend":  "10.0.2.0/24",
	}, subnets)
}
