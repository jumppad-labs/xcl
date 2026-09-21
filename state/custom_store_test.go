// The tests in this file stand where an implementer stands: outside the
// library, with nothing but its public API. They exercise a StateStore written
// here rather than one the library provides, which is the property the narrowed
// contract exists for.
package state_test

import (
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// memoryStore is a StateStore that keeps the entities it is given in memory.
//
// It is written against the public contract alone: the four methods exchange
// plain entities, so nothing here names a library type or constructs one, and
// the zero value is usable.
type memoryStore struct {
	mu       sync.Mutex
	entities []any
	saved    bool
}

func (m *memoryStore) Load() ([]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.entities, nil
}

func (m *memoryStore) Save(entities []any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.entities = entities
	m.saved = true

	return nil
}

func (m *memoryStore) Exists() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.saved
}

func (m *memoryStore) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.entities = nil
	m.saved = false

	return nil
}

// A store written outside the library persists an apply: the entities the
// configuration declared are handed to it and come back from it unchanged.
func TestCustomStateStoreRoundTripsAnApply(t *testing.T) {
	reg := testRegistry(t)
	store := &memoryStore{}

	c := xcl.NewConfig(
		xcl.WithPluginRegistry(reg),
		xcl.WithStateStore(store),
	)

	err := c.Apply(appliedConfig)
	require.NoError(t, err)

	require.True(t, store.Exists())

	loaded, err := store.Load()
	require.NoError(t, err)

	require.ElementsMatch(t, testAppliedIDs, testLoadedIDs(t, loaded))
	require.Len(t, loaded, len(testAppliedIDs))
}

// The entities a store written outside the library is handed carry both of the
// axes state records, so an implementer that only stores them loses neither.
func TestCustomStateStoreIsHandedEntitiesWithBothAxes(t *testing.T) {
	reg := testRegistry(t)
	store := &memoryStore{}

	c := xcl.NewConfig(
		xcl.WithPluginRegistry(reg),
		xcl.WithStateStore(store),
	)

	err := c.Apply(appliedConfig)
	require.NoError(t, err)

	loaded, err := store.Load()
	require.NoError(t, err)

	database, err := entityByID(loaded, "resource.database.main")
	require.NoError(t, err)

	databaseMeta, err := types.GetMeta(database)
	require.NoError(t, err)
	require.Equal(t, types.TypeResource, databaseMeta.Type)
	require.Equal(t, registered.TypeDatabase, databaseMeta.Subtype)
	require.Equal(t, "main", databaseMeta.Name)

	variable, err := entityByID(loaded, "variable.environment")
	require.NoError(t, err)

	variableMeta, err := types.GetMeta(variable)
	require.NoError(t, err)
	require.Equal(t, "variable", variableMeta.Type)
	require.Empty(t, variableMeta.Subtype)
	require.Equal(t, "environment", variableMeta.Name)
}

// A second run reads what the first run left in the store, so a configuration
// built on a store written outside the library sees the earlier entities.
func TestCustomStateStoreIsReadBackByALaterRun(t *testing.T) {
	reg := testRegistry(t)
	store := &memoryStore{}

	first := xcl.NewConfig(
		xcl.WithPluginRegistry(reg),
		xcl.WithStateStore(store),
	)

	err := first.Apply(appliedConfig)
	require.NoError(t, err)

	second := xcl.NewConfig(
		xcl.WithPluginRegistry(reg),
		xcl.WithStateStore(store),
	)

	err = second.Apply(appliedConfig)
	require.NoError(t, err)

	require.Equal(t, len(testAppliedIDs), second.EntityCount())

	loaded, err := store.Load()
	require.NoError(t, err)
	require.ElementsMatch(t, testAppliedIDs, testLoadedIDs(t, loaded))
}
