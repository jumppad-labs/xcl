package registry

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
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

func (p *thingProvider) Update(ctx context.Context, resource *Thing) (*Thing, error) {
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

func TestRegisterTypeSucceedsWithoutPlugins(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "resource", "thing")
	require.NoError(t, err)

	require.Empty(t, r.GetPluginHosts())
}

func TestCreateResourceReturnsRegisteredGoType(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "resource", "thing")
	require.NoError(t, err)

	resource, err := r.CreateEntity("resource", "thing", "my_thing")
	require.NoError(t, err)

	thing, ok := resource.(*Thing)
	require.True(t, ok, "expected *Thing, got %T", resource)
	require.Equal(t, "my_thing", thing.Meta.Name)
	require.Equal(t, types.TypeResource, thing.Meta.Type)
	require.Equal(t, "thing", thing.Meta.Subtype)
}

func TestRegisterTypeRejectsDuplicateName(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "resource", "thing")
	require.NoError(t, err)

	err = r.RegisterType(&Gadget{}, "resource", "thing")
	require.Error(t, err)
	require.ErrorContains(t, err, `"resource.thing"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "resource.thing", clash.Name)
	require.Equal(t, "registered type", clash.Existing)

	resource, err := r.CreateEntity("resource", "thing", "my_thing")
	require.NoError(t, err)

	thing, ok := resource.(*Thing)
	require.True(t, ok, "expected *Thing, got %T", resource)
	require.Equal(t, "my_thing", thing.Meta.Name)
	require.Equal(t, types.TypeResource, thing.Meta.Type)
	require.Equal(t, "thing", thing.Meta.Subtype)
}

func TestRegisterTypeRejectsBuiltinNameVariable(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "variable")
	require.Error(t, err)
	require.ErrorContains(t, err, `"variable"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "variable", clash.Name)
	require.Equal(t, "builtin", clash.Existing)
	require.False(t, r.IsRegisteredType("variable", ""))
}

func TestRegisterTypeRejectsBuiltinNameOutput(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "output")
	require.Error(t, err)
	require.ErrorContains(t, err, `"output"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "output", clash.Name)
	require.Equal(t, "builtin", clash.Existing)
	require.False(t, r.IsRegisteredType("output", ""))
}

func TestRegisterTypeRejectsBuiltinNameModule(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "module")
	require.Error(t, err)
	require.ErrorContains(t, err, `"module"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "module", clash.Name)
	require.Equal(t, "builtin", clash.Existing)
	require.False(t, r.IsRegisteredType("module", ""))
}

func TestRegisterTypeRejectsBuiltinNameRoot(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "root")
	require.Error(t, err)
	require.ErrorContains(t, err, `"root"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "root", clash.Name)
	require.Equal(t, "builtin", clash.Existing)
	require.False(t, r.IsRegisteredType("root", ""))
}

func TestRegisterTypeRejectsLoadedPluginProvidedName(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterPlugin(&thingPlugin{})
	require.NoError(t, err)

	err = r.Load(nil)
	require.NoError(t, err)

	err = r.RegisterType(&Thing{}, "resource", "thing")
	require.Error(t, err)
	require.ErrorContains(t, err, `"resource.thing"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "resource.thing", clash.Name)
	require.Equal(t, "plugin", clash.Existing)
	require.False(t, r.IsRegisteredType("resource", "thing"))
}

func TestLoadRejectsPluginClashingWithEarlierRegisteredType(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "resource", "thing")
	require.NoError(t, err)

	err = r.RegisterPlugin(&thingPlugin{})
	require.NoError(t, err)

	err = r.Load(nil)
	require.Error(t, err)
	require.ErrorContains(t, err, `"resource.thing"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "resource.thing", clash.Name)
	require.Equal(t, "registered type", clash.Existing)

	require.Empty(t, r.GetPluginHosts())
}

func TestLoadRejectsNameFromAnotherPlugin(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterPlugin(&thingPlugin{})
	require.NoError(t, err)

	err = r.RegisterPlugin(&thingPlugin{})
	require.NoError(t, err)

	err = r.Load(nil)
	require.Error(t, err)
	require.ErrorContains(t, err, `"resource.thing"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "resource.thing", clash.Name)
	require.Equal(t, "plugin", clash.Existing)

	require.Len(t, r.GetPluginHosts(), 1)
}

func TestLoadRejectsPluginReportingSameTypeTwice(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterPlugin(&doubleThingPlugin{})
	require.NoError(t, err)

	err = r.Load(nil)
	require.Error(t, err)
	require.ErrorContains(t, err, `"resource.thing"`)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "resource.thing", clash.Name)
	require.Equal(t, "the same plugin", clash.Existing)

	require.Empty(t, r.GetPluginHosts())
}

func TestLoadAcceptsPluginWithUniqueTypes(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Gadget{}, "resource", "gadget")
	require.NoError(t, err)

	err = r.RegisterPlugin(&thingPlugin{})
	require.NoError(t, err)

	err = r.Load(nil)
	require.NoError(t, err)

	require.Len(t, r.GetPluginHosts(), 1)
}

func TestRegisterTypeAcceptsComputedField(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&ComputedThing{}, "resource", "computed_thing")
	require.NoError(t, err)

	resource, err := r.CreateEntity("resource", "computed_thing", "my_thing")
	require.NoError(t, err)

	thing, ok := resource.(*ComputedThing)
	require.True(t, ok, "expected *ComputedThing, got %T", resource)
	require.Equal(t, "my_thing", thing.Meta.Name)
	require.Equal(t, types.TypeResource, thing.Meta.Type)
	require.Equal(t, "computed_thing", thing.Meta.Subtype)
}

func TestRegisterTypeRejectsNonPointer(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(Thing{}, "resource", "thing")
	require.Error(t, err)
	require.ErrorContains(t, err, `type "resource.thing" must be a pointer to a struct that embeds types.ResourceBase`)

	require.False(t, r.IsRegisteredType("resource", "thing"))
}

func TestRegisterTypeRejectsNil(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(nil, "resource", "thing")
	require.Error(t, err)
	require.ErrorContains(t, err, `type "resource.thing" must be a pointer to a struct that embeds types.ResourceBase`)

	require.False(t, r.IsRegisteredType("resource", "thing"))
}

func TestRegisterTypeRejectsNilPointer(t *testing.T) {
	r := NewPluginRegistry()

	var thing *Thing
	err := r.RegisterType(thing, "resource", "thing")
	require.Error(t, err)
	require.ErrorContains(t, err, `type "resource.thing" must be a pointer to a struct that embeds types.ResourceBase`)

	require.False(t, r.IsRegisteredType("resource", "thing"))
}

func TestRegisterTypeRejectsPointerToNonStruct(t *testing.T) {
	r := NewPluginRegistry()

	size := 1
	err := r.RegisterType(&size, "resource", "thing")
	require.Error(t, err)
	require.ErrorContains(t, err, `type "resource.thing" must be a pointer to a struct that embeds types.ResourceBase`)

	require.False(t, r.IsRegisteredType("resource", "thing"))
}

func TestRegisterTypeRejectsTypeWithoutResourceBase(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&NotAResource{}, "resource", "thing")
	require.Error(t, err)
	require.ErrorContains(t, err, `type "resource.thing" must be a pointer to a struct that embeds types.ResourceBase`)

	require.False(t, r.IsRegisteredType("resource", "thing"))
}

func TestIsRegisteredTypeReportsRegisteredTypes(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "resource", "thing")
	require.NoError(t, err)

	require.True(t, r.IsRegisteredType("resource", "thing"))
}

func TestIsRegisteredTypeIgnoresUnknownTypes(t *testing.T) {
	r := NewPluginRegistry()

	require.False(t, r.IsRegisteredType("resource", "thing"))
}

func TestIsRegisteredTypeIgnoresBuiltinTypes(t *testing.T) {
	r := NewPluginRegistry()

	require.False(t, r.IsRegisteredType("variable", ""))
}

func TestIsRegisteredTypeIgnoresPluginTypes(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterPlugin(&thingPlugin{})
	require.NoError(t, err)

	err = r.Load(nil)
	require.NoError(t, err)

	require.False(t, r.IsRegisteredType("resource", "thing"))
}

// An entity has a type and an optional subtype. Registering with a subtype
// declares the type with it as the first label, i.e. server "big" "web", and
// without one declares it with only the name, i.e. thing "my_thing".

func TestRegisterTypeWithoutASubtypeRecordsTheTypeAlone(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "thing")
	require.NoError(t, err)

	info, ok := r.Type("thing", "")
	require.True(t, ok)
	require.Equal(t, "thing", info.Type)
	require.Empty(t, info.Subtype)
}

func TestRegisterTypeWithASubtypeRecordsBoth(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "server", "big")
	require.NoError(t, err)

	info, ok := r.Type("server", "big")
	require.True(t, ok)
	require.Equal(t, "server", info.Type)
	require.Equal(t, "big", info.Subtype)
}

func TestCreateEntityWithoutASubtypeMakesTheTypeItsOwn(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "thing")
	require.NoError(t, err)

	entity, err := r.CreateEntity("thing", "", "my_thing")
	require.NoError(t, err)

	thing, ok := entity.(*Thing)
	require.True(t, ok, "expected *Thing, got %T", entity)
	require.Equal(t, "my_thing", thing.Meta.Name)
	require.Equal(t, "thing", thing.Meta.Type)
	require.Equal(t, "", thing.Meta.Subtype)
}

func TestCreateEntityWithASubtypeOfAnyTypeSetsBoth(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "server", "big")
	require.NoError(t, err)

	entity, err := r.CreateEntity("server", "big", "web")
	require.NoError(t, err)

	thing, ok := entity.(*Thing)
	require.True(t, ok, "expected *Thing, got %T", entity)
	require.Equal(t, "web", thing.Meta.Name)
	require.Equal(t, "server", thing.Meta.Type)
	require.Equal(t, "big", thing.Meta.Subtype)
}

func TestCreateEntityUnderTheResourceTypeSetsBoth(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "resource", "thing")
	require.NoError(t, err)

	entity, err := r.CreateEntity("resource", "thing", "my_thing")
	require.NoError(t, err)

	thing, ok := entity.(*Thing)
	require.True(t, ok, "expected *Thing, got %T", entity)
	require.Equal(t, types.TypeResource, thing.Meta.Type)
	require.Equal(t, "thing", thing.Meta.Subtype)
}

func TestCreateEntityFailsForATypeRegisteredWithADifferentSubtype(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "server", "big")
	require.NoError(t, err)

	_, err = r.CreateEntity("server", "small", "web")
	require.ErrorContains(t, err, "server.small")
}

func TestRegisterTypeAcceptsSeveralSubtypesOfOneType(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "server", "big")
	require.NoError(t, err)

	err = r.RegisterType(&Gadget{}, "server", "small")
	require.NoError(t, err)

	require.True(t, r.IsRegisteredType("server", "big"))
	require.True(t, r.IsRegisteredType("server", "small"))
}

func TestRegisterTypeRejectsTheSameSubtypeTwice(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "server", "big")
	require.NoError(t, err)

	err = r.RegisterType(&Gadget{}, "server", "big")

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "server.big", clash.Name)
	require.Equal(t, "registered type", clash.Existing)
}

// A type keyword takes a subtype for every registration or for none, so an
// address such as server.big.web can always be read by position.

func TestRegisterTypeRejectsASubtypeForATypeRegisteredWithoutOne(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "server")
	require.NoError(t, err)

	err = r.RegisterType(&Gadget{}, "server", "big")

	var form *TypeFormError
	require.True(t, errors.As(err, &form))
	require.Equal(t, "server", form.Type)
	require.False(t, form.TakesSubtype)
	require.False(t, r.IsRegisteredType("server", "big"))
}

func TestRegisterTypeRejectsNoSubtypeForATypeRegisteredWithOne(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "server", "big")
	require.NoError(t, err)

	err = r.RegisterType(&Gadget{}, "server")

	var form *TypeFormError
	require.True(t, errors.As(err, &form))
	require.Equal(t, "server", form.Type)
	require.True(t, form.TakesSubtype)
	require.False(t, r.IsRegisteredType("server", ""))
}

func TestRegisterTypeRejectsTheResourceTypeWithoutASubtype(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "resource")

	var form *TypeFormError
	require.True(t, errors.As(err, &form))
	require.Equal(t, "resource", form.Type)
	require.True(t, form.TakesSubtype)
}

func TestRegisterTypeRejectsABuiltinTypeWithASubtype(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "variable", "big")

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "variable.big", clash.Name)
	require.Equal(t, "builtin", clash.Existing)
}

func TestRegisterTypeRejectsMoreThanOneSubtype(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "server", "big", "small")
	require.ErrorContains(t, err, "at most one subtype")
	require.False(t, r.KnownType("server", "big"))
}

func TestRegisterTypeRejectsAnEmptySubtype(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "server", "")
	require.ErrorContains(t, err, "empty subtype")
	require.False(t, r.KnownType("server", ""))
}

func TestRegisterTypeRejectsNoName(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{})
	require.ErrorContains(t, err, "must be named")
}

func TestRegisterTypeRejectsAnEmptyType(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "")
	require.ErrorContains(t, err, "must be named")
}

func TestTakesSubtypeReportsTheFormOfARegisteredType(t *testing.T) {
	r := NewPluginRegistry()

	require.NoError(t, r.RegisterType(&Thing{}, "server", "big"))
	require.NoError(t, r.RegisterType(&Gadget{}, "cache"))

	takes, known := r.TakesSubtype("server")
	require.True(t, known)
	require.True(t, takes)

	takes, known = r.TakesSubtype("cache")
	require.True(t, known)
	require.False(t, takes)
}

func TestTakesSubtypeAlwaysKnowsTheResourceType(t *testing.T) {
	r := NewPluginRegistry()

	takes, known := r.TakesSubtype("resource")
	require.True(t, known)
	require.True(t, takes)
}

func TestTakesSubtypeIgnoresAnUnknownType(t *testing.T) {
	r := NewPluginRegistry()

	_, known := r.TakesSubtype("server")
	require.False(t, known)
}

func TestTypeFormErrorMessageForATypeThatTakesASubtype(t *testing.T) {
	err := &TypeFormError{Type: "server", TakesSubtype: true}

	require.Equal(t, `type "server" takes a subtype, i.e. 'server "<subtype>" "<name>" {}', so it must be registered with one`, err.Error())
}

func TestTypeFormErrorMessageForATypeWithoutASubtype(t *testing.T) {
	err := &TypeFormError{Type: "server"}

	require.Equal(t, `type "server" is declared without a subtype, i.e. 'server "<name>" {}', so it can not be registered with one`, err.Error())
}

// KnownType answers for every source the registry draws on, which is what the
// parser asks before accepting a leading keyword. IsRegisteredType stays the
// narrow question of whether RegisterType alone provided a name, because the
// lifecycle reads it to decide what never reaches a provider.

func TestKnownTypeReportsABuiltin(t *testing.T) {
	r := NewPluginRegistry()

	require.True(t, r.KnownType("variable", ""))
}

func TestKnownTypeReportsARegisteredType(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "resource", "thing")
	require.NoError(t, err)

	require.True(t, r.KnownType("resource", "thing"))
}

func TestKnownTypeReportsARegisteredTypeWithoutASubtype(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "thing")
	require.NoError(t, err)

	require.True(t, r.KnownType("thing", ""))
}

func TestKnownTypeReportsAPluginProvidedType(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterPlugin(&thingPlugin{})
	require.NoError(t, err)

	err = r.Load(nil)
	require.NoError(t, err)

	require.True(t, r.KnownType("resource", "thing"))
}

func TestKnownTypeIgnoresAPluginTypeBeforeLoad(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterPlugin(&thingPlugin{})
	require.NoError(t, err)

	require.False(t, r.KnownType("resource", "thing"))
}

func TestKnownTypeIgnoresAnUnknownName(t *testing.T) {
	r := NewPluginRegistry()

	require.False(t, r.KnownType("nosuchtype", ""))
}

func TestTypeNameClashErrorMessage(t *testing.T) {
	err := &TypeNameClashError{Name: "thing", Existing: "plugin"}

	require.Equal(t, `type "thing" is already provided by plugin`, err.Error())
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

func (p *chattyProvider) Update(ctx context.Context, resource *Thing) (*Thing, error) {
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
func createThing(t *testing.T, r *PluginRegistry) {
	t.Helper()

	data, err := json.Marshal(&Thing{Size: 1})
	require.NoError(t, err)

	thing := &Thing{}
	thing.Meta.Type = types.TypeResource
	thing.Meta.Subtype = "thing"

	provider := r.GetProvider(thing)
	require.NotNil(t, provider, "no loaded plugin provides the type thing")

	_, err = provider.Create(context.Background(), data)
	require.NoError(t, err)
}

func TestNewPluginRegistryNeedsNoLogger(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterPlugin(&thingPlugin{})
	require.NoError(t, err)

	err = r.RegisterPluginWithPath(filepath.Join(t.TempDir(), "xcl-plugin-example"))
	require.NoError(t, err)

	r.DiscoverPlugins([]string{t.TempDir()}, "xcl-plugin-*")
}

func TestRegisterPluginWithMissingPathReturnsNoError(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterPluginWithPath(filepath.Join(t.TempDir(), "does-not-exist"))
	require.NoError(t, err)
}

func TestLoadWithMissingPluginPathFailsNamingIt(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	r := NewPluginRegistry()

	err := r.RegisterPluginWithPath(missing)
	require.NoError(t, err)

	err = r.Load(nil)
	require.Error(t, err)
	require.ErrorIs(t, err, xclerrors.ErrPluginLoad)

	var loadErr *xclerrors.PluginLoadError
	require.True(t, errors.As(err, &loadErr))
	require.Equal(t, missing, loadErr.Plugin)
}

func TestLoadWithFailingInProcessPluginFailsNamingIt(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterPlugin(&failingPlugin{})
	require.NoError(t, err)

	err = r.Load(nil)
	require.Error(t, err)
	require.ErrorIs(t, err, xclerrors.ErrPluginLoad)
	require.ErrorContains(t, err, "boom")

	var loadErr *xclerrors.PluginLoadError
	require.True(t, errors.As(err, &loadErr))
	require.Equal(t, "failingPlugin", loadErr.Plugin)
}

func TestLoadEmitsLoadStartAndErrorForFailingInProcessPlugin(t *testing.T) {
	recorder := &testutil.EventRecorder{}
	r := NewPluginRegistry()

	err := r.RegisterPlugin(&failingPlugin{})
	require.NoError(t, err)

	loadErr := r.Load(recorder.Record)
	require.Error(t, loadErr)

	loads := lifecycleEvents(recorder.Events(), events.OperationLoad)
	require.Len(t, loads, 2)

	require.Equal(t, events.SourceCore, loads[0].Source)
	require.Equal(t, events.PhaseStart, loads[0].Phase)
	require.Equal(t, map[string]any{"plugin": "failingPlugin"}, loads[0].Meta)

	require.Equal(t, events.SourceCore, loads[1].Source)
	require.Equal(t, events.PhaseError, loads[1].Phase)
	require.Equal(t, loadErr, loads[1].Error)
	require.Equal(t, map[string]any{"plugin": "failingPlugin"}, loads[1].Meta)
}

func TestNoPluginHostsBeforeLoad(t *testing.T) {
	setup := newTestPluginSetup(t)
	examplePlugin := setup.buildExamplePlugin("test-plugin")

	r := NewPluginRegistry()

	err := r.RegisterPluginWithPath(examplePlugin)
	require.NoError(t, err)

	require.Empty(t, r.GetPluginHosts())
	require.False(t, r.Loaded())
}

func TestLoadedReportsTrueAfterLoad(t *testing.T) {
	r := NewPluginRegistry()

	err := r.Load(nil)
	require.NoError(t, err)

	require.True(t, r.Loaded())
}

func TestLoadedReportsTrueAfterAFailedLoad(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterPlugin(&failingPlugin{})
	require.NoError(t, err)

	err = r.Load(nil)
	require.Error(t, err)

	require.True(t, r.Loaded())
}

func TestNoPluginProcessRunsBeforeLoad(t *testing.T) {
	requireLinuxProc(t)

	setup := newTestPluginSetup(t)
	examplePlugin := setup.buildExamplePlugin("test-plugin")

	r := NewPluginRegistry()
	t.Cleanup(func() { stopHosts(r) })

	err := r.RegisterPluginWithPath(examplePlugin)
	require.NoError(t, err)

	require.Equal(t, 0, processesRunning(t, examplePlugin), "registering a plugin path must not start it")

	err = r.Load(nil)
	require.NoError(t, err)

	require.Equal(t, 1, processesRunning(t, examplePlugin), "Load starts the plugin")
}

func TestLoadRunsOnce(t *testing.T) {
	setup := newTestPluginSetup(t)
	examplePlugin := setup.buildExamplePlugin("test-plugin")

	r := NewPluginRegistry()
	t.Cleanup(func() { stopHosts(r) })

	err := r.RegisterPluginWithPath(examplePlugin)
	require.NoError(t, err)

	err = r.Load(nil)
	require.NoError(t, err)

	second := &testutil.EventRecorder{}
	err = r.Load(second.Record)
	require.NoError(t, err)

	require.Len(t, r.GetPluginHosts(), 1)
	require.Empty(t, second.Events())
}

func TestLoadCachesFailure(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterPluginWithPath(filepath.Join(t.TempDir(), "does-not-exist"))
	require.NoError(t, err)

	firstErr := r.Load(nil)
	require.Error(t, firstErr)

	second := &testutil.EventRecorder{}
	secondErr := r.Load(second.Record)

	require.Equal(t, firstErr, secondErr)
	require.Empty(t, second.Events())
}

func TestRegisterTypeClashingWithBuiltinFailsImmediately(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "output")
	require.Error(t, err)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "output", clash.Name)
	require.Equal(t, "builtin", clash.Existing)

	require.False(t, r.Loaded())
}

func TestRegisterTypeClashingWithRegisteredTypeFailsImmediately(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "resource", "thing")
	require.NoError(t, err)

	err = r.RegisterType(&Gadget{}, "resource", "thing")
	require.Error(t, err)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "resource.thing", clash.Name)
	require.Equal(t, "registered type", clash.Existing)

	require.False(t, r.Loaded())
}

// thing and resource.thing are different types, one declared thing "x" {} and
// the other resource "thing" "x" {}, so registering both is not a clash
func TestRegisterTypeAcceptsATypeNamedLikeAResourceSubtype(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType(&Thing{}, "thing")
	require.NoError(t, err)

	err = r.RegisterType(&Gadget{}, "resource", "thing")
	require.NoError(t, err)

	require.True(t, r.IsRegisteredType("thing", ""))
	require.True(t, r.IsRegisteredType("resource", "thing"))
}

func TestRegisterTypeMatchingPluginTypeSucceedsBeforeLoad(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterPlugin(&thingPlugin{})
	require.NoError(t, err)

	err = r.RegisterType(&Thing{}, "resource", "thing")
	require.NoError(t, err)

	require.True(t, r.IsRegisteredType("resource", "thing"))
}

func TestLoadFailsWithClashForRegisteredTypeMatchingPluginType(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterPlugin(&thingPlugin{})
	require.NoError(t, err)

	err = r.RegisterType(&Thing{}, "resource", "thing")
	require.NoError(t, err)

	err = r.Load(nil)
	require.Error(t, err)

	var clash *TypeNameClashError
	require.True(t, errors.As(err, &clash))
	require.Equal(t, "resource.thing", clash.Name)
	require.Equal(t, "registered type", clash.Existing)

	require.Empty(t, r.GetPluginHosts())
}

func TestLoadEmitsLoadStartAndSuccessForInProcessPlugin(t *testing.T) {
	recorder := &testutil.EventRecorder{}
	r := NewPluginRegistry()

	err := r.RegisterPlugin(&thingPlugin{})
	require.NoError(t, err)

	err = r.Load(recorder.Record)
	require.NoError(t, err)

	loads := lifecycleEvents(recorder.Events(), events.OperationLoad)
	require.Len(t, loads, 2)

	require.Equal(t, events.SourceCore, loads[0].Source)
	require.Equal(t, events.PhaseStart, loads[0].Phase)
	require.Equal(t, map[string]any{"plugin": "thingPlugin"}, loads[0].Meta)

	require.Equal(t, events.SourceCore, loads[1].Source)
	require.Equal(t, events.PhaseSuccess, loads[1].Phase)
	require.Equal(t, map[string]any{"plugin": "thingPlugin", "block_types": "thing"}, loads[1].Meta)
}

func TestLoadEmitsLoadStartAndSuccessForExternalPlugin(t *testing.T) {
	setup := newTestPluginSetup(t)
	examplePlugin := setup.buildExamplePlugin("test-plugin")

	recorder := &testutil.EventRecorder{}
	r := NewPluginRegistry()
	t.Cleanup(func() { stopHosts(r) })

	err := r.RegisterPluginWithPath(examplePlugin)
	require.NoError(t, err)

	err = r.Load(recorder.Record)
	require.NoError(t, err)

	loads := lifecycleEvents(recorder.Events(), events.OperationLoad)
	require.Len(t, loads, 2)

	require.Equal(t, events.SourceCore, loads[0].Source)
	require.Equal(t, events.PhaseStart, loads[0].Phase)
	require.Equal(t, map[string]any{"plugin": "test-plugin"}, loads[0].Meta)

	require.Equal(t, events.SourceCore, loads[1].Source)
	require.Equal(t, events.PhaseSuccess, loads[1].Phase)
	require.Equal(t, map[string]any{"plugin": "test-plugin", "block_types": "person"}, loads[1].Meta)
}

func TestLoadWithoutDiscoveryDirectoriesEmitsNoDiscoverEvents(t *testing.T) {
	recorder := &testutil.EventRecorder{}
	r := NewPluginRegistry()

	err := r.RegisterPlugin(&thingPlugin{})
	require.NoError(t, err)

	err = r.Load(recorder.Record)
	require.NoError(t, err)

	require.Empty(t, lifecycleEvents(recorder.Events(), events.OperationDiscover))
}

func TestLoadRoutesPluginInitLogsToLoadsEmitter(t *testing.T) {
	recorder := &testutil.EventRecorder{}
	r := NewPluginRegistry()

	err := r.RegisterPlugin(&chattyPlugin{})
	require.NoError(t, err)

	err = r.Load(recorder.Record)
	require.NoError(t, err)

	initialised := logsWithMessage(recorder.Events(), "chatty provider initialised")
	require.Len(t, initialised, 1)
	require.Equal(t, "chattyPlugin", initialised[0].Source)
}

func TestConcurrentLoadIsSafe(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterPlugin(&thingPlugin{})
	require.NoError(t, err)

	var wg sync.WaitGroup
	loadErrors := make([]error, 8)

	for i := range loadErrors {
		wg.Add(2)

		go func() {
			defer wg.Done()
			loadErrors[i] = r.Load(nil)
		}()

		go func() {
			defer wg.Done()
			_ = r.Types()
			_ = r.KnownType("resource", "thing")
			_ = r.GetPluginHosts()
		}()
	}

	wg.Wait()

	for _, loadErr := range loadErrors {
		require.NoError(t, loadErr)
	}

	require.Len(t, r.GetPluginHosts(), 1)
	require.True(t, r.KnownType("resource", "thing"))
}

func TestActivateRoutesPluginLogsToTheActiveEmitter(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterPlugin(&chattyPlugin{})
	require.NoError(t, err)

	err = r.Load(nil)
	require.NoError(t, err)

	recorder := &testutil.EventRecorder{}
	deactivate := r.Activate(recorder.Record)
	t.Cleanup(deactivate)

	createThing(t, r)

	created := logsWithMessage(recorder.Events(), "creating a thing")
	require.Len(t, created, 1)
	require.Equal(t, "chattyPlugin", created[0].Source)
}

func TestDeactivateStopsRouting(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterPlugin(&chattyPlugin{})
	require.NoError(t, err)

	err = r.Load(nil)
	require.NoError(t, err)

	recorder := &testutil.EventRecorder{}
	deactivate := r.Activate(recorder.Record)
	deactivate()

	createThing(t, r)

	require.Empty(t, recorder.Events())
}

func TestDeactivateOfAnOlderActivationKeepsTheNewer(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterPlugin(&chattyPlugin{})
	require.NoError(t, err)

	err = r.Load(nil)
	require.NoError(t, err)

	older := &testutil.EventRecorder{}
	deactivateOlder := r.Activate(older.Record)

	newer := &testutil.EventRecorder{}
	deactivateNewer := r.Activate(newer.Record)
	t.Cleanup(deactivateNewer)

	deactivateOlder()

	createThing(t, r)

	require.Empty(t, older.Events())
	require.Len(t, logsWithMessage(newer.Events(), "creating a thing"), 1)
}

// A plugin provides types the same way RegisterType does, by a type and an
// optional subtype, and its types need not be declared under resource.

func TestLoadedPluginTypeWithASubtypeIsKnown(t *testing.T) {
	r := NewPluginRegistry()

	require.NoError(t, r.RegisterPlugin(&bigServerPlugin{}))
	require.NoError(t, r.Load(nil))

	require.True(t, r.KnownType("server", "big"))

	takes, known := r.TakesSubtype("server")
	require.True(t, known)
	require.True(t, takes)
}

func TestCreateEntityCreatesALoadedPluginTypeWithASubtype(t *testing.T) {
	r := NewPluginRegistry()

	require.NoError(t, r.RegisterPlugin(&bigServerPlugin{}))
	require.NoError(t, r.Load(nil))

	entity, err := r.CreateEntity("server", "big", "web")
	require.NoError(t, err)

	meta, err := types.GetMeta(entity)
	require.NoError(t, err)
	require.Equal(t, "server", meta.Type)
	require.Equal(t, "big", meta.Subtype)
	require.Equal(t, "web", meta.Name)
}

func TestGetProviderFindsTheProviderOfALoadedPluginTypeWithASubtype(t *testing.T) {
	r := NewPluginRegistry()

	require.NoError(t, r.RegisterPlugin(&bigServerPlugin{}))
	require.NoError(t, r.Load(nil))

	entity, err := r.CreateEntity("server", "big", "web")
	require.NoError(t, err)

	require.NotNil(t, r.GetProvider(entity))
	require.NotNil(t, r.GetProviderForResource(entity))
}

func TestRegisterTypeRejectsTheOtherFormOfALoadedPluginType(t *testing.T) {
	r := NewPluginRegistry()

	require.NoError(t, r.RegisterPlugin(&bigServerPlugin{}))
	require.NoError(t, r.Load(nil))

	err := r.RegisterType(&Gadget{}, "server")

	var form *TypeFormError
	require.True(t, errors.As(err, &form))
	require.True(t, form.TakesSubtype)
}
