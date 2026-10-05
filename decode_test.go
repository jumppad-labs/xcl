package xcl

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// databasesAndApps gathers two registered types as collections
type databasesAndApps struct {
	Databases []*registered.Database
	Apps      []*registered.App
}

// publishedOutputs gathers every declared output as a collection
type publishedOutputs struct {
	Outputs []*types.Output
}

// renamedDatabases holds databases under a name that says nothing about them
type renamedDatabases struct {
	Anything []*registered.Database
}

// singleApp holds the one app the configuration declares
type singleApp struct {
	App *registered.App
}

// singleDatabase asks for one database where the basic fixture declares two
type singleDatabase struct {
	Database *registered.Database
}

// singleCache asks for one cache where the basic fixture declares none
type singleCache struct {
	Cache *registered.Cache
}

// orderedCaches gathers the caches of the ordered fixtures
type orderedCaches struct {
	Caches []*registered.Cache
}

// appSettings is the application's own struct, it is never registered
type appSettings struct {
	Region  string
	Retries int
}

// withUnrelated mixes a registered collection with fields that are not
// configuration at all
type withUnrelated struct {
	Databases []*registered.Database
	Name      string
	Settings  appSettings
}

// withUnrelatedPointer holds a pointer to a struct the registry cannot reach
type withUnrelatedPointer struct {
	Settings *appSettings
}

// threeTypes gathers every registered type the basic fixture declares
type threeTypes struct {
	Databases []*registered.Database
	App       *registered.App
	Consumers []*registered.Consumer
}

// databasesOnly is the application's struct before a new type is declared
type databasesOnly struct {
	Databases []*registered.Database
}

// databasesAndCaches is the same struct once a field for the new type is added
type databasesAndCaches struct {
	Databases []*registered.Database
	Caches    []*registered.Cache
}

// clients gathers the cache clients, whose nested blocks reference caches
type clients struct {
	Clients []*cacheClient
}

// Decode fills a struct of the application's own from the applied
// configuration in one call. It is exercised against the same applied
// configurations as the lookups, so what it fills is what an apply actually
// produced, and its answers are checked against those of All and FindOne,
// which it must never disagree with.
//
// setupDisabledConfig registers the database type and applies the disabled
// fixture, which declares a single database that is disabled.
func setupDisabledConfig(t *testing.T) *Config {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	reg := registry.NewPluginRegistry()

	err := reg.RegisterType(&registered.Database{}, "resource", registered.TypeDatabase)
	require.NoError(t, err)

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	c, err := NewConfig(
		WithPluginRegistry(reg),
		WithStateStore(store),
	)
	require.NoError(t, err)

	path, err := filepath.Abs("./internal/test_fixtures/config/registered/disabled/main.xcl")
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	return c
}

// setupOrderedConfig registers the cache type bare and applies one of the
// ordered fixtures, forward or reversed, which declare the same two caches in
// opposite orders.
func setupOrderedConfig(t *testing.T, fixture string) *Config {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	reg := registry.NewPluginRegistry()

	err := reg.RegisterType(&registered.Cache{}, registered.TypeCache)
	require.NoError(t, err)

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	c, err := NewConfig(
		WithPluginRegistry(reg),
		WithStateStore(store),
	)
	require.NoError(t, err)

	path, err := filepath.Abs(filepath.Join("./internal/test_fixtures/config/registered/ordered", fixture, "main.xcl"))
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	return c
}

// A collection field receives every entity of its type with the values the
// configuration gave it, in the order the configuration declared them.

func TestDecodeFillsCollectionFieldsFromTheConfiguration(t *testing.T) {
	c := setupFindConfig(t)

	target := databasesAndApps{}
	err := Decode(c, &target)
	require.NoError(t, err)

	require.Len(t, target.Databases, 2)

	databases := map[string]*registered.Database{}
	for _, database := range target.Databases {
		databases[database.Meta.ID] = database
	}

	main := databases["resource.database.main"]
	require.NotNil(t, main)
	require.Equal(t, "us-east", main.Location)
	require.Equal(t, 5432, main.Port)

	shared := databases["module.shared.resource.database.shared"]
	require.NotNil(t, shared)
	require.Equal(t, "eu-west", shared.Location)
	require.Equal(t, 5433, shared.Port)

	require.Len(t, target.Apps, 1)
	require.Equal(t, "resource.app.web", target.Apps[0].Meta.ID)
}

func TestDecodeKeepsDeclarationOrder(t *testing.T) {
	c := setupOrderedConfig(t, "forward")

	target := orderedCaches{}
	err := Decode(c, &target)
	require.NoError(t, err)

	require.Len(t, target.Caches, 2)
	require.Equal(t, "cache.first", target.Caches[0].Meta.ID)
	require.Equal(t, "cache.second", target.Caches[1].Meta.ID)
}

func TestDecodeFollowsTheOppositeDeclarationOrder(t *testing.T) {
	c := setupOrderedConfig(t, "reversed")

	target := orderedCaches{}
	err := Decode(c, &target)
	require.NoError(t, err)

	require.Len(t, target.Caches, 2)
	require.Equal(t, "cache.second", target.Caches[0].Meta.ID)
	require.Equal(t, "cache.first", target.Caches[1].Meta.ID)
}

// Outputs are entities of one Go type, so a collection field of them is filled
// like any other, with every output the configuration declares at any depth.

func TestDecodeFillsAnOutputsField(t *testing.T) {
	c := setupOutputEntitiesConfig(t)

	target := publishedOutputs{}

	err := Decode(c, &target)
	require.NoError(t, err)

	require.Len(t, target.Outputs, 2)

	require.Equal(t, "output.greeting", target.Outputs[0].Meta.ID)
	require.Equal(t, "hello", target.Outputs[0].Value)

	require.Equal(t, "module.inner.output.location", target.Outputs[1].Meta.ID)
	require.Equal(t, "eu-west", target.Outputs[1].Value)
}

// A collection field holds exactly what All returns for its type: the same
// entities, not equal copies, element for element, disabled ones included.

func TestDecodeMatchesAllElementForElement(t *testing.T) {
	c := setupFindConfig(t)

	target := databasesAndApps{}
	err := Decode(c, &target)
	require.NoError(t, err)

	databases, err := All[registered.Database](c)
	require.NoError(t, err)

	require.Len(t, target.Databases, len(databases))
	for i := range databases {
		require.Same(t, databases[i], target.Databases[i])
	}

	apps, err := All[registered.App](c)
	require.NoError(t, err)

	require.Len(t, target.Apps, len(apps))
	for i := range apps {
		require.Same(t, apps[i], target.Apps[i])
	}
}

func TestDecodeMatchesAllIncludingADisabledBlock(t *testing.T) {
	c := setupDisabledConfig(t)

	target := databasesOnly{}
	err := Decode(c, &target)
	require.NoError(t, err)

	databases, err := All[registered.Database](c)
	require.NoError(t, err)

	require.Len(t, target.Databases, 1)
	require.Len(t, databases, 1)
	require.Same(t, databases[0], target.Databases[0])

	require.True(t, target.Databases[0].Disabled)
}

func TestDecodeReturnsTheSameInstanceAsFind(t *testing.T) {
	c := setupFindConfig(t)

	target := databasesOnly{}
	err := Decode(c, &target)
	require.NoError(t, err)

	var decoded *registered.Database
	for _, database := range target.Databases {
		if database.Meta.ID == "resource.database.main" {
			decoded = database
		}
	}
	require.NotNil(t, decoded)

	found, err := Find[registered.Database](c, "resource.database.main")
	require.NoError(t, err)
	require.Same(t, found, decoded)

	// a change made through the decoded field is seen through the lookup
	decoded.Location = "ap-south"

	again, err := Find[registered.Database](c, "resource.database.main")
	require.NoError(t, err)
	require.Equal(t, "ap-south", again.Location)
}

// A single field receives the one entity of its type, is left nil when there
// is none, and fails the call when there is more than one, with exactly the
// error FindOne gives for the same type.

func TestDecodeSetsASingleFieldWhenExactlyOneIsDeclared(t *testing.T) {
	c := setupFindConfig(t)

	target := singleApp{}
	err := Decode(c, &target)
	require.NoError(t, err)

	require.NotNil(t, target.App)
	require.Equal(t, "resource.app.web", target.App.Meta.ID)
}

func TestDecodeLeavesASingleFieldUnsetWhenNoneIsDeclared(t *testing.T) {
	c := setupFindConfig(t)

	target := singleCache{}
	err := Decode(c, &target)
	require.NoError(t, err)

	require.Nil(t, target.Cache)
}

func TestDecodeFailsForASingleFieldDeclaredMoreThanOnce(t *testing.T) {
	c := setupFindConfig(t)

	sentinel := &registered.Database{}
	target := singleDatabase{Database: sentinel}

	err := Decode(c, &target)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotUnique)

	var notUnique *NotUniqueError
	require.True(t, errors.As(err, &notUnique))
	require.Equal(t, 2, notUnique.Count)

	_, findOneErr := FindOne[registered.Database](c, "resource", "database")
	require.Equal(t, findOneErr, err)

	// the failed call assigned nothing
	require.Same(t, sentinel, target.Database)
}

func TestDecodeLeavesOtherFieldsUnchangedWhenASingleFieldFails(t *testing.T) {
	c := setupFindConfig(t)

	target := struct {
		Apps     []*registered.App
		Database *registered.Database
	}{}
	require.Nil(t, target.Apps)

	err := Decode(c, &target)
	require.ErrorIs(t, err, ErrNotUnique)

	// the apps resolved before the database failed, and were still not set
	require.Nil(t, target.Apps)
	require.Nil(t, target.Database)
}

// Fields are matched by type alone. A field's name plays no part, and a field
// that is not configuration, whether a plain value or a struct the registry
// cannot reach, keeps whatever the application put there.

func TestDecodeIgnoresFieldNames(t *testing.T) {
	c := setupFindConfig(t)

	renamed := renamedDatabases{}
	err := Decode(c, &renamed)
	require.NoError(t, err)

	named := databasesAndApps{}
	err = Decode(c, &named)
	require.NoError(t, err)

	require.Len(t, renamed.Anything, 2)
	require.Len(t, renamed.Anything, len(named.Databases))
	for i := range named.Databases {
		require.Same(t, named.Databases[i], renamed.Anything[i])
	}
}

func TestDecodeLeavesUnrelatedFieldsUntouched(t *testing.T) {
	c := setupFindConfig(t)

	target := withUnrelated{
		Name:     "billing",
		Settings: appSettings{Region: "us-east", Retries: 3},
	}

	err := Decode(c, &target)
	require.NoError(t, err)

	require.Len(t, target.Databases, 2)
	require.Equal(t, "billing", target.Name)
	require.Equal(t, appSettings{Region: "us-east", Retries: 3}, target.Settings)
}

func TestDecodeLeavesAnUnregisteredPointerFieldUntouched(t *testing.T) {
	c := setupFindConfig(t)

	settings := &appSettings{Region: "eu-west", Retries: 5}
	target := withUnrelatedPointer{Settings: settings}

	err := Decode(c, &target)
	require.NoError(t, err)

	require.Same(t, settings, target.Settings)
	require.Equal(t, appSettings{Region: "eu-west", Retries: 5}, *target.Settings)
}

// Only a non-nil pointer to a struct can be filled. Anything else is refused
// with ErrInvalidDecodeTarget, and neither the value passed nor the
// configuration is changed by the attempt.

func TestDecodeRejectsAStructPassedByValue(t *testing.T) {
	c := setupFindConfig(t)

	entitiesBefore := c.Entities()
	require.NotEmpty(t, entitiesBefore)

	target := databasesAndApps{}
	err := Decode(c, target)
	require.ErrorIs(t, err, ErrInvalidDecodeTarget)

	var invalid *InvalidDecodeTargetError
	require.True(t, errors.As(err, &invalid))

	require.Nil(t, target.Databases)
	require.Nil(t, target.Apps)

	entitiesAfter := c.Entities()
	require.Len(t, entitiesAfter, len(entitiesBefore))
	require.Same(t, entitiesBefore[0], entitiesAfter[0])
}

func TestDecodeRejectsANilPointer(t *testing.T) {
	c := setupFindConfig(t)

	entitiesBefore := c.Entities()
	require.NotEmpty(t, entitiesBefore)

	err := Decode(c, (*databasesAndApps)(nil))
	require.ErrorIs(t, err, ErrInvalidDecodeTarget)

	var invalid *InvalidDecodeTargetError
	require.True(t, errors.As(err, &invalid))

	entitiesAfter := c.Entities()
	require.Len(t, entitiesAfter, len(entitiesBefore))
	require.Same(t, entitiesBefore[0], entitiesAfter[0])
}

func TestDecodeRejectsAnUntypedNil(t *testing.T) {
	c := setupFindConfig(t)

	entitiesBefore := c.Entities()
	require.NotEmpty(t, entitiesBefore)

	err := Decode(c, nil)
	require.ErrorIs(t, err, ErrInvalidDecodeTarget)

	var invalid *InvalidDecodeTargetError
	require.True(t, errors.As(err, &invalid))

	entitiesAfter := c.Entities()
	require.Len(t, entitiesAfter, len(entitiesBefore))
	require.Same(t, entitiesBefore[0], entitiesAfter[0])
}

func TestDecodeRejectsAPointerToANonStruct(t *testing.T) {
	c := setupFindConfig(t)

	entitiesBefore := c.Entities()
	require.NotEmpty(t, entitiesBefore)

	someInt := 42
	err := Decode(c, &someInt)
	require.ErrorIs(t, err, ErrInvalidDecodeTarget)

	var invalid *InvalidDecodeTargetError
	require.True(t, errors.As(err, &invalid))

	require.Equal(t, 42, someInt)

	entitiesAfter := c.Entities()
	require.Len(t, entitiesAfter, len(entitiesBefore))
	require.Same(t, entitiesBefore[0], entitiesAfter[0])
}

// Before Apply there is nothing declared, which is not an error: collections
// come back empty and singles unset.

func TestDecodeBeforeApplyLeavesCollectionsEmptyAndSinglesUnset(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	reg := registry.NewPluginRegistry()

	err := reg.RegisterType(&registered.Database{}, "resource", registered.TypeDatabase)
	require.NoError(t, err)

	err = reg.RegisterType(&registered.App{}, "resource", registered.TypeApp)
	require.NoError(t, err)

	c, err := NewConfig(WithPluginRegistry(reg))
	require.NoError(t, err)

	target := struct {
		Databases []*registered.Database
		App       *registered.App
	}{}

	err = Decode(c, &target)
	require.NoError(t, err)

	require.NotNil(t, target.Databases)
	require.Empty(t, target.Databases)
	require.Nil(t, target.App)
}

// The method and the package function are two spellings of one call, and give
// the same answer, success or failure.

func TestDecodeMethodAndFunctionGiveEqualResults(t *testing.T) {
	c := setupFindConfig(t)

	byFunction := databasesAndApps{}
	err := Decode(c, &byFunction)
	require.NoError(t, err)

	byMethod := databasesAndApps{}
	err = c.Decode(&byMethod)
	require.NoError(t, err)

	require.Equal(t, byFunction, byMethod)

	require.Len(t, byMethod.Databases, len(byFunction.Databases))
	for i := range byFunction.Databases {
		require.Same(t, byFunction.Databases[i], byMethod.Databases[i])
	}

	require.Len(t, byMethod.Apps, len(byFunction.Apps))
	for i := range byFunction.Apps {
		require.Same(t, byFunction.Apps[i], byMethod.Apps[i])
	}
}

func TestDecodeMethodAndFunctionGiveEqualErrors(t *testing.T) {
	c := setupFindConfig(t)

	byFunction := singleDatabase{}
	functionErr := Decode(c, &byFunction)
	require.ErrorIs(t, functionErr, ErrNotUnique)

	byMethod := singleDatabase{}
	methodErr := c.Decode(&byMethod)
	require.ErrorIs(t, methodErr, ErrNotUnique)

	require.Equal(t, functionErr, methodErr)
}

// An entity whose nested blocks reference a whole block arrives with those
// references resolved, whether the field they land in is a pointer or a value.

func TestDecodeFillsWholeBlockReferencesThroughAPointerField(t *testing.T) {
	c := setupCacheClientConfig(t)

	target := clients{}
	err := Decode(c, &target)
	require.NoError(t, err)

	require.Len(t, target.Clients, 1)
	require.NotNil(t, target.Clients[0].Primary)
	require.NotNil(t, target.Clients[0].Primary.Cache)

	require.Equal(t, "us-east", target.Clients[0].Primary.Cache.Location)
}

func TestDecodeFillsWholeBlockReferencesThroughAValueField(t *testing.T) {
	c := setupCacheClientConfig(t)

	target := clients{}
	err := Decode(c, &target)
	require.NoError(t, err)

	require.Len(t, target.Clients, 1)
	require.NotNil(t, target.Clients[0].Mirror)

	require.Equal(t, "eu-west", target.Clients[0].Mirror.Cache.Location)
}

// The point of Decode: one call gathers every registered type the application
// cares about, and supporting a newly declared type takes nothing beyond a
// field for it.

func TestDecodeFillsEveryRegisteredTypeInOneCall(t *testing.T) {
	c := setupFindConfig(t)

	target := threeTypes{}
	err := Decode(c, &target)
	require.NoError(t, err)

	databases, err := All[registered.Database](c)
	require.NoError(t, err)
	require.Len(t, target.Databases, 2)
	require.Len(t, target.Databases, len(databases))
	for i := range databases {
		require.Same(t, databases[i], target.Databases[i])
	}

	app, err := FindOne[registered.App](c, "resource", "app")
	require.NoError(t, err)
	require.Same(t, app, target.App)

	consumers, err := All[registered.Consumer](c)
	require.NoError(t, err)
	require.Len(t, target.Consumers, 1)
	require.Len(t, target.Consumers, len(consumers))
	for i := range consumers {
		require.Same(t, consumers[i], target.Consumers[i])
	}
}

func TestDecodeFillsAFieldForANewlyDeclaredTypeWithNoOtherCode(t *testing.T) {
	c := setupBareTypeConfig(t)

	before := databasesOnly{}
	err := Decode(c, &before)
	require.NoError(t, err)

	require.Len(t, before.Databases, 1)
	require.Equal(t, "resource.database.main", before.Databases[0].Meta.ID)

	// the same call, with only a field for the cache added to the struct
	after := databasesAndCaches{}
	err = Decode(c, &after)
	require.NoError(t, err)

	require.Len(t, after.Databases, 1)
	require.Same(t, before.Databases[0], after.Databases[0])

	require.Len(t, after.Caches, 1)
	require.Equal(t, "cache.main", after.Caches[0].Meta.ID)
	require.Equal(t, "us-east", after.Caches[0].Location)
}
