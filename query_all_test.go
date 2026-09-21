package xcl

import (
	"errors"
	"testing"

	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// The Go type lookup names no segments at all: the addressing is read back
// from how the type was registered. It is exercised against the same applied
// configurations as the address and kind lookups, setupFindConfig for the kind
// led types and setupBareTypeConfig for the bare one.
//
// What matters is that it answers exactly as the kind lookup does, because it
// delegates to it rather than scanning the configuration a second time.

func TestAllReturnsWhatTheEquivalentKindLookupReturns(t *testing.T) {
	c := setupFindConfig(t)

	byGoType, err := All[registered.Database](c)
	require.NoError(t, err)

	byKind, err := FindByType[registered.Database](c, "resource", "database")
	require.NoError(t, err)

	require.Len(t, byGoType, len(byKind))
	require.Len(t, byGoType, 2)

	// the same entities, not merely equal copies of them
	for i := range byKind {
		require.Same(t, byKind[i], byGoType[i])
	}
}

func TestAllReturnsEveryDeclaredEntityOfTheGoType(t *testing.T) {
	c := setupFindConfig(t)

	databases, err := All[registered.Database](c)
	require.NoError(t, err)

	found := []string{}
	for _, database := range databases {
		found = append(found, database.Meta.ID)
	}

	// both databases, the one at the root and the one inside the module
	require.ElementsMatch(t, []string{
		"resource.database.main",
		"module.shared.resource.database.shared",
	}, found)
}

func TestAllDerivesTheBareFormAddressingForABareRegisteredType(t *testing.T) {
	c := setupBareTypeConfig(t)

	byGoType, err := All[registered.Cache](c)
	require.NoError(t, err)

	// a bare type leads its own declaration, so the derived path is the single
	// segment rather than the resource keyword and a variety
	byKind, err := FindByType[registered.Cache](c, "cache")
	require.NoError(t, err)

	require.Len(t, byGoType, len(byKind))
	require.Len(t, byGoType, 1)
	require.Same(t, byKind[0], byGoType[0])
	require.Equal(t, "cache.main", byGoType[0].Meta.ID)
}

// A plugin provides a schema and no Go type, so there is nothing to derive an
// address from. The refusal says so and names the lookup that does work for it,
// rather than returning the empty result that reads as "you declared none".

func TestAllReportsAPluginProvidedTypeIsNotRegistered(t *testing.T) {
	// setupQueryConfig is the harness with a plugin registered, its network
	// type is declared twice in the fixture and still cannot be reached this way
	c := setupQueryConfig(t)

	networks, err := All[structs.Network](c)
	require.ErrorIs(t, err, ErrNotRegistered)
	require.Nil(t, networks)

	var notRegistered *NotRegisteredError
	require.True(t, errors.As(err, &notRegistered))
	require.Contains(t, notRegistered.Use, "FindByType")
	require.Contains(t, notRegistered.Use, "kind")
	require.Contains(t, notRegistered.Use, "variety")
}

func TestFindByTypeStillReachesThePluginProvidedTypeAllRefuses(t *testing.T) {
	c := setupQueryConfig(t)

	networks, err := FindByType[structs.Network](c, "resource", "network")
	require.NoError(t, err)
	require.Len(t, networks, 2)
}

// A block that only ever exists nested inside another declaration has no
// address of its own, so it is refused before any addressing is derived, and
// with the same condition the kind lookup reports for it.

func TestAllRejectsATypeThatIsOnlyANestedBlock(t *testing.T) {
	c := setupFindConfig(t)

	// Timeouts is the timeouts block of a Database, it embeds no ResourceBase
	timeouts, err := All[registered.Timeouts](c)
	require.ErrorIs(t, err, ErrNotAnEntity)
	require.Nil(t, timeouts)

	var notAnEntity *NotAnEntityError
	require.True(t, errors.As(err, &notAnEntity))
	require.Equal(t, "declaration", notAnEntity.ReachedThrough)
}

func TestAllRejectsANestedBlockWithTheSameConditionAsTheKindLookup(t *testing.T) {
	c := setupFindConfig(t)

	_, byGoType := All[registered.Timeouts](c)
	require.Error(t, byGoType)

	_, byKind := FindByType[registered.Timeouts](c, "resource", "database")
	require.Error(t, byKind)

	require.Equal(t, byKind.Error(), byGoType.Error())
}

// Enumerating the configuration yields every declaration, whatever kind it is.
// The name says entity rather than resource because variables, published
// values and modules are declarations too.

func TestEntitiesEnumeratesEveryKindOfDeclaration(t *testing.T) {
	c := setupFindConfig(t)

	kinds := map[string]int{}
	for _, entity := range c.Entities() {
		meta, err := types.GetMeta(entity)
		require.NoError(t, err)

		kinds[meta.Type]++
	}

	require.GreaterOrEqual(t, kinds[types.TypeResource], 1)
	require.GreaterOrEqual(t, kinds[resources.TypeVariable], 1)
	require.GreaterOrEqual(t, kinds[resources.TypeOutput], 1)
	require.GreaterOrEqual(t, kinds[resources.TypeModule], 1)
}

func TestEntitiesEnumeratesOneEntryPerDeclaration(t *testing.T) {
	c := setupFindConfig(t)

	addresses := []string{}
	for _, entity := range c.Entities() {
		meta, err := types.GetMeta(entity)
		require.NoError(t, err)

		addresses = append(addresses, meta.ID)
	}

	// the seven blocks the fixture declares, across its two files
	require.ElementsMatch(t, []string{
		"variable.environment",
		"resource.database.main",
		"module.shared",
		"module.shared.resource.database.shared",
		"module.shared.output.location",
		"resource.app.web",
		"resource.consumer.reader",
	}, addresses)
}

func TestEntitiesOmitsTheSynthesisedRoot(t *testing.T) {
	c := setupFindConfig(t)

	// a root is built to hang the dependency graph from, it is not parsed from
	// the configuration and has no place in what the configuration declares
	for _, entity := range c.Entities() {
		meta, err := types.GetMeta(entity)
		require.NoError(t, err)

		require.NotEqual(t, resources.TypeRoot, meta.Type)
	}
}

func TestEntityCountMatchesTheEntitiesEnumerated(t *testing.T) {
	c := setupFindConfig(t)

	require.Equal(t, len(c.Entities()), c.EntityCount())
	require.Equal(t, 7, c.EntityCount())
}

// An entity taken from the untyped enumeration becomes the caller's Go type in
// one call, and is the same value the address lookup returns for it.

func TestAsConvertsAnEntityFromEntitiesToWhatFindReturns(t *testing.T) {
	c := setupFindConfig(t)

	var enumerated any
	for _, entity := range c.Entities() {
		meta, err := types.GetMeta(entity)
		require.NoError(t, err)

		if meta.ID == "resource.database.main" {
			enumerated = entity
			break
		}
	}
	require.NotNil(t, enumerated)

	converted, err := As[registered.Database](enumerated)
	require.NoError(t, err)

	found, err := Find[registered.Database](c, "resource.database.main")
	require.NoError(t, err)

	require.Equal(t, found, converted)
	require.Equal(t, "us-east", converted.Location)
	require.Equal(t, 5432, converted.Port)
}

// Published values span whatever the configuration publishes, so they are
// never typeable as a collection. One call returns them all, keyed by address
// and holding the resolved value rather than the declaration.

func TestOutputsReturnsEveryPublishedValueKeyedByAddress(t *testing.T) {
	c := setupFindConfig(t)

	published := c.Outputs()
	require.Len(t, published, 1)

	value, ok := published["module.shared.output.location"]
	require.True(t, ok)
	require.Equal(t, "eu-west", value)
}

func TestOutputsReturnsTheResolvedValueRatherThanItsDeclaration(t *testing.T) {
	c := setupFindConfig(t)

	published := c.Outputs()

	_, isDeclaration := published["module.shared.output.location"].(*resources.Output)
	require.False(t, isDeclaration)

	// the same value the address lookup gives for that address
	location, err := Find[string](c, "module.shared.output.location")
	require.NoError(t, err)
	require.Equal(t, *location, published["module.shared.output.location"])
}

// The enumeration and the count were renamed, because they never counted only
// resources. Code written against the old names still compiles and still gets
// the same answer.

func TestGetResourcesReturnsWhatEntitiesReturns(t *testing.T) {
	c := setupFindConfig(t)

	deprecated := c.GetResources()
	current := c.Entities()

	require.Len(t, deprecated, len(current))
	for i := range current {
		require.Same(t, current[i], deprecated[i])
	}
}

func TestResourceCountReturnsWhatEntityCountReturns(t *testing.T) {
	c := setupFindConfig(t)

	require.Equal(t, c.EntityCount(), c.ResourceCount())
}
