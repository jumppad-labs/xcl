package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

func testCreateState(t *testing.T) (StateStore, string, *registry.PluginRegistry) {
	dir := t.TempDir()
	p := filepath.Join(dir, StateFileName)
	reg := registry.NewPluginRegistry()

	ss, err := NewFileStateStore(dir)

	require.NoError(t, err)
	require.NotNil(t, ss)
	require.FileExists(t, p)

	return ss, p, reg
}

func testSaveState(t *testing.T) (StateStore, string, *registry.PluginRegistry) {
	ss, p, reg := testCreateState(t)

	// Create a variable resource using the registry
	varResource, err := reg.CreateEntity(resources.TypeVariable, "", "example")
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

func testNewStateAtExistingPath(t *testing.T) (StateStore, string, *registry.PluginRegistry) {
	_, p, reg := testSaveState(t)

	// create the file first
	err := os.WriteFile(p, []byte("{}"), 0644)
	require.NoError(t, err)

	ss, err := NewFileStateStore(filepath.Dir(p))

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
	require.Len(t, s, 1)

	record, ok := s[0].(json.RawMessage)
	require.True(t, ok, "a saved entity loads as its raw record, got %T", s[0])
	require.Equal(t, "variable.example", recordID(t, record))
}

// The store reads and writes records and nothing more, so a record whose type
// nothing registers loads like any other. Whether it can be typed is decided
// by whoever consumes the state.
func TestLoadReturnsRecordsOfUnregisteredTypes(t *testing.T) {
	ss, p, _ := testCreateState(t)

	err := os.WriteFile(p, []byte(`[
  {"meta": {"id": "variable.example", "type": "variable", "name": "example"}},
  {"meta": {"id": "resource.postgres.main", "type": "resource", "subtype": "postgres", "name": "main"}}
]`), 0644)
	require.NoError(t, err)

	s, err := ss.Load()
	require.NoError(t, err)
	require.Len(t, s, 2)
	require.Equal(t, "resource.postgres.main", recordID(t, s[1].(json.RawMessage)))
}

// A state file that is not a JSON array of records can not be read at all.
func TestLoadFailsWhenStateFileIsNotAnArray(t *testing.T) {
	ss, p, _ := testCreateState(t)

	err := os.WriteFile(p, []byte(`{"meta": {}}`), 0644)
	require.NoError(t, err)

	s, err := ss.Load()
	require.ErrorContains(t, err, "unable to deserialize state file")
	require.Nil(t, s)
}

// Saving entities records both of their axes, the kind that declared each one
// and the variety it names where it has one, so a reader can type the record.
func TestSaveRecordsBothAxesOfEachEntity(t *testing.T) {
	ss, _, reg := testCreateState(t)

	err := reg.RegisterType(&registered.Database{}, "resource", registered.TypeDatabase)
	require.NoError(t, err)

	database, err := reg.CreateEntity("resource", registered.TypeDatabase, "main")
	require.NoError(t, err)

	databaseMeta, err := types.GetMeta(database)
	require.NoError(t, err)
	databaseMeta.ID = "resource.database.main"

	err = ss.Save([]any{database})
	require.NoError(t, err)

	loaded, err := ss.Load()
	require.NoError(t, err)
	require.Len(t, loaded, 1)

	var record struct {
		Meta struct {
			Type    string `json:"type"`
			Subtype string `json:"subtype"`
			Name    string `json:"name"`
		} `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(loaded[0].(json.RawMessage), &record))
	require.Equal(t, types.TypeResource, record.Meta.Type)
	require.Equal(t, registered.TypeDatabase, record.Meta.Subtype)
	require.Equal(t, "main", record.Meta.Name)
}

// recordID returns the id a raw saved record carries in its meta
func recordID(t *testing.T, record json.RawMessage) string {
	t.Helper()

	var saved struct {
		Meta struct {
			ID string `json:"id"`
		} `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(record, &saved))

	return saved.Meta.ID
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

// A directory that does not exist yet is created along with the state file in
// it, so a first run needs no setup.
func TestNewFileStateStoreCreatesMissingDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "state")

	ss, err := NewFileStateStore(dir)
	require.NoError(t, err)

	require.DirExists(t, dir)
	require.FileExists(t, filepath.Join(dir, StateFileName))
	require.Equal(t, filepath.Join(dir, StateFileName), ss.Path())

	loaded, err := ss.Load()
	require.NoError(t, err)
	require.Empty(t, loaded)
}

// A state file already in the directory is kept, not replaced with an empty
// one, so a later run reads what an earlier one saved.
func TestNewFileStateStoreKeepsExistingStateFile(t *testing.T) {
	_, p, _ := testSaveState(t)

	ss, err := NewFileStateStore(filepath.Dir(p))
	require.NoError(t, err)

	loaded, err := ss.Load()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	require.Equal(t, "variable.example", recordID(t, loaded[0].(json.RawMessage)))
}

// A path that is a file, not a directory, can not hold the state file.
func TestNewFileStateStoreFailsWhenDirectoryIsAFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0644))

	ss, err := NewFileStateStore(file)
	require.ErrorContains(t, err, "unable to create state directory")
	require.Nil(t, ss)
}
