// The tests in this file drive a real apply to produce the saved records they
// read back, as the project requires of any test that needs earlier state. An
// apply lives in the parser, which imports this package, so the tests sit in
// the external savedentity_test package where they can use the public xcl API
// without an import cycle.
//
// The tests that are about the record format itself, which the convention
// exempts, hand write their bytes: a record that is not valid JSON, or that
// carries no meta, cannot be produced by an apply at all.
package savedentity_test

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/jumppad-labs/xcl"
	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/savedentity"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/mask"
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

	err := reg.RegisterType(&registered.Database{}, "resource", registered.TypeDatabase)
	require.NoError(t, err)

	err = reg.RegisterType(&registered.App{}, "resource", registered.TypeApp)
	require.NoError(t, err)

	err = reg.RegisterType(&registered.Consumer{}, "resource", registered.TypeConsumer)
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

	err := reg.RegisterType(&registered.Database{}, "resource", registered.TypeDatabase)
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

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	statePath := store.Path()

	c, err := xcl.NewConfig(
		xcl.WithPluginRegistry(reg),
		xcl.WithStateStore(store),
	)
	require.NoError(t, err)

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

	entity, err := savedentity.Decode(reg, record, savedentity.ReadOptions{})
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

	entity, err := savedentity.Decode(reg, record, savedentity.ReadOptions{})
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

	variableEntity, err := savedentity.Decode(reg, variableRecord, savedentity.ReadOptions{})
	require.NoError(t, err)

	variable, ok := variableEntity.(*resources.Variable)
	require.True(t, ok)

	require.Equal(t, "variable.environment", variable.Meta.ID)
	require.Equal(t, "variable", variable.Meta.Type)
	require.Empty(t, variable.Meta.Subtype)
	require.Equal(t, "environment", variable.Meta.Name)

	outputRecord := testSavedRecord(t, statePath, "module.shared.output.location")

	outputEntity, err := savedentity.Decode(reg, outputRecord, savedentity.ReadOptions{})
	require.NoError(t, err)

	output, ok := outputEntity.(*types.Output)
	require.True(t, ok)

	require.Equal(t, "module.shared.output.location", output.Meta.ID)
	require.Equal(t, "output", output.Meta.Type)
	require.Empty(t, output.Meta.Subtype)
	require.Equal(t, "location", output.Meta.Name)
	require.Equal(t, "eu-west", output.Value)

	moduleRecord := testSavedRecord(t, statePath, "module.shared")

	moduleEntity, err := savedentity.Decode(reg, moduleRecord, savedentity.ReadOptions{})
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

	entity, err := savedentity.Decode(reg, record, savedentity.ReadOptions{})

	require.Error(t, err)
	require.Nil(t, entity)

	require.ErrorIs(t, err, xclerrors.ErrUnregisteredType)

	var detail *xclerrors.UnregisteredTypeError
	require.True(t, errors.As(err, &detail))
	require.Equal(t, "resource.widget", detail.Type)
}

// Bytes that are not JSON at all are not a record, and the failure has to be
// the one a caller can match rather than whatever encoding/json returned.
func TestDecodeFailsForInvalidJSON(t *testing.T) {
	reg := registry.NewPluginRegistry()

	record := []byte("this is not json")

	entity, err := savedentity.Decode(reg, record, savedentity.ReadOptions{})

	require.Error(t, err)
	require.Nil(t, entity)

	require.ErrorIs(t, err, xclerrors.ErrInvalidSavedData)
}

// Valid JSON that carries no meta names no type and no entity, so it is not a
// record either, and fails the same way as bytes that would not parse.
func TestDecodeFailsForRecordWithoutMeta(t *testing.T) {
	reg := registry.NewPluginRegistry()

	record := []byte(`{"location":"us-east","port":5432}`)

	entity, err := savedentity.Decode(reg, record, savedentity.ReadOptions{})

	require.Error(t, err)
	require.Nil(t, entity)

	require.ErrorIs(t, err, xclerrors.ErrInvalidSavedData)
}

// State saved before parents were dropped from meta still carries a parents
// list on every record. Reading it back has to ignore that key rather than
// fail, so an older state can still be destroyed. No apply writes the key any
// more, so the record is hand written.
func TestDecodeIgnoresLegacyParentsKey(t *testing.T) {
	reg := registry.NewPluginRegistry()

	record := []byte(`{"meta":{"id":"variable.example","name":"example","type":"variable","parents":["x"]}}`)

	entity, err := savedentity.Decode(reg, record, savedentity.ReadOptions{})
	require.NoError(t, err)

	variable, ok := entity.(*resources.Variable)
	require.True(t, ok, "expected *resources.Variable, got %T", entity)
	require.Equal(t, "variable.example", variable.Meta.ID)
}

// testSecretID is the id of the hand written secret record the masking tests
// read back.
const testSecretID = "resource.secret.db"

// testSecretRegistry returns a registry holding registered.Secret, the
// registered type whose password field is sensitive.
func testSecretRegistry(t *testing.T) *registry.PluginRegistry {
	t.Helper()

	reg := registry.NewPluginRegistry()

	err := reg.RegisterType(&registered.Secret{}, "resource", registered.TypeSecret)
	require.NoError(t, err)

	return reg
}

// testSecretRecord returns a saved record of a registered.Secret whose
// password holds the given JSON exactly as written, either the real value or
// a masker's envelope. The masking tests are about the record format, so the
// record is hand written.
func testSecretRecord(t *testing.T, password json.RawMessage) []byte {
	t.Helper()

	record := map[string]any{
		"meta": map[string]any{
			"id":      testSecretID,
			"type":    "resource",
			"subtype": registered.TypeSecret,
			"name":    "db",
		},
		"username": "admin",
		"password": password,
	}

	data, err := json.Marshal(record)
	require.NoError(t, err)

	return data
}

// testAESMasker returns an AES-256-GCM masker whose 32 byte key is fill
// repeated, so two tests can build the same key or a different one.
func testAESMasker(t *testing.T, fill byte) mask.Masker {
	t.Helper()

	key := make([]byte, 32)
	for i := range key {
		key[i] = fill
	}

	masker, err := mask.EncryptAES256GCM(key)
	require.NoError(t, err)

	return masker
}

// testHMACMasker returns an HMAC-SHA256 masker, which is one way.
func testHMACMasker(t *testing.T) mask.Masker {
	t.Helper()

	masker, err := mask.HashHMACSHA256([]byte("an hmac key used only in tests"))
	require.NoError(t, err)

	return masker
}

// testEnvelope masks the real password hunter2 with masker and returns the
// envelope to write in its place.
func testEnvelope(t *testing.T, masker mask.Masker) json.RawMessage {
	t.Helper()

	envelope, err := mask.Envelope(json.RawMessage(`"hunter2"`), masker)
	require.NoError(t, err)

	return envelope
}

// A value masked with AES opens with the same masker, so the typed entity
// holds the real secret again, which is what lets state be applied after it
// was saved masked.
func TestDecodeOpensAESEnvelopeWithTheSameMasker(t *testing.T) {
	reg := testSecretRegistry(t)
	masker := testAESMasker(t, 1)

	record := testSecretRecord(t, testEnvelope(t, masker))

	entity, err := savedentity.Decode(reg, record, savedentity.ReadOptions{Mask: masker})
	require.NoError(t, err)

	secret, ok := entity.(*registered.Secret)
	require.True(t, ok, "expected *registered.Secret, got %T", entity)

	require.Equal(t, testSecretID, secret.Meta.ID)
	require.Equal(t, "admin", secret.Username)
	require.Equal(t, "hunter2", secret.Password.Reveal())
	require.False(t, types.IsRedacted(secret.Password))
}

// A value masked by one masker cannot be opened by another, and the failure
// names the record and the masker that produced the value, so the user knows
// which key the state needs.
func TestDecodeFailsForEnvelopeFromADifferentMasker(t *testing.T) {
	reg := testSecretRegistry(t)

	record := testSecretRecord(t, testEnvelope(t, testAESMasker(t, 1)))

	entity, err := savedentity.Decode(reg, record, savedentity.ReadOptions{Mask: testHMACMasker(t)})

	require.Error(t, err)
	require.Nil(t, entity)

	require.ErrorIs(t, err, xclerrors.ErrUnrecoverable)

	var unrecoverable *xclerrors.UnrecoverableError
	require.True(t, errors.As(err, &unrecoverable))
	require.Equal(t, testSecretID, unrecoverable.ID)
	require.Equal(t, mask.AES256GCMName, unrecoverable.MaskedBy)
	require.Equal(t, "aes-256-gcm", unrecoverable.MaskedBy)
}

// A one way masker leaves nothing to open, even when it is the masker that
// produced the value, so reading the value back fails rather than handing the
// hash on as though it were the secret.
func TestDecodeFailsForOneWayEnvelopeReadWithTheSameMasker(t *testing.T) {
	reg := testSecretRegistry(t)
	masker := testHMACMasker(t)

	record := testSecretRecord(t, testEnvelope(t, masker))

	entity, err := savedentity.Decode(reg, record, savedentity.ReadOptions{Mask: masker})

	require.Error(t, err)
	require.Nil(t, entity)

	require.ErrorIs(t, err, xclerrors.ErrUnrecoverable)

	var unrecoverable *xclerrors.UnrecoverableError
	require.True(t, errors.As(err, &unrecoverable))
	require.Equal(t, testSecretID, unrecoverable.ID)
	require.Equal(t, mask.HMACSHA256Name, unrecoverable.MaskedBy)
}

// A value masked with one AES key does not open with another, and the
// failure is reported rather than a wrong value returned.
func TestDecodeFailsForAESEnvelopeReadWithTheWrongKey(t *testing.T) {
	reg := testSecretRegistry(t)

	record := testSecretRecord(t, testEnvelope(t, testAESMasker(t, 1)))

	entity, err := savedentity.Decode(reg, record, savedentity.ReadOptions{Mask: testAESMasker(t, 2)})

	require.Error(t, err)
	require.Nil(t, entity)

	require.ErrorIs(t, err, xclerrors.ErrUnrecoverable)

	var unrecoverable *xclerrors.UnrecoverableError
	require.True(t, errors.As(err, &unrecoverable))
	require.Equal(t, testSecretID, unrecoverable.ID)
	require.Equal(t, mask.AES256GCMName, unrecoverable.MaskedBy)
}

// A masked value with no state masker configured cannot be opened, and the
// failure says that no masker is configured, which is the fix to make.
func TestDecodeFailsForEnvelopeWithoutAMasker(t *testing.T) {
	reg := testSecretRegistry(t)

	record := testSecretRecord(t, testEnvelope(t, testAESMasker(t, 1)))

	entity, err := savedentity.Decode(reg, record, savedentity.ReadOptions{})

	require.Error(t, err)
	require.Nil(t, entity)

	require.ErrorIs(t, err, xclerrors.ErrUnrecoverable)
	require.Contains(t, err.Error(), "no state masker is configured")

	var unrecoverable *xclerrors.UnrecoverableError
	require.True(t, errors.As(err, &unrecoverable))
	require.Equal(t, testSecretID, unrecoverable.ID)
	require.Equal(t, mask.AES256GCMName, unrecoverable.MaskedBy)
}

// Reading for display turns an AES envelope into the marker, so the typed
// entity is redacted and never carries the ciphertext as a value.
func TestDecodeForDisplayGivesTheMarkerForAESEnvelope(t *testing.T) {
	reg := testSecretRegistry(t)

	envelope := testEnvelope(t, testAESMasker(t, 1))

	var masked mask.Masked
	err := json.Unmarshal(envelope, &masked)
	require.NoError(t, err)

	var ciphertext string
	err = json.Unmarshal(masked.Value, &ciphertext)
	require.NoError(t, err)
	require.NotEmpty(t, ciphertext)

	record := testSecretRecord(t, envelope)

	entity, err := savedentity.Decode(reg, record, savedentity.ReadOptions{ForDisplay: true})
	require.NoError(t, err)

	secret, ok := entity.(*registered.Secret)
	require.True(t, ok, "expected *registered.Secret, got %T", entity)

	require.True(t, types.IsRedacted(secret.Password))
	require.Equal(t, types.SensitiveMarker, secret.Password.String())
	require.Empty(t, secret.Password.Reveal())

	encoded, err := json.Marshal(secret)
	require.NoError(t, err)
	require.Contains(t, string(encoded), types.SensitiveMarker)
	require.NotContains(t, string(encoded), ciphertext)
	require.NotContains(t, string(encoded), "hunter2")
}

// Reading for display turns an HMAC envelope into the marker too, so a hash
// is never presented as the value either.
func TestDecodeForDisplayGivesTheMarkerForHMACEnvelope(t *testing.T) {
	reg := testSecretRegistry(t)

	envelope := testEnvelope(t, testHMACMasker(t))

	var masked mask.Masked
	err := json.Unmarshal(envelope, &masked)
	require.NoError(t, err)

	var hash string
	err = json.Unmarshal(masked.Value, &hash)
	require.NoError(t, err)
	require.NotEmpty(t, hash)

	record := testSecretRecord(t, envelope)

	entity, err := savedentity.Decode(reg, record, savedentity.ReadOptions{ForDisplay: true})
	require.NoError(t, err)

	secret, ok := entity.(*registered.Secret)
	require.True(t, ok, "expected *registered.Secret, got %T", entity)

	require.True(t, types.IsRedacted(secret.Password))
	require.Equal(t, types.SensitiveMarker, secret.Password.String())
	require.Empty(t, secret.Password.Reveal())

	encoded, err := json.Marshal(secret)
	require.NoError(t, err)
	require.Contains(t, string(encoded), types.SensitiveMarker)
	require.NotContains(t, string(encoded), hash)
}

// A record holding the real value, as state saved without a masker does,
// reads back unchanged when no masker is configured.
func TestDecodeReadsPlainRecordWithoutAMasker(t *testing.T) {
	reg := testSecretRegistry(t)

	record := testSecretRecord(t, json.RawMessage(`"hunter2"`))

	entity, err := savedentity.Decode(reg, record, savedentity.ReadOptions{})
	require.NoError(t, err)

	secret, ok := entity.(*registered.Secret)
	require.True(t, ok, "expected *registered.Secret, got %T", entity)

	require.Equal(t, testSecretID, secret.Meta.ID)
	require.Equal(t, "admin", secret.Username)
	require.Equal(t, "hunter2", secret.Password.Reveal())
	require.False(t, types.IsRedacted(secret.Password))
}

// A record holding the real value reads back unchanged when a masker is
// configured too, so state saved before masking was turned on still loads.
func TestDecodeReadsPlainRecordWithAnAESMasker(t *testing.T) {
	reg := testSecretRegistry(t)

	record := testSecretRecord(t, json.RawMessage(`"hunter2"`))

	entity, err := savedentity.Decode(reg, record, savedentity.ReadOptions{Mask: testAESMasker(t, 1)})
	require.NoError(t, err)

	secret, ok := entity.(*registered.Secret)
	require.True(t, ok, "expected *registered.Secret, got %T", entity)

	require.Equal(t, testSecretID, secret.Meta.ID)
	require.Equal(t, "admin", secret.Username)
	require.Equal(t, "hunter2", secret.Password.Reveal())
	require.False(t, types.IsRedacted(secret.Password))
}
