package xcl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	hcl "github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
	"github.com/jumppad-labs/xcl/internal/xcl/hclwrite"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/stretchr/testify/require"
)

// the ids of the entities the encode fixture declares, each covering one shape
// the conversion to configuration text has to handle
const (
	encodeDatabaseID  = "resource.database.main"
	encodeCacheID     = "cache.main"
	encodeNetworkID   = "resource.network.main"
	encodeContainerID = "resource.container.web"
	encodeVariableID  = "variable.region"
)

// applyEncodeFixture applies the encode fixture with a file state store and
// returns the applied configuration, the registry it was applied with and the
// path the state was written to, so a test can reach both a live entity and
// the saved record for the same thing
func applyEncodeFixture(t *testing.T) (*Config, *registry.PluginRegistry, string) {
	t.Helper()

	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())

	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	pr := encodeRegistry(t)

	statePath := filepath.Join(t.TempDir(), "state.json")

	store, err := state.NewFileStateStore(statePath, pr)
	require.NoError(t, err)

	c := NewConfig(
		WithPluginRegistry(pr),
		WithStateStore(store),
	)

	path, err := filepath.Abs("./internal/test_fixtures/config/encode/main.xcl")
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	return c, pr, statePath
}

// encodeRegistry returns a registry holding every type the encode fixture
// declares: a kind led registered type, a bare registered type and the plugin
// that provides the network and container types
func encodeRegistry(t *testing.T) *registry.PluginRegistry {
	t.Helper()

	pr := registry.NewPluginRegistry()

	err := pr.RegisterType(registered.TypeDatabase, &registered.Database{})
	require.NoError(t, err)

	err = pr.RegisterBareType(registered.TypeCache, &registered.Cache{})
	require.NoError(t, err)

	err = pr.RegisterPlugin(&parser.TestPlugin{})
	require.NoError(t, err)

	return pr
}

// encodeEntityByID returns the applied entity the configuration holds under id
func encodeEntityByID(t *testing.T, c *Config, id string) any {
	t.Helper()

	entity, err := entityByID(c.Entities(), id)
	require.NoError(t, err)
	require.NotNil(t, entity)

	return entity
}

// encodeSavedRecordByID returns the raw saved record the state file holds for
// id, exactly as state wrote it and as an event carries it
func encodeSavedRecordByID(t *testing.T, statePath, id string) json.RawMessage {
	t.Helper()

	data, err := os.ReadFile(statePath)
	require.NoError(t, err)

	records := []json.RawMessage{}
	err = json.Unmarshal(data, &records)
	require.NoError(t, err)

	for _, record := range records {
		meta := struct {
			Meta struct {
				ID string `json:"id"`
			} `json:"meta"`
		}{}

		err = json.Unmarshal(record, &meta)
		require.NoError(t, err)

		if meta.Meta.ID == id {
			return record
		}
	}

	require.Failf(t, "saved record not found", "the state at %s holds no record with the id %q", statePath, id)

	return nil
}

func TestEncodeEntityWritesResourceHeaderAndValues(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	database := encodeEntityByID(t, c, encodeDatabaseID)

	out, err := EncodeEntity(database)
	require.NoError(t, err)

	text := string(out)

	require.True(t, strings.HasPrefix(text, "resource \"database\" \"main\" {"), "expected the text to begin with the resource header, got:\n%s", text)
	// the formatter aligns the = signs against the longest name in the block,
	// so match the padding rather than baking in today's width
	require.Regexp(t, `port\s+= 5432`, text)
	require.Contains(t, text, "timeouts {")
	require.Contains(t, text, "connect = 30")
}

func TestEncodeSavedEntityMatchesEncodeEntity(t *testing.T) {
	c, reg, statePath := applyEncodeFixture(t)

	database := encodeEntityByID(t, c, encodeDatabaseID)

	fromEntity, err := EncodeEntity(database)
	require.NoError(t, err)

	record := encodeSavedRecordByID(t, statePath, encodeDatabaseID)

	fromSaved, err := EncodeSavedEntity(reg, record)
	require.NoError(t, err)

	require.Equal(t, fromEntity, fromSaved)
}

func TestEncodeEntityUsesConfigurationNames(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	out, err := EncodeEntity(container)
	require.NoError(t, err)

	text := string(out)

	require.Contains(t, text, "ip_address")
	require.NotContains(t, text, "IPAddress")
}

func TestEncodeEntityWritesRepeatedBlocks(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	out, err := EncodeEntity(container)
	require.NoError(t, err)

	text := string(out)

	require.Equal(t, 2, strings.Count(text, "network {"), "expected two network blocks, got:\n%s", text)
	require.Contains(t, text, "10.0.0.10")
	require.Contains(t, text, "10.0.0.11")
}

func TestEncodeEntityWritesBareHeader(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	cache := encodeEntityByID(t, c, encodeCacheID)

	out, err := EncodeEntity(cache)
	require.NoError(t, err)

	expected := `cache "main" {
  location = "us-east"
}
`

	require.Equal(t, expected, string(out))
	require.NotContains(t, string(out), "resource")
}

func TestEncodeEntityOmitsComputedByDefault(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	network := encodeEntityByID(t, c, encodeNetworkID)

	out, err := EncodeEntity(network)
	require.NoError(t, err)

	expected := `resource "network" "main" {
  subnet = "10.0.0.0/16"
}
`

	require.Equal(t, expected, string(out))
	require.NotContains(t, string(out), "provider_id")
}

func TestEncodeEntityIncludesComputedWhenAsked(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	network := encodeEntityByID(t, c, encodeNetworkID)

	out, err := EncodeEntity(network, IncludeComputed())
	require.NoError(t, err)

	require.Contains(t, string(out), `provider_id = "id-main"`)
}

func TestEncodeEntityOmitsDependsOn(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	out, err := EncodeEntity(container)
	require.NoError(t, err)

	// by the time an entity is parsed depends_on holds the references xcl
	// resolved as well as anything the author wrote, so it is never written
	require.NotContains(t, string(out), "depends_on")
}

func TestEncodeEntityOmitsBookkeepingInsideObjectAttribute(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	out, err := EncodeEntity(container)
	require.NoError(t, err)

	text := string(out)

	require.Contains(t, text, "networkobj = {")
	require.Contains(t, text, "subnet")

	// the object holds a copy of a network, whose base carries the bookkeeping
	// every entity has and which nobody wrote here
	require.NotContains(t, text, "meta")
	require.NotContains(t, text, "disabled")
	require.NotContains(t, text, "depends_on")
}

func TestEncodeEntityMarksComputedValuesWithAComment(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	network := encodeEntityByID(t, c, encodeNetworkID)

	out, err := EncodeEntity(network, IncludeComputed())
	require.NoError(t, err)

	text := string(out)

	require.Contains(t, text, `provider_id = "id-main" # set by the provider`)

	// the configured value is not marked, it is what the author wrote
	require.Regexp(t, `subnet\s+= "10.0.0.0/16"\n`, text)
}

func TestEncodeEntityWritesNoCommentsByDefault(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	network := encodeEntityByID(t, c, encodeNetworkID)

	out, err := EncodeEntity(network)
	require.NoError(t, err)

	require.NotContains(t, string(out), "#")
}

func TestEncodeEntityIsDeterministic(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	first, err := EncodeEntity(container)
	require.NoError(t, err)

	second, err := EncodeEntity(container)
	require.NoError(t, err)

	require.Equal(t, first, second)
}

func TestEncodeEntityIsFormatterStable(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	out, err := EncodeEntity(container)
	require.NoError(t, err)

	require.Equal(t, out, hclwrite.Format(out))
}

func TestEncodeEntityConvertsInProcessPluginType(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	out, err := EncodeEntity(container)
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(string(out), "resource \"container\" \"web\" {"), "expected the container header, got:\n%s", string(out))
}

func TestEncodeEntityConvertsKeywordRegisteredType(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	database := encodeEntityByID(t, c, encodeDatabaseID)

	out, err := EncodeEntity(database)
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(string(out), "resource \"database\" \"main\" {"), "expected the database header, got:\n%s", string(out))
}

func TestEncodeEntityConvertsBareRegisteredType(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	cache := encodeEntityByID(t, c, encodeCacheID)

	out, err := EncodeEntity(cache)
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(string(out), "cache \"main\" {"), "expected the cache header, got:\n%s", string(out))
}

func TestEncodeSavedEntityFailsForUnregisteredType(t *testing.T) {
	_, reg, _ := applyEncodeFixture(t)

	record := []byte(`{"meta":{"id":"resource.unknown.thing","type":"resource","subtype":"unknown","name":"thing"}}`)

	out, err := EncodeSavedEntity(reg, record)

	require.Nil(t, out)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrUnregisteredType)

	unregistered := &UnregisteredTypeError{}
	require.ErrorAs(t, err, &unregistered)
	require.Equal(t, "unknown", unregistered.Type)
}

func TestEncodeSavedEntityFailsForInvalidData(t *testing.T) {
	_, reg, _ := applyEncodeFixture(t)

	out, err := EncodeSavedEntity(reg, []byte("this is not a saved entity record"))

	require.Nil(t, out)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidSavedData)
}

func TestEncodeEntityRefusesBuiltin(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	variable := encodeEntityByID(t, c, encodeVariableID)

	out, err := EncodeEntity(variable)

	require.Nil(t, out)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotEncodable)
	require.Contains(t, err.Error(), "variable")
}

func TestEncodeEntityWritesOneBlockPerCall(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	database := encodeEntityByID(t, c, encodeDatabaseID)
	cache := encodeEntityByID(t, c, encodeCacheID)
	container := encodeEntityByID(t, c, encodeContainerID)

	databaseText, err := EncodeEntity(database)
	require.NoError(t, err)
	require.Equal(t, 1, encodeTopLevelBlockCount(t, databaseText))

	cacheText, err := EncodeEntity(cache)
	require.NoError(t, err)
	require.Equal(t, 1, encodeTopLevelBlockCount(t, cacheText))

	containerText, err := EncodeEntity(container)
	require.NoError(t, err)
	require.Equal(t, 1, encodeTopLevelBlockCount(t, containerText))
}

// encodeTopLevelBlockCount parses text as configuration and returns how many
// blocks it declares at the top level
func encodeTopLevelBlockCount(t *testing.T, text []byte) int {
	t.Helper()

	file, diags := hclsyntax.ParseConfig(text, "encoded.xcl", hcl.InitialPos)
	require.False(t, diags.HasErrors(), "expected the text to parse, got %s for:\n%s", diags.Error(), string(text))

	body, ok := file.Body.(*hclsyntax.Body)
	require.True(t, ok, "expected an hclsyntax body")

	return len(body.Blocks)
}

func TestEncodeSavedEntityLoadsUnloadedRegistry(t *testing.T) {
	_, _, statePath := applyEncodeFixture(t)

	record := encodeSavedRecordByID(t, statePath, encodeContainerID)

	// a registry no configuration has ever used, so the plugin's types resolve
	// only because EncodeSavedEntity loads it itself
	fresh := encodeRegistry(t)

	out, err := EncodeSavedEntity(fresh, record)
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(string(out), "resource \"container\" \"web\" {"), "expected the container header, got:\n%s", string(out))
}
