// The tests in this file stand where an implementer stands: outside the
// library, with nothing but its public API. They exercise a StateStore written
// here rather than one the library provides, which is the property the narrowed
// contract exists for.
package state_test

import (
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/internal/savedentity"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// memoryStore is a StateStore that keeps the entities it is given in memory.
//
// It is written against the public contract alone: the four methods exchange
// plain values, each entity arriving as its raw JSON record, so nothing here
// names a library type or constructs one, and the zero value is usable.
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

	c, err := xcl.NewConfig(
		xcl.WithPluginRegistry(reg),
		xcl.WithStateStore(store),
	)
	require.NoError(t, err)

	err = c.Apply(appliedConfig)
	require.NoError(t, err)

	require.True(t, store.Exists())

	records, err := store.Load()
	require.NoError(t, err)

	// the store is handed raw JSON records, typing them needs the registry
	loaded, err := savedentity.DecodeAll(reg, records, savedentity.ReadOptions{})
	require.NoError(t, err)

	require.ElementsMatch(t, testAppliedIDs, testLoadedIDs(t, loaded))
	require.Len(t, loaded, len(testAppliedIDs))
}

// The entities a store written outside the library is handed carry both of the
// axes state records, so an implementer that only stores them loses neither.
func TestCustomStateStoreIsHandedEntitiesWithBothAxes(t *testing.T) {
	reg := testRegistry(t)
	store := &memoryStore{}

	c, err := xcl.NewConfig(
		xcl.WithPluginRegistry(reg),
		xcl.WithStateStore(store),
	)
	require.NoError(t, err)

	err = c.Apply(appliedConfig)
	require.NoError(t, err)

	records, err := store.Load()
	require.NoError(t, err)

	// the store is handed raw JSON records, typing them needs the registry
	loaded, err := savedentity.DecodeAll(reg, records, savedentity.ReadOptions{})
	require.NoError(t, err)

	database, err := testutil.EntityByID(loaded, "resource.database.main")
	require.NoError(t, err)

	databaseMeta, err := types.GetMeta(database)
	require.NoError(t, err)
	require.Equal(t, types.TypeResource, databaseMeta.Type)
	require.Equal(t, registered.TypeDatabase, databaseMeta.Subtype)
	require.Equal(t, "main", databaseMeta.Name)

	variable, err := testutil.EntityByID(loaded, "variable.environment")
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

	first, err := xcl.NewConfig(
		xcl.WithPluginRegistry(reg),
		xcl.WithStateStore(store),
	)
	require.NoError(t, err)

	err = first.Apply(appliedConfig)
	require.NoError(t, err)

	second, err := xcl.NewConfig(
		xcl.WithPluginRegistry(reg),
		xcl.WithStateStore(store),
	)
	require.NoError(t, err)

	err = second.Apply(appliedConfig)
	require.NoError(t, err)

	require.Equal(t, len(testAppliedIDs), second.EntityCount())

	records, err := store.Load()
	require.NoError(t, err)

	// the store is handed raw JSON records, typing them needs the registry
	loaded, err := savedentity.DecodeAll(reg, records, savedentity.ReadOptions{})
	require.NoError(t, err)
	require.ElementsMatch(t, testAppliedIDs, testLoadedIDs(t, loaded))
}
