package xcl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/stretchr/testify/require"
)

// appSchemaDocument is a schema v1 document declaring the "app" block type
// with every shape of field a schema can describe: a required attribute, two
// optional attributes with defaults, a sensitive attribute, a computed
// attribute, an optional nested block, a repeated block with a label and a
// map of named blocks
const appSchemaDocument = `{"schema_version": 1, "type": "app", "fields": [
  {"name": "image", "type": "string"},
  {"name": "replicas", "type": "number", "optional": true, "default": 2},
  {"name": "region", "type": "string", "optional": true, "default": "eu"},
  {"name": "password", "type": "string", "optional": true, "sensitive": true},
  {"name": "endpoint", "type": "string", "computed": true},
  {"name": "health", "kind": "block", "optional": true, "type": {"object": [{"name": "path", "type": "string"}]}},
  {"name": "volume", "kind": "block", "repeated": true, "label": "name", "type": {"object": [{"name": "source", "type": "string"}]}},
  {"name": "listener", "kind": "block", "label": "name", "type": {"map": {"object": [{"name": "port", "type": "number"}]}}}
]}`

// databaseSchemaDocument is a schema v1 document declaring a type with a
// subtype, which configuration declares with two labels
const databaseSchemaDocument = `{"schema_version": 1, "type": "database", "subtype": "postgres", "fields": [
  {"name": "version", "type": "string"}
]}`

// appWebConfig is a block of the app type that sets every kind of field the
// schema declares
const appWebConfig = `
app "web" {
  image = "nginx"
  password = "s3cret"
  health { path = "/healthz" }
  volume "data" { source = "/data" }
  volume { source = "/logs" }
  listener "http" { port = 80 }
  listener "https" { port = 443 }
}
`

// schemaFixture is a Config wired to a local registry holding the app type
// declared from appSchemaDocument, with a real file state store and the
// configuration it applies written to a temporary file
type schemaFixture struct {
	config     *Config
	store      *state.FileStateStore
	stateDir   string
	configFile string
}

// newSchemaRegistry declares every type in the schema documents in a fresh
// local registry
func newSchemaRegistry(t *testing.T, documents ...string) *registry.Local {
	t.Helper()

	local := registry.NewLocal()

	for _, doc := range documents {
		typ, err := registry.TypeFromSchema([]byte(doc))
		require.NoError(t, err)

		if typ.Subtype == "" {
			local.RegisterType(typ.Prototype, typ.Type)
		} else {
			local.RegisterType(typ.Prototype, typ.Type, typ.Subtype)
		}
	}

	return local
}

// setupSchemaConfig declares the app type from its schema, creates a file
// state store in stateDir, writes contents to a configuration file and builds
// a Config with the given options
func setupSchemaConfig(t *testing.T, stateDir, contents string, opts ...ConfigOption) *schemaFixture {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	local := newSchemaRegistry(t, appSchemaDocument)

	store, err := state.NewFileStateStore(stateDir)
	require.NoError(t, err)

	configFile := filepath.Join(t.TempDir(), "main.xcl")
	err = os.WriteFile(configFile, []byte(contents), 0644)
	require.NoError(t, err)

	options := append([]ConfigOption{
		WithRegistry(local),
		WithStateStore(store),
	}, opts...)

	c, err := NewConfig(options...)
	require.NoError(t, err)

	return &schemaFixture{
		config:     c,
		store:      store,
		stateDir:   stateDir,
		configFile: configFile,
	}
}

// findAsMap looks up a resource and returns its JSON form as a map, which is
// how a test reads the fields of the anonymous struct a schema type builds
func findAsMap(t *testing.T, c *Config, id string) map[string]any {
	t.Helper()

	r, err := c.FindResource(id)
	require.NoError(t, err)
	require.NotNil(t, r)

	data, err := json.Marshal(r)
	require.NoError(t, err)

	decoded := map[string]any{}
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	return decoded
}

// TestSchemaTypeDecodesAndIsFound asserts that a block of a type declared
// from a schema validates, applies, and is found by its id with its
// attributes decoded
func TestSchemaTypeDecodesAndIsFound(t *testing.T) {
	f := setupSchemaConfig(t, t.TempDir(), appWebConfig)

	err := f.config.Validate(f.configFile)
	require.NoError(t, err)

	err = f.config.Apply(f.configFile)
	require.NoError(t, err)

	web := findAsMap(t, f.config, "app.web")
	require.Equal(t, "nginx", web["image"])

	meta, ok := web["meta"].(map[string]any)
	require.True(t, ok, "the resource should carry meta")
	require.Equal(t, "app.web", meta["id"])
	require.Equal(t, "app", meta["type"])
}

// TestSchemaTypeWithSubtypeDecodesAndIsFound asserts that a schema type that
// names a subtype is declared with two labels and found by its three part id
func TestSchemaTypeWithSubtypeDecodesAndIsFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	local := newSchemaRegistry(t, databaseSchemaDocument)

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	configFile := filepath.Join(t.TempDir(), "main.xcl")
	err = os.WriteFile(configFile, []byte(`
database "postgres" "main" {
  version = "16"
}
`), 0644)
	require.NoError(t, err)

	c, err := NewConfig(WithRegistry(local), WithStateStore(store))
	require.NoError(t, err)

	err = c.Apply(configFile)
	require.NoError(t, err)

	main := findAsMap(t, c, "database.postgres.main")
	require.Equal(t, "16", main["version"])

	meta, ok := main["meta"].(map[string]any)
	require.True(t, ok, "the resource should carry meta")
	require.Equal(t, "database.postgres.main", meta["id"])
}

// TestSchemaTypeRequiredFieldMissingFailsValidation asserts that leaving out
// a required attribute fails with an error positioned at the block that
// misses it. A missing attribute is found when the block is decoded, which
// Validate does not do for any type, so it is reported by Apply, as it is for
// a hand-written type
func TestSchemaTypeRequiredFieldMissingFailsValidation(t *testing.T) {
	f := setupSchemaConfig(t, t.TempDir(), `
app "web" {
  password = "s3cret"
}
`)

	err := f.config.Apply(f.configFile)
	require.Error(t, err)

	ce, ok := err.(*errors.ConfigError)
	require.True(t, ok, "Validate should report failure as a *errors.ConfigError")
	require.NotEmpty(t, ce.Errors)

	d, ok := ce.Errors[0].(*errors.Diagnostic)
	require.True(t, ok, "the problem should be a *errors.Diagnostic, got %T", ce.Errors[0])
	require.Equal(t, f.configFile, d.Subject.Filename)
	require.Equal(t, 2, d.Subject.Start.Line)
	require.Contains(t, d.Error(), "image")
}

// TestSchemaTypeOptionalFieldTakesDefault asserts that an optional attribute
// left out of configuration takes the default the schema declares
func TestSchemaTypeOptionalFieldTakesDefault(t *testing.T) {
	f := setupSchemaConfig(t, t.TempDir(), `
app "web" {
  image = "nginx"
}
`)

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	web := findAsMap(t, f.config, "app.web")
	require.Equal(t, float64(2), web["replicas"])
	require.Equal(t, "eu", web["region"])
}

// TestSchemaTypeSensitiveFieldMaskedInEvents asserts that processed event
// data carries a sensitive attribute as the redact envelope, never its value
func TestSchemaTypeSensitiveFieldMaskedInEvents(t *testing.T) {
	recorder := &eventRecorder{}

	f := setupSchemaConfig(t, t.TempDir(), appWebConfig,
		WithEventHandler(recorder.Record),
		WithEventData(EventDataProcessed),
	)

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	succeeded := recorder.find("app.web", "create", "success")
	require.Len(t, succeeded, 1)
	require.NotEmpty(t, succeeded[0].Data)

	require.Contains(t, string(succeeded[0].Data), `"password":{"xcl_masked":"redact","value":"(sensitive)"}`)
	require.NotContains(t, string(succeeded[0].Data), "s3cret")
}

// TestSchemaTypeSensitiveFieldKeptInStateWithoutMask asserts that with no
// state mask the state file holds the real sensitive value, which is what
// lets a later load restore it
func TestSchemaTypeSensitiveFieldKeptInStateWithoutMask(t *testing.T) {
	f := setupSchemaConfig(t, t.TempDir(), appWebConfig)

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	stateData, err := os.ReadFile(f.store.Path())
	require.NoError(t, err)
	require.Contains(t, string(stateData), "s3cret")
}

// TestSchemaTypeSensitiveFieldMaskedInStateWithMask asserts that with a
// state mask the state file never holds the sensitive value. The mask must be
// reversible so a later load can recover the value, so it is the AES masker
// rather than redact, which NewConfig refuses as a state mask
func TestSchemaTypeSensitiveFieldMaskedInStateWithMask(t *testing.T) {
	f := setupSchemaConfig(t, t.TempDir(), appWebConfig, WithStateMask(newAESStateMasker(t, stateMaskKey)))

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	stateData, err := os.ReadFile(f.store.Path())
	require.NoError(t, err)
	require.NotContains(t, string(stateData), "s3cret")
	require.Contains(t, string(stateData), "password")
}

// TestSchemaTypeLookupShowsSensitiveMarker asserts that the JSON form of a
// looked up resource shows the sensitive marker in place of the value
func TestSchemaTypeLookupShowsSensitiveMarker(t *testing.T) {
	f := setupSchemaConfig(t, t.TempDir(), appWebConfig)

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	r, err := f.config.FindResource("app.web")
	require.NoError(t, err)

	data, err := json.Marshal(r)
	require.NoError(t, err)

	require.Contains(t, string(data), `"password":"(sensitive)"`)
	require.NotContains(t, string(data), "s3cret")
}

// TestSchemaTypeComputedFieldCannotBeSet asserts that configuration that
// sets a computed attribute fails validation naming the field
func TestSchemaTypeComputedFieldCannotBeSet(t *testing.T) {
	f := setupSchemaConfig(t, t.TempDir(), `
app "web" {
  image = "nginx"
  endpoint = "http://web"
}
`)

	err := f.config.Validate(f.configFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "endpoint")
	require.Contains(t, err.Error(), "computed")

	err = f.config.Apply(f.configFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "endpoint")
	require.Contains(t, err.Error(), "computed")
}

// TestSchemaTypeVolumesListInOrder asserts that a repeated block decodes to a
// list in configuration order, with the label filled where it was given
func TestSchemaTypeVolumesListInOrder(t *testing.T) {
	f := setupSchemaConfig(t, t.TempDir(), appWebConfig)

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	web := findAsMap(t, f.config, "app.web")

	volumes, ok := web["volume"].([]any)
	require.True(t, ok, "volume should decode to a list")
	require.Len(t, volumes, 2)

	first, ok := volumes[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "data", first["name"])
	require.Equal(t, "/data", first["source"])

	second, ok := volumes[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "/logs", second["source"])
	require.NotContains(t, second, "name")
}

// TestSchemaTypeListenersMapByName asserts that a map of named blocks decodes
// to a map keyed by the block's label, with the label set on each element
func TestSchemaTypeListenersMapByName(t *testing.T) {
	f := setupSchemaConfig(t, t.TempDir(), appWebConfig)

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	web := findAsMap(t, f.config, "app.web")

	listeners, ok := web["listener"].(map[string]any)
	require.True(t, ok, "listener should decode to a map")
	require.Len(t, listeners, 2)

	http, ok := listeners["http"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "http", http["name"])
	require.Equal(t, float64(80), http["port"])

	https, ok := listeners["https"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "https", https["name"])
	require.Equal(t, float64(443), https["port"])
}

// TestSchemaTypeNestedBlockDecoded asserts that an optional nested block
// decodes with its attributes
func TestSchemaTypeNestedBlockDecoded(t *testing.T) {
	f := setupSchemaConfig(t, t.TempDir(), appWebConfig)

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	web := findAsMap(t, f.config, "app.web")

	health, ok := web["health"].(map[string]any)
	require.True(t, ok, "health should decode to an object")
	require.Equal(t, "/healthz", health["path"])
}

// TestSchemaTypeOptionalNestedBlockLeftOut asserts that an optional nested
// block left out of configuration is absent from the resource
func TestSchemaTypeOptionalNestedBlockLeftOut(t *testing.T) {
	f := setupSchemaConfig(t, t.TempDir(), `
app "web" {
  image = "nginx"
}
`)

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	web := findAsMap(t, f.config, "app.web")
	require.Nil(t, web["health"])
}

// TestSchemaTypeLoadsBackFromState asserts that a new Config with the same
// schema type declared loads the applied resource back from state with its
// attributes and nested collections intact
func TestSchemaTypeLoadsBackFromState(t *testing.T) {
	stateDir := t.TempDir()

	f := setupSchemaConfig(t, stateDir, appWebConfig)

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	local := newSchemaRegistry(t, appSchemaDocument)

	store, err := state.NewFileStateStore(stateDir)
	require.NoError(t, err)

	loaded, err := NewConfig(WithRegistry(local), WithStateStore(store))
	require.NoError(t, err)

	err = loaded.Load()
	require.NoError(t, err)

	web := findAsMap(t, loaded, "app.web")
	require.Equal(t, "nginx", web["image"])
	require.Equal(t, float64(2), web["replicas"])

	listeners, ok := web["listener"].(map[string]any)
	require.True(t, ok, "listener should load back as a map")
	require.Len(t, listeners, 2)

	http, ok := listeners["http"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "http", http["name"])
	require.Equal(t, float64(80), http["port"])

	https, ok := listeners["https"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "https", https["name"])
	require.Equal(t, float64(443), https["port"])

	volumes, ok := web["volume"].([]any)
	require.True(t, ok, "volume should load back as a list")
	require.Len(t, volumes, 2)
}

// TestPluginLegacySchemaStillLoads asserts that a plugin whose types reach
// the catalog through the legacy, unversioned schema still loads and applies
func TestPluginLegacySchemaStillLoads(t *testing.T) {
	f := setupDeliveryConfig(t, independentNetworksConfig)

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	r, err := f.config.FindResource("resource.network.one")
	require.NoError(t, err)
	require.NotNil(t, r)

	require.Len(t, f.plugin.GetCreatedResources(), 3)
}
