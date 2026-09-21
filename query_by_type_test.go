package xcl

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/stretchr/testify/require"
)

// The kind lookup is exercised against the same applied configuration as the
// address lookup, so what it enumerates is what an apply actually produced.
// setupFindConfig is that harness; the basic fixture declares two databases,
// one at the root and one inside a module, which is what makes a query that
// matches more than one entity natural.
//
// setupBareTypeConfig is the one exception. A type registered in the bare form
// leads its own declaration rather than sitting under the resource keyword, so
// it needs the bare fixture and a registry that registers it that way.
func setupBareTypeConfig(t *testing.T) *Config {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	reg := registry.NewPluginRegistry(logger.NewTestLogger(t))

	err := reg.RegisterBareType(registered.TypeCache, &registered.Cache{})
	require.NoError(t, err)

	err = reg.RegisterType(registered.TypeDatabase, &registered.Database{})
	require.NoError(t, err)

	store, err := state.NewFileStateStore(filepath.Join(t.TempDir(), "state.json"), reg)
	require.NoError(t, err)

	c := NewConfig(
		WithPluginRegistry(reg),
		WithStateStore(store),
	)

	path, err := filepath.Abs("./internal/test_fixtures/config/registered/bare/main.xcl")
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	return c
}

// A kind lookup names the kind and the variety, positionally, exactly as an
// address spells them. It returns every entity of that variety wherever it is
// declared, and nothing of any other variety.

func TestFindByTypeReturnsEveryEntityOfTheVarietyNamed(t *testing.T) {
	c := setupFindConfig(t)

	databases, err := FindByType[registered.Database](c, "resource", "database")
	require.NoError(t, err)
	require.Len(t, databases, 2)

	found := []string{}
	for _, database := range databases {
		found = append(found, database.Meta.ID)
	}

	// both databases, the one at the root and the one inside the module, and
	// neither the app nor the consumer that are also resources
	require.ElementsMatch(t, []string{
		"resource.database.main",
		"module.shared.resource.database.shared",
	}, found)
}

func TestFindByTypeReturnsOnlyTheVarietyNamedWhenSeveralVarietiesExist(t *testing.T) {
	c := setupFindConfig(t)

	apps, err := FindByType[registered.App](c, "resource", "app")
	require.NoError(t, err)
	require.Len(t, apps, 1)

	require.Equal(t, "resource.app.web", apps[0].Meta.ID)
	require.Equal(t, "production", apps[0].Environment)
}

func TestFindByTypeFillsInTheEntitiesItReturns(t *testing.T) {
	c := setupFindConfig(t)

	consumers, err := FindByType[registered.Consumer](c, "resource", "consumer")
	require.NoError(t, err)
	require.Len(t, consumers, 1)

	require.Equal(t, "resource.consumer.reader", consumers[0].Meta.ID)
	require.Equal(t, "production", consumers[0].AppEnvironment)
}

// A variety that is registered but never declared is a well formed question
// with an empty answer, not a failure. The caller ranges over the result and
// does nothing, rather than having to tell one kind of nothing from another.

func TestFindByTypeReturnsAnEmptyResultForAVarietyWithNothingDeclared(t *testing.T) {
	c := setupFindConfig(t)

	caches, err := FindByType[registered.Cache](c, "resource", "cache")
	require.NoError(t, err)

	require.NotNil(t, caches)
	require.Empty(t, caches)
}

// Segments are matched positionally from the root, so a variety given on its
// own is not a kind and is refused rather than quietly matching in the wrong
// position and returning the entities anyway.

func TestFindByTypeRejectsAVarietyNamedWithoutItsKind(t *testing.T) {
	c := setupFindConfig(t)

	databases, err := FindByType[registered.Database](c, "database")
	require.ErrorIs(t, err, ErrUnknownType)
	require.Nil(t, databases)

	var unknown *UnknownTypeError
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, "database", unknown.Name)
}

func TestFindByTypeRejectsAKindItDoesNotKnow(t *testing.T) {
	c := setupFindConfig(t)

	databases, err := FindByType[registered.Database](c, "nonsense")
	require.ErrorIs(t, err, ErrUnknownType)
	require.Nil(t, databases)

	var unknown *UnknownTypeError
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, "nonsense", unknown.Name)
}

func TestFindByTypeRejectsAVarietyItDoesNotKnow(t *testing.T) {
	c := setupFindConfig(t)

	databases, err := FindByType[registered.Database](c, "resource", "nonsense")
	require.ErrorIs(t, err, ErrUnknownType)
	require.Nil(t, databases)

	var unknown *UnknownTypeError
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, "nonsense", unknown.Name)
}

// A query whose segments span more than one Go type cannot be typed at all, so
// it is refused rather than answered with the empty result that is
// indistinguishable from "you declared none of those".

func TestFindByTypeRejectsTheResourceKindOnItsOwn(t *testing.T) {
	c := setupFindConfig(t)

	databases, err := FindByType[registered.Database](c, "resource")
	require.ErrorIs(t, err, ErrNotTypeable)
	require.Nil(t, databases)

	var notTypeable *NotTypeableError
	require.True(t, errors.As(err, &notTypeable))
	require.Equal(t, []string{"resource"}, notTypeable.Segments)
	require.Contains(t, notTypeable.Use, "variety")
}

func TestFindByTypeRejectsPublishedValuesAndNamesTheCallThatReturnsThem(t *testing.T) {
	c := setupFindConfig(t)

	outputs, err := FindByType[resources.Output](c, "output")
	require.ErrorIs(t, err, ErrNotTypeable)
	require.Nil(t, outputs)

	var notTypeable *NotTypeableError
	require.True(t, errors.As(err, &notTypeable))
	require.Equal(t, "Outputs", notTypeable.Use)
}

func TestFindByTypeRejectsAQueryWithNoSegments(t *testing.T) {
	c := setupFindConfig(t)

	databases, err := FindByType[registered.Database](c)
	require.ErrorIs(t, err, ErrNotTypeable)
	require.Nil(t, databases)

	// enumerating the whole configuration is a different call, and the error
	// says so rather than returning everything
	var notTypeable *NotTypeableError
	require.True(t, errors.As(err, &notTypeable))
	require.Equal(t, "All", notTypeable.Use)
}

// A kind that is exactly one Go type needs no variety after it, whether it is
// one of the kinds xcl interprets itself or a type registered in the bare
// form, which leads its own declaration.

func TestFindByTypeReturnsTheVariablesForTheVariableKind(t *testing.T) {
	c := setupFindConfig(t)

	variables, err := FindByType[resources.Variable](c, "variable")
	require.NoError(t, err)
	require.Len(t, variables, 1)

	require.Equal(t, "variable.environment", variables[0].Meta.ID)
}

func TestFindByTypeReturnsTheModulesForTheModuleKind(t *testing.T) {
	c := setupFindConfig(t)

	modules, err := FindByType[resources.Module](c, "module")
	require.NoError(t, err)
	require.Len(t, modules, 1)

	require.Equal(t, "module.shared", modules[0].Meta.ID)
	require.Equal(t, "./module", modules[0].Source)
}

func TestFindByTypeAcceptsABareFormTypeAsTheKind(t *testing.T) {
	c := setupBareTypeConfig(t)

	caches, err := FindByType[registered.Cache](c, "cache")
	require.NoError(t, err)
	require.Len(t, caches, 1)

	require.Equal(t, "cache.main", caches[0].Meta.ID)
	require.Equal(t, "us-east", caches[0].Location)
}

// A block that only ever exists nested inside another declaration has no
// address of its own. Answering a query for one with an empty result would
// read as "you declared none of those", so it is refused and the error names
// the declaration it is reached through instead.

func TestFindByTypeRejectsATypeThatIsOnlyANestedBlock(t *testing.T) {
	c := setupFindConfig(t)

	// Timeouts is the timeouts block of a Database, it embeds no ResourceBase
	timeouts, err := FindByType[registered.Timeouts](c, "resource", "database")
	require.ErrorIs(t, err, ErrNotAnEntity)
	require.Nil(t, timeouts)

	var notAnEntity *NotAnEntityError
	require.True(t, errors.As(err, &notAnEntity))
	require.Equal(t, "declaration", notAnEntity.ReachedThrough)
	require.Contains(t, err.Error(), "reach it through the declaration that contains it")
}

// The single expected lookup is for the entity a configuration declares
// exactly one of. It tells all three outcomes apart: the one entity, none at
// all, and more than one.

func TestFindOneReturnsTheSingleEntityMatched(t *testing.T) {
	c := setupFindConfig(t)

	app, err := FindOne[registered.App](c, "resource", "app")
	require.NoError(t, err)
	require.NotNil(t, app)

	require.Equal(t, "resource.app.web", app.Meta.ID)
	require.Equal(t, "production", app.Environment)
	require.Equal(t, "us-east", app.DatabaseLocation)
}

func TestFindOneReturnsNotFoundWhenNothingMatches(t *testing.T) {
	c := setupFindConfig(t)

	cache, err := FindOne[registered.Cache](c, "resource", "cache")
	require.ErrorIs(t, err, ErrNotFound)
	require.Nil(t, cache)

	var notFound *NotFoundError
	require.True(t, errors.As(err, &notFound))
	require.Equal(t, "resource.cache", notFound.Address)
}

func TestFindOneReturnsNotUniqueWhenMoreThanOneMatches(t *testing.T) {
	c := setupFindConfig(t)

	database, err := FindOne[registered.Database](c, "resource", "database")
	require.ErrorIs(t, err, ErrNotUnique)
	require.Nil(t, database)

	var notUnique *NotUniqueError
	require.True(t, errors.As(err, &notUnique))
	require.Equal(t, 2, notUnique.Count)
	require.Equal(t, []string{"resource", "database"}, notUnique.Segments)
}

// A kind lookup hands back what the configuration holds, entity by entity, for
// a type the configuration holds as the caller's own Go type. A plugin backed
// type is not held that way, so it comes back as a copy, filled in from the
// entity the plugin's schema produced.

func TestFindByTypeReturnsTheEntitiesTheConfigurationHolds(t *testing.T) {
	c := setupFindConfig(t)

	databases, err := FindByType[registered.Database](c, "resource", "database")
	require.NoError(t, err)
	require.Len(t, databases, 2)

	for _, database := range databases {
		stored, err := c.FindResource(database.Meta.ID)
		require.NoError(t, err)

		require.Same(t, stored, database)
	}
}

func TestFindByTypeFillsInPluginProvidedEntities(t *testing.T) {
	// setupQueryConfig is the harness with a plugin registered, its network
	// type is declared twice in the fixture
	c := setupQueryConfig(t)

	networks, err := FindByType[structs.Network](c, "resource", "network")
	require.NoError(t, err)
	require.Len(t, networks, 2)

	subnets := map[string]string{}
	for _, network := range networks {
		subnets[network.Meta.ID] = network.Subnet
	}

	require.Equal(t, map[string]string{
		"resource.network.frontend": "10.0.1.0/24",
		"resource.network.backend":  "10.0.2.0/24",
	}, subnets)
}
