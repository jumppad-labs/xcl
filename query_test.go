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
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// The address lookup is exercised against a real applied configuration rather
// than a hand built state, so what it resolves is what an apply actually
// produced: a resource with a nested block at the root, a module holding a
// further resource and a published value, a variable, and two resources that
// read the others.
//
// setupFindConfig registers the database, app and consumer types, applies the
// basic registered fixture and returns the configuration it produced.
//
// The cache type is registered too and the fixture declares none of it, which
// is how a query for a variety that is known but undeclared is reached.
func setupFindConfig(t *testing.T) *Config {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	reg := registry.NewPluginRegistry(logger.NewTestLogger(t))

	err := reg.RegisterType(registered.TypeDatabase, &registered.Database{})
	require.NoError(t, err)

	err = reg.RegisterType(registered.TypeApp, &registered.App{})
	require.NoError(t, err)

	err = reg.RegisterType(registered.TypeConsumer, &registered.Consumer{})
	require.NoError(t, err)

	err = reg.RegisterType(registered.TypeCache, &registered.Cache{})
	require.NoError(t, err)

	store, err := state.NewFileStateStore(filepath.Join(t.TempDir(), "state.json"), reg)
	require.NoError(t, err)

	c := NewConfig(
		WithPluginRegistry(reg),
		WithStateStore(store),
	)

	path, err := filepath.Abs("./internal/test_fixtures/config/registered/basic/main.xcl")
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	return c
}

// An address lookup is one call that yields the caller's own Go type, filled
// in from the applied configuration, nested blocks and all.

func TestFindReturnsThePopulatedResourceAtAnAddress(t *testing.T) {
	c := setupFindConfig(t)

	database, err := Find[registered.Database](c, "resource.database.main")
	require.NoError(t, err)
	require.NotNil(t, database)

	require.Equal(t, "resource.database.main", database.Meta.ID)
	require.Equal(t, "us-east", database.Location)
	require.Equal(t, 5432, database.Port)

	require.NotNil(t, database.Timeouts)
	require.Equal(t, 30, database.Timeouts.Connect)
	require.Equal(t, 60, database.Timeouts.Read)
}

// One entity is reachable by more than one spelling of its address: the
// module relative form an entity inside a module carries, the same address
// with a trailing attribute, and the non normalised form a stored reference
// between entities holds, whose attribute path runs several segments deep.
// Each names the entity, and the attribute is not part of the question.

func TestFindResolvesAModuleRelativeAddress(t *testing.T) {
	c := setupFindConfig(t)

	database, err := Find[registered.Database](c, "module.shared.resource.database.shared")
	require.NoError(t, err)
	require.NotNil(t, database)

	require.Equal(t, "module.shared.resource.database.shared", database.Meta.ID)
	require.Equal(t, "eu-west", database.Location)
	require.Equal(t, 5433, database.Port)
}

func TestFindResolvesANonNormalisedAddress(t *testing.T) {
	c := setupFindConfig(t)

	// the form a raw reference holds, an address followed by an attribute
	// path of more than one segment
	database, err := Find[registered.Database](c, "module.shared.resource.database.shared.meta.id")
	require.NoError(t, err)
	require.NotNil(t, database)

	require.Equal(t, "module.shared.resource.database.shared", database.Meta.ID)
	require.Equal(t, "eu-west", database.Location)
}

func TestFindResolvesAnAddressCarryingATrailingAttribute(t *testing.T) {
	c := setupFindConfig(t)

	database, err := Find[registered.Database](c, "module.shared.resource.database.shared.location")
	require.NoError(t, err)
	require.NotNil(t, database)

	require.Equal(t, "module.shared.resource.database.shared", database.Meta.ID)
	require.Equal(t, "eu-west", database.Location)
}

func TestFindResolvesTheSameEntityFromEveryFormOfItsAddress(t *testing.T) {
	c := setupFindConfig(t)

	moduleRelative, err := Find[registered.Database](c, "module.shared.resource.database.shared")
	require.NoError(t, err)

	nonNormalised, err := Find[registered.Database](c, "module.shared.resource.database.shared.meta.id")
	require.NoError(t, err)

	withAttribute, err := Find[registered.Database](c, "module.shared.resource.database.shared.location")
	require.NoError(t, err)

	require.Same(t, moduleRelative, nonNormalised)
	require.Same(t, moduleRelative, withAttribute)
}

func TestFindResolvesATrailingAttributeOnARootAddress(t *testing.T) {
	c := setupFindConfig(t)

	withAttribute, err := Find[registered.Database](c, "resource.database.main.location")
	require.NoError(t, err)

	withoutAttribute, err := Find[registered.Database](c, "resource.database.main")
	require.NoError(t, err)

	require.Same(t, withoutAttribute, withAttribute)
	require.Equal(t, "us-east", withAttribute.Location)
}

// The same address forms resolve without the caller naming a Go type at all.
// These are the acceptance cases that storage used to hold, they moved here
// with the lookup: a module relative address, a non normalised address and an
// address carrying a trailing attribute each resolve to the entity they name.

func TestFindResourceResolvesARootAddress(t *testing.T) {
	c := setupFindConfig(t)

	entity, err := c.FindResource("resource.database.main")
	require.NoError(t, err)

	database, isDatabase := entity.(*registered.Database)
	require.True(t, isDatabase, "expected a *registered.Database, got %T", entity)

	require.Equal(t, "resource.database.main", database.Meta.ID)
	require.Equal(t, "us-east", database.Location)
}

func TestFindResourceResolvesAModuleRelativeAddress(t *testing.T) {
	c := setupFindConfig(t)

	entity, err := c.FindResource("module.shared.resource.database.shared")
	require.NoError(t, err)

	database, isDatabase := entity.(*registered.Database)
	require.True(t, isDatabase, "expected a *registered.Database, got %T", entity)

	require.Equal(t, "module.shared.resource.database.shared", database.Meta.ID)
	require.Equal(t, "eu-west", database.Location)
}

func TestFindResourceResolvesAnAddressCarryingATrailingAttribute(t *testing.T) {
	c := setupFindConfig(t)

	want, err := c.FindResource("resource.database.main")
	require.NoError(t, err)

	got, err := c.FindResource("resource.database.main.location")
	require.NoError(t, err)

	require.Same(t, want, got)
}

func TestFindResourceResolvesAModuleRelativeAddressCarryingATrailingAttribute(t *testing.T) {
	c := setupFindConfig(t)

	want, err := c.FindResource("module.shared.resource.database.shared")
	require.NoError(t, err)

	got, err := c.FindResource("module.shared.resource.database.shared.location")
	require.NoError(t, err)

	require.Same(t, want, got)
}

func TestFindResourceResolvesANonNormalisedAddress(t *testing.T) {
	c := setupFindConfig(t)

	want, err := c.FindResource("module.shared.resource.database.shared")
	require.NoError(t, err)

	// the form a raw reference holds, an address followed by an attribute
	// path of more than one segment
	got, err := c.FindResource("module.shared.resource.database.shared.meta.id")
	require.NoError(t, err)

	require.Same(t, want, got)
}

// A value the configuration publishes is asked for by address like anything
// else, and the answer is the value, not the declaration that produced it.

func TestFindReturnsThePublishedValueRatherThanItsDeclaration(t *testing.T) {
	c := setupFindConfig(t)

	location, err := Find[string](c, "module.shared.output.location")
	require.NoError(t, err)
	require.NotNil(t, location)

	require.Equal(t, "eu-west", *location)
}

func TestFindDoesNotReturnThePublishedValuesDeclaration(t *testing.T) {
	c := setupFindConfig(t)

	// the configuration holds the address as an output declaration
	declaration, err := c.FindResource("module.shared.output.location")
	require.NoError(t, err)
	require.IsType(t, &resources.Output{}, declaration)

	// asking for that declaration by address does not yield it, because the
	// address names the published value
	output, err := Find[resources.Output](c, "module.shared.output.location")
	require.Error(t, err)
	require.Nil(t, output)
}

func TestFindResolvesAPublishedValueDeclaredInsideAModule(t *testing.T) {
	c := setupFindConfig(t)

	location, err := Find[string](c, "module.shared.output.location")
	require.NoError(t, err)
	require.NotNil(t, location)
	require.Equal(t, "eu-west", *location)
}

func TestFindRejectsThePublishedValuesBareName(t *testing.T) {
	c := setupFindConfig(t)

	location, err := Find[string](c, "location")
	require.Error(t, err)
	require.Nil(t, location)
}

func TestFindRejectsAPublishedValueAddressedWithoutItsModule(t *testing.T) {
	c := setupFindConfig(t)

	location, err := Find[string](c, "output.location")
	require.ErrorIs(t, err, ErrNotFound)
	require.Nil(t, location)
}

// Converting an entity to a Go type that does not correspond to it is refused,
// across entities of every kind the configuration holds. The copy underneath
// would otherwise yield a value with most of its fields unset, so the refusal
// returns nothing at all rather than a partly filled value.

func TestFindRefusesADatabaseAskedForAsAnApp(t *testing.T) {
	c := setupFindConfig(t)

	app, err := Find[registered.App](c, "resource.database.main")
	require.ErrorIs(t, err, ErrTypeMismatch)
	require.Nil(t, app)

	var mismatch *TypeMismatchError
	require.True(t, errors.As(err, &mismatch))
	require.Equal(t, "resource.database.main", mismatch.Address)
	require.Equal(t, "registered.Database", mismatch.Got)
}

func TestFindRefusesAnAppAskedForAsAConsumer(t *testing.T) {
	c := setupFindConfig(t)

	consumer, err := Find[registered.Consumer](c, "resource.app.web")
	require.ErrorIs(t, err, ErrTypeMismatch)
	require.Nil(t, consumer)
}

func TestFindRefusesAConsumerAskedForAsADatabase(t *testing.T) {
	c := setupFindConfig(t)

	database, err := Find[registered.Database](c, "resource.consumer.reader")
	require.ErrorIs(t, err, ErrTypeMismatch)
	require.Nil(t, database)
}

func TestFindRefusesAVariableAskedForAsADatabase(t *testing.T) {
	c := setupFindConfig(t)

	database, err := Find[registered.Database](c, "variable.environment")
	require.ErrorIs(t, err, ErrTypeMismatch)
	require.Nil(t, database)
}

func TestFindRefusesAModuleAskedForAsADatabase(t *testing.T) {
	c := setupFindConfig(t)

	database, err := Find[registered.Database](c, "module.shared")
	require.ErrorIs(t, err, ErrTypeMismatch)
	require.Nil(t, database)

	var mismatch *TypeMismatchError
	require.True(t, errors.As(err, &mismatch))
	require.Equal(t, "module.shared", mismatch.Address)
	require.Equal(t, "resources.Module", mismatch.Got)
}

// An address that matches nothing is a recognisable condition, not an empty
// value the caller has to notice for themselves.

func TestFindReturnsNotFoundForAnAddressThatMatchesNothing(t *testing.T) {
	c := setupFindConfig(t)

	database, err := Find[registered.Database](c, "resource.database.missing")
	require.ErrorIs(t, err, ErrNotFound)
	require.Nil(t, database)

	var notFound *NotFoundError
	require.True(t, errors.As(err, &notFound))
	require.Equal(t, "resource.database.missing", notFound.Address)
}

func TestFindReturnsNotFoundForAModuleEntityAddressedAtTheRoot(t *testing.T) {
	c := setupFindConfig(t)

	// resource.database.shared exists, but only inside module.shared
	database, err := Find[registered.Database](c, "resource.database.shared")
	require.ErrorIs(t, err, ErrNotFound)
	require.Nil(t, database)
}

// As converts an entity taken from an untyped enumeration into a caller's Go
// type, and is the seam between enumerating a configuration and typing what
// comes back.

func TestAsReturnsTheEntityItselfWhenItAlreadyIsTheRequestedType(t *testing.T) {
	c := setupFindConfig(t)

	stored, err := c.FindResource("resource.database.main")
	require.NoError(t, err)

	database, err := As[registered.Database](stored)
	require.NoError(t, err)

	// the caller holds what the configuration holds, not a copy of it
	require.Same(t, stored, database)
}

func TestAsRefusesAnEntityOfAnotherNamedType(t *testing.T) {
	c := setupFindConfig(t)

	stored, err := c.FindResource("resource.database.main")
	require.NoError(t, err)

	app, err := As[registered.App](stored)
	require.ErrorIs(t, err, ErrTypeMismatch)
	require.Nil(t, app)
}

func TestAsRefusesNothingToConvert(t *testing.T) {
	database, err := As[registered.Database](nil)
	require.ErrorIs(t, err, ErrTypeMismatch)
	require.Nil(t, database)
}

func TestAsConvertsAnEnumeratedEntityToWhatFindReturns(t *testing.T) {
	c := setupFindConfig(t)

	var enumerated any
	for _, entity := range c.GetResources() {
		meta, err := types.GetMeta(entity)
		require.NoError(t, err)

		if meta.ID == "resource.app.web" {
			enumerated = entity
			break
		}
	}
	require.NotNil(t, enumerated)

	converted, err := As[registered.App](enumerated)
	require.NoError(t, err)

	found, err := Find[registered.App](c, "resource.app.web")
	require.NoError(t, err)

	require.Equal(t, found, converted)
	require.Equal(t, "production", converted.Environment)
	require.Equal(t, "us-east", converted.DatabaseLocation)
	require.Equal(t, "eu-west", converted.SharedLocation)
}

// An entity the configuration already holds as the caller's Go type is handed
// back as it stands, so the caller holds the configuration's value rather than
// a copy of it. A plugin backed entity has no Go type in the configuration at
// all, so there is nothing to hand back and the caller gets a filled in copy.

func TestFindReturnsTheEntityTheConfigurationHolds(t *testing.T) {
	c := setupFindConfig(t)

	stored, err := c.FindResource("resource.database.main")
	require.NoError(t, err)

	database, err := Find[registered.Database](c, "resource.database.main")
	require.NoError(t, err)

	require.Same(t, stored, database)
	require.Equal(t, "us-east", database.Location)
	require.Equal(t, 5432, database.Port)
}

func TestFindCopiesAPluginProvidedEntity(t *testing.T) {
	// setupQueryConfig is the harness with a plugin registered, its network
	// type is declared twice in the fixture
	c := setupQueryConfig(t)

	stored, err := c.FindResource("resource.network.frontend")
	require.NoError(t, err)

	_, isNetwork := stored.(*structs.Network)
	require.False(t, isNetwork, "a plugin backed entity is held as a generated type, not the plugin's Go type")

	network, err := Find[structs.Network](c, "resource.network.frontend")
	require.NoError(t, err)

	require.Equal(t, "resource.network.frontend", network.Meta.ID)
	require.Equal(t, "10.0.1.0/24", network.Subnet)
}
