// The tests in this file drive a real apply to produce the saved records they
// read back, as the project requires of any test that needs earlier state. An
// apply lives in the parser, which imports state, which imports this package,
// so the tests sit in the external savedentity_test package where they can use
// the public xcl API without an import cycle.
//
// The tests that are about the record format itself, which the convention
// exempts, hand write their bytes: a record that is not valid JSON, or that
// carries no meta, cannot be produced by an apply at all.
package savedentity_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl"
	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/savedentity"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// registeredConfig declares a variable, a resource carrying a variety, a module
// and, inside that module, a further resource and an output. Between them they
// cover a registered type and every builtin the state store has to read back.
const registeredConfig = "../test_fixtures/config/registered/basic/main.xcl"

// pluginConfig declares resources of types the TestPlugin provides, which have
// no Go declaration here and resolve only once the registry has loaded.
const pluginConfig = "../test_fixtures/config/query/main.xcl"

// testRegisteredRegistry returns a registry holding the types
// registeredConfig declares, and points HOME at the test's temp directory so
// an apply never writes to the user's home folder.
func testRegisteredRegistry(t *testing.T) *registry.PluginRegistry {
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

// testPluginRegistry returns a registry holding the types pluginConfig
// declares: database is a registered Go type, and container, network, template
// and sidecar come from the TestPlugin.
func testPluginRegistry(t *testing.T) *registry.PluginRegistry {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	reg := registry.NewPluginRegistry()

	err := reg.RegisterType(registered.TypeDatabase, &registered.Database{})
	require.NoError(t, err)

	err = reg.RegisterPlugin(&parser.TestPlugin{})
	require.NoError(t, err)

	return reg
}

// testApplyToStateFile applies config with reg through a file state store and
// returns the path the state was written to, so the saved records can be read
// straight off disk.
func testApplyToStateFile(t *testing.T, reg *registry.PluginRegistry, config string) string {
	t.Helper()

	statePath := filepath.Join(t.TempDir(), "state.json")

	store, err := state.NewFileStateStore(statePath, reg)
	require.NoError(t, err)

	c := xcl.NewConfig(
		xcl.WithPluginRegistry(reg),
		xcl.WithStateStore(store),
	)

	err = c.Apply(config)
	require.NoError(t, err)

	return statePath
}

// testSavedRecord returns the one saved record the state file at statePath
// holds under the given id, exactly as it was written
func testSavedRecord(t *testing.T, statePath string, id string) []byte {
	t.Helper()

	data, err := os.ReadFile(statePath)
	require.NoError(t, err)

	var records []json.RawMessage
	err = json.Unmarshal(data, &records)
	require.NoError(t, err)

	for _, record := range records {
		var envelope struct {
			Meta struct {
				ID string `json:"id"`
			} `json:"meta"`
		}

		err = json.Unmarshal(record, &envelope)
		require.NoError(t, err)

		if envelope.Meta.ID == id {
			return record
		}
	}

	t.Fatalf("the state file at %s holds no record with the id %q", statePath, id)

	return nil
}

// A record written for a registered Go type comes back as that Go type, with
// the values the apply wrote still on it. Reading a record is only useful if
// the caller gets the type it declared, not a bag of fields.
func TestDecodeReturnsTypedRegisteredEntity(t *testing.T) {
	reg := testRegisteredRegistry(t)

	statePath := testApplyToStateFile(t, reg, registeredConfig)

	record := testSavedRecord(t, statePath, "resource.database.main")

	entity, err := savedentity.Decode(reg, record)
	require.NoError(t, err)

	database, ok := entity.(*registered.Database)
	require.True(t, ok)

	require.Equal(t, "resource.database.main", database.Meta.ID)
	require.Equal(t, "resource", database.Meta.Type)
	require.Equal(t, "database", database.Meta.Subtype)
	require.Equal(t, "main", database.Meta.Name)

	require.Equal(t, "us-east", database.Location)
	require.Equal(t, 5432, database.Port)

	require.NotNil(t, database.Timeouts)
	require.Equal(t, 30, database.Timeouts.Connect)
	require.Equal(t, 60, database.Timeouts.Read)
}

// A record written for a type a plugin provides comes back too. The type has
// no Go declaration here, so it resolves only through the loaded plugin, which
// is the case a reader that only knew about registered types would miss.
func TestDecodeReturnsTypedPluginEntity(t *testing.T) {
	reg := testPluginRegistry(t)

	statePath := testApplyToStateFile(t, reg, pluginConfig)

	// a plugin's types resolve only once the registry has loaded
	err := reg.Load(nil)
	require.NoError(t, err)

	record := testSavedRecord(t, statePath, "resource.network.frontend")

	entity, err := savedentity.Decode(reg, record)
	require.NoError(t, err)
	require.NotNil(t, entity)

	meta, err := types.GetMeta(entity)
	require.NoError(t, err)

	require.Equal(t, "resource.network.frontend", meta.ID)
	require.Equal(t, "resource", meta.Type)
	require.Equal(t, "network", meta.Subtype)
	require.Equal(t, "frontend", meta.Name)

	// the instance is built from the plugin's schema, so there is no Go type to
	// assert against: read the field back the way the record carries it
	encoded, err := json.Marshal(entity)
	require.NoError(t, err)

	var fields struct {
		Subnet string `json:"subnet"`
	}

	err = json.Unmarshal(encoded, &fields)
	require.NoError(t, err)

	require.Equal(t, "10.0.1.0/24", fields.Subnet)
}

// Builtins are saved alongside everything else, and the state store loads the
// whole file, so a reader that handled only resources would break a load. A
// builtin carries no variety, which the omitted subtype has to read back as.
func TestDecodeReturnsTypedBuiltin(t *testing.T) {
	reg := testRegisteredRegistry(t)

	statePath := testApplyToStateFile(t, reg, registeredConfig)

	variableRecord := testSavedRecord(t, statePath, "variable.environment")

	variableEntity, err := savedentity.Decode(reg, variableRecord)
	require.NoError(t, err)

	variable, ok := variableEntity.(*resources.Variable)
	require.True(t, ok)

	require.Equal(t, "variable.environment", variable.Meta.ID)
	require.Equal(t, "variable", variable.Meta.Type)
	require.Empty(t, variable.Meta.Subtype)
	require.Equal(t, "environment", variable.Meta.Name)

	outputRecord := testSavedRecord(t, statePath, "module.shared.output.location")

	outputEntity, err := savedentity.Decode(reg, outputRecord)
	require.NoError(t, err)

	output, ok := outputEntity.(*resources.Output)
	require.True(t, ok)

	require.Equal(t, "module.shared.output.location", output.Meta.ID)
	require.Equal(t, "output", output.Meta.Type)
	require.Empty(t, output.Meta.Subtype)
	require.Equal(t, "location", output.Meta.Name)
	require.Equal(t, "eu-west", output.Value)

	moduleRecord := testSavedRecord(t, statePath, "module.shared")

	moduleEntity, err := savedentity.Decode(reg, moduleRecord)
	require.NoError(t, err)

	module, ok := moduleEntity.(*resources.Module)
	require.True(t, ok)

	require.Equal(t, "module.shared", module.Meta.ID)
	require.Equal(t, "module", module.Meta.Type)
	require.Empty(t, module.Meta.Subtype)
	require.Equal(t, "shared", module.Meta.Name)
	require.Equal(t, "./module", module.Source)
}

// A record naming a type nobody registered has to say which type that was, so
// the caller can register it or its plugin rather than guess.
func TestDecodeFailsForUnregisteredType(t *testing.T) {
	reg := registry.NewPluginRegistry()

	record := []byte(`{"meta":{"id":"resource.widget.main","name":"main","type":"resource","subtype":"widget"}}`)

	entity, err := savedentity.Decode(reg, record)

	require.Error(t, err)
	require.Nil(t, entity)

	require.ErrorIs(t, err, xclerrors.ErrUnregisteredType)

	var detail *xclerrors.UnregisteredTypeError
	require.True(t, errors.As(err, &detail))
	require.Equal(t, "widget", detail.Type)
}

// Bytes that are not JSON at all are not a record, and the failure has to be
// the one a caller can match rather than whatever encoding/json returned.
func TestDecodeFailsForInvalidJSON(t *testing.T) {
	reg := registry.NewPluginRegistry()

	record := []byte("this is not json")

	entity, err := savedentity.Decode(reg, record)

	require.Error(t, err)
	require.Nil(t, entity)

	require.ErrorIs(t, err, xclerrors.ErrInvalidSavedData)
}

// Valid JSON that carries no meta names no type and no entity, so it is not a
// record either, and fails the same way as bytes that would not parse.
func TestDecodeFailsForRecordWithoutMeta(t *testing.T) {
	reg := registry.NewPluginRegistry()

	record := []byte(`{"location":"us-east","port":5432}`)

	entity, err := savedentity.Decode(reg, record)

	require.Error(t, err)
	require.Nil(t, entity)

	require.ErrorIs(t, err, xclerrors.ErrInvalidSavedData)
}
