package parser

import (
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl/internal/parser/mocks"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

const (
	registeredBasicConfig         = "../test_fixtures/config/registered/basic/main.xcl"
	registeredDisabledConfig      = "../test_fixtures/config/registered/disabled/main.xcl"
	registeredRemovedBeforeConfig = "../test_fixtures/config/registered/removed/before/main.xcl"
	registeredRemovedAfterConfig  = "../test_fixtures/config/registered/removed/after/main.xcl"
	registeredBareConfig          = "../test_fixtures/config/registered/bare/main.xcl"

	registeredVariableID       = "variable.environment"
	registeredDatabaseID       = "resource.database.main"
	registeredAppID            = "resource.app.web"
	registeredConsumerID       = "resource.consumer.reader"
	registeredModuleDatabaseID = "module.shared.resource.database.shared"
	registeredDisabledID       = "resource.database.off"
	registeredKeptID           = "resource.database.kept"
	registeredRemovedID        = "resource.database.removed"
	registeredCacheID          = "cache.main"
)

// registeredHarness holds what every apply in a registered type scenario
// shares: one plugin registry with the registered fixture types and NO
// plugins, and one file state store. Each apply builds a fresh Parser from
// these, just as separate runs would.
type registeredHarness struct {
	registry *registry.PluginRegistry
	store    *state.FileStateStore
}

// setupRegisteredTypes redirects HOME to a temp dir, registers the database,
// app and consumer types into a new registry without registering any plugin,
// and creates a file state store in a temp dir.
func setupRegisteredTypes(t *testing.T) *registeredHarness {
	t.Helper()

	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())
	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	reg := registry.NewPluginRegistry(logger.NewTestLogger(t))

	err := reg.RegisterType(registered.TypeDatabase, &registered.Database{})
	require.NoError(t, err)

	err = reg.RegisterType(registered.TypeApp, &registered.App{})
	require.NoError(t, err)

	err = reg.RegisterType(registered.TypeConsumer, &registered.Consumer{})
	require.NoError(t, err)

	// cache is the one type registered in the bare form, it is declared by its
	// own keyword with a single label rather than under the resource kind
	err = reg.RegisterBareType(registered.TypeCache, &registered.Cache{})
	require.NoError(t, err)

	store, err := state.NewFileStateStore(filepath.Join(t.TempDir(), "state.json"), reg)
	require.NoError(t, err)

	return &registeredHarness{
		registry: reg,
		store:    store,
	}
}

// newParser builds a fresh Parser that shares the harness registry and state
// store. log and onEvent may be nil.
func (h *registeredHarness) newParser(t *testing.T, log logger.Logger, onEvent func(ParserEvent)) *Parser {
	t.Helper()

	options := testOptions(t)
	if log != nil {
		options.Logger = log
	}

	options.PluginRegistry = h.registry
	options.StateStore = h.store
	options.OnParserEvent = onEvent

	return NewParser(options)
}

// applyAndSave applies the config at path with a fresh Parser, requires it to
// succeed, saves the returned state as Config.Apply does and returns it.
func (h *registeredHarness) applyAndSave(t *testing.T, path string) *State {
	t.Helper()

	p := h.newParser(t, nil, nil)

	st, err := p.Apply(path)
	require.NoError(t, err)
	require.NotNil(t, st)

	err = h.store.Save(st.GetResources())
	require.NoError(t, err)

	return st
}

// requireMeta returns the Meta of the entity with the given ID in entities
func requireMeta(t *testing.T, entities []any, id string) *types.Meta {
	t.Helper()

	r, err := findByID(entities, id)
	require.NoError(t, err)

	meta, err := types.GetMeta(r)
	require.NoError(t, err)

	return meta
}

// requireDatabase returns the entity with the given ID in entities, requiring
// it to be exactly a *registered.Database
func requireDatabase(t *testing.T, entities []any, id string) *registered.Database {
	t.Helper()

	r, err := findByID(entities, id)
	require.NoError(t, err)

	db, ok := r.(*registered.Database)
	require.True(t, ok, "expected *registered.Database, got %T", r)

	return db
}

func TestApplyRegisteredTypesSucceedsWithoutProvider(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)
	require.NotNil(t, st)

	variableStatus := requireMeta(t, st.GetResources(), registeredVariableID).Status

	require.Equal(t, variableStatus, requireMeta(t, st.GetResources(), registeredDatabaseID).Status)
	require.Equal(t, variableStatus, requireMeta(t, st.GetResources(), registeredAppID).Status)
	require.Equal(t, variableStatus, requireMeta(t, st.GetResources(), registeredConsumerID).Status)
	require.Equal(t, variableStatus, requireMeta(t, st.GetResources(), registeredModuleDatabaseID).Status)
}

func TestApplyRegisteredTypeDecodesValuesAndNestedBlock(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)

	db := requireDatabase(t, st.GetResources(), registeredDatabaseID)

	require.Equal(t, "us-east", db.Location)
	require.Equal(t, 5432, db.Port)
	require.NotNil(t, db.Timeouts)
	require.Equal(t, 30, db.Timeouts.Connect)
	require.Equal(t, 60, db.Timeouts.Read)
}

func TestApplyRegisteredTypeReturnsRegisteredGoType(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)

	r, err := findByID(st.GetResources(), registeredDatabaseID)
	require.NoError(t, err)
	require.Equal(t, reflect.TypeOf(&registered.Database{}), reflect.TypeOf(r))

	_, ok := r.(*registered.Database)
	require.True(t, ok)

	r, err = findByID(st.GetResources(), registeredAppID)
	require.NoError(t, err)
	require.Equal(t, reflect.TypeOf(&registered.App{}), reflect.TypeOf(r))

	r, err = findByID(st.GetResources(), registeredConsumerID)
	require.NoError(t, err)
	require.Equal(t, reflect.TypeOf(&registered.Consumer{}), reflect.TypeOf(r))
}

func TestApplyRegisteredTypeResolvesReferences(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)

	r, err := findByID(st.GetResources(), registeredAppID)
	require.NoError(t, err)
	app, ok := r.(*registered.App)
	require.True(t, ok)

	require.Equal(t, "production", app.Environment)
	require.Equal(t, "us-east", app.DatabaseLocation)
	require.Equal(t, 5432, app.DatabasePort)

	r, err = findByID(st.GetResources(), registeredConsumerID)
	require.NoError(t, err)
	consumer, ok := r.(*registered.Consumer)
	require.True(t, ok)

	require.Equal(t, "production", consumer.AppEnvironment)
}

func TestApplyRegisteredTypeReadsModuleOutput(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)

	r, err := findByID(st.GetResources(), registeredAppID)
	require.NoError(t, err)
	app, ok := r.(*registered.App)
	require.True(t, ok)

	require.Equal(t, "eu-west", app.SharedLocation)
}

func TestApplyProcessesDependentAfterRegisteredType(t *testing.T) {
	h := setupRegisteredTypes(t)

	// events fire from the walker's parallel goroutines
	var mu sync.Mutex
	created := []string{}
	onEvent := func(e ParserEvent) {
		if e.Operation != "create" || e.Phase != "success" {
			return
		}

		mu.Lock()
		defer mu.Unlock()
		created = append(created, e.ResourceID)
	}

	p := h.newParser(t, nil, onEvent)

	_, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()

	requireBefore(t, registeredDatabaseID, registeredAppID, created)
	requireBefore(t, registeredAppID, registeredConsumerID, created)
}

func TestReapplyRegisteredTypesSucceedsWithoutProvider(t *testing.T) {
	h := setupRegisteredTypes(t)

	h.applyAndSave(t, registeredBasicConfig)

	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)
	require.NotNil(t, st)

	db := requireDatabase(t, st.GetResources(), registeredDatabaseID)
	require.Equal(t, "us-east", db.Location)
	require.Equal(t, 5432, db.Port)
}

func TestApplyAfterRemovingRegisteredBlockSucceedsWithoutProvider(t *testing.T) {
	h := setupRegisteredTypes(t)

	first := h.applyAndSave(t, registeredRemovedBeforeConfig)
	requireDatabase(t, first.GetResources(), registeredRemovedID)

	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredRemovedAfterConfig)
	require.NoError(t, err)
	require.NotNil(t, st)

	requireDatabase(t, st.GetResources(), registeredKeptID)

	_, err = findByID(st.GetResources(), registeredRemovedID)
	require.Error(t, err)

	err = h.store.Save(st.GetResources())
	require.NoError(t, err)

	saved, err := h.store.Load()
	require.NoError(t, err)
	require.NotContains(t, stateIDs(t, saved), registeredRemovedID)
	require.Contains(t, stateIDs(t, saved), registeredKeptID)
}

func TestApplyAfterRemovingRegisteredBlockNeverCallsProvider(t *testing.T) {
	h := setupRegisteredTypes(t)
	h.applyAndSave(t, registeredRemovedBeforeConfig)

	options := testOptions(t)
	options.PluginRegistry = h.registry
	options.StateStore = h.store
	// no expectations, any provider lookup fails the test
	options.ProviderResolver = mocks.NewMockProviderResolver(t)

	p := NewParser(options)

	st, err := p.Apply(registeredRemovedAfterConfig)
	require.NoError(t, err)
	require.NotNil(t, st)

	_, err = findByID(st.GetResources(), registeredRemovedID)
	require.Error(t, err)
}

func TestApplyRegisteredTypeInModuleIsStoredUnderModulePath(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)

	db := requireDatabase(t, st.GetResources(), registeredModuleDatabaseID)

	require.Equal(t, registeredModuleDatabaseID, db.Meta.ID)
	require.Equal(t, "shared", db.Meta.Module)
	require.Equal(t, "eu-west", db.Location)
	require.Equal(t, 5433, db.Port)
	require.NotNil(t, db.Timeouts)
	require.Equal(t, 10, db.Timeouts.Connect)

	moduleResources, err := findModule(st, resources.NewAddressParser(nil), "module.shared", false)
	require.NoError(t, err)
	require.Contains(t, moduleResources, any(db))
}

func TestApplyMarksDisabledRegisteredTypeDisabled(t *testing.T) {
	h := setupRegisteredTypes(t)

	var mu sync.Mutex
	eventIDs := []string{}
	onEvent := func(e ParserEvent) {
		// a disabled resource is still parsed, only the walk skips it
		if e.Operation == "parse" {
			return
		}

		mu.Lock()
		defer mu.Unlock()
		eventIDs = append(eventIDs, e.ResourceID)
	}

	p := h.newParser(t, nil, onEvent)

	st, err := p.Apply(registeredDisabledConfig)
	require.NoError(t, err)

	r, err := findByID(st.GetResources(), registeredDisabledID)
	require.NoError(t, err)

	disabled, err := types.GetDisabled(r)
	require.NoError(t, err)
	require.True(t, disabled)

	meta, err := types.GetMeta(r)
	require.NoError(t, err)
	require.Empty(t, meta.Status)

	// a disabled resource is skipped by the walk, no lifecycle event fires for it
	mu.Lock()
	defer mu.Unlock()
	require.NotContains(t, eventIDs, registeredDisabledID)
}

func TestApplyRegisteredTypeWithComputedFieldLogsNoWarning(t *testing.T) {
	h := setupRegisteredTypes(t)

	log := &recordingLogger{}
	p := h.newParser(t, log, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)

	require.Empty(t, log.warnings())

	db := requireDatabase(t, st.GetResources(), registeredDatabaseID)
	require.Equal(t, "", db.ConnectionString)
}

func TestRegisteredResourcesAreInStateAndFoundByPath(t *testing.T) {
	h := setupRegisteredTypes(t)

	h.applyAndSave(t, registeredBasicConfig)

	// load the state saved by the apply, this round trips it through json
	loaded, err := h.store.Load()
	require.NoError(t, err)

	db := requireDatabase(t, loaded, registeredDatabaseID)
	require.Equal(t, "us-east", db.Location)
	require.Equal(t, 5432, db.Port)
	require.NotNil(t, db.Timeouts)
	require.Equal(t, 30, db.Timeouts.Connect)
	require.Equal(t, 60, db.Timeouts.Read)

	moduleDB := requireDatabase(t, loaded, registeredModuleDatabaseID)
	require.Equal(t, "eu-west", moduleDB.Location)

	r, err := findByID(loaded, registeredAppID)
	require.NoError(t, err)
	app, ok := r.(*registered.App)
	require.True(t, ok, "expected *registered.App, got %T", r)
	require.Equal(t, "production", app.Environment)
	require.Equal(t, "us-east", app.DatabaseLocation)
	require.Equal(t, 5432, app.DatabasePort)
	require.Equal(t, "eu-west", app.SharedLocation)

	r, err = findByID(loaded, registeredConsumerID)
	require.NoError(t, err)
	consumer, ok := r.(*registered.Consumer)
	require.True(t, ok, "expected *registered.Consumer, got %T", r)
	require.Equal(t, "production", consumer.AppEnvironment)
	require.Equal(t, []string{registeredAppID}, consumer.DependsOn)
}

func TestDestroyWalkSkipsProviderForRegisteredType(t *testing.T) {
	h := setupRegisteredTypes(t)

	// no expectations, any provider lookup fails the test
	resolver := mocks.NewMockProviderResolver(t)

	db := &registered.Database{
		ResourceBase: types.ResourceBase{
			Meta: types.Meta{
				ID:      registeredDatabaseID,
				Name:    "main",
				Type:    types.TypeResource,
				Subtype: registered.TypeDatabase,
			},
		},
		Location: "us-east",
		Port:     5432,
	}

	working := NewState()
	err := working.AppendResource(db)
	require.NoError(t, err)

	d := &destroyer{
		working:  working,
		resolver: resolver,
		types:    h.registry,
		options:  testOptions(t),
	}

	callback := destroyWalkCallback(d)

	diags := callback(db)
	require.False(t, diags.HasErrors())
	require.Empty(t, diags)
	require.Equal(t, 0, working.ResourceCount())
}

// The two axes of Meta. Type is the stanza kind, always one of "resource",
// "variable", "output", "module" or "root". Subtype is the variety a resource
// stanza carries in its second label, and is empty for every other stanza.

func TestApplyReportsResourceKindAndVarietyOnSeparateAxes(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)

	meta := requireMeta(t, st.GetResources(), registeredDatabaseID)

	require.Equal(t, types.TypeResource, meta.Type)
	require.Equal(t, registered.TypeDatabase, meta.Subtype)
	require.Equal(t, registered.TypeDatabase, meta.AddressType())
}

func TestApplyReportsVariableKindWithoutAVariety(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)

	meta := requireMeta(t, st.GetResources(), registeredVariableID)

	require.Equal(t, resources.TypeVariable, meta.Type)
	require.Empty(t, meta.Subtype)
	require.Equal(t, resources.TypeVariable, meta.AddressType())
}

func TestApplyReportsOutputKindWithoutAVariety(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)

	meta := requireMeta(t, st.GetResources(), registeredModuleOutputID)

	require.Equal(t, resources.TypeOutput, meta.Type)
	require.Empty(t, meta.Subtype)
	require.Equal(t, resources.TypeOutput, meta.AddressType())
}

func TestApplyReportsModuleKindWithoutAVariety(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)

	meta := requireMeta(t, st.GetResources(), registeredSharedModuleID)

	require.Equal(t, resources.TypeModule, meta.Type)
	require.Empty(t, meta.Subtype)
	require.Equal(t, resources.TypeModule, meta.AddressType())
}

// The expression namespace is keyed by the segment an entity is addressed by,
// so a reference written against a resource, a variable or a module output has
// to land the value it names on the resource that read it.

func TestApplyGivesResourceTheFieldValuesOfAnotherResource(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)

	r, err := findByID(st.GetResources(), registeredAppID)
	require.NoError(t, err)
	app, ok := r.(*registered.App)
	require.True(t, ok, "expected *registered.App, got %T", r)

	// app.web reads resource.database.main.location and .port
	require.Equal(t, "us-east", app.DatabaseLocation)
	require.Equal(t, 5432, app.DatabasePort)

	r, err = findByID(st.GetResources(), registeredConsumerID)
	require.NoError(t, err)
	consumer, ok := r.(*registered.Consumer)
	require.True(t, ok, "expected *registered.Consumer, got %T", r)

	// consumer.reader reads resource.app.web.environment, itself a value the
	// app only holds because it resolved a reference of its own
	require.Equal(t, "production", consumer.AppEnvironment)
}

func TestApplyGivesResourceTheValueOfAVariable(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)

	r, err := findByID(st.GetResources(), registeredAppID)
	require.NoError(t, err)
	app, ok := r.(*registered.App)
	require.True(t, ok, "expected *registered.App, got %T", r)

	// app.web reads variable.environment, whose default is "production"
	require.Equal(t, "production", app.Environment)
}

func TestApplyGivesResourceTheValueOfAModuleOutput(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)

	// the module output reads a resource declared inside the module
	moduleDB := requireDatabase(t, st.GetResources(), registeredModuleDatabaseID)
	require.Equal(t, "eu-west", moduleDB.Location)

	r, err := findByID(st.GetResources(), registeredModuleOutputID)
	require.NoError(t, err)
	out, ok := r.(*resources.Output)
	require.True(t, ok, "expected *resources.Output, got %T", r)
	require.Equal(t, "eu-west", out.CtyValue.AsString())

	// and app.web, outside the module, reads module.shared.output.location
	r, err = findByID(st.GetResources(), registeredAppID)
	require.NoError(t, err)
	app, ok := r.(*registered.App)
	require.True(t, ok, "expected *registered.App, got %T", r)
	require.Equal(t, "eu-west", app.SharedLocation)
}

// handledWithoutProvider answers on two axes: the builtins are stanza kinds,
// read from Type, while a registered Go type is registered under its variety,
// read from Subtype. Any provider lookup here fails the test.
func TestApplyNeverLooksUpAProviderForRegisteredOrBuiltinBlocks(t *testing.T) {
	h := setupRegisteredTypes(t)

	options := testOptions(t)
	options.PluginRegistry = h.registry
	options.StateStore = h.store
	// no expectations, any provider lookup fails the test
	options.ProviderResolver = mocks.NewMockProviderResolver(t)

	p := NewParser(options)

	st, err := p.Apply(registeredBasicConfig)
	require.NoError(t, err)
	require.Equal(t, 7, st.ResourceCount())
}

// A type registered in the bare form leads its own declaration, cache "main",
// and carries no variety. It is a different type from the kind led
// resource "database" "main", not another spelling of it, and is addressed
// cache.main rather than resource.cache.main.

func TestParseBareDeclarationParsesWithoutError(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBareConfig)
	require.NoError(t, err)
	require.NotNil(t, st)
}

func TestParseBareDeclarationReportsKeywordAsKindWithoutAVariety(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBareConfig)
	require.NoError(t, err)

	meta := requireMeta(t, st.GetResources(), registeredCacheID)

	require.Equal(t, registered.TypeCache, meta.Type)
	require.Equal(t, "", meta.Subtype)
	require.Equal(t, "main", meta.Name)
	require.Equal(t, registeredCacheID, meta.ID)
}

func TestParseGivesEachDeclarationFormItsOwnAddress(t *testing.T) {
	h := setupRegisteredTypes(t)
	p := h.newParser(t, nil, nil)

	st, err := p.Apply(registeredBareConfig)
	require.NoError(t, err)

	// both declarations name "main", only the form they are declared in
	// separates them
	bare, err := findByID(st.GetResources(), registeredCacheID)
	require.NoError(t, err)

	kindLed, err := findByID(st.GetResources(), registeredDatabaseID)
	require.NoError(t, err)

	require.NotSame(t, bare, kindLed)

	_, ok := bare.(*registered.Cache)
	require.True(t, ok, "expected *registered.Cache, got %T", bare)

	_, ok = kindLed.(*registered.Database)
	require.True(t, ok, "expected *registered.Database, got %T", kindLed)

	require.Equal(t, registeredCacheID, requireMeta(t, st.GetResources(), registeredCacheID).ID)
	require.Equal(t, registeredDatabaseID, requireMeta(t, st.GetResources(), registeredDatabaseID).ID)
}
