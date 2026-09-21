package state

import (
	"encoding/json"
	"errors"
	"os"
	"path"
	"testing"

	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

func testCreateState(t *testing.T) (StateStore, string, *registry.PluginRegistry) {
	p := path.Join(t.TempDir(), "state.json")
	reg := registry.NewPluginRegistry(logger.NewTestLogger(t))

	ss, err := NewFileStateStore(p, reg)

	require.NoError(t, err)
	require.NotNil(t, ss)
	require.FileExists(t, p)

	return ss, p, reg
}

func testSaveState(t *testing.T) (StateStore, string, *registry.PluginRegistry) {
	ss, p, reg := testCreateState(t)

	// Create a variable resource using the registry
	varResource, err := reg.CreateResource(resources.TypeVariable, "example")
	require.NoError(t, err)

	// the parser assigns an entity its id while parsing, storage records what
	// it is handed, so the id is set here the way a parse sets it
	meta, err := types.GetMeta(varResource)
	require.NoError(t, err)
	meta.ID = "variable.example"

	err = ss.Save([]any{varResource})
	require.NoError(t, err)
	require.FileExists(t, p)

	return ss, p, reg
}

// entityByID returns the entity the given slice holds under the given id.
// Storage answers no questions about addresses, so a test that wants one
// entity back scans what was loaded and compares the id each entity records
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

	return nil, ResourceNotFoundError{Resource: id}
}

func testNewStateAtExistingPath(t *testing.T) (StateStore, string, *registry.PluginRegistry) {
	_, p, reg := testSaveState(t)

	// create the file first
	err := os.WriteFile(p, []byte("{}"), 0644)
	require.NoError(t, err)

	ss, err := NewFileStateStore(p, reg)

	require.NoError(t, err)
	require.NotNil(t, ss)

	return ss, p, reg
}

func TestCreatesStateAtEmptyPath(t *testing.T) {
	testCreateState(t)
}

func TestSaveSavesStateToFile(t *testing.T) {
	_, p, _ := testSaveState(t)

	// load the file and check contents
	data, err := os.ReadFile(p)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	res := []*resources.Variable{}
	err = json.Unmarshal(data, &res)
	require.NoError(t, err)
	require.Equal(t, "variable.example", res[0].Meta.ID)
}

func TestNewStateAtExistingPath(t *testing.T) {
	testNewStateAtExistingPath(t)
}

func TestLoadStateContainsResources(t *testing.T) {
	ss, _, _ := testSaveState(t)

	s, err := ss.Load()
	require.NoError(t, err)
	require.NotNil(t, s)

	_, err = entityByID(s, "variable.example")
	require.NoError(t, err)
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

// Loading a state holding a type nobody registered fails naming that type,
// rather than returning a state that silently drops the resource.
func TestLoadFailsWhenStateHoldsUnknownType(t *testing.T) {
	ss, p, _ := testCreateState(t)

	err := os.WriteFile(p, []byte(stateWithUnknownType), 0644)
	require.NoError(t, err)

	s, err := ss.Load()
	require.Error(t, err)
	require.Nil(t, s)

	unknown := UnknownTypesError{}
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, []string{"postgres"}, unknown.Types)
	require.Contains(t, err.Error(), "postgres")
}

// Every unknown type is named once, in sorted order, however many resources
// of that type the state holds.
func TestLoadNamesEachUnknownTypeOnceInSortedOrder(t *testing.T) {
	ss, p, _ := testCreateState(t)

	err := os.WriteFile(p, []byte(stateWithUnknownTypes), 0644)
	require.NoError(t, err)

	s, err := ss.Load()
	require.Error(t, err)
	require.Nil(t, s)

	unknown := UnknownTypesError{}
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, []string{"postgres", "redis"}, unknown.Types)
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

// A record with no meta at all is reported by its position in the file, and
// the load returns nothing rather than a configuration missing that record.
func TestLoadFailsWhenRecordHasNoMeta(t *testing.T) {
	ss, p, _ := testCreateState(t)

	err := os.WriteFile(p, []byte(stateWithRecordMissingMeta), 0644)
	require.NoError(t, err)

	s, err := ss.Load()
	require.Error(t, err)
	require.Nil(t, s)

	unknown := UnknownTypesError{}
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, []string{"entry 1"}, unknown.Types)
}

// A record that does not say which kind declared it is reported by its id, and
// the load returns nothing rather than a configuration missing that record.
func TestLoadFailsWhenRecordHasNoType(t *testing.T) {
	ss, p, _ := testCreateState(t)

	err := os.WriteFile(p, []byte(stateWithRecordMissingType), 0644)
	require.NoError(t, err)

	s, err := ss.Load()
	require.Error(t, err)
	require.Nil(t, s)

	unknown := UnknownTypesError{}
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, []string{"resource.database.main"}, unknown.Types)
}

// An empty kind is reported in the same way as a missing one.
func TestLoadFailsWhenRecordHasEmptyType(t *testing.T) {
	ss, p, _ := testCreateState(t)

	err := os.WriteFile(p, []byte(stateWithRecordEmptyType), 0644)
	require.NoError(t, err)

	s, err := ss.Load()
	require.Error(t, err)
	require.Nil(t, s)

	unknown := UnknownTypesError{}
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, []string{"resource.database.main"}, unknown.Types)
}

// A record that does not say what the entity is called is reported by its id,
// and the load returns nothing rather than a configuration missing it.
func TestLoadFailsWhenRecordHasNoName(t *testing.T) {
	ss, p, _ := testCreateState(t)

	err := os.WriteFile(p, []byte(stateWithRecordMissingName), 0644)
	require.NoError(t, err)

	s, err := ss.Load()
	require.Error(t, err)
	require.Nil(t, s)

	unknown := UnknownTypesError{}
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, []string{"variable.broken"}, unknown.Types)
}

// An empty name is reported in the same way as a missing one.
func TestLoadFailsWhenRecordHasEmptyName(t *testing.T) {
	ss, p, _ := testCreateState(t)

	err := os.WriteFile(p, []byte(stateWithRecordEmptyName), 0644)
	require.NoError(t, err)

	s, err := ss.Load()
	require.Error(t, err)
	require.Nil(t, s)

	unknown := UnknownTypesError{}
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, []string{"variable.broken"}, unknown.Types)
}

// A record that is not an object at all is reported by its position, and the
// load returns nothing rather than a configuration missing that record.
func TestLoadFailsWhenRecordIsMalformed(t *testing.T) {
	ss, p, _ := testCreateState(t)

	err := os.WriteFile(p, []byte(stateWithMalformedRecord), 0644)
	require.NoError(t, err)

	s, err := ss.Load()
	require.Error(t, err)
	require.Nil(t, s)

	unknown := UnknownTypesError{}
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, []string{"entry 1"}, unknown.Types)
}

// Every record that can not be understood is named once, in sorted order,
// whichever way each of them failed.
func TestLoadNamesEveryUnreadableRecordOnceInSortedOrder(t *testing.T) {
	ss, p, _ := testCreateState(t)

	err := os.WriteFile(p, []byte(stateWithSeveralUnreadableRecords), 0644)
	require.NoError(t, err)

	s, err := ss.Load()
	require.Error(t, err)
	require.Nil(t, s)

	unknown := UnknownTypesError{}
	require.True(t, errors.As(err, &unknown))
	require.Equal(t, []string{"entry 0", "entry 4", "postgres", "variable.broken"}, unknown.Types)
}

// A store exchanges plain entities: a slice goes in and the same entities come
// back, each carrying both of the axes state records, the kind that declared
// it and the variety it names where it has one.
func TestSaveAndLoadRoundTripsEntitiesWithBothAxes(t *testing.T) {
	ss, _, reg := testCreateState(t)

	err := reg.RegisterType(registered.TypeDatabase, &registered.Database{})
	require.NoError(t, err)

	database, err := reg.CreateResource(registered.TypeDatabase, "main")
	require.NoError(t, err)

	// the parser assigns an entity its id while parsing, storage records what
	// it is handed, so the id is set here the way a parse sets it
	databaseMeta, err := types.GetMeta(database)
	require.NoError(t, err)
	databaseMeta.ID = "resource.database.main"

	variable, err := reg.CreateResource(resources.TypeVariable, "environment")
	require.NoError(t, err)

	variableMeta, err := types.GetMeta(variable)
	require.NoError(t, err)
	variableMeta.ID = "variable.environment"

	err = ss.Save([]any{database, variable})
	require.NoError(t, err)

	loaded, err := ss.Load()
	require.NoError(t, err)
	require.Len(t, loaded, 2)

	loadedDatabase, err := entityByID(loaded, "resource.database.main")
	require.NoError(t, err)

	loadedDatabaseMeta, err := types.GetMeta(loadedDatabase)
	require.NoError(t, err)
	require.Equal(t, types.TypeResource, loadedDatabaseMeta.Type)
	require.Equal(t, registered.TypeDatabase, loadedDatabaseMeta.Subtype)
	require.Equal(t, "main", loadedDatabaseMeta.Name)

	loadedVariable, err := entityByID(loaded, "variable.environment")
	require.NoError(t, err)

	loadedVariableMeta, err := types.GetMeta(loadedVariable)
	require.NoError(t, err)
	require.Equal(t, resources.TypeVariable, loadedVariableMeta.Type)
	require.Empty(t, loadedVariableMeta.Subtype)
	require.Equal(t, "environment", loadedVariableMeta.Name)
}

// Saving nothing writes an empty state rather than failing, which is what
// NewFileStateStore relies on when it creates a state file for a first run.
func TestSaveWithNoEntitiesWritesAnEmptyState(t *testing.T) {
	ss, p, _ := testSaveState(t)

	err := ss.Save(nil)
	require.NoError(t, err)
	require.FileExists(t, p)

	data, err := os.ReadFile(p)
	require.NoError(t, err)
	require.JSONEq(t, "[]", string(data))
}

// A state holding nothing loads as no entities, not as a failure.
func TestLoadReturnsNoEntitiesForAnEmptyState(t *testing.T) {
	ss, _, _ := testCreateState(t)

	loaded, err := ss.Load()
	require.NoError(t, err)
	require.Empty(t, loaded)
}
