package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl/entity"
	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// Thing is a plain Go resource type used to test type registration
type Thing struct {
	types.ResourceBase `xcl:",remain"`

	Size int `xcl:"size" json:"size"`
}

// Gadget is a second plain Go resource type, used to check that a clash
// leaves the first registered type in place
type Gadget struct {
	types.ResourceBase `xcl:",remain"`

	Colour string `xcl:"colour" json:"colour"`
}

// ComputedThing is a resource type with a computed field
type ComputedThing struct {
	types.ResourceBase `xcl:",remain"`

	Size int `xcl:"size" json:"size"`

	// ProviderID is a computed field
	ProviderID string `xcl:"provider_id,optional,computed" json:"provider_id,omitempty"`
}

// NotAResource is a struct that does not embed types.ResourceBase
type NotAResource struct {
	Size int `xcl:"size" json:"size"`
}

// thingProvider is a no-op provider for Thing resources
type thingProvider struct {
	plugins.DefaultChanged[*Thing]
}

var _ plugins.ResourceProvider[*Thing] = (*thingProvider)(nil)

func (p *thingProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
	return nil
}

func (p *thingProvider) Create(ctx context.Context, resource *Thing) (*Thing, error) {
	return resource, nil
}

func (p *thingProvider) Destroy(ctx context.Context, resource *Thing, force bool) error {
	return nil
}

func (p *thingProvider) Read(ctx context.Context, old *Thing, new *Thing) (*Thing, error) {
	return new, nil
}

func (p *thingProvider) Update(ctx context.Context, resource *Thing, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*Thing, error) {
	return resource, nil
}

func (p *thingProvider) Functions() plugins.ProviderFunctions {
	return nil
}

// thingPlugin is an in-process plugin that provides the resource type "thing"
type thingPlugin struct {
	plugins.PluginBase
}

var _ plugins.Plugin = (*thingPlugin)(nil)

func (p *thingPlugin) Init(logger logger.Logger, state plugins.State) error {
	return plugins.RegisterResourceProvider(&p.PluginBase, logger, state, "resource", "thing", &Thing{}, &thingProvider{})
}

// doubleThingPlugin is an in-process plugin that reports the resource type
// "thing" twice
type doubleThingPlugin struct {
	plugins.PluginBase
}

var _ plugins.Plugin = (*doubleThingPlugin)(nil)

func (p *doubleThingPlugin) Init(logger logger.Logger, state plugins.State) error {
	err := plugins.RegisterResourceProvider(&p.PluginBase, logger, state, "resource", "thing", &Thing{}, &thingProvider{})
	if err != nil {
		return err
	}

	return plugins.RegisterResourceProvider(&p.PluginBase, logger, state, "resource", "thing", &Thing{}, &thingProvider{})
}

// bigServerPlugin is an in-process plugin that provides the type "server"
// with the subtype "big", a type other than resource that takes a subtype
type bigServerPlugin struct {
	plugins.PluginBase
}

var _ plugins.Plugin = (*bigServerPlugin)(nil)

func (p *bigServerPlugin) Init(logger logger.Logger, state plugins.State) error {
	return plugins.RegisterResourceProvider(&p.PluginBase, logger, state, "server", "big", &Thing{}, &thingProvider{})
}

// chattyProvider keeps the plugin scoped logger it is initialised with, and
// logs through it rather than the call's logger when a Thing is created, the
// way a plugin logs outside a provider call
type chattyProvider struct {
	plugins.DefaultChanged[*Thing]

	log logger.Logger
}

var _ plugins.ResourceProvider[*Thing] = (*chattyProvider)(nil)

func (p *chattyProvider) Init(state plugins.State, functions plugins.ProviderFunctions, log logger.Logger) error {
	p.log = log
	p.log.Info("chatty provider initialised")
	return nil
}

func (p *chattyProvider) Create(ctx context.Context, resource *Thing) (*Thing, error) {
	p.log.Info("creating a thing")
	return resource, nil
}

func (p *chattyProvider) Destroy(ctx context.Context, resource *Thing, force bool) error {
	return nil
}

func (p *chattyProvider) Read(ctx context.Context, old *Thing, new *Thing) (*Thing, error) {
	return new, nil
}

func (p *chattyProvider) Update(ctx context.Context, resource *Thing, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*Thing, error) {
	return resource, nil
}

func (p *chattyProvider) Functions() plugins.ProviderFunctions {
	return nil
}

// chattyPlugin is an in-process plugin that provides the resource type
// "thing" through a chattyProvider
type chattyPlugin struct {
	plugins.PluginBase
}

var _ plugins.Plugin = (*chattyPlugin)(nil)

func (p *chattyPlugin) Init(log logger.Logger, state plugins.State) error {
	return plugins.RegisterResourceProvider(&p.PluginBase, log, state, "resource", "thing", &Thing{}, &chattyProvider{})
}

// failingPlugin is an in-process plugin whose Init fails
type failingPlugin struct {
	plugins.PluginBase
}

var _ plugins.Plugin = (*failingPlugin)(nil)

func (p *failingPlugin) Init(log logger.Logger, state plugins.State) error {
	return errors.New("boom")
}

// createThing creates a Thing through the provider of the loaded plugin that
// provides the type "thing"
func createThing(t *testing.T, c *Catalog) {
	t.Helper()

	data, err := json.Marshal(&Thing{Size: 1})
	require.NoError(t, err)

	thing := &Thing{}
	thing.Meta.Type = types.TypeResource
	thing.Meta.Subtype = "thing"

	provider := c.GetProvider(thing)
	require.NotNil(t, provider, "no loaded plugin provides the type thing")

	_, err = provider.Create(context.Background(), data)
	require.NoError(t, err)
}

// Registering types

func TestRegisterTypeSucceedsWithoutPlugins(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Thing{}, "resource", "thing"))

	require.True(t, c.IsRegisteredType("resource", "thing"))
	require.Empty(t, c.GetPluginHosts())
}

func TestCreateResourceReturnsRegisteredGoType(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Thing{}, "resource", "thing"))

	resource, err := c.CreateEntity("resource", "thing", "my_thing")
	require.NoError(t, err)

	thing, ok := resource.(*Thing)
	require.True(t, ok, "expected *Thing, got %T", resource)
	require.Equal(t, "my_thing", thing.Meta.Name)
	require.Equal(t, types.TypeResource, thing.Meta.Type)
	require.Equal(t, "thing", thing.Meta.Subtype)
}

// A declared type that clashes is returned, not panicked, because a clash
// between registries comes from how they are combined, not from one bad line

func TestDeclareTypeFailsForDuplicateNameInOneRegistry(t *testing.T) {
	c := New()

	err := c.DeclareType(registry.Type{Type: "resource", Subtype: "thing", Prototype: &Thing{}}, "local")
	require.NoError(t, err)

	err = c.DeclareType(registry.Type{Type: "resource", Subtype: "thing", Prototype: &Gadget{}}, "local")
	require.EqualError(t, err,
		`type "resource.thing" is provided by both type *catalog.Thing (registry local) and type *catalog.Gadget (registry local)`)

	var clash *xclerrors.TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "resource.thing", clash.Name)
	require.Equal(t, "type *catalog.Gadget", clash.Provider)
	require.Equal(t, "local", clash.Registry)
	require.Equal(t, "type *catalog.Thing", clash.Existing)
	require.Equal(t, "local", clash.ExistingRegistry)
}

func TestDeclareTypeFailsForDuplicateNameAcrossRegistries(t *testing.T) {
	c := New()

	err := c.DeclareType(registry.Type{Type: "resource", Subtype: "thing", Prototype: &Thing{}}, "first")
	require.NoError(t, err)

	err = c.DeclareType(registry.Type{Type: "resource", Subtype: "thing", Prototype: &Gadget{}}, "second")

	var clash *xclerrors.TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "resource.thing", clash.Name)
	require.Equal(t, "type *catalog.Gadget", clash.Provider)
	require.Equal(t, "second", clash.Registry)
	require.Equal(t, "type *catalog.Thing", clash.Existing)
	require.Equal(t, "first", clash.ExistingRegistry)
}

func TestDeclareTypeFailureForDuplicateNameKeepsTheFirstType(t *testing.T) {
	c := New()

	err := c.DeclareType(registry.Type{Type: "resource", Subtype: "thing", Prototype: &Thing{}}, "local")
	require.NoError(t, err)

	err = c.DeclareType(registry.Type{Type: "resource", Subtype: "thing", Prototype: &Gadget{}}, "local")
	require.Error(t, err)

	resource, err := c.CreateEntity("resource", "thing", "my_thing")
	require.NoError(t, err)

	thing, ok := resource.(*Thing)
	require.True(t, ok, "expected *Thing, got %T", resource)
	require.Equal(t, "my_thing", thing.Meta.Name)
}

func TestDeclareTypeFailsForBuiltinNameVariable(t *testing.T) {
	c := New()

	err := c.DeclareType(registry.Type{Type: "variable", Prototype: &Thing{}}, "local")

	var clash *xclerrors.TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "variable", clash.Name)
	require.Equal(t, "type *catalog.Thing", clash.Provider)
	require.Equal(t, "local", clash.Registry)
	require.Equal(t, "builtin", clash.Existing)
	require.Empty(t, clash.ExistingRegistry)

	require.False(t, c.IsRegisteredType("variable", ""))
}

func TestDeclareTypeFailsForBuiltinNameOutput(t *testing.T) {
	c := New()

	err := c.DeclareType(registry.Type{Type: "output", Prototype: &Thing{}}, "local")
	require.EqualError(t, err, `type "output" is provided by both builtin and type *catalog.Thing (registry local)`)

	require.False(t, c.IsRegisteredType("output", ""))
}

func TestDeclareTypeFailsForBuiltinNameModule(t *testing.T) {
	c := New()

	err := c.DeclareType(registry.Type{Type: "module", Prototype: &Thing{}}, "local")
	require.EqualError(t, err, `type "module" is provided by both builtin and type *catalog.Thing (registry local)`)

	require.False(t, c.IsRegisteredType("module", ""))
}

func TestDeclareTypeFailsForBuiltinNameRoot(t *testing.T) {
	c := New()

	err := c.DeclareType(registry.Type{Type: "root", Prototype: &Thing{}}, "local")
	require.EqualError(t, err, `type "root" is provided by both builtin and type *catalog.Thing (registry local)`)

	require.False(t, c.IsRegisteredType("root", ""))
}

func TestDeclareTypeFailsForABuiltinTypeWithASubtype(t *testing.T) {
	c := New()

	err := c.DeclareType(registry.Type{Type: "variable", Subtype: "big", Prototype: &Thing{}}, "local")

	var clash *xclerrors.TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "variable.big", clash.Name)
	require.Equal(t, "builtin", clash.Existing)
}

func TestRegisterTypeReturnsAClashWithoutARegistry(t *testing.T) {
	c := New()

	err := c.RegisterType(&Thing{}, "resource", "thing")
	require.NoError(t, err)

	err = c.RegisterType(&Gadget{}, "resource", "thing")
	require.EqualError(t, err, `type "resource.thing" is provided by both type *catalog.Thing and type *catalog.Gadget`)
}

func TestRegisterTypeAcceptsComputedField(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&ComputedThing{}, "resource", "computed_thing"))

	resource, err := c.CreateEntity("resource", "computed_thing", "my_thing")
	require.NoError(t, err)

	thing, ok := resource.(*ComputedThing)
	require.True(t, ok, "expected *ComputedThing, got %T", resource)
	require.Equal(t, "my_thing", thing.Meta.Name)
	require.Equal(t, types.TypeResource, thing.Meta.Type)
	require.Equal(t, "computed_thing", thing.Meta.Subtype)
}

// The shape of a declaration is checked by the declaration package, whose
// tests cover each case. A registry other than the local one may hand the
// catalog a malformed type, so it is returned rather than registered.

func TestDeclareTypeFailsForAMalformedDeclaration(t *testing.T) {
	c := New()

	err := c.DeclareType(registry.Type{Type: "resource", Subtype: "thing", Prototype: &NotAResource{}}, "custom")
	require.ErrorContains(t, err, `type "resource.thing" must be a pointer to a struct that embeds types.ResourceBase`)

	require.False(t, c.IsRegisteredType("resource", "thing"))
}

func TestDeclareTypeFailsForAnUnnamedDeclaration(t *testing.T) {
	c := New()

	err := c.DeclareType(registry.Type{Prototype: &Thing{}}, "custom")
	require.EqualError(t, err, "an entity type must be named")
}

func TestRegisterTypeFailsForMoreThanOneSubtype(t *testing.T) {
	c := New()

	err := c.RegisterType(&Thing{}, "server", "big", "small")
	require.EqualError(t, err, `type "server" takes at most one subtype, got 2`)

	require.False(t, c.KnownType("server", "big"))
}

func TestIsRegisteredTypeReportsRegisteredTypes(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Thing{}, "resource", "thing"))

	require.True(t, c.IsRegisteredType("resource", "thing"))
}

func TestIsRegisteredTypeIgnoresUnknownTypes(t *testing.T) {
	c := New()

	require.False(t, c.IsRegisteredType("resource", "thing"))
}

func TestIsRegisteredTypeIgnoresBuiltinTypes(t *testing.T) {
	c := New()

	require.False(t, c.IsRegisteredType("variable", ""))
}

func TestIsRegisteredTypeIgnoresPluginTypes(t *testing.T) {
	c := New()
	c.AddRegistry(localWith(&thingPlugin{}))

	err := c.Load(nil)
	require.NoError(t, err)

	require.False(t, c.IsRegisteredType("resource", "thing"))
}

// An entity has a type and an optional subtype. Registering with a subtype
// declares the type with it as the first label, i.e. server "big" "web", and
// without one declares it with only the name, i.e. thing "my_thing".

func TestRegisterTypeWithoutASubtypeRecordsTheTypeAlone(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Thing{}, "thing"))

	info, ok := c.Type("thing", "")
	require.True(t, ok)
	require.Equal(t, "thing", info.Type)
	require.Empty(t, info.Subtype)
}

func TestRegisterTypeWithASubtypeRecordsBoth(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Thing{}, "server", "big"))

	info, ok := c.Type("server", "big")
	require.True(t, ok)
	require.Equal(t, "server", info.Type)
	require.Equal(t, "big", info.Subtype)
}

func TestCreateEntityWithoutASubtypeMakesTheTypeItsOwn(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Thing{}, "thing"))

	entity, err := c.CreateEntity("thing", "", "my_thing")
	require.NoError(t, err)

	thing, ok := entity.(*Thing)
	require.True(t, ok, "expected *Thing, got %T", entity)
	require.Equal(t, "my_thing", thing.Meta.Name)
	require.Equal(t, "thing", thing.Meta.Type)
	require.Equal(t, "", thing.Meta.Subtype)
}

func TestCreateEntityWithASubtypeOfAnyTypeSetsBoth(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Thing{}, "server", "big"))

	entity, err := c.CreateEntity("server", "big", "web")
	require.NoError(t, err)

	thing, ok := entity.(*Thing)
	require.True(t, ok, "expected *Thing, got %T", entity)
	require.Equal(t, "web", thing.Meta.Name)
	require.Equal(t, "server", thing.Meta.Type)
	require.Equal(t, "big", thing.Meta.Subtype)
}

func TestCreateEntityUnderTheResourceTypeSetsBoth(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Thing{}, "resource", "thing"))

	entity, err := c.CreateEntity("resource", "thing", "my_thing")
	require.NoError(t, err)

	thing, ok := entity.(*Thing)
	require.True(t, ok, "expected *Thing, got %T", entity)
	require.Equal(t, types.TypeResource, thing.Meta.Type)
	require.Equal(t, "thing", thing.Meta.Subtype)
}

func TestCreateEntityFailsForATypeRegisteredWithADifferentSubtype(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Thing{}, "server", "big"))

	_, err := c.CreateEntity("server", "small", "web")
	require.ErrorContains(t, err, "server.small")
}

func TestRegisterTypeAcceptsSeveralSubtypesOfOneType(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Thing{}, "server", "big"))
	require.NoError(t, c.RegisterType(&Gadget{}, "server", "small"))

	require.True(t, c.IsRegisteredType("server", "big"))
	require.True(t, c.IsRegisteredType("server", "small"))
}

func TestDeclareTypeFailsForTheSameSubtypeTwice(t *testing.T) {
	c := New()

	err := c.DeclareType(registry.Type{Type: "server", Subtype: "big", Prototype: &Thing{}}, "local")
	require.NoError(t, err)

	err = c.DeclareType(registry.Type{Type: "server", Subtype: "big", Prototype: &Gadget{}}, "local")
	require.EqualError(t, err,
		`type "server.big" is provided by both type *catalog.Thing (registry local) and type *catalog.Gadget (registry local)`)
}

// A type keyword takes a subtype for every registration or for none, so an
// address such as server.big.web can always be read by position.

func TestDeclareTypeFailsForASubtypeForATypeDeclaredWithoutOne(t *testing.T) {
	c := New()

	err := c.DeclareType(registry.Type{Type: "server", Prototype: &Thing{}}, "local")
	require.NoError(t, err)

	err = c.DeclareType(registry.Type{Type: "server", Subtype: "big", Prototype: &Gadget{}}, "local")
	require.EqualError(t, err,
		`type "server" is declared without a subtype, i.e. 'server "<name>" {}', so it can not be registered with one`)

	var form *xclerrors.TypeFormError
	require.True(t, errors.As(err, &form))
	require.Equal(t, "server", form.Type)
	require.False(t, form.TakesSubtype)

	require.False(t, c.IsRegisteredType("server", "big"))
}

func TestDeclareTypeFailsForNoSubtypeForATypeDeclaredWithOne(t *testing.T) {
	c := New()

	err := c.DeclareType(registry.Type{Type: "server", Subtype: "big", Prototype: &Thing{}}, "local")
	require.NoError(t, err)

	err = c.DeclareType(registry.Type{Type: "server", Prototype: &Gadget{}}, "local")
	require.EqualError(t, err,
		`type "server" takes a subtype, i.e. 'server "<subtype>" "<name>" {}', so it must be registered with one`)

	var form *xclerrors.TypeFormError
	require.True(t, errors.As(err, &form))
	require.Equal(t, "server", form.Type)
	require.True(t, form.TakesSubtype)

	require.False(t, c.IsRegisteredType("server", ""))
}

func TestDeclareTypeFailsForTheResourceTypeWithoutASubtype(t *testing.T) {
	c := New()

	err := c.DeclareType(registry.Type{Type: "resource", Prototype: &Thing{}}, "local")

	var form *xclerrors.TypeFormError
	require.True(t, errors.As(err, &form))
	require.Equal(t, "resource", form.Type)
	require.True(t, form.TakesSubtype)
}

func TestTakesSubtypeReportsTheFormOfARegisteredTypeWithASubtype(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Thing{}, "server", "big"))

	takes, known := c.TakesSubtype("server")
	require.True(t, known)
	require.True(t, takes)
}

func TestTakesSubtypeReportsTheFormOfARegisteredTypeWithoutASubtype(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Gadget{}, "cache"))

	takes, known := c.TakesSubtype("cache")
	require.True(t, known)
	require.False(t, takes)
}

func TestTakesSubtypeAlwaysKnowsTheResourceType(t *testing.T) {
	c := New()

	takes, known := c.TakesSubtype("resource")
	require.True(t, known)
	require.True(t, takes)
}

func TestTakesSubtypeIgnoresAnUnknownType(t *testing.T) {
	c := New()

	_, known := c.TakesSubtype("server")
	require.False(t, known)
}

// KnownType answers for every source the catalog draws on, which is what the
// parser asks before accepting a leading keyword. IsRegisteredType stays the
// narrow question of whether a declared Go type provided a name, because the
// lifecycle reads it to decide what never reaches a provider.

func TestKnownTypeReportsABuiltin(t *testing.T) {
	c := New()

	require.True(t, c.KnownType("variable", ""))
}

func TestKnownTypeReportsARegisteredType(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Thing{}, "resource", "thing"))

	require.True(t, c.KnownType("resource", "thing"))
}

func TestKnownTypeReportsARegisteredTypeWithoutASubtype(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Thing{}, "thing"))

	require.True(t, c.KnownType("thing", ""))
}

func TestKnownTypeReportsAPluginProvidedType(t *testing.T) {
	c := New()
	c.AddRegistry(localWith(&thingPlugin{}))

	err := c.Load(nil)
	require.NoError(t, err)

	require.True(t, c.KnownType("resource", "thing"))
}

func TestKnownTypeIgnoresAPluginTypeBeforeLoad(t *testing.T) {
	c := New()
	c.AddRegistry(localWith(&thingPlugin{}))

	require.False(t, c.KnownType("resource", "thing"))
}

func TestKnownTypeIgnoresAnUnknownName(t *testing.T) {
	c := New()

	require.False(t, c.KnownType("nosuchtype", ""))
}

// thing and resource.thing are different types, one declared thing "x" {} and
// the other resource "thing" "x" {}, so registering both is not a clash
func TestRegisterTypeAcceptsATypeNamedLikeAResourceSubtype(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Thing{}, "thing"))
	require.NoError(t, c.RegisterType(&Gadget{}, "resource", "thing"))

	require.True(t, c.IsRegisteredType("thing", ""))
	require.True(t, c.IsRegisteredType("resource", "thing"))
}

func TestRegisterTypeMatchingPluginTypeSucceedsBeforeLoad(t *testing.T) {
	c := New()
	c.AddRegistry(localWith(&thingPlugin{}))

	require.NoError(t, c.RegisterType(&Thing{}, "resource", "thing"))

	require.True(t, c.IsRegisteredType("resource", "thing"))
}

// Adding registries

func TestAddRegistryPanicsOnNil(t *testing.T) {
	c := New()

	require.PanicsWithValue(t, "xcl: registry must not be nil", func() { c.AddRegistry(nil) })
}

func TestAddRegistryWithMissingPluginPathDoesNotLoad(t *testing.T) {
	c := New()

	local := registry.NewLocal()
	local.RegisterExternalPlugin(filepath.Join(t.TempDir(), "does-not-exist"))
	c.AddRegistry(local)

	require.False(t, c.Loaded())
	require.Empty(t, c.GetPluginHosts())
}

// Loading plugins and checking their types

func TestLoadFailsForPluginTypeClashingWithDeclaredType(t *testing.T) {
	c := New()

	err := c.DeclareType(registry.Type{Type: "resource", Subtype: "thing", Prototype: &Thing{}}, "types")
	require.NoError(t, err)
	c.AddRegistry(localWith(&thingPlugin{}))

	err = c.Load(nil)
	require.Error(t, err)
	require.ErrorContains(t, err, `"resource.thing"`)

	var clash *xclerrors.TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "resource.thing", clash.Name)
	require.Equal(t, "thingPlugin", clash.Provider)
	require.Equal(t, registry.LocalName, clash.Registry)
	require.Equal(t, "type *catalog.Thing", clash.Existing)
	require.Equal(t, "types", clash.ExistingRegistry)
}

func TestLoadFailsForPluginClashingWithEarlierRegisteredTypeLeavingNoHost(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Thing{}, "resource", "thing"))
	c.AddRegistry(localWith(&thingPlugin{}))

	err := c.Load(nil)
	require.Error(t, err)

	require.Empty(t, c.GetPluginHosts())
	require.True(t, c.IsRegisteredType("resource", "thing"))
}

func TestLoadFailsWithClashForTypeRegisteredAfterRegistryAdded(t *testing.T) {
	c := New()

	c.AddRegistry(localWith(&thingPlugin{}))
	err := c.DeclareType(registry.Type{Type: "resource", Subtype: "thing", Prototype: &Thing{}}, "types")
	require.NoError(t, err)

	err = c.Load(nil)
	require.Error(t, err)

	var clash *xclerrors.TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "resource.thing", clash.Name)
	require.Equal(t, "type *catalog.Thing", clash.Existing)
	require.Equal(t, "types", clash.ExistingRegistry)

	require.Empty(t, c.GetPluginHosts())
}

func TestLoadFailsForPluginTypeInTheOtherFormOfADeclaredType(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Gadget{}, "server"))
	c.AddRegistry(localWith(&bigServerPlugin{}))

	err := c.Load(nil)
	require.Error(t, err)

	var form *xclerrors.TypeFormError
	require.True(t, errors.As(err, &form))
	require.Equal(t, "server", form.Type)
	require.False(t, form.TakesSubtype)
}

func TestLoadFailsForDuplicateTypeWithinRegistry(t *testing.T) {
	c := New()
	c.AddRegistry(localWith(&thingPlugin{}, &chattyPlugin{}))

	err := c.Load(nil)
	require.Error(t, err)
	require.ErrorContains(t, err, `"resource.thing"`)

	var clash *xclerrors.TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "resource.thing", clash.Name)
	require.Equal(t, "chattyPlugin", clash.Provider)
	require.Equal(t, registry.LocalName, clash.Registry)
	require.Equal(t, "thingPlugin", clash.Existing)
	require.Equal(t, registry.LocalName, clash.ExistingRegistry)

	require.Len(t, c.GetPluginHosts(), 1)
}

func TestLoadFailsForDuplicateTypeAcrossRegistries(t *testing.T) {
	c := New()
	c.AddRegistry(namedRegistry{Local: localWith(&thingPlugin{}), name: "first"})
	c.AddRegistry(namedRegistry{Local: localWith(&chattyPlugin{}), name: "second"})

	err := c.Load(nil)
	require.Error(t, err)

	var clash *xclerrors.TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "resource.thing", clash.Name)
	require.Equal(t, "chattyPlugin", clash.Provider)
	require.Equal(t, "second", clash.Registry)
	require.Equal(t, "thingPlugin", clash.Existing)
	require.Equal(t, "first", clash.ExistingRegistry)
	require.Equal(t, `type "resource.thing" is provided by both thingPlugin (registry first) and chattyPlugin (registry second)`, clash.Error())
}

func TestLoadFailsForPluginReportingSameTypeTwice(t *testing.T) {
	c := New()
	c.AddRegistry(localWith(&doubleThingPlugin{}))

	err := c.Load(nil)
	require.Error(t, err)
	require.ErrorContains(t, err, `"resource.thing"`)

	var clash *xclerrors.TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "resource.thing", clash.Name)
	require.Equal(t, "doubleThingPlugin", clash.Provider)
	require.Equal(t, "doubleThingPlugin", clash.Existing)

	require.Empty(t, c.GetPluginHosts())
}

func TestLoadAcceptsPluginWithUniqueTypes(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Gadget{}, "resource", "gadget"))
	c.AddRegistry(localWith(&thingPlugin{}))

	err := c.Load(nil)
	require.NoError(t, err)

	require.Len(t, c.GetPluginHosts(), 1)
}

func TestLoadOrderFollowsRegistriesThenRegistration(t *testing.T) {
	recorder := &testutil.EventRecorder{}
	c := New()
	c.AddRegistry(namedRegistry{Local: localWith(&thingPlugin{}, &bigServerPlugin{}), name: "a"})
	c.AddRegistry(namedRegistry{Local: localWith(&vaultPlugin{}), name: "b"})

	err := c.Load(recorder.Record)
	require.NoError(t, err)

	starts := []map[string]any{}
	for _, e := range lifecycleEvents(recorder.Events(), events.OperationLoad) {
		if e.Phase == events.PhaseStart {
			starts = append(starts, e.Meta)
		}
	}

	require.Equal(t, []map[string]any{
		{"plugin": "thingPlugin", "registry": "a"},
		{"plugin": "bigServerPlugin", "registry": "a"},
		{"plugin": "vaultPlugin", "registry": "b"},
	}, starts)
}

func TestLoadFailsWhenRegistryPluginsFails(t *testing.T) {
	c := New()
	c.AddRegistry(failingRegistry{name: "broken"})

	err := c.Load(nil)
	require.Error(t, err)
	require.ErrorIs(t, err, xclerrors.ErrPluginLoad)

	var loadErr *xclerrors.PluginLoadError
	require.True(t, errors.As(err, &loadErr))
	require.Equal(t, "broken", loadErr.Registry)
	require.Empty(t, loadErr.Plugin)
}

func TestLoadEmitsLoadErrorNamingRegistryWhenRegistryPluginsFails(t *testing.T) {
	recorder := &testutil.EventRecorder{}
	c := New()
	c.AddRegistry(failingRegistry{name: "broken"})

	loadErr := c.Load(recorder.Record)
	require.Error(t, loadErr)

	loads := lifecycleEvents(recorder.Events(), events.OperationLoad)
	require.Len(t, loads, 1)
	require.Equal(t, events.SourceCore, loads[0].Source)
	require.Equal(t, events.PhaseError, loads[0].Phase)
	require.Equal(t, loadErr, loads[0].Error)
	require.Equal(t, map[string]any{"registry": "broken"}, loads[0].Meta)
}

func TestLoadSkipsLaterRegistriesAfterAFailure(t *testing.T) {
	c := New()
	c.AddRegistry(failingRegistry{name: "broken"})
	c.AddRegistry(localWith(&thingPlugin{}))

	err := c.Load(nil)
	require.Error(t, err)

	require.Empty(t, c.GetPluginHosts())
}

func TestLoadKeepsPluginsLoadedBeforeAFailure(t *testing.T) {
	c := New()
	c.AddRegistry(localWith(&thingPlugin{}, &failingPlugin{}))

	err := c.Load(nil)
	require.Error(t, err)

	require.Len(t, c.GetPluginHosts(), 1)
}

func TestLoadWithMissingPluginPathFailsNamingIt(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	c := New()
	local := registry.NewLocal()
	local.RegisterExternalPlugin(missing)
	c.AddRegistry(local)

	err := c.Load(nil)
	require.Error(t, err)
	require.ErrorIs(t, err, xclerrors.ErrPluginLoad)

	var loadErr *xclerrors.PluginLoadError
	require.True(t, errors.As(err, &loadErr))
	require.Equal(t, "does-not-exist", loadErr.Plugin)
	require.Equal(t, registry.LocalName, loadErr.Registry)
}

func TestLoadFailsNamingRegistryForMissingBinary(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	c := New()
	local := registry.NewLocal()
	local.RegisterExternalPlugin(missing)
	c.AddRegistry(local)

	err := c.Load(nil)
	require.Error(t, err)
	require.ErrorContains(t, err, missing)
	require.ErrorContains(t, err, "registry local")
}

func TestLoadDiscoveredPluginThatFailsFailsTheLoad(t *testing.T) {
	setup := newTestPluginSetup(t)
	dir := t.TempDir()
	setup.createBrokenPlugin(dir, "xcl-plugin-broken")

	c := New()
	t.Cleanup(func() { stopHosts(c) })

	local := registry.NewLocal()
	local.RegisterPluginDirectory(dir)
	c.AddRegistry(local)

	err := c.Load(nil)
	require.Error(t, err)
	require.ErrorIs(t, err, xclerrors.ErrPluginLoad)

	var loadErr *xclerrors.PluginLoadError
	require.True(t, errors.As(err, &loadErr))
	require.Equal(t, "xcl-plugin-broken", loadErr.Plugin)
	require.Equal(t, registry.LocalName, loadErr.Registry)
}

func TestLoadWithFailingInProcessPluginFailsNamingIt(t *testing.T) {
	c := New()
	c.AddRegistry(localWith(&failingPlugin{}))

	err := c.Load(nil)
	require.Error(t, err)
	require.ErrorIs(t, err, xclerrors.ErrPluginLoad)
	require.ErrorContains(t, err, "boom")

	var loadErr *xclerrors.PluginLoadError
	require.True(t, errors.As(err, &loadErr))
	require.Equal(t, "failingPlugin", loadErr.Plugin)
	require.Equal(t, registry.LocalName, loadErr.Registry)
}

func TestLoadEmitsLoadStartAndErrorForFailingInProcessPlugin(t *testing.T) {
	recorder := &testutil.EventRecorder{}
	c := New()
	c.AddRegistry(localWith(&failingPlugin{}))

	loadErr := c.Load(recorder.Record)
	require.Error(t, loadErr)

	loads := lifecycleEvents(recorder.Events(), events.OperationLoad)
	require.Len(t, loads, 2)

	require.Equal(t, events.SourceCore, loads[0].Source)
	require.Equal(t, events.PhaseStart, loads[0].Phase)
	require.Equal(t, map[string]any{"plugin": "failingPlugin", "registry": "local"}, loads[0].Meta)

	require.Equal(t, events.SourceCore, loads[1].Source)
	require.Equal(t, events.PhaseError, loads[1].Phase)
	require.Equal(t, loadErr, loads[1].Error)
	require.Equal(t, map[string]any{"plugin": "failingPlugin", "registry": "local"}, loads[1].Meta)
}

func TestNoPluginHostsBeforeLoad(t *testing.T) {
	setup := newTestPluginSetup(t)
	examplePlugin := setup.buildExamplePlugin("test-plugin")

	c := New()
	local := registry.NewLocal()
	local.RegisterExternalPlugin(examplePlugin)
	c.AddRegistry(local)

	require.Empty(t, c.GetPluginHosts())
	require.False(t, c.Loaded())
}

func TestLoadedReportsTrueAfterLoad(t *testing.T) {
	c := New()

	err := c.Load(nil)
	require.NoError(t, err)

	require.True(t, c.Loaded())
}

func TestLoadedReportsTrueAfterAFailedLoad(t *testing.T) {
	c := New()
	c.AddRegistry(localWith(&failingPlugin{}))

	err := c.Load(nil)
	require.Error(t, err)

	require.True(t, c.Loaded())
}

func TestNoPluginProcessRunsBeforeLoad(t *testing.T) {
	requireLinuxProc(t)

	setup := newTestPluginSetup(t)
	examplePlugin := setup.buildExamplePlugin("test-plugin")

	c := New()
	t.Cleanup(func() { stopHosts(c) })

	local := registry.NewLocal()
	local.RegisterExternalPlugin(examplePlugin)
	c.AddRegistry(local)

	require.Equal(t, 0, processesRunning(t, examplePlugin), "adding a registry must not start its plugins")

	err := c.Load(nil)
	require.NoError(t, err)

	require.Equal(t, 1, processesRunning(t, examplePlugin), "Load starts the plugin")
}

func TestLoadRunsOnce(t *testing.T) {
	setup := newTestPluginSetup(t)
	examplePlugin := setup.buildExamplePlugin("test-plugin")

	c := New()
	t.Cleanup(func() { stopHosts(c) })

	local := registry.NewLocal()
	local.RegisterExternalPlugin(examplePlugin)
	c.AddRegistry(local)

	err := c.Load(nil)
	require.NoError(t, err)

	second := &testutil.EventRecorder{}
	err = c.Load(second.Record)
	require.NoError(t, err)

	require.Len(t, c.GetPluginHosts(), 1)
	require.Empty(t, second.Events())
}

func TestLoadCachesFailure(t *testing.T) {
	c := New()
	local := registry.NewLocal()
	local.RegisterExternalPlugin(filepath.Join(t.TempDir(), "does-not-exist"))
	c.AddRegistry(local)

	firstErr := c.Load(nil)
	require.Error(t, firstErr)

	second := &testutil.EventRecorder{}
	secondErr := c.Load(second.Record)

	require.Equal(t, firstErr, secondErr)
	require.Empty(t, second.Events())
}

func TestLoadEmitsLoadStartAndSuccessForInProcessPlugin(t *testing.T) {
	recorder := &testutil.EventRecorder{}
	c := New()
	c.AddRegistry(localWith(&thingPlugin{}))

	err := c.Load(recorder.Record)
	require.NoError(t, err)

	loads := lifecycleEvents(recorder.Events(), events.OperationLoad)
	require.Len(t, loads, 2)

	require.Equal(t, events.SourceCore, loads[0].Source)
	require.Equal(t, events.PhaseStart, loads[0].Phase)
	require.Equal(t, map[string]any{"plugin": "thingPlugin", "registry": "local"}, loads[0].Meta)

	require.Equal(t, events.SourceCore, loads[1].Source)
	require.Equal(t, events.PhaseSuccess, loads[1].Phase)
	require.Equal(t, map[string]any{"plugin": "thingPlugin", "registry": "local", "block_types": "thing"}, loads[1].Meta)
}

func TestLoadEmitsLoadStartAndSuccessForExternalPlugin(t *testing.T) {
	setup := newTestPluginSetup(t)
	examplePlugin := setup.buildExamplePlugin("test-plugin")

	recorder := &testutil.EventRecorder{}
	c := New()
	t.Cleanup(func() { stopHosts(c) })

	local := registry.NewLocal()
	local.RegisterExternalPlugin(examplePlugin)
	c.AddRegistry(local)

	err := c.Load(recorder.Record)
	require.NoError(t, err)

	loads := lifecycleEvents(recorder.Events(), events.OperationLoad)
	require.Len(t, loads, 2)

	require.Equal(t, events.SourceCore, loads[0].Source)
	require.Equal(t, events.PhaseStart, loads[0].Phase)
	require.Equal(t, map[string]any{"plugin": "test-plugin", "registry": "local"}, loads[0].Meta)

	require.Equal(t, events.SourceCore, loads[1].Source)
	require.Equal(t, events.PhaseSuccess, loads[1].Phase)
	require.Equal(t, map[string]any{"plugin": "test-plugin", "registry": "local", "block_types": "person"}, loads[1].Meta)
}

func TestLoadWithoutDiscoveryDirectoriesEmitsNoDiscoverEvents(t *testing.T) {
	recorder := &testutil.EventRecorder{}
	c := New()
	c.AddRegistry(localWith(&thingPlugin{}))

	err := c.Load(recorder.Record)
	require.NoError(t, err)

	require.Empty(t, lifecycleEvents(recorder.Events(), events.OperationDiscover))
}

func TestLoadRoutesPluginInitLogsToLoadsEmitter(t *testing.T) {
	recorder := &testutil.EventRecorder{}
	c := New()
	c.AddRegistry(localWith(&chattyPlugin{}))

	err := c.Load(recorder.Record)
	require.NoError(t, err)

	initialised := logsWithMessage(recorder.Events(), "chatty provider initialised")
	require.Len(t, initialised, 1)
	require.Equal(t, "chattyPlugin", initialised[0].Source)
}

func TestConcurrentLoadIsSafe(t *testing.T) {
	c := New()
	c.AddRegistry(localWith(&thingPlugin{}))

	var wg sync.WaitGroup
	loadErrors := make([]error, 8)

	for i := range loadErrors {
		wg.Add(2)

		go func() {
			defer wg.Done()
			loadErrors[i] = c.Load(nil)
		}()

		go func() {
			defer wg.Done()
			_ = c.Types()
			_ = c.KnownType("resource", "thing")
			_ = c.GetPluginHosts()
		}()
	}

	wg.Wait()

	for _, loadErr := range loadErrors {
		require.NoError(t, loadErr)
	}

	require.Len(t, c.GetPluginHosts(), 1)
	require.True(t, c.KnownType("resource", "thing"))
}

func TestActivateRoutesPluginLogsToTheActiveEmitter(t *testing.T) {
	c := New()
	c.AddRegistry(localWith(&chattyPlugin{}))

	err := c.Load(nil)
	require.NoError(t, err)

	recorder := &testutil.EventRecorder{}
	deactivate := c.Activate(recorder.Record)
	t.Cleanup(deactivate)

	createThing(t, c)

	created := logsWithMessage(recorder.Events(), "creating a thing")
	require.Len(t, created, 1)
	require.Equal(t, "chattyPlugin", created[0].Source)
}

func TestDeactivateStopsRouting(t *testing.T) {
	c := New()
	c.AddRegistry(localWith(&chattyPlugin{}))

	err := c.Load(nil)
	require.NoError(t, err)

	recorder := &testutil.EventRecorder{}
	deactivate := c.Activate(recorder.Record)
	deactivate()

	createThing(t, c)

	require.Empty(t, recorder.Events())
}

func TestDeactivateOfAnOlderActivationKeepsTheNewer(t *testing.T) {
	c := New()
	c.AddRegistry(localWith(&chattyPlugin{}))

	err := c.Load(nil)
	require.NoError(t, err)

	older := &testutil.EventRecorder{}
	deactivateOlder := c.Activate(older.Record)

	newer := &testutil.EventRecorder{}
	deactivateNewer := c.Activate(newer.Record)
	t.Cleanup(deactivateNewer)

	deactivateOlder()

	createThing(t, c)

	require.Empty(t, older.Events())
	require.Len(t, logsWithMessage(newer.Events(), "creating a thing"), 1)
}

// A plugin provides types the same way RegisterType does, by a type and an
// optional subtype, and its types need not be declared under resource.

func TestLoadedPluginTypeWithASubtypeIsKnown(t *testing.T) {
	c := New()
	c.AddRegistry(localWith(&bigServerPlugin{}))

	require.NoError(t, c.Load(nil))

	require.True(t, c.KnownType("server", "big"))

	takes, known := c.TakesSubtype("server")
	require.True(t, known)
	require.True(t, takes)
}

func TestCreateEntityCreatesALoadedPluginTypeWithASubtype(t *testing.T) {
	c := New()
	c.AddRegistry(localWith(&bigServerPlugin{}))

	require.NoError(t, c.Load(nil))

	entity, err := c.CreateEntity("server", "big", "web")
	require.NoError(t, err)

	meta, err := types.GetMeta(entity)
	require.NoError(t, err)
	require.Equal(t, "server", meta.Type)
	require.Equal(t, "big", meta.Subtype)
	require.Equal(t, "web", meta.Name)
}

func TestGetProviderFindsTheProviderOfALoadedPluginTypeWithASubtype(t *testing.T) {
	c := New()
	c.AddRegistry(localWith(&bigServerPlugin{}))

	require.NoError(t, c.Load(nil))

	entity, err := c.CreateEntity("server", "big", "web")
	require.NoError(t, err)

	require.NotNil(t, c.GetProvider(entity))
	require.NotNil(t, c.GetProviderForResource(entity))
}

// Use starts and stops the plugins around each operation

func TestUseStopsPluginsStartedBeforeAFailedLoad(t *testing.T) {
	requireLinuxProc(t)

	setup := newTestPluginSetup(t)
	examplePlugin := setup.buildExamplePlugin("test-plugin")

	c := New()
	t.Cleanup(func() { stopHosts(c) })

	local := registry.NewLocal()
	local.RegisterExternalPlugin(examplePlugin)
	local.RegisterPlugin(&failingPlugin{})
	c.AddRegistry(local)

	done, err := c.Use(nil)
	require.Error(t, err)
	require.Nil(t, done)

	require.Len(t, c.GetPluginHosts(), 1)
	require.Eventually(t, func() bool { return processesRunning(t, examplePlugin) == 0 }, defaultWait, pollInterval)
}
