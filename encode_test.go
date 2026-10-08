package xcl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/internal/testutil"
	hcl "github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
	"github.com/jumppad-labs/xcl/internal/xcl/hclwrite"
	"github.com/jumppad-labs/xcl/registry"
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
	// writes depends_on naming the database and references the network
	encodeDependentContainerID = "resource.container.api"
	encodeVariableID           = "variable.region"
)

// applyEncodeFixture applies the encode fixture with a file state store and
// returns the applied configuration and the path the state was written to, so
// a test can reach both a live entity and the saved record for the same thing
func applyEncodeFixture(t *testing.T) (*Config, string) {
	t.Helper()

	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())

	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	statePath := store.Path()

	options := append(encodeFixtureOptions(), WithStateStore(store))

	c, err := NewConfig(options...)
	require.NoError(t, err)

	path, err := filepath.Abs("./internal/test_fixtures/config/encode/main.xcl")
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	return c, statePath
}

// encodeFixtureOptions returns the options declaring every type the encode
// fixture uses, on one local registry: a type declared under resource, one
// declared without a subtype and the plugin that provides the network and
// container types. Each call returns a new registry and plugin.
func encodeFixtureOptions() []ConfigOption {
	local := registry.NewLocal()
	local.RegisterPlugin(&parser.TestPlugin{})
	local.RegisterType(&registered.Database{}, "resource", registered.TypeDatabase)
	local.RegisterType(&registered.Cache{}, registered.TypeCache)

	return []ConfigOption{
		WithRegistry(local),
	}
}

// encodeEntityByID returns the applied entity the configuration holds under id
func encodeEntityByID(t *testing.T, c *Config, id string) any {
	t.Helper()

	entity, err := testutil.EntityByID(c.Entities(), id)
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
	c, _ := applyEncodeFixture(t)

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
	c, statePath := applyEncodeFixture(t)

	database := encodeEntityByID(t, c, encodeDatabaseID)

	fromEntity, err := EncodeEntity(database)
	require.NoError(t, err)

	record := encodeSavedRecordByID(t, statePath, encodeDatabaseID)

	fromSaved, err := c.EncodeSavedEntity(record)
	require.NoError(t, err)

	require.Equal(t, fromEntity, fromSaved)
}

func TestEncodeEntityUsesConfigurationNames(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	out, err := EncodeEntity(container)
	require.NoError(t, err)

	text := string(out)

	require.Contains(t, text, "ip_address")
	require.NotContains(t, text, "IPAddress")
}

func TestEncodeEntityWritesRepeatedBlocks(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	out, err := EncodeEntity(container)
	require.NoError(t, err)

	text := string(out)

	require.Equal(t, 2, strings.Count(text, "network {"), "expected two network blocks, got:\n%s", text)
	require.Contains(t, text, "10.0.0.10")
	require.Contains(t, text, "10.0.0.11")
}

func TestEncodeEntityWritesBareHeader(t *testing.T) {
	c, _ := applyEncodeFixture(t)

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
	c, _ := applyEncodeFixture(t)

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
	c, _ := applyEncodeFixture(t)

	network := encodeEntityByID(t, c, encodeNetworkID)

	out, err := EncodeEntity(network, IncludeComputed())
	require.NoError(t, err)

	require.Contains(t, string(out), `provider_id = "id-main"`)
}

func TestEncodeEntityWritesWrittenDependsOn(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	api := encodeEntityByID(t, c, encodeDependentContainerID)

	out, err := EncodeEntity(api)
	require.NoError(t, err)

	text := string(out)

	require.Regexp(t, `depends_on\s+= \["resource\.database\.main"\]`, text)

	// the container also references the network, a dependency xcl works out
	// for itself and keeps out of what the author wrote
	dependsOnLine := encodeLineContaining(t, text, "depends_on")
	require.NotContains(t, dependsOnLine, encodeNetworkID)
}

func TestEncodeEntityOmitsDependsOnWhenNoneWritten(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	out, err := EncodeEntity(container)
	require.NoError(t, err)

	text := string(out)

	// the container references the network, so it depends on it, but the
	// author wrote no depends_on and the text shows none
	require.Contains(t, text, "network {")
	require.NotContains(t, text, "depends_on")
}

func TestEncodeSavedEntityMatchesLiveWithDependsOn(t *testing.T) {
	c, statePath := applyEncodeFixture(t)

	api := encodeEntityByID(t, c, encodeDependentContainerID)

	fromEntity, err := EncodeEntity(api)
	require.NoError(t, err)

	record := encodeSavedRecordByID(t, statePath, encodeDependentContainerID)

	fromSaved, err := c.EncodeSavedEntity(record)
	require.NoError(t, err)

	require.Equal(t, string(fromEntity), string(fromSaved))
	require.Contains(t, string(fromSaved), "depends_on")
}

// encodeLineContaining returns the first line of text holding part, and fails
// the test when no line holds it
func encodeLineContaining(t *testing.T, text, part string) string {
	t.Helper()

	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, part) {
			return line
		}
	}

	require.Failf(t, "no line found", "no line contains %q in:\n%s", part, text)

	return ""
}

func TestEncodeEntityOmitsBookkeepingInsideObjectAttribute(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	// the fixture leaves networkobj unset, IncludeEmpty writes it as a zero
	// object so there is an object attribute to look inside
	out, err := EncodeEntity(container, IncludeEmpty())
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
	c, _ := applyEncodeFixture(t)

	network := encodeEntityByID(t, c, encodeNetworkID)

	out, err := EncodeEntity(network, IncludeComputed())
	require.NoError(t, err)

	text := string(out)

	require.Contains(t, text, `provider_id = "id-main" # set by the provider`)

	// the configured value is not marked, it is what the author wrote
	require.Regexp(t, `subnet\s+= "10.0.0.0/16"\n`, text)
}

func TestEncodeEntityWritesNoCommentsByDefault(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	network := encodeEntityByID(t, c, encodeNetworkID)

	out, err := EncodeEntity(network)
	require.NoError(t, err)

	require.NotContains(t, string(out), "#")
}

func TestEncodeEntityIsDeterministic(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	first, err := EncodeEntity(container)
	require.NoError(t, err)

	second, err := EncodeEntity(container)
	require.NoError(t, err)

	require.Equal(t, first, second)
}

func TestEncodeEntityIsFormatterStable(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	out, err := EncodeEntity(container)
	require.NoError(t, err)

	require.Equal(t, out, hclwrite.Format(out))
}

func TestEncodeEntityConvertsInProcessPluginType(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	out, err := EncodeEntity(container)
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(string(out), "resource \"container\" \"web\" {"), "expected the container header, got:\n%s", string(out))
}

func TestEncodeEntityConvertsKeywordRegisteredType(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	database := encodeEntityByID(t, c, encodeDatabaseID)

	out, err := EncodeEntity(database)
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(string(out), "resource \"database\" \"main\" {"), "expected the database header, got:\n%s", string(out))
}

func TestEncodeEntityConvertsBareRegisteredType(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	cache := encodeEntityByID(t, c, encodeCacheID)

	out, err := EncodeEntity(cache)
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(string(out), "cache \"main\" {"), "expected the cache header, got:\n%s", string(out))
}

func TestEncodeSavedEntityFailsForUnregisteredType(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	record := []byte(`{"meta":{"id":"resource.unknown.thing","type":"resource","subtype":"unknown","name":"thing"}}`)

	out, err := c.EncodeSavedEntity(record)

	require.Nil(t, out)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrUnregisteredType)

	unregistered := &UnregisteredTypeError{}
	require.ErrorAs(t, err, &unregistered)
	require.Equal(t, "resource.unknown", unregistered.Type)
}

func TestEncodeSavedEntityFailsForInvalidData(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	out, err := c.EncodeSavedEntity([]byte("this is not a saved entity record"))

	require.Nil(t, out)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidSavedData)
}

func TestEncodeEntityRefusesBuiltin(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	variable := encodeEntityByID(t, c, encodeVariableID)

	out, err := EncodeEntity(variable)

	require.Nil(t, out)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotEncodable)
	require.Contains(t, err.Error(), "variable")
}

func TestEncodeEntityWritesOneBlockPerCall(t *testing.T) {
	c, _ := applyEncodeFixture(t)

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

func TestConfigEncodeSavedEntityLoadsPluginsFirst(t *testing.T) {
	_, statePath := applyEncodeFixture(t)

	record := encodeSavedRecordByID(t, statePath, encodeContainerID)

	// a configuration that has never run an operation, so the plugin's types
	// resolve only because EncodeSavedEntity loads the plugins itself
	fresh, err := NewConfig(encodeFixtureOptions()...)
	require.NoError(t, err)

	out, err := fresh.EncodeSavedEntity(record)
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(string(out), "resource \"container\" \"web\" {"), "expected the container header, got:\n%s", string(out))
}

func TestEncodeEntityOmitsOptionalZeroAttributes(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	out, err := EncodeEntity(container)
	require.NoError(t, err)

	// each network block leaves its optional id at zero
	require.NotRegexp(t, `(?m)^\s+id\s+= 0$`, string(out))
}

func TestEncodeEntityIncludeEmptyWritesOptionalZeroAttributes(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	out, err := EncodeEntity(container, IncludeEmpty())
	require.NoError(t, err)

	require.Regexp(t, `(?m)^\s+id\s+= 0$`, string(out))
}
