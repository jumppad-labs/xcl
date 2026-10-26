package coreservice

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	"github.com/jumppad-labs/xcl"
	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
	"github.com/jumppad-labs/xcl/state"
)

// entities is Entities, returning each entity's JSON
func (c *testClient) entities(handle uint64) [][]byte {
	c.t.Helper()

	resp := &corepb.EntitiesResponse{}
	c.ok(handle, "Entities", &corepb.EntitiesRequest{}, resp)

	return resp.GetEntities()
}

// appRecord is the saved record of app.web, taken from Entities
func (c *testClient) appRecord(handle uint64) []byte {
	c.t.Helper()

	for _, entity := range c.entities(handle) {
		if strings.Contains(string(entity), `"app.web"`) {
			return entity
		}
	}

	require.FailNow(c.t, "app.web is not among the entities")

	return nil
}

// isJSONObject reports whether data is one JSON object
func isJSONObject(data []byte) bool {
	var object map[string]any

	return json.Unmarshal(data, &object) == nil
}

func TestEntitiesReturnsEveryEntityAsJSON(t *testing.T) {
	c, handle := appliedApp(t)

	entities := c.entities(handle)

	// the variable, the app and the output
	require.Len(t, entities, 3)

	joined := ""
	for _, entity := range entities {
		require.True(t, isJSONObject(entity), "entity is not a JSON object: %s", entity)
		joined += string(entity)
	}

	require.Contains(t, joined, `"variable.region"`)
	require.Contains(t, joined, `"app.web"`)
	require.Contains(t, joined, `"output.where"`)
}

func TestFindReturnsEntityJSON(t *testing.T) {
	c, handle := appliedApp(t)

	entity := c.find(handle, "app.web")

	require.True(t, isJSONObject(entity))
	require.Contains(t, string(entity), `"image":"nginx"`)
	require.Contains(t, string(entity), `"app.web"`)
}

func TestFindUnknownAddressIsNotFound(t *testing.T) {
	c, handle := appliedApp(t)

	out := c.fail(handle, "Find", &corepb.FindRequest{Address: "app.nope"})

	require.Equal(t, CodeNotFound, out.GetCode())
	require.Equal(t, "app.nope", out.GetDetails()["address"])
}

func TestFindUnknownAddressHasNoReport(t *testing.T) {
	c, handle := appliedApp(t)

	out := c.fail(handle, "Find", &corepb.FindRequest{Address: "app.nope"})

	require.Equal(t, CodeNotFound, out.GetCode())
	require.Empty(t, out.GetReport())
	require.Equal(t, (&xcl.NotFoundError{Address: "app.nope"}).Error(), out.GetMessage())
}

func TestFindEmptyAddressIsInvalidArgument(t *testing.T) {
	c, handle := appliedApp(t)

	out := c.fail(handle, "Find", &corepb.FindRequest{})

	require.Equal(t, CodeInvalidArgument, out.GetCode())
	require.Equal(t, "address", out.GetDetails()["argument"])
}

func TestFindByTypeReturnsEntities(t *testing.T) {
	c, handle := appliedApp(t)

	resp := &corepb.EntitiesResponse{}
	c.ok(handle, "FindByType", &corepb.FindByTypeRequest{Path: []string{"app"}}, resp)

	require.Len(t, resp.GetEntities(), 1)
	require.Contains(t, string(resp.GetEntities()[0]), `"app.web"`)
}

func TestFindByTypeUnknownTypeIsUnknownType(t *testing.T) {
	c, handle := appliedApp(t)

	out := c.fail(handle, "FindByType", &corepb.FindByTypeRequest{Path: []string{"nope"}})

	require.Equal(t, CodeUnknownType, out.GetCode())
	require.Equal(t, "nope", out.GetDetails()["name"])
}

func TestFindOneReturnsEntity(t *testing.T) {
	c, handle := appliedApp(t)

	resp := &corepb.EntityResponse{}
	c.ok(handle, "FindOne", &corepb.FindByTypeRequest{Path: []string{"app"}}, resp)

	require.Contains(t, string(resp.GetEntity()), `"app.web"`)
	require.Contains(t, string(resp.GetEntity()), `"image":"nginx"`)
}

func TestLookupsReturnSensitiveMarker(t *testing.T) {
	c, handle := appliedApp(t)

	entity := c.find(handle, "app.web")
	require.Contains(t, string(entity), `"password":"(sensitive)"`)
	require.NotContains(t, string(entity), "s3cret")

	for _, entity := range c.entities(handle) {
		require.NotContains(t, string(entity), "s3cret")
	}
}

func TestOutputsReturnJSONObject(t *testing.T) {
	c, handle := appliedApp(t)

	resp := &corepb.OutputsResponse{}
	c.ok(handle, "Outputs", &corepb.OutputsRequest{}, resp)

	require.JSONEq(t, `{"output.where":"us"}`, string(resp.GetOutputsJson()))
}

func TestOutputsEmptyIsEmptyObject(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(appRequest(t))

	resp := &corepb.OutputsResponse{}
	c.ok(handle, "Outputs", &corepb.OutputsRequest{}, resp)

	require.JSONEq(t, `{}`, string(resp.GetOutputsJson()))
}

func TestEncodeSavedEntityRendersText(t *testing.T) {
	c, handle := appliedApp(t)

	resp := &corepb.EncodeSavedEntityResponse{}
	c.ok(handle, "EncodeSavedEntity", &corepb.EncodeSavedEntityRequest{Entity: c.appRecord(handle)}, resp)

	require.Contains(t, string(resp.GetText()), `app "web"`)
	require.Contains(t, string(resp.GetText()), `image`)
	require.NotContains(t, string(resp.GetText()), "\x1b[")
}

func TestEncodeSavedEntityEmptyIsInvalidArgument(t *testing.T) {
	c, handle := appliedApp(t)

	out := c.fail(handle, "EncodeSavedEntity", &corepb.EncodeSavedEntityRequest{})

	require.Equal(t, CodeInvalidArgument, out.GetCode())
	require.Equal(t, "entity", out.GetDetails()["argument"])
}

// encodeEntity is EncodeEntity, returning the text
func (c *testClient) encodeEntity(handle uint64, req *corepb.EncodeEntityRequest) string {
	c.t.Helper()

	resp := &corepb.EncodeEntityResponse{}
	c.ok(handle, "EncodeEntity", req, resp)

	return string(resp.GetText())
}

// encodeSavedEntity is EncodeSavedEntity, returning the text
func (c *testClient) encodeSavedEntity(handle uint64, req *corepb.EncodeSavedEntityRequest) string {
	c.t.Helper()

	resp := &corepb.EncodeSavedEntityResponse{}
	c.ok(handle, "EncodeSavedEntity", req, resp)

	return string(resp.GetText())
}

// appliedHostApp returns a client holding a configuration with a host state
// store to which appConfig has been applied, its handle and the store holding
// the records the apply saved, where app.web's password is the real value
func appliedHostApp(t *testing.T) (*testClient, uint64, *fakeStore) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(hostRequest(t))

	store := &fakeStore{}
	opID := c.start(handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, appConfig)}})
	require.Nil(t, c.drive(handle, opID, store, nil).GetError())

	return c, handle, store
}

// appliedChain returns a client holding a TestPlugin configuration to which
// chainConfig has been applied through the service, so network.first holds
// the provider_id its provider computed and container.second holds a
// reference to the network
func appliedChain(t *testing.T) (*testClient, *pluginFixture) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	f := newPluginConfig(t, c)

	opID := c.start(f.handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, chainConfig)}})
	require.Nil(t, c.drive(f.handle, opID, nil, nil).GetError())

	return c, f
}

// savedRecord is the record of the entity id as the file state store under
// dir holds it
func savedRecord(t *testing.T, dir, id string) []byte {
	t.Helper()

	store, err := state.NewFileStateStore(dir)
	require.NoError(t, err)

	records, err := store.Load()
	require.NoError(t, err)

	saved := [][]byte{}
	for _, record := range records {
		raw, ok := record.(json.RawMessage)
		require.True(t, ok, "the file store loads raw records, got %T", record)

		saved = append(saved, raw)
	}

	return recordFor(t, saved, id)
}

func TestEncodeEntityRendersText(t *testing.T) {
	c, handle := appliedApp(t)

	text := c.encodeEntity(handle, &corepb.EncodeEntityRequest{Entity: c.find(handle, "app.web")})

	expected := `app "web" {
  image    = "nginx"
  password = "(sensitive)"
}
`
	require.Equal(t, expected, text)
}

func TestEncodeEntityRevealShowsConfiguredSecret(t *testing.T) {
	c, handle := appliedApp(t)

	// the looked-up record holds only the marker
	record := c.find(handle, "app.web")
	require.Contains(t, string(record), `"password":"(sensitive)"`)

	text := c.encodeEntity(handle, &corepb.EncodeEntityRequest{
		Entity:  record,
		Options: &corepb.EncodeOptions{RevealSensitive: true},
	})

	expected := `app "web" {
  image    = "nginx"
  password = "s3cret"
}
`
	require.Equal(t, expected, text)
}

func TestEncodeEntityWithoutRevealShowsMarker(t *testing.T) {
	c, handle := appliedApp(t)

	text := c.encodeEntity(handle, &corepb.EncodeEntityRequest{Entity: c.find(handle, "app.web")})

	require.Contains(t, text, `password = "(sensitive)"`)
	require.NotContains(t, text, "s3cret")
}

func TestEncodeEntityIncludeComputed(t *testing.T) {
	c, f := appliedChain(t)

	text := c.encodeEntity(f.handle, &corepb.EncodeEntityRequest{
		Entity:  c.find(f.handle, "resource.network.first"),
		Options: &corepb.EncodeOptions{IncludeComputed: true},
	})

	expected := `resource "network" "first" {
  subnet      = "10.0.0.0/16"
  provider_id = "id-first" # set by the provider
}
`
	require.Equal(t, expected, text)
}

func TestEncodeEntityWithoutIncludeComputedLeavesComputedOut(t *testing.T) {
	c, f := appliedChain(t)

	text := c.encodeEntity(f.handle, &corepb.EncodeEntityRequest{Entity: c.find(f.handle, "resource.network.first")})

	expected := `resource "network" "first" {
  subnet = "10.0.0.0/16"
}
`
	require.Equal(t, expected, text)
}

func TestEncodeEntityIncludeEmpty(t *testing.T) {
	c, f := appliedChain(t)

	text := c.encodeEntity(f.handle, &corepb.EncodeEntityRequest{
		Entity:  c.find(f.handle, "resource.container.second"),
		Options: &corepb.EncodeOptions{IncludeEmpty: true},
	})

	expected := `resource "container" "second" {
  default = "hello world"

  network {
    id         = 0
    name       = "first"
    ip_address = ""
    aliases    = []
  }

  networkobj = {
    subnet = ""
  }
  entrypoint          = []
  command             = []
  env                 = {}
  dns                 = []
  privileged          = false
  max_restart_count   = 0
  created_network     = []
  created_network_map = {}
  container_id        = ""
}
`
	require.Equal(t, expected, text)
}

func TestEncodeEntityShowReferences(t *testing.T) {
	c, f := appliedChain(t)

	text := c.encodeEntity(f.handle, &corepb.EncodeEntityRequest{
		Entity:  c.find(f.handle, "resource.container.second"),
		Options: &corepb.EncodeOptions{ShowReferences: true},
	})

	expected := `resource "container" "second" {
  default = "hello world"

  network {
    name = resource.network.first.meta.name
  }
}
`
	require.Equal(t, expected, text)
}

func TestEncodeEntityWithoutShowReferencesShowsResolvedValue(t *testing.T) {
	c, f := appliedChain(t)

	text := c.encodeEntity(f.handle, &corepb.EncodeEntityRequest{Entity: c.find(f.handle, "resource.container.second")})

	expected := `resource "container" "second" {
  default = "hello world"

  network {
    name = "first"
  }
}
`
	require.Equal(t, expected, text)
}

func TestEncodeEntityWithoutOptionsUnchanged(t *testing.T) {
	c, handle := appliedApp(t)

	record := c.find(handle, "app.web")

	withoutOptions := c.encodeEntity(handle, &corepb.EncodeEntityRequest{Entity: record})
	withEmptyOptions := c.encodeEntity(handle, &corepb.EncodeEntityRequest{Entity: record, Options: &corepb.EncodeOptions{}})

	require.Equal(t, withEmptyOptions, withoutOptions)
}

func TestEncodeEntityEmptyIsInvalidArgument(t *testing.T) {
	c, handle := appliedApp(t)

	out := c.fail(handle, "EncodeEntity", &corepb.EncodeEntityRequest{})

	require.Equal(t, CodeInvalidArgument, out.GetCode())
	require.Equal(t, "entity", out.GetDetails()["argument"])
}

func TestEncodeEntityVariableIsNotEncodable(t *testing.T) {
	c, handle := appliedApp(t)

	out := c.fail(handle, "EncodeEntity", &corepb.EncodeEntityRequest{Entity: c.find(handle, "variable.region")})

	require.Equal(t, CodeNotEncodable, out.GetCode())
	require.Equal(t, "variable.region", out.GetDetails()["what"])
}

func TestEncodeEntityUnregisteredType(t *testing.T) {
	c, handle := appliedApp(t)

	record := []byte(`{"meta":{"id":"bogus.x","name":"x","type":"bogus"},"image":"nginx"}`)

	out := c.fail(handle, "EncodeEntity", &corepb.EncodeEntityRequest{Entity: record})

	require.Equal(t, CodeUnregisteredType, out.GetCode())
	require.Equal(t, "bogus", out.GetDetails()["type"])
}

func TestEncodeEntityWithoutMetaIsInvalidSavedData(t *testing.T) {
	c, handle := appliedApp(t)

	out := c.fail(handle, "EncodeEntity", &corepb.EncodeEntityRequest{Entity: []byte(`{"image":"nginx"}`)})

	require.Equal(t, CodeInvalidSavedData, out.GetCode())
}

func TestEncodeSavedEntityVariableIsNotEncodable(t *testing.T) {
	c, handle := appliedApp(t)

	out := c.fail(handle, "EncodeSavedEntity", &corepb.EncodeSavedEntityRequest{Entity: c.find(handle, "variable.region")})

	require.Equal(t, CodeNotEncodable, out.GetCode())
	require.Equal(t, "variable.region", out.GetDetails()["what"])
}

func TestEncodeSavedEntityUnregisteredType(t *testing.T) {
	c, handle := appliedApp(t)

	record := []byte(`{"meta":{"id":"bogus.x","name":"x","type":"bogus"},"image":"nginx"}`)

	out := c.fail(handle, "EncodeSavedEntity", &corepb.EncodeSavedEntityRequest{Entity: record})

	require.Equal(t, CodeUnregisteredType, out.GetCode())
	require.Equal(t, "bogus", out.GetDetails()["type"])
}

func TestEncodeSavedEntityWithoutMetaIsInvalidSavedData(t *testing.T) {
	c, handle := appliedApp(t)

	out := c.fail(handle, "EncodeSavedEntity", &corepb.EncodeSavedEntityRequest{Entity: []byte(`{"image":"nginx"}`)})

	require.Equal(t, CodeInvalidSavedData, out.GetCode())
}

func TestEncodeSavedEntityIncludeComputed(t *testing.T) {
	c, f := appliedChain(t)

	options := &corepb.EncodeOptions{IncludeComputed: true}

	fromSaved := c.encodeSavedEntity(f.handle, &corepb.EncodeSavedEntityRequest{
		Entity:  savedRecord(t, f.stateDir, "resource.network.first"),
		Options: options,
	})

	expected := `resource "network" "first" {
  subnet      = "10.0.0.0/16"
  provider_id = "id-first" # set by the provider
}
`
	require.Equal(t, expected, fromSaved)

	fromEntity := c.encodeEntity(f.handle, &corepb.EncodeEntityRequest{
		Entity:  c.find(f.handle, "resource.network.first"),
		Options: options,
	})
	require.Equal(t, fromSaved, fromEntity)
}

func TestEncodeSavedEntityIncludeEmpty(t *testing.T) {
	c, f := appliedChain(t)

	options := &corepb.EncodeOptions{IncludeEmpty: true}

	fromSaved := c.encodeSavedEntity(f.handle, &corepb.EncodeSavedEntityRequest{
		Entity:  savedRecord(t, f.stateDir, "resource.container.second"),
		Options: options,
	})

	// the attributes the configuration leaves unset are written at their
	// zero value
	require.Contains(t, fromSaved, `container_id        = ""`)
	require.Contains(t, fromSaved, `privileged          = false`)
	require.Contains(t, fromSaved, `entrypoint          = []`)
	require.Contains(t, fromSaved, `env                 = {}`)

	fromEntity := c.encodeEntity(f.handle, &corepb.EncodeEntityRequest{
		Entity:  c.find(f.handle, "resource.container.second"),
		Options: options,
	})
	require.Equal(t, fromSaved, fromEntity)
}

func TestEncodeSavedEntityRevealSensitive(t *testing.T) {
	c, handle, store := appliedHostApp(t)

	options := &corepb.EncodeOptions{RevealSensitive: true}

	fromSaved := c.encodeSavedEntity(handle, &corepb.EncodeSavedEntityRequest{
		Entity:  recordFor(t, store.saved, "app.web"),
		Options: options,
	})

	expected := `app "web" {
  image    = "nginx"
  password = "s3cret"
}
`
	require.Equal(t, expected, fromSaved)

	fromEntity := c.encodeEntity(handle, &corepb.EncodeEntityRequest{
		Entity:  c.find(handle, "app.web"),
		Options: options,
	})
	require.Equal(t, fromSaved, fromEntity)
}

func TestEncodeSavedEntityShowReferences(t *testing.T) {
	c, f := appliedChain(t)

	options := &corepb.EncodeOptions{ShowReferences: true}

	fromSaved := c.encodeSavedEntity(f.handle, &corepb.EncodeSavedEntityRequest{
		Entity:  savedRecord(t, f.stateDir, "resource.container.second"),
		Options: options,
	})

	expected := `resource "container" "second" {
  default = "hello world"

  network {
    name = resource.network.first.meta.name
  }
}
`
	require.Equal(t, expected, fromSaved)

	fromEntity := c.encodeEntity(f.handle, &corepb.EncodeEntityRequest{
		Entity:  c.find(f.handle, "resource.container.second"),
		Options: options,
	})
	require.Equal(t, fromSaved, fromEntity)
}

func TestEncodeSavedEntityWithoutOptionsUnchanged(t *testing.T) {
	c, handle, store := appliedHostApp(t)

	record := recordFor(t, store.saved, "app.web")

	withoutOptions := c.encodeSavedEntity(handle, &corepb.EncodeSavedEntityRequest{Entity: record})
	withEmptyOptions := c.encodeSavedEntity(handle, &corepb.EncodeSavedEntityRequest{Entity: record, Options: &corepb.EncodeOptions{}})

	// the saved record holds the real password, which the default output
	// still shows as the marker
	expected := `app "web" {
  image    = "nginx"
  password = "(sensitive)"
}
`
	require.Equal(t, expected, withoutOptions)
	require.Equal(t, withEmptyOptions, withoutOptions)
}

func TestEncodeSavedEntityReturnsTokensJoiningToText(t *testing.T) {
	c, handle := appliedApp(t)

	resp := &corepb.EncodeSavedEntityResponse{}
	c.ok(handle, "EncodeSavedEntity", &corepb.EncodeSavedEntityRequest{Entity: c.appRecord(handle), Tokens: true}, resp)

	require.NotEmpty(t, resp.GetTokens())
	require.Equal(t, string(resp.GetText()), joinTokens(resp.GetTokens()))
	require.True(t, hasToken(resp.GetTokens(), corepb.TokenKind_TOKEN_KIND_BLOCK_TYPE, "app"), "app is labelled a block type")
}

func TestEncodeEntityReturnsTokensForLookedUpRecord(t *testing.T) {
	c, handle := appliedApp(t)

	resp := &corepb.EncodeEntityResponse{}
	c.ok(handle, "EncodeEntity", &corepb.EncodeEntityRequest{Entity: c.find(handle, "app.web"), Tokens: true}, resp)

	require.NotEmpty(t, resp.GetTokens())
	require.Equal(t, string(resp.GetText()), joinTokens(resp.GetTokens()))
	require.True(t, hasToken(resp.GetTokens(), corepb.TokenKind_TOKEN_KIND_BLOCK_TYPE, "app"), "app is labelled a block type")
}

func TestEncodeEntityReturnsTokensForEditedRecord(t *testing.T) {
	c, handle := appliedApp(t)

	record := map[string]any{}
	require.NoError(t, json.Unmarshal(c.find(handle, "app.web"), &record))
	record["image"] = "caddy"

	edited, err := json.Marshal(record)
	require.NoError(t, err)

	resp := &corepb.EncodeEntityResponse{}
	c.ok(handle, "EncodeEntity", &corepb.EncodeEntityRequest{Entity: edited, Tokens: true}, resp)

	require.Contains(t, string(resp.GetText()), `"caddy"`)
	require.NotEmpty(t, resp.GetTokens())
	require.Equal(t, string(resp.GetText()), joinTokens(resp.GetTokens()))
	require.True(t, hasToken(resp.GetTokens(), corepb.TokenKind_TOKEN_KIND_STRING, `"caddy"`), "the edited value is labelled a string")
}

func TestEncodeSavedEntityWithoutTokensReturnsNone(t *testing.T) {
	c, handle := appliedApp(t)

	record := c.appRecord(handle)

	plain := &corepb.EncodeSavedEntityResponse{}
	c.ok(handle, "EncodeSavedEntity", &corepb.EncodeSavedEntityRequest{Entity: record}, plain)

	labelled := &corepb.EncodeSavedEntityResponse{}
	c.ok(handle, "EncodeSavedEntity", &corepb.EncodeSavedEntityRequest{Entity: record, Tokens: true}, labelled)

	require.Empty(t, plain.GetTokens())
	require.Equal(t, string(labelled.GetText()), string(plain.GetText()))
}

func TestEncodeEntityWithoutTokensReturnsNone(t *testing.T) {
	c, handle := appliedApp(t)

	record := c.find(handle, "app.web")

	plain := &corepb.EncodeEntityResponse{}
	c.ok(handle, "EncodeEntity", &corepb.EncodeEntityRequest{Entity: record}, plain)

	labelled := &corepb.EncodeEntityResponse{}
	c.ok(handle, "EncodeEntity", &corepb.EncodeEntityRequest{Entity: record, Tokens: true}, labelled)

	require.Empty(t, plain.GetTokens())
	require.Equal(t, string(labelled.GetText()), string(plain.GetText()))
}

// hasToken reports whether a token has kind and text
func hasToken(tokens []*corepb.Token, kind corepb.TokenKind, text string) bool {
	for _, token := range tokens {
		if token.GetKind() == kind && token.GetText() == text {
			return true
		}
	}

	return false
}

// withOldColourField is a marshalled request with field 2, the retired
// highlight, set to true, as a caller built against the previous protocol
// sends it to ask for terminal colour
func withOldColourField(t *testing.T, req proto.Message) []byte {
	t.Helper()

	body, err := proto.Marshal(req)
	require.NoError(t, err)

	body = protowire.AppendTag(body, 2, protowire.VarintType)
	body = protowire.AppendVarint(body, 1)

	return body
}

func TestEncodeSavedEntityOldColourRequestReturnsPlainText(t *testing.T) {
	c, handle := appliedApp(t)

	record := c.appRecord(handle)
	plain := c.encodeSavedEntity(handle, &corepb.EncodeSavedEntityRequest{Entity: record})

	body, status := c.s.Call(handle, "EncodeSavedEntity", withOldColourField(t, &corepb.EncodeSavedEntityRequest{Entity: record}))
	require.Equal(t, StatusOK, status, errorMessage(body))

	resp := &corepb.EncodeSavedEntityResponse{}
	require.NoError(t, proto.Unmarshal(body, resp))

	require.Equal(t, plain, string(resp.GetText()))
	require.NotContains(t, string(resp.GetText()), "\x1b[")
	require.Empty(t, resp.GetTokens())
}

func TestEncodeEntityOldColourRequestReturnsPlainText(t *testing.T) {
	c, handle := appliedApp(t)

	record := c.find(handle, "app.web")
	plain := c.encodeEntity(handle, &corepb.EncodeEntityRequest{Entity: record})

	body, status := c.s.Call(handle, "EncodeEntity", withOldColourField(t, &corepb.EncodeEntityRequest{Entity: record}))
	require.Equal(t, StatusOK, status, errorMessage(body))

	resp := &corepb.EncodeEntityResponse{}
	require.NoError(t, proto.Unmarshal(body, resp))

	require.Equal(t, plain, string(resp.GetText()))
	require.NotContains(t, string(resp.GetText()), "\x1b[")
	require.Empty(t, resp.GetTokens())
}
