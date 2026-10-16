package xcl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// encodeRecordTypeVault is the type of encodeRecordVault entities
const encodeRecordTypeVault = "vault"

// encodeRecordVault holds a list of blocks each carrying a sensitive value,
// the one shape no shared fixture type has, so the fill can be followed into
// a slice of blocks by index
type encodeRecordVault struct {
	Meta types.Meta

	Keys []encodeRecordVaultKey `xcl:"key,block" json:"keys,omitempty"`
}

// encodeRecordVaultKey is one key block of a vault, a plain name beside a
// sensitive value
type encodeRecordVaultKey struct {
	Name  string                  `xcl:"name" json:"name"`
	Value types.Sensitive[string] `xcl:"value" json:"value"`
}

// applyEncodeRecordConfig applies the given configuration text with a file
// state store, declaring the secret, the secret shape and the vault types
// beside the test plugin, and returns the applied configuration
func applyEncodeRecordConfig(t *testing.T, text string) *Config {
	t.Helper()

	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, "main.xcl"), []byte(text), 0o600)
	require.NoError(t, err)

	return applyEncodeRecordDir(t, dir)
}

// applyEncodeRecordDir applies the main.xcl in dir as applyEncodeRecordConfig
// does, for a configuration that needs files beside it, such as a module
func applyEncodeRecordDir(t *testing.T, dir string) *Config {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	local := registry.NewLocal()
	local.RegisterPlugin(&parser.TestPlugin{})

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	local.RegisterType(&registered.Secret{}, "resource", registered.TypeSecret)
	local.RegisterType(&registered.SecretShape{}, "resource", registered.TypeSecretShapes)
	local.RegisterType(&encodeRecordVault{}, "resource", encodeRecordTypeVault)

	c, err := NewConfig(
		WithRegistry(local),
		WithStateStore(store),
	)
	require.NoError(t, err)

	err = c.Apply(filepath.Join(dir, "main.xcl"))
	require.NoError(t, err)

	return c
}

// encodeRecordOf returns the JSON record a lookup hands out for the entity the
// configuration holds under id, decoded into a map so a test can edit it
func encodeRecordOf(t *testing.T, c *Config, id string) map[string]any {
	t.Helper()

	entity := encodeEntityByID(t, c, id)

	data, err := json.Marshal(entity)
	require.NoError(t, err)

	record := map[string]any{}
	err = json.Unmarshal(data, &record)
	require.NoError(t, err)

	return record
}

// encodeRecordBytes marshals an edited record back into the bytes
// EncodeEntityRecord reads
func encodeRecordBytes(t *testing.T, record map[string]any) []byte {
	t.Helper()

	data, err := json.Marshal(record)
	require.NoError(t, err)

	return data
}

// encodeRecordMeta returns the meta object of a record, failing the test when
// the record has none
func encodeRecordMeta(t *testing.T, record map[string]any) map[string]any {
	t.Helper()

	meta, ok := record["meta"].(map[string]any)
	require.True(t, ok, "expected the record to hold a meta object, got %#v", record["meta"])

	return meta
}

func TestEncodeEntityRecordMatchesEncodeEntity(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	database := encodeEntityByID(t, c, encodeDatabaseID)

	fromEntity, err := EncodeEntity(database)
	require.NoError(t, err)

	record := encodeRecordBytes(t, encodeRecordOf(t, c, encodeDatabaseID))

	fromRecord, err := c.EncodeEntityRecord(record)
	require.NoError(t, err)

	require.Equal(t, string(fromEntity), string(fromRecord))
}

func TestEncodeEntityRecordIncludeComputedMatchesEncodeEntity(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	network := encodeEntityByID(t, c, encodeNetworkID)

	fromEntity, err := EncodeEntity(network, IncludeComputed())
	require.NoError(t, err)

	record := encodeRecordBytes(t, encodeRecordOf(t, c, encodeNetworkID))

	fromRecord, err := c.EncodeEntityRecord(record, IncludeComputed())
	require.NoError(t, err)

	require.Equal(t, string(fromEntity), string(fromRecord))
	require.Contains(t, string(fromRecord), `provider_id = "id-main"`)
}

func TestEncodeEntityRecordIncludeEmptyMatchesEncodeEntity(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	fromEntity, err := EncodeEntity(container, IncludeEmpty())
	require.NoError(t, err)

	record := encodeRecordBytes(t, encodeRecordOf(t, c, encodeContainerID))

	fromRecord, err := c.EncodeEntityRecord(record, IncludeEmpty())
	require.NoError(t, err)

	require.Equal(t, string(fromEntity), string(fromRecord))
	require.Contains(t, string(fromRecord), "networkobj = {")
}

func TestEncodeEntityRecordShowReferencesMatchesEncodeEntity(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	database := encodeEntityByID(t, c, encodeDatabaseID)

	fromEntity, err := EncodeEntity(database, ShowReferences())
	require.NoError(t, err)

	record := encodeRecordBytes(t, encodeRecordOf(t, c, encodeDatabaseID))

	fromRecord, err := c.EncodeEntityRecord(record, ShowReferences())
	require.NoError(t, err)

	require.Equal(t, string(fromEntity), string(fromRecord))
	require.Regexp(t, `location\s+= variable\.region`, string(fromRecord))
}

func TestEncodeEntityRecordRevealFillsFromStoredEntity(t *testing.T) {
	c := applyEncodeRecordConfig(t, encodeSensitiveSecretConfig)

	record := encodeRecordOf(t, c, "resource.secret.login")

	// a lookup hands out only the marker
	require.Equal(t, types.SensitiveMarker, record["password"])

	out, err := c.EncodeEntityRecord(encodeRecordBytes(t, record), RevealSensitive())
	require.NoError(t, err)

	text := string(out)

	require.Regexp(t, `password\s+= "hunter2-value"`, text)
	require.Regexp(t, `username\s+= "admin"`, text)
	require.NotContains(t, text, "(sensitive)")
}

func TestEncodeEntityRecordRevealMatchesEncodeEntity(t *testing.T) {
	c := applyEncodeRecordConfig(t, encodeSensitiveSecretConfig)

	secret := encodeEntityByID(t, c, "resource.secret.login")

	fromEntity, err := EncodeEntity(secret, RevealSensitive())
	require.NoError(t, err)

	record := encodeRecordBytes(t, encodeRecordOf(t, c, "resource.secret.login"))

	fromRecord, err := c.EncodeEntityRecord(record, RevealSensitive())
	require.NoError(t, err)

	require.Equal(t, string(fromEntity), string(fromRecord))
}

func TestEncodeEntityRecordWithoutRevealShowsMarker(t *testing.T) {
	c := applyEncodeRecordConfig(t, encodeSensitiveSecretConfig)

	record := encodeRecordBytes(t, encodeRecordOf(t, c, "resource.secret.login"))

	out, err := c.EncodeEntityRecord(record)
	require.NoError(t, err)

	text := string(out)

	require.Regexp(t, `password\s+= "\(sensitive\)"`, text)
	require.Regexp(t, `username\s+= "admin"`, text)
	require.NotContains(t, text, "hunter2-value")
}

func TestEncodeEntityRecordWithoutRevealShowsMarkerForEditedSensitiveValue(t *testing.T) {
	c := applyEncodeRecordConfig(t, encodeSensitiveSecretConfig)

	record := encodeRecordOf(t, c, "resource.secret.login")
	record["password"] = "edited-value"

	out, err := c.EncodeEntityRecord(encodeRecordBytes(t, record))
	require.NoError(t, err)

	text := string(out)

	require.Regexp(t, `password\s+= "\(sensitive\)"`, text)
	require.NotContains(t, text, "edited-value")
	require.NotContains(t, text, "hunter2-value")
}

func TestEncodeEntityRecordRevealKeepsEditedSensitiveValue(t *testing.T) {
	c := applyEncodeRecordConfig(t, encodeSensitiveSecretConfig)

	record := encodeRecordOf(t, c, "resource.secret.login")
	record["password"] = "edited-value"

	out, err := c.EncodeEntityRecord(encodeRecordBytes(t, record), RevealSensitive())
	require.NoError(t, err)

	text := string(out)

	require.Regexp(t, `password\s+= "edited-value"`, text)
	require.NotContains(t, text, "hunter2-value")
	require.NotContains(t, text, "(sensitive)")
}

func TestEncodeEntityRecordKeepsEditedValue(t *testing.T) {
	c := applyEncodeRecordConfig(t, encodeSensitiveSecretConfig)

	record := encodeRecordOf(t, c, "resource.secret.login")
	record["username"] = "operator"

	out, err := c.EncodeEntityRecord(encodeRecordBytes(t, record), RevealSensitive())
	require.NoError(t, err)

	text := string(out)

	require.Regexp(t, `username\s+= "operator"`, text)
	require.NotContains(t, text, `"admin"`)
	require.Regexp(t, `password\s+= "hunter2-value"`, text)
}

func TestEncodeEntityRecordRevealWithoutIDShowsMarker(t *testing.T) {
	c := applyEncodeRecordConfig(t, encodeSensitiveSecretConfig)

	record := encodeRecordOf(t, c, "resource.secret.login")
	meta := encodeRecordMeta(t, record)
	delete(meta, "id")

	out, err := c.EncodeEntityRecord(encodeRecordBytes(t, record), RevealSensitive())
	require.NoError(t, err)

	text := string(out)

	require.Regexp(t, `password\s+= "\(sensitive\)"`, text)
	require.NotContains(t, text, "hunter2-value")
}

func TestEncodeEntityRecordRevealWithUnknownIDShowsMarker(t *testing.T) {
	c := applyEncodeRecordConfig(t, encodeSensitiveSecretConfig)

	record := encodeRecordOf(t, c, "resource.secret.login")
	meta := encodeRecordMeta(t, record)
	meta["id"] = "resource.secret.nobody"

	out, err := c.EncodeEntityRecord(encodeRecordBytes(t, record), RevealSensitive())
	require.NoError(t, err)

	text := string(out)

	require.Regexp(t, `password\s+= "\(sensitive\)"`, text)
	require.NotContains(t, text, "hunter2-value")
}

func TestEncodeEntityRecordRevealFillsNestedSensitiveValues(t *testing.T) {
	c := applyEncodeRecordConfig(t, `
resource "secret_shape" "nested" {
  secrets = {
    api = "api-key-value"
  }

  login = {
    user     = "operator"
    password = "login-password-value"
  }
}
`)

	record := encodeRecordOf(t, c, "resource.secret_shape.nested")

	out, err := c.EncodeEntityRecord(encodeRecordBytes(t, record), RevealSensitive())
	require.NoError(t, err)

	text := string(out)

	require.Contains(t, text, `"api-key-value"`)
	require.Contains(t, text, `"login-password-value"`)
	require.Contains(t, text, `"operator"`)
	require.NotContains(t, text, "(sensitive)")
}

func TestEncodeEntityRecordWithoutRevealShowsMarkerForNestedSensitiveValues(t *testing.T) {
	c := applyEncodeRecordConfig(t, `
resource "secret_shape" "nested" {
  secrets = {
    api = "api-key-value"
  }

  login = {
    user     = "operator"
    password = "login-password-value"
  }
}
`)

	record := encodeRecordOf(t, c, "resource.secret_shape.nested")

	out, err := c.EncodeEntityRecord(encodeRecordBytes(t, record))
	require.NoError(t, err)

	text := string(out)

	require.Contains(t, text, "(sensitive)")
	require.Contains(t, text, `"operator"`)
	require.NotContains(t, text, "api-key-value")
	require.NotContains(t, text, "login-password-value")
}

func TestEncodeEntityRecordRevealFillsSensitiveValuesInListOfBlocks(t *testing.T) {
	c := applyEncodeRecordConfig(t, `
resource "vault" "keys" {
  key {
    name  = "first"
    value = "first-key-value"
  }

  key {
    name  = "second"
    value = "second-key-value"
  }
}
`)

	record := encodeRecordOf(t, c, "resource.vault.keys")

	out, err := c.EncodeEntityRecord(encodeRecordBytes(t, record), RevealSensitive())
	require.NoError(t, err)

	text := string(out)

	require.Regexp(t, `value\s+= "first-key-value"`, text)
	require.Regexp(t, `value\s+= "second-key-value"`, text)
	require.NotContains(t, text, "(sensitive)")
}

func TestEncodeEntityRecordRevealKeepsEditedSensitiveValueInListOfBlocks(t *testing.T) {
	c := applyEncodeRecordConfig(t, `
resource "vault" "keys" {
  key {
    name  = "first"
    value = "first-key-value"
  }

  key {
    name  = "second"
    value = "second-key-value"
  }
}
`)

	record := encodeRecordOf(t, c, "resource.vault.keys")

	keys, ok := record["keys"].([]any)
	require.True(t, ok, "expected the record to hold a list of keys, got %#v", record["keys"])
	require.Len(t, keys, 2)

	second, ok := keys[1].(map[string]any)
	require.True(t, ok)
	second["value"] = "edited-key-value"

	out, err := c.EncodeEntityRecord(encodeRecordBytes(t, record), RevealSensitive())
	require.NoError(t, err)

	text := string(out)

	require.Regexp(t, `value\s+= "first-key-value"`, text)
	require.Regexp(t, `value\s+= "edited-key-value"`, text)
	require.NotContains(t, text, "second-key-value")
	require.NotContains(t, text, "(sensitive)")
}

func TestEncodeEntityRecordRefusesVariable(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	record := encodeRecordBytes(t, encodeRecordOf(t, c, encodeVariableID))

	out, err := c.EncodeEntityRecord(record)

	require.Nil(t, out)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotEncodable)
	require.Contains(t, err.Error(), "variable")
}

func TestEncodeEntityRecordRefusesOutput(t *testing.T) {
	c := applyEncodeRecordConfig(t, `
output "greeting" {
  value = "hello"
}
`)

	record := encodeRecordBytes(t, encodeRecordOf(t, c, "output.greeting"))

	out, err := c.EncodeEntityRecord(record)

	require.Nil(t, out)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotEncodable)
	require.Contains(t, err.Error(), "output")
}

func TestEncodeEntityRecordRefusesModule(t *testing.T) {
	dir := t.TempDir()

	err := os.Mkdir(filepath.Join(dir, "leaf"), 0o700)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(dir, "leaf", "leaf.xcl"), []byte(`
resource "secret" "inner" {
  password = "inner-value"
}
`), 0o600)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(dir, "main.xcl"), []byte(`
module "leaf" {
  source = "./leaf"
}
`), 0o600)
	require.NoError(t, err)

	c := applyEncodeRecordDir(t, dir)

	record := encodeRecordBytes(t, encodeRecordOf(t, c, "module.leaf"))

	out, err := c.EncodeEntityRecord(record)

	require.Nil(t, out)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotEncodable)
	require.Contains(t, err.Error(), "module")
}

func TestEncodeEntityRecordRecordWithoutMetaIsInvalid(t *testing.T) {
	c := applyEncodeRecordConfig(t, encodeSensitiveSecretConfig)

	record := encodeRecordOf(t, c, "resource.secret.login")
	delete(record, "meta")

	out, err := c.EncodeEntityRecord(encodeRecordBytes(t, record), RevealSensitive())

	require.Nil(t, out)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidSavedData)
}

func TestEncodeEntityRecordUnregisteredType(t *testing.T) {
	c := applyEncodeRecordConfig(t, encodeSensitiveSecretConfig)

	record := encodeRecordOf(t, c, "resource.secret.login")
	meta := encodeRecordMeta(t, record)
	meta["id"] = "resource.unknown.login"
	meta["subtype"] = "unknown"

	out, err := c.EncodeEntityRecord(encodeRecordBytes(t, record), RevealSensitive())

	require.Nil(t, out)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrUnregisteredType)

	unregistered := &UnregisteredTypeError{}
	require.ErrorAs(t, err, &unregistered)
	require.Equal(t, "resource.unknown", unregistered.Type)
}

func TestEncodeEntityRecordRevealFillsPluginTypeFromStoredEntity(t *testing.T) {
	c := applyEncodeRecordConfig(t, encodeSensitiveCredentialConfig)

	record := encodeRecordOf(t, c, "resource.credential.db")

	out, err := c.EncodeEntityRecord(encodeRecordBytes(t, record), RevealSensitive())
	require.NoError(t, err)

	text := string(out)

	require.Regexp(t, `password\s+= "pw-value"`, text)
	require.Regexp(t, `pin\s+= 4821`, text)
	require.NotContains(t, text, "(sensitive)")
}

func TestEncodeEntityRecordLeavesOutUnsetSensitiveValue(t *testing.T) {
	c := applyEncodeRecordConfig(t, `
resource "secret_shape" "bare" {
  login = {
    user = "operator"
  }
}
`)

	record := encodeRecordOf(t, c, "resource.secret_shape.bare")

	// a lookup writes the marker for a sensitive value even when none is set
	login, ok := record["login"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, types.SensitiveMarker, login["password"])

	out, err := c.EncodeEntityRecord(encodeRecordBytes(t, record))
	require.NoError(t, err)

	require.Equal(t, `resource "secret_shape" "bare" {
  login = {
    user = "operator"
  }
}
`, string(out))
}

func TestEncodeEntityRecordUnsetSensitiveValueMatchesEncodeEntity(t *testing.T) {
	c := applyEncodeRecordConfig(t, `
resource "secret_shape" "bare" {
  login = {
    user = "operator"
  }
}
`)

	record := encodeRecordOf(t, c, "resource.secret_shape.bare")
	stored := encodeEntityByID(t, c, "resource.secret_shape.bare")

	want, err := EncodeEntity(stored)
	require.NoError(t, err)

	got, err := c.EncodeEntityRecord(encodeRecordBytes(t, record))
	require.NoError(t, err)

	require.Equal(t, string(want), string(got))
}

func TestEncodeEntityRecordKeepsConfiguredEmptyList(t *testing.T) {
	c := applyEncodeRecordConfig(t, `
resource "secret_shape" "bare" {
  items = []
}
`)

	record := encodeRecordOf(t, c, "resource.secret_shape.bare")

	// a lookup leaves an empty list out of its JSON
	_, found := record["items"]
	require.False(t, found)

	out, err := c.EncodeEntityRecord(encodeRecordBytes(t, record))
	require.NoError(t, err)

	require.Contains(t, string(out), "items = []")
}

func TestEncodeEntityRecordConfiguredEmptyListMatchesEncodeEntity(t *testing.T) {
	c := applyEncodeRecordConfig(t, `
resource "secret_shape" "bare" {
  items = []
}
`)

	record := encodeRecordOf(t, c, "resource.secret_shape.bare")
	stored := encodeEntityByID(t, c, "resource.secret_shape.bare")

	want, err := EncodeEntity(stored)
	require.NoError(t, err)

	got, err := c.EncodeEntityRecord(encodeRecordBytes(t, record))
	require.NoError(t, err)

	require.Equal(t, string(want), string(got))
}

func TestEncodeEntityRecordWithoutIDLeavesOutEmptyList(t *testing.T) {
	c := applyEncodeRecordConfig(t, `
resource "secret_shape" "bare" {
  items = []
}
`)

	record := encodeRecordOf(t, c, "resource.secret_shape.bare")
	delete(encodeRecordMeta(t, record), "id")

	out, err := c.EncodeEntityRecord(encodeRecordBytes(t, record))
	require.NoError(t, err)

	require.NotContains(t, string(out), "items")
}
