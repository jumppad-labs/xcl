// The tests in this file are about how loaded state is typed. A state store
// only reads and writes records, so typing them, and reporting the ones that
// can not be typed, is DecodeAll's job. They are about the record format
// itself, so they hand write their records.
package savedentity_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/savedentity"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// testRecords returns the records of a saved state the way a file store loads
// them, one json.RawMessage per record
func testRecords(t *testing.T, saved string) []any {
	t.Helper()

	var raw []json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(saved), &raw))

	records := []any{}
	for _, record := range raw {
		records = append(records, record)
	}

	return records
}

// testEntityByID returns the entity the given slice holds under the given id
func testEntityByID(t *testing.T, entities []any, id string) any {
	t.Helper()

	for _, e := range entities {
		meta, err := types.GetMeta(e)
		require.NoError(t, err)

		if meta.ID == id {
			return e
		}
	}

	require.FailNow(t, "no entity with id "+id)
	return nil
}

// stateWithUnknownTypes is a saved state holding a known variable alongside
// resources whose types nothing registers: two postgres databases and a redis
// cache.
const stateWithUnknownTypes = `[
  {
    "meta": {"id": "variable.example", "type": "variable", "name": "example"}
  },
  {
    "meta": {"id": "resource.redis.cache", "type": "resource", "subtype": "redis", "name": "cache"}
  },
  {
    "meta": {"id": "resource.postgres.main", "type": "resource", "subtype": "postgres", "name": "main"}
  },
  {
    "meta": {"id": "resource.postgres.replica", "type": "resource", "subtype": "postgres", "name": "replica"}
  }
]`

// stateWithUnknownType is a saved state holding a known variable alongside a
// postgres resource whose type nothing registers.
const stateWithUnknownType = `[
  {
    "meta": {"id": "variable.example", "type": "variable", "name": "example"}
  },
  {
    "meta": {"id": "resource.postgres.main", "type": "resource", "subtype": "postgres", "name": "main"}
  }
]`

// Decoding a state holding a type nobody registered fails naming that type,
// rather than returning a state that silently drops the resource.
func TestDecodeAllFailsWhenStateHoldsUnknownType(t *testing.T) {
	s, err := savedentity.DecodeAll(registry.NewPluginRegistry(), testRecords(t, stateWithUnknownType))
	require.Error(t, err)
	require.Nil(t, s)

	unknown := state.UnknownTypesError{}
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, []string{"resource.postgres"}, unknown.Types)
	require.Contains(t, err.Error(), "resource.postgres")
}

// Every unknown type is named once, in sorted order, however many resources
// of that type the state holds.
func TestDecodeAllNamesEachUnknownTypeOnceInSortedOrder(t *testing.T) {
	s, err := savedentity.DecodeAll(registry.NewPluginRegistry(), testRecords(t, stateWithUnknownTypes))
	require.Error(t, err)
	require.Nil(t, s)

	unknown := state.UnknownTypesError{}
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, []string{"resource.postgres", "resource.redis"}, unknown.Types)
}

// stateWithRecordMissingMeta is a saved state whose second record carries no
// meta object at all, so nothing in it can name the record but its position.
const stateWithRecordMissingMeta = `[
  {
    "meta": {"id": "variable.example", "type": "variable", "name": "example"}
  },
  {
    "location": "us-east"
  }
]`

// stateWithRecordMissingType is a saved state whose second record has a meta
// object that does not say which kind declared it.
const stateWithRecordMissingType = `[
  {
    "meta": {"id": "variable.example", "type": "variable", "name": "example"}
  },
  {
    "meta": {"id": "resource.database.main", "subtype": "database", "name": "main"}
  }
]`

// stateWithRecordEmptyType is a saved state whose second record has a meta
// object holding an empty kind.
const stateWithRecordEmptyType = `[
  {
    "meta": {"id": "variable.example", "type": "variable", "name": "example"}
  },
  {
    "meta": {"id": "resource.database.main", "type": "", "subtype": "database", "name": "main"}
  }
]`

// stateWithRecordMissingName is a saved state whose second record has a meta
// object that does not say what the entity is called.
const stateWithRecordMissingName = `[
  {
    "meta": {"id": "variable.example", "type": "variable", "name": "example"}
  },
  {
    "meta": {"id": "variable.broken", "type": "variable"}
  }
]`

// stateWithRecordEmptyName is a saved state whose second record has a meta
// object holding an empty name.
const stateWithRecordEmptyName = `[
  {
    "meta": {"id": "variable.example", "type": "variable", "name": "example"}
  },
  {
    "meta": {"id": "variable.broken", "type": "variable", "name": ""}
  }
]`

// stateWithMalformedRecord is a saved state whose second record is not an
// object, so there is no meta to read and nothing to name it but its position.
const stateWithMalformedRecord = `[
  {
    "meta": {"id": "variable.example", "type": "variable", "name": "example"}
  },
  "not a record"
]`

// stateWithSeveralUnreadableRecords holds one of every way a record can fail
// to be understood: a record that is not an object, two records naming the
// same entity that both lack a name, a record whose type nothing registers,
// and a record with no meta at all.
const stateWithSeveralUnreadableRecords = `[
  "not a record",
  {
    "meta": {"id": "variable.broken", "type": "variable"}
  },
  {
    "meta": {"id": "variable.broken", "type": "variable", "name": ""}
  },
  {
    "meta": {"id": "resource.postgres.main", "type": "resource", "subtype": "postgres", "name": "main"}
  },
  {
    "location": "us-east"
  }
]`

// A record with no meta at all is reported by its position in the state, and
// decoding returns nothing rather than a configuration missing that record.
func TestDecodeAllFailsWhenRecordHasNoMeta(t *testing.T) {
	s, err := savedentity.DecodeAll(registry.NewPluginRegistry(), testRecords(t, stateWithRecordMissingMeta))
	require.Error(t, err)
	require.Nil(t, s)

	unknown := state.UnknownTypesError{}
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, []string{"entry 1"}, unknown.Types)
}

// A record that does not say which kind declared it is reported by its id, and
// decoding returns nothing rather than a configuration missing that record.
func TestDecodeAllFailsWhenRecordHasNoType(t *testing.T) {
	s, err := savedentity.DecodeAll(registry.NewPluginRegistry(), testRecords(t, stateWithRecordMissingType))
	require.Error(t, err)
	require.Nil(t, s)

	unknown := state.UnknownTypesError{}
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, []string{"resource.database.main"}, unknown.Types)
}

// An empty kind is reported in the same way as a missing one.
func TestDecodeAllFailsWhenRecordHasEmptyType(t *testing.T) {
	s, err := savedentity.DecodeAll(registry.NewPluginRegistry(), testRecords(t, stateWithRecordEmptyType))
	require.Error(t, err)
	require.Nil(t, s)

	unknown := state.UnknownTypesError{}
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, []string{"resource.database.main"}, unknown.Types)
}

// A record that does not say what the entity is called is reported by its id,
// and decoding returns nothing rather than a configuration missing it.
func TestDecodeAllFailsWhenRecordHasNoName(t *testing.T) {
	s, err := savedentity.DecodeAll(registry.NewPluginRegistry(), testRecords(t, stateWithRecordMissingName))
	require.Error(t, err)
	require.Nil(t, s)

	unknown := state.UnknownTypesError{}
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, []string{"variable.broken"}, unknown.Types)
}

// An empty name is reported in the same way as a missing one.
func TestDecodeAllFailsWhenRecordHasEmptyName(t *testing.T) {
	s, err := savedentity.DecodeAll(registry.NewPluginRegistry(), testRecords(t, stateWithRecordEmptyName))
	require.Error(t, err)
	require.Nil(t, s)

	unknown := state.UnknownTypesError{}
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, []string{"variable.broken"}, unknown.Types)
}

// A record that is not an object at all is reported by its position, and
// decoding returns nothing rather than a configuration missing that record.
func TestDecodeAllFailsWhenRecordIsMalformed(t *testing.T) {
	s, err := savedentity.DecodeAll(registry.NewPluginRegistry(), testRecords(t, stateWithMalformedRecord))
	require.Error(t, err)
	require.Nil(t, s)

	unknown := state.UnknownTypesError{}
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, []string{"entry 1"}, unknown.Types)
}

// Every record that can not be understood is named once, in sorted order,
// whichever way each of them failed.
func TestDecodeAllNamesEveryUnreadableRecordOnceInSortedOrder(t *testing.T) {
	s, err := savedentity.DecodeAll(registry.NewPluginRegistry(), testRecords(t, stateWithSeveralUnreadableRecords))
	require.Error(t, err)
	require.Nil(t, s)

	unknown := state.UnknownTypesError{}
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, []string{"entry 0", "entry 4", "resource.postgres", "variable.broken"}, unknown.Types)
}

// Entities saved to a file store and decoded from what it loads come back
// typed, each carrying both of the axes state records, the kind that declared
// it and the variety it names where it has one.
func TestDecodeAllTypesWhatAFileStoreLoaded(t *testing.T) {
	reg := registry.NewPluginRegistry()
	err := reg.RegisterType(&registered.Database{}, "resource", registered.TypeDatabase)
	require.NoError(t, err)

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	database, err := reg.CreateEntity("resource", registered.TypeDatabase, "main")
	require.NoError(t, err)

	databaseMeta, err := types.GetMeta(database)
	require.NoError(t, err)
	databaseMeta.ID = "resource.database.main"

	variable, err := reg.CreateEntity(resources.TypeVariable, "", "environment")
	require.NoError(t, err)

	variableMeta, err := types.GetMeta(variable)
	require.NoError(t, err)
	variableMeta.ID = "variable.environment"

	err = store.Save([]any{database, variable})
	require.NoError(t, err)

	loaded, err := store.Load()
	require.NoError(t, err)

	decoded, err := savedentity.DecodeAll(reg, loaded)
	require.NoError(t, err)
	require.Len(t, decoded, 2)

	loadedDatabase, ok := testEntityByID(t, decoded, "resource.database.main").(*registered.Database)
	require.True(t, ok)
	require.Equal(t, types.TypeResource, loadedDatabase.Meta.Type)
	require.Equal(t, registered.TypeDatabase, loadedDatabase.Meta.Subtype)
	require.Equal(t, "main", loadedDatabase.Meta.Name)

	loadedVariableMeta, err := types.GetMeta(testEntityByID(t, decoded, "variable.environment"))
	require.NoError(t, err)
	require.Equal(t, resources.TypeVariable, loadedVariableMeta.Type)
	require.Empty(t, loadedVariableMeta.Subtype)
	require.Equal(t, "environment", loadedVariableMeta.Name)
}

// A store that keeps entities in memory hands them back already typed, and
// they are returned as they are.
func TestDecodeAllReturnsTypedEntitiesUnchanged(t *testing.T) {
	reg := registry.NewPluginRegistry()

	variable, err := reg.CreateEntity(resources.TypeVariable, "", "environment")
	require.NoError(t, err)

	decoded, err := savedentity.DecodeAll(reg, []any{variable})
	require.NoError(t, err)
	require.Len(t, decoded, 1)
	require.Same(t, variable, decoded[0])
}

// A record held as a map, as a store that decodes JSON generically keeps it,
// is typed like a raw record.
func TestDecodeAllTypesRecordsHeldAsMaps(t *testing.T) {
	record := map[string]any{
		"meta": map[string]any{"id": "variable.example", "type": "variable", "name": "example"},
	}

	decoded, err := savedentity.DecodeAll(registry.NewPluginRegistry(), []any{record})
	require.NoError(t, err)
	require.Len(t, decoded, 1)

	meta, err := types.GetMeta(decoded[0])
	require.NoError(t, err)
	require.Equal(t, "variable.example", meta.ID)
}

// Nothing loaded decodes to no entities.
func TestDecodeAllReturnsNoEntitiesForNothingLoaded(t *testing.T) {
	decoded, err := savedentity.DecodeAll(registry.NewPluginRegistry(), []any{})
	require.NoError(t, err)
	require.Empty(t, decoded)
}

// Records can not be typed without a registry, and decoding says so rather
// than failing on a nil registry.
func TestDecodeAllFailsForRecordsWithoutARegistry(t *testing.T) {
	_, err := savedentity.DecodeAll(nil, testRecords(t, stateWithUnknownType))
	require.ErrorContains(t, err, "no plugin registry")
}

// A type registered without a subtype is declared by its own keyword, so it is
// saved with that keyword as its type and no subtype, and it decodes back to
// the registered Go type with the same type and subtype.
func TestDecodeAllTypesABareTypeWhatAFileStoreLoaded(t *testing.T) {
	reg := registry.NewPluginRegistry()
	err := reg.RegisterType(&registered.Database{}, registered.TypeDatabase)
	require.NoError(t, err)

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	database, err := reg.CreateEntity(registered.TypeDatabase, "", "main")
	require.NoError(t, err)

	databaseMeta, err := types.GetMeta(database)
	require.NoError(t, err)
	databaseMeta.ID = "database.main"

	err = store.Save([]any{database})
	require.NoError(t, err)

	loaded, err := store.Load()
	require.NoError(t, err)

	decoded, err := savedentity.DecodeAll(reg, loaded)
	require.NoError(t, err)
	require.Len(t, decoded, 1)

	loadedDatabase, ok := decoded[0].(*registered.Database)
	require.True(t, ok)
	require.Equal(t, registered.TypeDatabase, loadedDatabase.Meta.Type)
	require.Empty(t, loadedDatabase.Meta.Subtype)
	require.Equal(t, "main", loadedDatabase.Meta.Name)
}
