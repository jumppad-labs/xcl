package xcl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/savedentity"
	"github.com/jumppad-labs/xcl/internal/schema"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

const credentialConfig = `
resource "credential" "db" {
  username = "admin"
  password = "s3cret"
  pin      = 1234
}
`

// writeCredentialConfig writes the credential configuration to a temp file
func writeCredentialConfig(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "credential.xcl")
	require.NoError(t, os.WriteFile(path, []byte(credentialConfig), 0o600))

	return path
}

// newCredentialConfig builds a Config with its own registry and TestPlugin on
// the state directory, so two of them on one directory share only the state
func newCredentialConfig(t *testing.T, dir string, options ...ConfigOption) (*Config, *parser.TestPlugin) {
	t.Helper()

	local := registry.NewLocal()
	testPlugin := &parser.TestPlugin{}
	local.RegisterPlugin(testPlugin)

	store, err := state.NewFileStateStore(dir)
	require.NoError(t, err)

	options = append(options, WithRegistry(local), WithStateStore(store))

	c, err := NewConfig(options...)
	require.NoError(t, err)

	return c, testPlugin
}

// A second Config on the state the first one wrote applies the same
// configuration and still reveals the real value.
func TestRegisteredSensitiveFieldRevealsAfterSecondConfigApply(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()

	firstStore, err := state.NewFileStateStore(dir)
	require.NoError(t, err)

	first, err := NewConfig(withSecretTypes(), WithStateStore(firstStore))
	require.NoError(t, err)
	require.NoError(t, first.Apply(sensitiveBasicPath(t)))

	secondStore, err := state.NewFileStateStore(dir)
	require.NoError(t, err)

	second, err := NewConfig(withSecretTypes(), WithStateStore(secondStore))
	require.NoError(t, err)
	require.NoError(t, second.Apply(sensitiveBasicPath(t)))

	secret, err := Find[registered.Secret](second, "resource.secret.literal")
	require.NoError(t, err)

	require.Equal(t, "admin", secret.Username)
	require.Equal(t, "from-literal", secret.Password.Reveal())
}

// Applying again must not degrade what is saved: the state file written by the
// second apply still holds the real value and never the marker.
func TestRegisteredSensitiveFieldStaysRealInStateAfterSecondApply(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()

	firstStore, err := state.NewFileStateStore(dir)
	require.NoError(t, err)

	first, err := NewConfig(withSecretTypes(), WithStateStore(firstStore))
	require.NoError(t, err)
	require.NoError(t, first.Apply(sensitiveBasicPath(t)))

	secondStore, err := state.NewFileStateStore(dir)
	require.NoError(t, err)

	second, err := NewConfig(withSecretTypes(), WithStateStore(secondStore))
	require.NoError(t, err)
	require.NoError(t, second.Apply(sensitiveBasicPath(t)))

	contents, err := os.ReadFile(secondStore.Path())
	require.NoError(t, err)

	require.Contains(t, string(contents), "from-literal")
	require.Contains(t, string(contents), "from-variable")
	require.NotContains(t, string(contents), "(sensitive)")
}

func TestPluginSensitiveFieldRevealsAfterApplyToFileStateStore(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c, _ := newCredentialConfig(t, t.TempDir())
	require.NoError(t, c.Apply(writeCredentialConfig(t)))

	credential, err := Find[structs.Credential](c, "resource.credential.db")
	require.NoError(t, err)

	require.Equal(t, "s3cret", credential.Password.Reveal())
	require.Equal(t, 1234, credential.Pin.Reveal())
}

func TestPluginSensitiveFieldIsRealInStateFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()

	c, _ := newCredentialConfig(t, dir)
	require.NoError(t, c.Apply(writeCredentialConfig(t)))

	store, err := state.NewFileStateStore(dir)
	require.NoError(t, err)

	contents, err := os.ReadFile(store.Path())
	require.NoError(t, err)

	require.Contains(t, string(contents), `"password": "s3cret"`)
	require.Contains(t, string(contents), "1234")
	require.NotContains(t, string(contents), "(sensitive)")
}

func TestPluginSensitiveFieldRevealsInDecodedState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()

	c, _ := newCredentialConfig(t, dir)
	require.NoError(t, c.Apply(writeCredentialConfig(t)))

	store, err := state.NewFileStateStore(dir)
	require.NoError(t, err)

	records, err := store.Load()
	require.NoError(t, err)

	loaded, err := savedentity.DecodeAll(c.catalog, records, savedentity.ReadOptions{})
	require.NoError(t, err)

	// plugin types decode to a type built from the plugin's schema, so the
	// entity is read into the concrete struct the way the plugin would
	var found *structs.Credential
	for _, entity := range loaded {
		meta, err := types.GetMeta(entity)
		require.NoError(t, err)

		if meta.ID == "resource.credential.db" {
			found = &structs.Credential{}
			require.NoError(t, schema.UnmarshalUntyped(entity, found))
		}
	}

	require.NotNil(t, found)
	require.Equal(t, "s3cret", found.Password.Reveal())
	require.Equal(t, 1234, found.Pin.Reveal())
}

// The second Config has a fresh plugin and only the state file in common with
// the first, so the provider's Read seeing the real value in the saved copy
// proves it was reloaded from state.
func TestPluginProviderReceivesRealSavedValueAfterReload(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()
	path := writeCredentialConfig(t)

	first, _ := newCredentialConfig(t, dir)
	require.NoError(t, first.Apply(path))

	second, secondPlugin := newCredentialConfig(t, dir)
	require.NoError(t, second.Apply(path))

	readCalls := secondPlugin.GetReadCalls()
	require.Len(t, readCalls, 1)
	require.Equal(t, "resource.credential.db", readCalls[0].ID)

	require.Contains(t, string(readCalls[0].Old), `"password":"s3cret"`)
	require.NotContains(t, string(readCalls[0].Old), "(sensitive)")
}

func TestPluginProviderReceivesRealConfiguredValueOnRead(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()
	path := writeCredentialConfig(t)

	first, _ := newCredentialConfig(t, dir)
	require.NoError(t, first.Apply(path))

	second, secondPlugin := newCredentialConfig(t, dir)
	require.NoError(t, second.Apply(path))

	readCalls := secondPlugin.GetReadCalls()
	require.Len(t, readCalls, 1)

	require.Contains(t, string(readCalls[0].New), `"password":"s3cret"`)
	require.NotContains(t, string(readCalls[0].New), "(sensitive)")
}

func TestPluginSensitiveFieldRevealsAfterSecondConfigApply(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()
	path := writeCredentialConfig(t)

	first, _ := newCredentialConfig(t, dir)
	require.NoError(t, first.Apply(path))

	second, _ := newCredentialConfig(t, dir)
	require.NoError(t, second.Apply(path))

	credential, err := Find[structs.Credential](second, "resource.credential.db")
	require.NoError(t, err)

	require.Equal(t, "admin", credential.Username)
	require.Equal(t, "s3cret", credential.Password.Reveal())
	require.Equal(t, 1234, credential.Pin.Reveal())
}

func TestPluginSensitiveFieldStaysRealInStateAfterSecondApply(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()
	path := writeCredentialConfig(t)

	first, _ := newCredentialConfig(t, dir)
	require.NoError(t, first.Apply(path))

	second, _ := newCredentialConfig(t, dir)
	require.NoError(t, second.Apply(path))

	store, err := state.NewFileStateStore(dir)
	require.NoError(t, err)

	contents, err := os.ReadFile(store.Path())
	require.NoError(t, err)

	require.Contains(t, string(contents), `"password": "s3cret"`)
	require.NotContains(t, string(contents), "(sensitive)")
}

// applyCredentialWithEventData applies the credential configuration with an
// event handler at the given level and returns the recorder
func applyCredentialWithEventData(t *testing.T, level EventDataLevel) *eventRecorder {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	recorder := &eventRecorder{}

	c, _ := newCredentialConfig(t, t.TempDir(), WithEventHandler(recorder.Record), WithEventData(level))
	require.NoError(t, c.Apply(writeCredentialConfig(t)))

	return recorder
}

func TestPluginProcessedEventDataShowsOnlyTheSensitiveMarker(t *testing.T) {
	recorder := applyCredentialWithEventData(t, EventDataProcessed)

	succeeded := eventDataSingleEvent(t, recorder, "resource.credential.db", "create", "success")
	require.NotEmpty(t, succeeded.Data)

	require.Contains(t, string(succeeded.Data), `"password":{"xcl_masked":"redact","value":"(sensitive)"}`)
	require.NotContains(t, string(succeeded.Data), "s3cret")
}

func TestPluginRawEventDataShowsOnlyTheSensitiveMarker(t *testing.T) {
	recorder := applyCredentialWithEventData(t, EventDataRaw)

	succeeded := eventDataSingleEvent(t, recorder, "resource.credential.db", "create", "success")
	require.NotEmpty(t, succeeded.Data)

	require.Contains(t, string(succeeded.Data), `"password":{"xcl_masked":"redact","value":"(sensitive)"}`)
	require.NotContains(t, string(succeeded.Data), "s3cret")
}

func TestPluginEventDataShowsTheMarkerForTheSensitiveInteger(t *testing.T) {
	recorder := applyCredentialWithEventData(t, EventDataProcessed)

	succeeded := eventDataSingleEvent(t, recorder, "resource.credential.db", "create", "success")
	require.NotEmpty(t, succeeded.Data)

	require.Contains(t, string(succeeded.Data), `"pin":{"xcl_masked":"redact","value":"(sensitive)"}`)

	// only the pin field is checked for the value, the rest of the data
	// holds a temporary path that can contain any digits
	var record map[string]any
	require.NoError(t, json.Unmarshal(succeeded.Data, &record))
	pin, err := json.Marshal(record["pin"])
	require.NoError(t, err)
	require.NotContains(t, string(pin), "1234")
}
