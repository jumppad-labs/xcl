// The tests in this file drive a real apply to produce the saved state they
// read back, as the project requires of any test that needs earlier state.
// An apply lives in the parser, which imports state, so these tests sit in the
// external state_test package where they can use the public xcl API without an
// import cycle. The tests that are about the on-disk format itself, which the
// convention exempts, hand write their fixtures in file_state_store_test.go.
package state_test

import (
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// appliedConfig declares a variable, a resource carrying a variety, a module
// and, inside that module, a further resource and an output. Between them they
// cover both axes that state records: a resource kind entity with a variety,
// and builtin entities that have none.
const appliedConfig = "../internal/test_fixtures/config/registered/basic/main.xcl"

// testAppliedIDs is the id of every entity appliedConfig declares, which is
// what a store is handed and has to give back.
var testAppliedIDs = []string{
	"variable.environment",
	"resource.database.main",
	"module.shared",
	"module.shared.resource.database.shared",
	"module.shared.output.location",
	"resource.app.web",
	"resource.consumer.reader",
}

// testRegistry returns a registry holding the types appliedConfig declares,
// and points HOME at the test's temp directory so an apply never writes to the
// user's home folder.
func testRegistry(t *testing.T) *registry.PluginRegistry {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	reg := registry.NewPluginRegistry()

	err := reg.RegisterType(registered.TypeDatabase, &registered.Database{})
	require.NoError(t, err)

	err = reg.RegisterType(registered.TypeApp, &registered.App{})
	require.NoError(t, err)

	err = reg.RegisterType(registered.TypeConsumer, &registered.Consumer{})
	require.NoError(t, err)

	return reg
}

// testApplyToStateFile registers the fixture types, applies appliedConfig with
// a file state store and returns the path the state was written to along with
// the registry it was written with, so a fresh store can read it back.
func testApplyToStateFile(t *testing.T) (string, *registry.PluginRegistry) {
	t.Helper()

	reg := testRegistry(t)

	statePath := filepath.Join(t.TempDir(), "state.json")

	store, err := state.NewFileStateStore(statePath, reg)
	require.NoError(t, err)

	c := xcl.NewConfig(
		xcl.WithPluginRegistry(reg),
		xcl.WithStateStore(store),
	)

	err = c.Apply(appliedConfig)
	require.NoError(t, err)

	return statePath, reg
}

// testLoadSavedState opens a fresh store at path, as a later run would, and
// loads the saved entities from it
func testLoadSavedState(t *testing.T, path string, reg *registry.PluginRegistry) []any {
	t.Helper()

	store, err := state.NewFileStateStore(path, reg)
	require.NoError(t, err)

	s, err := store.Load()
	require.NoError(t, err)
	require.NotNil(t, s)

	return s
}

// entityByID returns the entity the loaded state holds under the given id.
// Storage answers no questions about addresses, so the entities are scanned
// and the id each entity already records is compared, an entity's id being its
// rendered address
func entityByID(entities []any, id string) (any, error) {
	for _, e := range entities {
		meta, err := types.GetMeta(e)
		if err != nil {
			continue
		}

		if meta.ID == id {
			return e, nil
		}
	}

	return nil, state.ResourceNotFoundError{Resource: id}
}

// testLoadedIDs returns the ID of every entity in entities
func testLoadedIDs(t *testing.T, entities []any) []string {
	t.Helper()

	ids := []string{}
	for _, r := range entities {
		meta, err := types.GetMeta(r)
		require.NoError(t, err)

		ids = append(ids, meta.ID)
	}

	return ids
}

// State written by the current version loads again, returning every entity the
// apply put in it rather than a shorter list.
func TestLoadReturnsEveryResourceWrittenByApply(t *testing.T) {
	statePath, reg := testApplyToStateFile(t)

	s := testLoadSavedState(t, statePath, reg)

	ids := testLoadedIDs(t, s)

	require.ElementsMatch(t, testAppliedIDs, ids)

	require.Len(t, s, len(testAppliedIDs))
}

// A resource kind entity round trips with both axes intact: the kind that
// declared it and the variety it carries.
func TestLoadRestoresBothAxesOfAResource(t *testing.T) {
	statePath, reg := testApplyToStateFile(t)

	s := testLoadSavedState(t, statePath, reg)

	r, err := entityByID(s, "resource.database.main")
	require.NoError(t, err)

	meta, err := types.GetMeta(r)
	require.NoError(t, err)

	require.Equal(t, "resource", meta.Type)
	require.Equal(t, "database", meta.Subtype)
	require.Equal(t, "main", meta.Name)
	require.Equal(t, "resource.database.main", meta.ID)
}

// A builtin round trips carrying the keyword that declared it and no variety,
// which is what the omitted subtype in the saved file has to read back as.
func TestLoadRestoresBuiltinsWithoutASubtype(t *testing.T) {
	statePath, reg := testApplyToStateFile(t)

	s := testLoadSavedState(t, statePath, reg)

	variable, err := entityByID(s, "variable.environment")
	require.NoError(t, err)

	variableMeta, err := types.GetMeta(variable)
	require.NoError(t, err)

	require.Equal(t, "variable", variableMeta.Type)
	require.Empty(t, variableMeta.Subtype)

	output, err := entityByID(s, "module.shared.output.location")
	require.NoError(t, err)

	outputMeta, err := types.GetMeta(output)
	require.NoError(t, err)

	require.Equal(t, "output", outputMeta.Type)
	require.Empty(t, outputMeta.Subtype)

	module, err := entityByID(s, "module.shared")
	require.NoError(t, err)

	moduleMeta, err := types.GetMeta(module)
	require.NoError(t, err)

	require.Equal(t, "module", moduleMeta.Type)
	require.Empty(t, moduleMeta.Subtype)
}

// The values a resource was applied with come back as they were written, not
// only its identity.
func TestLoadRestoresResourceValues(t *testing.T) {
	statePath, reg := testApplyToStateFile(t)

	s := testLoadSavedState(t, statePath, reg)

	r, err := entityByID(s, "resource.database.main")
	require.NoError(t, err)

	database, ok := r.(*registered.Database)
	require.True(t, ok)

	require.Equal(t, "us-east", database.Location)
	require.Equal(t, 5432, database.Port)
	require.NotNil(t, database.Timeouts)
	require.Equal(t, 30, database.Timeouts.Connect)
	require.Equal(t, 60, database.Timeouts.Read)
}
